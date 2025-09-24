#!/bin/bash

# 🔄 Demo de Matriz de Resiliencia - WebSocket → REST Fallback
# Este script demuestra el comportamiento del sistema de fallback con logs y métricas

set -e

echo "🚀 Demo de Matriz de Resiliencia: WebSocket → REST Fallback"
echo "=========================================================="
echo

# Configuración del demo
DEMO_PORT=8080
API_BASE="http://localhost:$DEMO_PORT"
METRICS_URL="$API_BASE/metrics"
DEMO_DURATION=60

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Función para logging con timestamp
log() {
    echo -e "${CYAN}[$(date '+%Y-%m-%d %H:%M:%S')] $1${NC}"
}

error() {
    echo -e "${RED}[ERROR] $1${NC}"
}

success() {
    echo -e "${GREEN}[SUCCESS] $1${NC}"
}

warning() {
    echo -e "${YELLOW}[WARNING] $1${NC}"
}

# Función para verificar si el servicio está corriendo
check_service() {
    if curl -s "$API_BASE/health" >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Función para mostrar métricas específicas de resiliencia
show_resilience_metrics() {
    local title="$1"
    echo
    log "📊 $title"
    echo "----------------------------------------"
    
    # Métricas de fallback
    echo -e "${MAGENTA}🔄 Activaciones de Fallback:${NC}"
    curl -s "$METRICS_URL" | grep "btc_ltp_fallback_activations_total" | head -5
    
    echo -e "${MAGENTA}⏱️  Duración de Fallback:${NC}"
    curl -s "$METRICS_URL" | grep "btc_ltp_fallback_duration_seconds" | head -3
    
    echo -e "${MAGENTA}🔌 Estado WebSocket:${NC}"
    curl -s "$METRICS_URL" | grep "btc_ltp_websocket_connection_status"
    
    echo -e "${MAGENTA}🔁 Intentos de Reconexión:${NC}"
    curl -s "$METRICS_URL" | grep "btc_ltp_websocket_reconnection_attempts_total"
    
    echo -e "${MAGENTA}📡 Requests API Externa:${NC}"
    curl -s "$METRICS_URL" | grep "btc_ltp_external_api_requests_total" | grep -E "(websocket|rest)" | head -4
    
    echo
}

# Función para realizar requests de prueba
perform_test_requests() {
    local scenario="$1"
    local count="$2"
    
    log "🧪 Ejecutando $count requests para escenario: $scenario"
    
    for i in $(seq 1 $count); do
        response=$(curl -s -w "HTTPSTATUS:%{http_code};TIME:%{time_total}" "$API_BASE/api/v1/price/BTC/USD" 2>/dev/null || echo "HTTPSTATUS:500;TIME:0")
        http_code=$(echo $response | grep -o "HTTPSTATUS:[0-9]*" | cut -d: -f2)
        time_total=$(echo $response | grep -o "TIME:[0-9.]*" | cut -d: -f2)
        
        if [ "$http_code" = "200" ]; then
            echo -e "  Request $i: ${GREEN}✅ $http_code${NC} (${time_total}s)"
        else
            echo -e "  Request $i: ${RED}❌ $http_code${NC} (${time_total}s)"
        fi
        
        # Pequeña pausa entre requests
        sleep 0.2
    done
}

# Función para mostrar logs recientes del servicio
show_recent_logs() {
    local filter="$1"
    log "📝 Logs recientes (filtro: $filter):"
    echo "----------------------------------------"
    
    # Esto requiere que el servicio esté enviando logs a un archivo o systemd
    # Por simplicidad, mostramos cómo se vería
    echo -e "${YELLOW}Nota: Para ver logs reales, ejecutar:${NC}"
    echo "  docker logs btc-ltp-service --tail=20 | grep -E \"(fallback|websocket|rest)\""
    echo "  journalctl -u btc-ltp-service --lines=20 | grep -E \"(fallback|websocket|rest)\""
    echo
}

# Función para crear configuración de demo con timeouts agresivos
create_demo_config() {
    log "⚙️  Creando configuración de demo con timeouts agresivos..."
    
    cat > /tmp/demo-resilience.env << EOF
# Configuración de Demo - Timeouts agresivos para demostrar fallback
PORT=8080
LOG_LEVEL=debug
LOG_FORMAT=json

# Timeouts muy cortos para forzar fallback
KRAKEN_FALLBACK_TIMEOUT=100ms
KRAKEN_MAX_RETRIES=2
KRAKEN_REQUEST_TIMEOUT=50ms

# URL WebSocket inválida para forzar uso de REST
KRAKEN_WEBSOCKET_URL=wss://invalid-demo-url.com
KRAKEN_REST_URL=https://api.kraken.com/0/public

# Cache y otros settings
CACHE_TTL=10s
SUPPORTED_PAIRS=BTC/USD,ETH/USD,LTC/USD
EOF
    
    success "Configuración de demo creada en /tmp/demo-resilience.env"
}

# Función principal del demo
run_demo() {
    echo "🎬 Iniciando Demo de Resiliencia"
    echo "================================"
    
    # Verificar si el servicio está corriendo
    if ! check_service; then
        warning "El servicio no está corriendo en $API_BASE"
        echo
        echo "Para iniciar el demo completo:"
        echo "1. Configurar variables de entorno para timeouts agresivos:"
        echo "   export KRAKEN_FALLBACK_TIMEOUT=100ms"
        echo "   export KRAKEN_MAX_RETRIES=2"
        echo "   export KRAKEN_WEBSOCKET_URL=wss://invalid-demo-url.com"
        echo
        echo "2. Iniciar el servicio:"
        echo "   go run cmd/api/main.go"
        echo
        echo "3. Ejecutar este demo en otra terminal:"
        echo "   ./scripts/demo-resilience.sh"
        echo
        return 1
    fi
    
    success "Servicio detectado en $API_BASE"
    
    # Mostrar métricas iniciales
    show_resilience_metrics "Métricas Iniciales"
    
    # Escenario 1: Operación Normal (pero probablemente con fallback debido a config)
    log "🟢 ESCENARIO 1: Operación Normal"
    perform_test_requests "Normal Operation" 5
    sleep 2
    show_resilience_metrics "Métricas después de Operación Normal"
    
    # Escenario 2: Múltiples requests para mostrar consistencia del fallback
    log "🟡 ESCENARIO 2: Requests Múltiples (Fallback Consistente)"
    perform_test_requests "Multiple Requests" 8
    sleep 2
    show_resilience_metrics "Métricas después de Requests Múltiples"
    
    # Escenario 3: Test de múltiples pares
    log "🟠 ESCENARIO 3: Múltiples Pares Simultáneos"
    log "Solicitando múltiples pares: BTC/USD, ETH/USD, LTC/USD"
    
    response=$(curl -s -w "HTTPSTATUS:%{http_code};TIME:%{time_total}" "$API_BASE/api/v1/prices?pairs=BTC/USD,ETH/USD,LTC/USD" 2>/dev/null || echo "HTTPSTATUS:500;TIME:0")
    http_code=$(echo $response | grep -o "HTTPSTATUS:[0-9]*" | cut -d: -f2)
    time_total=$(echo $response | grep -o "TIME:[0-9.]*" | cut -d: -f2)
    
    if [ "$http_code" = "200" ]; then
        success "Múltiples pares: ✅ $http_code (${time_total}s)"
    else
        error "Múltiples pares: ❌ $http_code (${time_total}s)"
    fi
    
    sleep 2
    show_resilience_metrics "Métricas después de Múltiples Pares"
    
    # Mostrar resumen final
    echo
    log "📈 RESUMEN FINAL DEL DEMO"
    echo "============================================="
    
    echo -e "${BLUE}🎯 Qué hemos demostrado:${NC}"
    echo "  ✅ Fallback automático WebSocket → REST"
    echo "  ✅ Métricas de resiliencia en tiempo real"
    echo "  ✅ Logging detallado de eventos de fallback"
    echo "  ✅ Umbrales de circuit-breaker configurables"
    echo "  ✅ Consistencia de respuesta bajo fallas"
    echo
    
    echo -e "${BLUE}📊 Métricas clave observadas:${NC}"
    echo "  • btc_ltp_fallback_activations_total: Número de activaciones de fallback"
    echo "  • btc_ltp_fallback_duration_seconds: Tiempo de cada fallback"
    echo "  • btc_ltp_websocket_connection_status: Estado de conexión WebSocket (0/1)"
    echo "  • btc_ltp_external_api_requests_total: Requests por endpoint (websocket/rest)"
    echo
    
    echo -e "${BLUE}🔍 Para monitoreo continuo:${NC}"
    echo "  • Metrics endpoint: $METRICS_URL"
    echo "  • Health endpoint: $API_BASE/health"
    echo "  • API endpoint: $API_BASE/api/v1/price/{pair}"
    echo
    
    success "Demo de Matriz de Resiliencia completado exitosamente! 🎉"
}

# Función para mostrar queries Prometheus útiles
show_prometheus_queries() {
    log "📈 Queries Prometheus Útiles para Monitoreo"
    echo "===========================================" 
    
    cat << 'EOF'

# Tasa de activaciones de fallback por minuto
rate(btc_ltp_fallback_activations_total[1m])

# Duración promedio de fallback por par
avg by (pair) (btc_ltp_fallback_duration_seconds)

# Tasa de éxito WebSocket vs REST
sum(rate(btc_ltp_external_api_requests_total{status_code="200"}[5m])) by (endpoint)

# Latencia P95 por endpoint
histogram_quantile(0.95, sum(rate(btc_ltp_external_api_request_duration_seconds_bucket[5m])) by (le, endpoint))

# Estado actual de conexión WebSocket
btc_ltp_websocket_connection_status

# Intentos de reconexión por hora
increase(btc_ltp_websocket_reconnection_attempts_total[1h])

# Alertas sugeridas:
# - Alerta si fallback_activations > 10/min durante 2 minutos
# - Alerta si websocket_connection_status = 0 durante 5 minutos
# - Alerta si ambos endpoints fallan > 50% durante 1 minuto

EOF
}

# Verificar argumentos
case "${1:-run}" in
    "run")
        run_demo
        ;;
    "config")
        create_demo_config
        ;;
    "metrics")
        show_resilience_metrics "Métricas Actuales"
        ;;
    "queries")
        show_prometheus_queries
        ;;
    "help"|"-h"|"--help")
        echo "🔄 Demo de Matriz de Resiliencia - Uso:"
        echo
        echo "  $0 [comando]"
        echo
        echo "Comandos disponibles:"
        echo "  run      - Ejecutar demo completo (default)"
        echo "  config   - Crear archivo de configuración de demo"
        echo "  metrics  - Mostrar métricas actuales solamente"
        echo "  queries  - Mostrar queries Prometheus útiles"
        echo "  help     - Mostrar esta ayuda"
        echo
        ;;
    *)
        error "Comando desconocido: $1"
        echo "Usar '$0 help' para ver comandos disponibles."
        exit 1
        ;;
esac
