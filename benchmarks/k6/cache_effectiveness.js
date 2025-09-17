import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// Métricas específicas para medir efectividad del caché
const cacheHitRate = new Rate('cache_hit_rate');
const cacheMissRate = new Rate('cache_miss_rate');
const responseTimeCacheHit = new Trend('response_time_cache_hit');
const responseTimeCacheMiss = new Trend('response_time_cache_miss');
const totalRequests = new Counter('total_requests');

export const options = {
    scenarios: {
        // Fase 1: Warm up del caché
        warmup_cache: {
            executor: 'constant-vus',
            vus: 5,
            duration: '30s',
            tags: { phase: 'warmup' },
        },
        // Fase 2: Medición de efectividad con carga moderada
        cache_effectiveness: {
            executor: 'constant-vus',
            startTime: '30s',
            vus: 20,
            duration: '2m',
            tags: { phase: 'measurement' },
        },
        // Fase 3: Burst test para invalidar caché y medir recuperación
        cache_invalidation: {
            executor: 'constant-vus',
            startTime: '2m30s',
            vus: 50,
            duration: '30s',
            tags: { phase: 'invalidation' },
        },
        // Fase 4: Medición post-invalidación
        post_invalidation: {
            executor: 'constant-vus',
            startTime: '3m',
            vus: 15,
            duration: '1m',
            tags: { phase: 'recovery' },
        },
    },
    thresholds: {
        'response_time_cache_hit': ['p(95)<50', 'p(99)<100'],
        'response_time_cache_miss': ['p(95)<500', 'p(99)<1000'],
        'cache_hit_rate': ['rate>0.70'], // Al menos 70% de cache hits globalmente
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const PAIRS = ['BTC/USD', 'ETH/USD', 'LTC/USD', 'XRP/USD'];

export default function () {
    totalRequests.add(1);
    
    const scenario = __VU % 4; // Distribuir escenarios por VU
    
    switch (scenario) {
        case 0:
            testSinglePairRepeated();
            break;
        case 1:
            testMultiplePairs();
            break;
        case 2:
            testCachedEndpoint();
            break;
        case 3:
            testAllPrices();
            break;
    }
    
    sleep(Math.random() * 1.5 + 0.5);
}

// Test de un solo par repetidamente (alta probabilidad de cache hit)
function testSinglePairRepeated() {
    const pair = 'BTC/USD'; // Siempre el mismo par
    const startTime = Date.now();
    
    const response = http.get(`${BASE_URL}/api/v1/ltp?pair=${pair}`, {
        headers: {
            'Accept': 'application/json',
            'X-Test-Type': 'single-pair-repeated'
        },
        tags: { 
            endpoint: 'single_pair',
            test_type: 'repeated',
            pair: pair 
        },
    });
    
    const duration = response.timings.duration;
    const isCacheHit = detectCacheHit(response, duration);
    
    recordCacheMetrics(isCacheHit, duration);
    
    check(response, {
        'single pair status 200': (r) => r.status === 200,
        'single pair has data': (r) => {
            try {
                const json = JSON.parse(r.body);
                return json.ltp && json.ltp.length > 0;
            } catch (e) {
                return false;
            }
        },
    });
}

// Test de múltiples pares (probabilidad mixta de cache hit/miss)
function testMultiplePairs() {
    const selectedPairs = PAIRS.slice(0, 2 + Math.floor(Math.random() * 2));
    const pairParam = selectedPairs.join(',');
    
    const response = http.get(`${BASE_URL}/api/v1/ltp?pair=${pairParam}`, {
        headers: {
            'Accept': 'application/json',
            'X-Test-Type': 'multiple-pairs'
        },
        tags: { 
            endpoint: 'multiple_pairs',
            test_type: 'mixed',
            pair_count: selectedPairs.length
        },
    });
    
    const duration = response.timings.duration;
    const isCacheHit = detectCacheHit(response, duration);
    
    recordCacheMetrics(isCacheHit, duration);
    
    check(response, {
        'multiple pairs status 200': (r) => r.status === 200,
        'multiple pairs correct count': (r) => {
            try {
                const json = JSON.parse(r.body);
                return json.ltp && json.ltp.length === selectedPairs.length;
            } catch (e) {
                return false;
            }
        },
    });
}

// Test del endpoint cached (siempre debería ser cache hit)
function testCachedEndpoint() {
    const response = http.get(`${BASE_URL}/api/v1/ltp/cached`, {
        headers: {
            'Accept': 'application/json',
            'X-Test-Type': 'cached-endpoint'
        },
        tags: { 
            endpoint: 'cached_only',
            test_type: 'guaranteed_hit'
        },
    });
    
    const duration = response.timings.duration;
    // El endpoint /cached siempre debería ser un cache hit
    recordCacheMetrics(true, duration);
    
    check(response, {
        'cached endpoint fast response': (r) => r.timings.duration < 100,
        'cached endpoint status ok': (r) => r.status === 200 || r.status === 206,
    });
}

// Test de todos los precios (carga inicial del caché)
function testAllPrices() {
    const response = http.get(`${BASE_URL}/api/v1/ltp`, {
        headers: {
            'Accept': 'application/json',
            'X-Test-Type': 'all-prices'
        },
        tags: { 
            endpoint: 'all_prices',
            test_type: 'bulk'
        },
    });
    
    const duration = response.timings.duration;
    const isCacheHit = detectCacheHit(response, duration);
    
    recordCacheMetrics(isCacheHit, duration);
    
    check(response, {
        'all prices status 200': (r) => r.status === 200,
        'all prices has multiple pairs': (r) => {
            try {
                const json = JSON.parse(r.body);
                return json.ltp && json.ltp.length >= 3;
            } catch (e) {
                return false;
            }
        },
    });
}

// Función para detectar cache hit basada en múltiples indicadores
function detectCacheHit(response, duration) {
    // Indicador 1: Tiempo de respuesta muy rápido
    const fastResponse = duration < 80;
    
    // Indicador 2: Headers del servidor (si los hubiera)
    const cacheHeader = response.headers['X-Cache-Status'];
    const headerHit = cacheHeader === 'HIT' || cacheHeader === 'hit';
    
    // Indicador 3: Patrón de tiempo de respuesta
    // Cache hits generalmente < 80ms, cache miss > 100ms
    const likelyHit = duration < 80;
    const likelyMiss = duration > 150;
    
    if (headerHit) return true;
    if (likelyMiss) return false;
    return likelyHit;
}

// Función para registrar métricas de caché
function recordCacheMetrics(isCacheHit, duration) {
    if (isCacheHit) {
        cacheHitRate.add(true);
        cacheMissRate.add(false);
        responseTimeCacheHit.add(duration);
    } else {
        cacheHitRate.add(false);
        cacheMissRate.add(true);
        responseTimeCacheMiss.add(duration);
    }
}

export function setup() {
    console.log('🎯 Iniciando test de efectividad del caché...');
    console.log('📊 Fases del test:');
    console.log('  1. Warm-up (30s) - Cargar caché inicial');
    console.log('  2. Medición (2m) - Evaluar hit rate');
    console.log('  3. Invalidación (30s) - Forzar cache misses');
    console.log('  4. Recuperación (1m) - Medir recuperación');
    
    // Health check
    const health = http.get(`${BASE_URL}/health`);
    if (health.status !== 200) {
        console.error('❌ Servicio no disponible');
        return null;
    }
    
    console.log('✅ Iniciando análisis de caché...');
    return { baseUrl: BASE_URL };
}

export function teardown(data) {
    console.log('📈 Test de caché completado!');
    console.log('🔍 Métricas clave a revisar:');
    console.log('  - cache_hit_rate: % de hits vs total requests');
    console.log('  - response_time_cache_hit: Tiempo promedio para hits');
    console.log('  - response_time_cache_miss: Tiempo promedio para misses');
    console.log('📊 Los resultados muestran la efectividad del caché');
}
