import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend, Gauge } from 'k6/metrics';

// Métricas personalizadas para el análisis del caché
const cacheHitRate = new Rate('cache_hit_rate');
const cacheHitCounter = new Counter('cache_hits');
const cacheMissCounter = new Counter('cache_misses');
const responseTime = new Trend('response_time');
const activeUsers = new Gauge('active_users');

// Configuración del test
export const options = {
    scenarios: {
        // Escenario 1: Carga básica constante
        constant_load: {
            executor: 'constant-vus',
            vus: 10,
            duration: '2m',
            tags: { test_type: 'constant_load' },
        },
        // Escenario 2: Picos de carga
        spike_test: {
            executor: 'ramping-vus',
            startTime: '2m',
            stages: [
                { duration: '30s', target: 50 },
                { duration: '1m', target: 100 },
                { duration: '30s', target: 10 },
            ],
            tags: { test_type: 'spike_test' },
        },
        // Escenario 3: Carga sostenida
        sustained_load: {
            executor: 'ramping-vus',
            startTime: '4m',
            stages: [
                { duration: '30s', target: 20 },
                { duration: '2m', target: 50 },
                { duration: '30s', target: 0 },
            ],
            tags: { test_type: 'sustained_load' },
        },
    },
    thresholds: {
        http_req_duration: ['p(95)<200', 'p(99)<500'],
        http_req_failed: ['rate<0.01'],
        cache_hit_rate: ['rate>0.80'], // Al menos 80% cache hit rate
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Pares de trading para las pruebas
const TRADING_PAIRS = [
    'BTC/USD',
    'ETH/USD', 
    'LTC/USD',
    'XRP/USD',
    'BTC/EUR',
    'ETH/EUR'
];

// Función principal de test
export default function () {
    activeUsers.add(1);
    
    const testScenarios = [
        // Test 1: Obtener todos los precios (carga caché)
        () => getAllPrices(),
        
        // Test 2: Obtener precios específicos (aprovecha caché)
        () => getSpecificPairs(),
        
        // Test 3: Refrescar caché
        () => refreshCache(),
        
        // Test 4: Obtener precios cached directamente
        () => getCachedPrices(),
        
        // Test 5: Health check
        () => healthCheck(),
    ];
    
    // Ejecutar escenario aleatorio
    const scenario = testScenarios[Math.floor(Math.random() * testScenarios.length)];
    scenario();
    
    // Pausa variable para simular comportamiento real
    sleep(Math.random() * 2 + 0.5);
    
    activeUsers.add(-1);
}

// Función para obtener todos los precios
function getAllPrices() {
    const response = http.get(`${BASE_URL}/api/v1/ltp`, {
        headers: {
            'Accept': 'application/json',
            'User-Agent': 'k6-load-test'
        },
        tags: { endpoint: 'ltp_all' },
    });
    
    const success = check(response, {
        'status is 200': (r) => r.status === 200,
        'response time < 200ms': (r) => r.timings.duration < 200,
        'has valid JSON': (r) => {
            try {
                const json = JSON.parse(r.body);
                return json.ltp && Array.isArray(json.ltp);
            } catch (e) {
                return false;
            }
        },
    });
    
    responseTime.add(response.timings.duration);
    
    // Detectar cache hit basado en response time
    const isCacheHit = response.timings.duration < 50; // < 50ms = cache hit
    
    if (isCacheHit) {
        cacheHitCounter.add(1);
        cacheHitRate.add(true);
    } else {
        cacheMissCounter.add(1);
        cacheHitRate.add(false);
    }
}

// Función para obtener pares específicos
function getSpecificPairs() {
    const pair = TRADING_PAIRS[Math.floor(Math.random() * TRADING_PAIRS.length)];
    const response = http.get(`${BASE_URL}/api/v1/ltp?pair=${pair}`, {
        headers: {
            'Accept': 'application/json',
            'User-Agent': 'k6-load-test'
        },
        tags: { endpoint: 'ltp_specific', pair: pair },
    });
    
    check(response, {
        'status is 200': (r) => r.status === 200,
        'response time < 100ms': (r) => r.timings.duration < 100,
        'has correct pair': (r) => {
            try {
                const json = JSON.parse(r.body);
                return json.ltp && json.ltp.some(p => p.pair === pair);
            } catch (e) {
                return false;
            }
        },
    });
    
    responseTime.add(response.timings.duration);
    
    const isCacheHit = response.timings.duration < 30;
    if (isCacheHit) {
        cacheHitCounter.add(1);
        cacheHitRate.add(true);
    } else {
        cacheMissCounter.add(1);
        cacheHitRate.add(false);
    }
}

// Función para refrescar el caché
function refreshCache() {
    const pairs = TRADING_PAIRS.slice(0, 3).join(',');
    const response = http.post(`${BASE_URL}/api/v1/ltp/refresh`, JSON.stringify({
        pairs: pairs.split(',')
    }), {
        headers: {
            'Content-Type': 'application/json',
            'Accept': 'application/json',
            'User-Agent': 'k6-load-test'
        },
        tags: { endpoint: 'ltp_refresh' },
    });
    
    check(response, {
        'refresh status is 200': (r) => r.status === 200,
        'refresh time reasonable': (r) => r.timings.duration < 2000,
    });
    
    responseTime.add(response.timings.duration);
    cacheMissCounter.add(1); // Refresh siempre es cache miss
    cacheHitRate.add(false);
}

// Función para obtener precios cached
function getCachedPrices() {
    const response = http.get(`${BASE_URL}/api/v1/ltp/cached`, {
        headers: {
            'Accept': 'application/json',
            'User-Agent': 'k6-load-test'
        },
        tags: { endpoint: 'ltp_cached' },
    });
    
    check(response, {
        'cached status is 200 or 206': (r) => r.status === 200 || r.status === 206,
        'cached response time < 50ms': (r) => r.timings.duration < 50,
    });
    
    responseTime.add(response.timings.duration);
    
    // Cached endpoint siempre debería ser cache hit
    cacheHitCounter.add(1);
    cacheHitRate.add(true);
}

// Función para health check
function healthCheck() {
    const response = http.get(`${BASE_URL}/health`, {
        headers: {
            'Accept': 'application/json',
            'User-Agent': 'k6-load-test'
        },
        tags: { endpoint: 'health' },
    });
    
    check(response, {
        'health status is 200': (r) => r.status === 200,
        'health response time < 100ms': (r) => r.timings.duration < 100,
    });
    
    responseTime.add(response.timings.duration);
}

// Setup function - se ejecuta una vez al inicio
export function setup() {
    console.log('🚀 Iniciando load test del BTC LTP Service...');
    console.log(`📊 Base URL: ${BASE_URL}`);
    console.log(`⏰ Duración total: ~7 minutos`);
    console.log(`🎯 Pares de prueba: ${TRADING_PAIRS.join(', ')}`);
    
    // Verificar que el servicio esté funcionando
    const health = http.get(`${BASE_URL}/health`);
    if (health.status !== 200) {
        console.error(`❌ Servicio no disponible: ${health.status}`);
        return null;
    }
    
    console.log('✅ Servicio verificado - iniciando pruebas...');
    return { baseUrl: BASE_URL };
}

// Teardown function - se ejecuta una vez al final
export function teardown(data) {
    console.log('📈 Load test completado!');
    console.log(`📊 Revisa los resultados en: benchmarks/results/`);
}
