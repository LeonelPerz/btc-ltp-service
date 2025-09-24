#!/bin/bash

# 🔍 Validador de Matriz de Resiliencia
# Este script valida que el sistema de fallback funcione correctamente

set -e

API_BASE="http://localhost:8080"
METRICS_URL="$API_BASE/metrics"

# Colores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() {
    echo -e "${BLUE}[$(date '+%H:%M:%S')] $1${NC}"
}

success() {
    echo -e "${GREEN}✅ $1${NC}"
}

error() {
    echo -e "${RED}❌ $1${NC}"
}

warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# Función para obtener valor de métrica
get_metric_value() {
    local metric_name="$1"
    local labels="$2"
    
    if [ -n "$labels" ]; then
        curl -s "$METRICS_URL" | grep "^$metric_name{.*$labels" | tail -1 | awk '{print $2}' | head -1
    else
        curl -s "$METRICS_URL" | grep "^$metric_name " | awk '{print $2}' | head -1
    fi
}

# Función para verificar que una métrica existe
check_metric_exists() {
    local metric_name="$1"
    local description="$2"
    
    if curl -s "$METRICS_URL" | grep -q "^$metric_name"; then
        success "$description existe"
        return 0
    else
        error "$description no encontrada"
        return 1
    fi
}

# Función para validar fallback
validate_fallback_behavior() {
    log "🔄 Validando comportamiento de fallback..."
    
    # Obtener métricas iniciales
    initial_fallbacks=$(get_metric_value "btc_ltp_fallback_activations_total" "")
    initial_rest_requests=$(get_metric_value "btc_ltp_external_api_requests_total" "endpoint=\"rest\"")
    
    log "Métricas iniciales:"
    echo "  - Fallback activations: ${initial_fallbacks:-0}"
    echo "  - REST requests: ${initial_rest_requests:-0}"
    echo
    
    # Realizar requests de prueba
    log "Ejecutando 5 requests de prueba..."
    for i in {1..5}; do
        response=$(curl -s -w "HTTPSTATUS:%{http_code}" "$API_BASE/api/v1/price/BTC/USD" 2>/dev/null || echo "HTTPSTATUS:500")
        http_code=$(echo $response | grep -o "HTTPSTATUS:[0-9]*" | cut -d: -f2)
        
        if [ "$http_code" = "200" ]; then
            echo "  Request $i: ✅ $http_code"
        else
            echo "  Request $i: ❌ $http_code"
        fi
        sleep 0.5
    done
    
    echo
    
    # Verificar métricas finales
    sleep 2  # Dar tiempo para que se actualicen las métricas
    
    final_fallbacks=$(get_metric_value "btc_ltp_fallback_activations_total" "")
    final_rest_requests=$(get_metric_value "btc_ltp_external_api_requests_total" "endpoint=\"rest\"")
    
    log "Métricas finales:"
    echo "  - Fallback activations: ${final_fallbacks:-0}"
    echo "  - REST requests: ${final_rest_requests:-0}"
    echo
    
    # Validar incremento
    if [ -n "$final_fallbacks" ] && [ -n "$initial_fallbacks" ]; then
        fallback_increase=$((final_fallbacks - initial_fallbacks))
        if [ $fallback_increase -gt 0 ]; then
            success "Fallbacks incrementaron en $fallback_increase (como se esperaba)"
        else
            warning "No se detectó incremento en fallbacks (puede ser normal si WebSocket funciona)"
        fi
    fi
    
    if [ -n "$final_rest_requests" ] && [ -n "$initial_rest_requests" ]; then
        rest_increase=$((final_rest_requests - initial_rest_requests))
        if [ $rest_increase -gt 0 ]; then
            success "Requests REST incrementaron en $rest_increase"
        else
            error "No se detectó incremento en requests REST"
        fi
    fi
}

