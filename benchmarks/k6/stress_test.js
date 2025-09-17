import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// Métricas para stress test
const errorRate = new Rate('error_rate');
const requestRate = new Counter('request_rate');
const responseTime = new Trend('response_time');

export const options = {
    stages: [
        // Ramp up gradually
        { duration: '1m', target: 20 },
        { duration: '2m', target: 50 },
        { duration: '2m', target: 100 },
        
        // Stress phase - push limits
        { duration: '3m', target: 200 },
        { duration: '2m', target: 300 },
        { duration: '1m', target: 400 }, // Peak stress
        
        // Recovery phase
        { duration: '2m', target: 100 },
        { duration: '1m', target: 50 },
        { duration: '1m', target: 0 },
    ],
    thresholds: {
        http_req_duration: ['p(95)<1000', 'p(99)<2000'], // More lenient for stress test
        http_req_failed: ['rate<0.05'], // Allow up to 5% errors under stress
        error_rate: ['rate<0.05'],
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const PAIRS = ['BTC/USD', 'ETH/USD', 'LTC/USD', 'XRP/USD', 'BTC/EUR', 'ETH/EUR'];

export default function () {
    requestRate.add(1);
    
    // Más agresivo en el stress test - menos tiempo entre requests
    const scenarios = [
        () => rapidFireRequests(),
        () => bulkPairRequests(),
        () => concurrentEndpoints(),
    ];
    
    const scenario = scenarios[Math.floor(Math.random() * scenarios.length)];
    scenario();
    
    // Shorter sleep for higher load
    sleep(Math.random() * 0.5 + 0.1);
}

function rapidFireRequests() {
    // Hacer múltiples requests rápidos para el mismo par
    const pair = PAIRS[Math.floor(Math.random() * PAIRS.length)];
    
    for (let i = 0; i < 3; i++) {
        const response = http.get(`${BASE_URL}/api/v1/ltp?pair=${pair}`, {
            headers: { 'Accept': 'application/json' },
            tags: { test_type: 'rapid_fire', iteration: i },
        });
        
        const success = check(response, {
            'rapid fire status ok': (r) => r.status === 200,
            'rapid fire response time ok': (r) => r.timings.duration < 2000,
        });
        
        responseTime.add(response.timings.duration);
        errorRate.add(!success);
        
        if (i < 2) sleep(0.1); // Very short pause between rapid requests
    }
}

function bulkPairRequests() {
    // Request todos los pares de una vez
    const allPairs = PAIRS.join(',');
    const response = http.get(`${BASE_URL}/api/v1/ltp?pair=${allPairs}`, {
        headers: { 'Accept': 'application/json' },
        tags: { test_type: 'bulk_pairs' },
    });
    
    const success = check(response, {
        'bulk pairs status ok': (r) => r.status === 200,
        'bulk pairs has data': (r) => {
            try {
                const json = JSON.parse(r.body);
                return json.ltp && json.ltp.length >= PAIRS.length;
            } catch (e) {
                return false;
            }
        },
    });
    
    responseTime.add(response.timings.duration);
    errorRate.add(!success);
}

function concurrentEndpoints() {
    // Atacar diferentes endpoints concurrentemente
    const requests = [
        ['GET', `${BASE_URL}/api/v1/ltp`, null],
        ['GET', `${BASE_URL}/api/v1/ltp/cached`, null],
        ['GET', `${BASE_URL}/health`, null],
        ['POST', `${BASE_URL}/api/v1/ltp/refresh`, JSON.stringify({ pairs: ['BTC/USD'] })],
    ];
    
    const responses = http.batch(requests.map(([method, url, body]) => ({
        method,
        url,
        body,
        params: {
            headers: {
                'Content-Type': 'application/json',
                'Accept': 'application/json',
            },
            tags: { test_type: 'concurrent_endpoints' },
        },
    })));
    
    responses.forEach((response, index) => {
        const success = check(response, {
            'concurrent status ok': (r) => r.status >= 200 && r.status < 400,
        });
        
        responseTime.add(response.timings.duration);
        errorRate.add(!success);
    });
}

export function setup() {
    console.log('🔥 Iniciando STRESS TEST del BTC LTP Service...');
    console.log('⚠️  ADVERTENCIA: Este test llevará el servicio al límite');
    console.log('📊 Perfil de carga:');
    console.log('  - Escalada gradual hasta 400 usuarios concurrentes');
    console.log('  - Requests rápidos con mínimas pausas');
    console.log('  - Múltiples endpoints atacados concurrentemente');
    console.log('  - Duración total: ~15 minutos');
    
    const health = http.get(`${BASE_URL}/health`);
    if (health.status !== 200) {
        console.error('❌ Servicio no disponible para stress test');
        return null;
    }
    
    console.log('🚀 Iniciando stress test - monitored closely...');
    return { baseUrl: BASE_URL };
}

export function teardown(data) {
    console.log('🔥 Stress test completado!');
    console.log('📊 Revisa:');
    console.log('  - Error rate durante picos de carga');
    console.log('  - Response times bajo estrés extremo'); 
    console.log('  - Capacidad máxima soportada');
    console.log('  - Comportamiento de recuperación');
}
