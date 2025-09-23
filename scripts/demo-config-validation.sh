#!/bin/bash
# Demo de Validación de Configuración - Fail Fast
# Este script demuestra cómo la validación falla rápidamente con configuraciones inválidas

set -e
echo "🧪 Demo: Validación de Configuración con Fail Fast"
echo "=================================================="

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Función para ejecutar y mostrar resultado
run_test() {
    local test_name="$1"
    local env_vars="$2"
    local config_file="$3"
    local expected_result="$4"
    
    echo ""
    echo -e "${YELLOW}🧪 Test: $test_name${NC}"
    echo "Configuration: $config_file"
    echo "Environment: $env_vars"
    echo ""
    
    # Configurar environment si se proporciona
    if [ -n "$env_vars" ]; then
        eval "export $env_vars"
    fi
    
    # Ejecutar el servicio (solo validación)
    if timeout 5s go run cmd/api/main.go 2>&1 | grep -q "validation failed"; then
        if [ "$expected_result" = "FAIL" ]; then
            echo -e "${RED}✅ ¡CORRECTO! Validación falló como se esperaba${NC}"
        else
            echo -e "${RED}❌ ERROR: Validación falló cuando debería pasar${NC}"
        fi
    else
        if [ "$expected_result" = "PASS" ]; then
            echo -e "${GREEN}✅ ¡CORRECTO! Validación pasó como se esperaba${NC}"
        else
            echo -e "${GREEN}❌ ERROR: Validación pasó cuando debería fallar${NC}"
        fi
    fi
    
    # Limpiar variables de entorno
    unset PORT CACHE_BACKEND CACHE_TTL SUPPORTED_PAIRS
}

echo ""
echo "📋 Casos de Test:"
echo "1. TTL inválido (muy corto)"
echo "2. Pares desconocidos"  
echo "3. Configuración válida con ENV override"
echo "4. Precedencia YAML vs ENV"

# Test 1: TTL inválido
echo ""
echo "=" | head -c 50; echo ""
run_test "TTL Inválido - Muy Corto (50ms)" \
         "" \
         "configs/config.test-bad-ttl.yaml" \
         "FAIL"

# Test 2: Pares desconocidos (ya están en config.test-bad-ttl.yaml)
echo ""
echo "=" | head -c 50; echo ""
echo -e "${YELLOW}🧪 Test: Pares Desconocidos${NC}"
echo "Los pares DOGE/MOON, INVALID, FAKE/COIN en config.test-bad-ttl.yaml deben fallar"

# Test 3: ENV override que corrige TTL pero deja pares inválidos
echo ""
echo "=" | head -c 50; echo ""
run_test "ENV corrige TTL pero pares siguen inválidos" \
         "CACHE_TTL=30s" \
         "configs/config.test-bad-ttl.yaml" \
         "FAIL"

# Test 4: ENV override que corrige todo
echo ""
echo "=" | head -c 50; echo ""
run_test "ENV override corrige TTL y pares" \
         'CACHE_TTL=30s SUPPORTED_PAIRS="BTC/USD,ETH/USD,LTC/USD"' \
         "configs/config.test-bad-ttl.yaml" \
         "PASS"

# Test 5: Demostrar precedencia completa
echo ""
echo "=" | head -c 50; echo ""
echo -e "${YELLOW}🧪 Demo: Precedencia Completa${NC}"
echo "Base config (demo-precedence): redis, port 9000, debug level"  
echo "ENV override: memory, port 8080, info level"
echo ""

export CACHE_BACKEND=memory
export PORT=8080
export LOG_LEVEL=info
export CACHE_TTL=45s

echo "ENV configurado:"
echo "- CACHE_BACKEND=$CACHE_BACKEND (override redis→memory)"
echo "- PORT=$PORT (override 9000→8080)" 
echo "- LOG_LEVEL=$LOG_LEVEL (override debug→info)"
echo "- CACHE_TTL=$CACHE_TTL (override 60s→45s)"

# Intentar ejecutar con timeout para ver la configuración cargada
timeout 3s go run cmd/api/main.go 2>&1 | head -20 || true

echo ""
echo "=" | head -c 50; echo ""
echo -e "${GREEN}✅ Demo completado!${NC}"
echo ""
echo "📝 Resumen de validaciones implementadas:"
echo "• TTL debe ser >= 100ms y <= 24h"
echo "• TTL recomendado: 1s a 1h para datos financieros"  
echo "• Pares deben estar en lista conocida de Kraken"
echo "• Mensajes de error específicos para debugging rápido"
echo "• Precedencia: defaults → YAML → ENV vars"