# Función principal de validación
validate_resilience_matrix() {
    echo "🛡️  Validación de Matriz de Resiliencia"
    echo "======================================"
    echo
    
    # 1. Verificar que el servicio esté activo
    log "1. Verificando servicio..."
    if curl -s "$API_BASE/health" >/dev/null 2>&1; then
        success "Servicio activo en $API_BASE"
    else
        error "Servicio no disponible en $API_BASE"
        return 1
    fi
    echo
    
    # 2. Verificar endpoint de métricas
    log "2. Verificando endpoint de métricas..."
    if curl -s "$METRICS_URL" >/dev/null 2>&1; then
        success "Endpoint de métricas disponible"
    else
        error "Endpoint de métricas no disponible"
        return 1
    fi
    echo
    
    # 3. Verificar métricas de resiliencia existen
    log "3. Verificando existencia de métricas de resiliencia..."
    
    check_metric_exists "btc_ltp_fallback_activations_total" "Métrica de activaciones de fallback"
    check_metric_exists "btc_ltp_fallback_duration_seconds" "Métrica de duración de fallback" 
    check_metric_exists "btc_ltp_websocket_connection_status" "Métrica de estado WebSocket"
    check_metric_exists "btc_ltp_external_api_requests_total" "Métrica de requests API externa"
    check_metric_exists "btc_ltp_websocket_reconnection_attempts_total" "Métrica de intentos de reconexión"
    echo
    
    # 4. Verificar estado actual del sistema
    log "4. Verificando estado actual del sistema..."
    
    ws_status=$(get_metric_value "btc_ltp_websocket_connection_status" "")
    if [ "$ws_status" = "1" ]; then
        success "WebSocket conectado (status: $ws_status)"
    elif [ "$ws_status" = "0" ]; then
        warning "WebSocket desconectado - fallback activo (status: $ws_status)"
    else
        error "Estado WebSocket desconocido: $ws_status"
    fi
    echo
    
    # 5. Validar comportamiento de fallback
    validate_fallback_behavior
    
    # 6. Resumen de métricas actuales
    log "6. Resumen de métricas actuales..."
    echo
    echo "📊 RESUMEN DE MÉTRICAS:"
    echo "======================"
    
    echo -e "${BLUE}Fallback Activations por razón:${NC}"
    curl -s "$METRICS_URL" | grep "btc_ltp_fallback_activations_total{" | head -5
    
    echo -e "${BLUE}Duración de Fallback (últimas mediciones):${NC}"
    curl -s "$METRICS_URL" | grep "btc_ltp_fallback_duration_seconds_bucket" | tail -3
    
    echo -e "${BLUE}Requests por Endpoint:${NC}"
    curl -s "$METRICS_URL" | grep "btc_ltp_external_api_requests_total" | grep -E "(websocket|rest)" | head -4
    
    echo -e "${BLUE}Intentos de Reconexión:${NC}"
    curl -s "$METRICS_URL" | grep "btc_ltp_websocket_reconnection_attempts_total" | head -3
    
    echo
    success "Validación de matriz de resiliencia completada! 🎉"
}

# Función para tests de carga
load_test() {
    local requests=${1:-20}
    local concurrency=${2:-5}
    
    log "🚀 Ejecutando test de carga: $requests requests con concurrencia $concurrency"
    
    if ! command -v ab >/dev/null 2>&1; then
        warning "Apache Bench (ab) no disponible. Usando curl secuencial..."
        
        start_time=$(date +%s)
        successful=0
        failed=0
        
        for i in $(seq 1 $requests); do
            response=$(curl -s -w "HTTPSTATUS:%{http_code}" "$API_BASE/api/v1/price/BTC/USD" 2>/dev/null || echo "HTTPSTATUS:500")
            http_code=$(echo $response | grep -o "HTTPSTATUS:[0-9]*" | cut -d: -f2)
            
            if [ "$http_code" = "200" ]; then
                ((successful++))
            else
                ((failed++))
            fi
            
            if [ $((i % 5)) -eq 0 ]; then
                echo "  Progreso: $i/$requests requests"
            fi
        done
        
        end_time=$(date +%s)
        duration=$((end_time - start_time))
        
        echo
        log "Resultados del test de carga:"
        echo "  - Requests exitosos: $successful"
        echo "  - Requests fallidos: $failed"
        echo "  - Duración total: ${duration}s"
        echo "  - Requests por segundo: $(echo "scale=2; $requests / $duration" | bc 2>/dev/null || echo "N/A")"
        
    else
        log "Usando Apache Bench para test de carga..."
        ab -n $requests -c $concurrency "$API_BASE/api/v1/price/BTC/USD" | tail -20
    fi
}

# Función para mostrar ayuda
show_help() {
    echo "🔍 Validador de Matriz de Resiliencia - Uso:"
    echo
    echo "  $0 [comando] [argumentos]"
    echo
    echo "Comandos disponibles:"
    echo "  validate    - Ejecutar validación completa (default)"  
    echo "  metrics     - Mostrar solo métricas actuales"
    echo "  load [n] [c] - Test de carga (n requests, c concurrencia)"
    echo "  help        - Mostrar esta ayuda"
    echo
    echo "Ejemplos:"
    echo "  $0 validate"
    echo "  $0 load 50 10"
    echo "  $0 metrics"
    echo
}

# Main
case "${1:-validate}" in
    "validate")
        validate_resilience_matrix
        ;;
    "metrics")
        log "📊 Métricas actuales de resiliencia:"
        echo
        curl -s "$METRICS_URL" | grep -E "(btc_ltp_fallback|btc_ltp_websocket)" | head -20
        ;;
    "load")
        load_test "${2:-20}" "${3:-5}"
        ;;
    "help"|"-h"|"--help")
        show_help
        ;;
    *)
        error "Comando desconocido: $1"
        show_help
        exit 1
        ;;
esac
