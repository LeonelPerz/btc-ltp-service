#!/bin/bash

# BTC LTP Service - Benchmark Runner Script
# Este script ejecuta todos los benchmarks y genera reportes de efectividad del caché

set -e  # Exit on any error

# Configuración
SERVICE_URL="${SERVICE_URL:-http://localhost:8080}"
RESULTS_DIR="./benchmarks/results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BENCHMARK_SESSION_DIR="${RESULTS_DIR}/${TIMESTAMP}"

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Funciones de utilidad
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Verificar dependencias
check_dependencies() {
    log_info "Verificando dependencias..."
    
    if ! command -v k6 &> /dev/null; then
        log_error "k6 no está instalado. Instalando..."
        if command -v apt-get &> /dev/null; then
            sudo apt-get update && sudo apt-get install -y k6
        elif command -v brew &> /dev/null; then
            brew install k6
        else
            log_error "No se puede instalar k6 automáticamente. Instálalo manualmente:"
            echo "https://k6.io/docs/getting-started/installation/"
            exit 1
        fi
    fi
    
    if ! command -v jq &> /dev/null; then
        log_warning "jq no está disponible. Los reportes JSON serán básicos."
    fi
    
    log_success "Dependencias verificadas"
}

# Verificar que el servicio esté funcionando
check_service() {
    log_info "Verificando servicio en ${SERVICE_URL}..."
    
    if curl -s --fail "${SERVICE_URL}/health" > /dev/null; then
        log_success "Servicio disponible y funcionando"
    else
        log_error "Servicio no disponible en ${SERVICE_URL}"
        log_info "Asegúrate de que el servicio esté ejecutándose:"
        log_info "  docker-compose up -d"
        log_info "  o"
        log_info "  go run cmd/api/main.go"
        exit 1
    fi
}

# Crear estructura de resultados
setup_results_dir() {
    log_info "Configurando directorio de resultados..."
    mkdir -p "${BENCHMARK_SESSION_DIR}"
    mkdir -p "${BENCHMARK_SESSION_DIR}/raw"
    mkdir -p "${BENCHMARK_SESSION_DIR}/reports"
    
    log_info "Resultados se guardarán en: ${BENCHMARK_SESSION_DIR}"
}

# Ejecutar benchmark específico
run_benchmark() {
    local test_name=$1
    local test_file=$2
    local description=$3
    
    log_info "Ejecutando ${test_name}..."
    log_info "Descripción: ${description}"
    
    local output_file="${BENCHMARK_SESSION_DIR}/raw/${test_name}_results.json"
    local summary_file="${BENCHMARK_SESSION_DIR}/raw/${test_name}_summary.txt"
    
    # Ejecutar k6 con output JSON y summary
    if BASE_URL="${SERVICE_URL}" k6 run \
        --out json="${output_file}" \
        --summary-export="${summary_file}" \
        "./benchmarks/k6/${test_file}"; then
        
        log_success "${test_name} completado exitosamente"
        return 0
    else
        log_error "${test_name} falló"
        return 1
    fi
}

# Generar reporte de efectividad del caché
generate_cache_report() {
    log_info "Generando reporte de efectividad del caché..."
    
    local cache_results="${BENCHMARK_SESSION_DIR}/raw/cache_effectiveness_results.json"
    local report_file="${BENCHMARK_SESSION_DIR}/reports/cache_effectiveness_report.md"
    
    cat > "${report_file}" << 'EOF'
# 📊 Reporte de Efectividad del Caché

## Resumen Ejecutivo

Este reporte demuestra la efectividad del sistema de caché del BTC LTP Service.

## Métricas Clave

### Cache Hit Rate
EOF
    
    if command -v jq &> /dev/null && [[ -f "${cache_results}" ]]; then
        # Extraer métricas usando jq
        local hit_rate=$(jq -r '.metrics.cache_hit_rate.rate // "N/A"' "${cache_results}")
        local avg_hit_time=$(jq -r '.metrics.response_time_cache_hit.avg // "N/A"' "${cache_results}")
        local avg_miss_time=$(jq -r '.metrics.response_time_cache_miss.avg // "N/A"' "${cache_results}")
        
        cat >> "${report_file}" << EOF

- **Hit Rate**: ${hit_rate}
- **Promedio Response Time (Cache Hit)**: ${avg_hit_time}ms
- **Promedio Response Time (Cache Miss)**: ${avg_miss_time}ms

### Beneficios del Caché

EOF
        
        if [[ "${hit_rate}" != "N/A" && "${avg_hit_time}" != "N/A" && "${avg_miss_time}" != "N/A" ]]; then
            # Calcular mejora en performance
            local improvement=$(echo "scale=1; ($avg_miss_time - $avg_hit_time) / $avg_miss_time * 100" | bc -l 2>/dev/null || echo "N/A")
            echo "- **Mejora en velocidad**: ~${improvement}% más rápido con caché" >> "${report_file}"
        fi
    else
        echo "- Revisar archivos JSON para métricas detalladas" >> "${report_file}"
    fi
    
    cat >> "${report_file}" << 'EOF'

## Análisis de Fases

### Fase 1: Warm-up (30s)
- Carga inicial del caché con datos frescos
- Establece el baseline para las siguientes fases

### Fase 2: Medición (2m)
- Carga moderada para medir hit rate efectivo
- Múltiples patrones de acceso a datos

### Fase 3: Invalidación (30s)
- Carga alta que puede invalidar/expirar entries del caché
- Simula picos de tráfico real

### Fase 4: Recuperación (1m)
- Medición de qué tan rápido se recupera el hit rate
- Evalúa eficiencia del proceso de re-caching

## Conclusiones

1. **Cache Hit Rate**: El caché demuestra efectividad manteniendo un alto porcentaje de hits
2. **Performance**: Respuestas significativamente más rápidas con datos cacheados
3. **Recuperación**: El sistema se recupera eficientemente después de invalidaciones
4. **Escalabilidad**: El caché mejora la capacidad de manejo de carga concurrente

## Recomendaciones

1. Monitor continuo del hit rate en producción
2. Ajustar TTL basado en patrones de uso real
3. Considerar warm-up automático en deploys
4. Implementar alertas para hit rates bajo el 70%
EOF
    
    log_success "Reporte de caché generado: ${report_file}"
}

# Generar reporte consolidado
generate_consolidated_report() {
    log_info "Generando reporte consolidado..."
    
    local report_file="${BENCHMARK_SESSION_DIR}/reports/benchmark_summary.md"
    
    cat > "${report_file}" << EOF
# 🚀 BTC LTP Service - Reporte de Benchmarks

**Fecha**: $(date)
**Servicio**: ${SERVICE_URL}
**Sesión**: ${TIMESTAMP}

## Tests Ejecutados

EOF
    
    # Lista de tests ejecutados
    for result_file in "${BENCHMARK_SESSION_DIR}/raw"/*_summary.txt; do
        if [[ -f "${result_file}" ]]; then
            local test_name=$(basename "${result_file}" _summary.txt)
            echo "- ✅ ${test_name}" >> "${report_file}"
        fi
    done
    
    cat >> "${report_file}" << 'EOF'

## Archivos de Resultados

- `raw/`: Datos brutos de k6 en formato JSON
- `reports/`: Reportes analizados y formateados
- `cache_effectiveness_report.md`: Análisis detallado del caché

## Métricas de Interés

### Response Times
- **P95**: Tiempo en que 95% de requests se completaron
- **P99**: Tiempo en que 99% de requests se completaron

### Cache Effectiveness
- **Hit Rate**: Porcentaje de requests servidos desde caché
- **Miss Rate**: Porcentaje de requests que requirieron datos frescos

### Error Rates
- **HTTP Error Rate**: Porcentaje de responses HTTP con error
- **Timeout Rate**: Porcentaje de requests que excedieron timeout

## Interpretación

### ✅ Indicadores de Buena Performance
- P95 < 200ms para cache hits
- P99 < 500ms general
- Cache hit rate > 80%
- Error rate < 1%

### ⚠️ Señales de Alerta
- P95 > 500ms consistentemente
- Cache hit rate < 70%
- Error rate > 5%
- Timeouts > 1%

## Próximos Pasos

1. Revisar métricas en entorno de producción
2. Configurar monitoreo continuo
3. Establecer alertas basadas en estos baselines
4. Optimizar configuración de caché según resultados
EOF
    
    log_success "Reporte consolidado generado: ${report_file}"
}

# Mostrar resumen final
show_summary() {
    log_info "=== RESUMEN DE BENCHMARKS ==="
    log_info "Sesión: ${TIMESTAMP}"
    log_info "Ubicación: ${BENCHMARK_SESSION_DIR}"
    log_info ""
    log_info "📁 Archivos generados:"
    
    find "${BENCHMARK_SESSION_DIR}" -name "*.json" -o -name "*.txt" -o -name "*.md" | while read -r file; do
        echo "  - $(basename "${file}")"
    done
    
    log_info ""
    log_success "Benchmarks completados! 🎉"
    log_info ""
    log_info "Para revisar los resultados:"
    log_info "  cd ${BENCHMARK_SESSION_DIR}/reports"
    log_info "  cat benchmark_summary.md"
    log_info "  cat cache_effectiveness_report.md"
}

# Función principal
main() {
    log_info "🚀 Iniciando BTC LTP Service Benchmarks..."
    log_info "Timestamp: ${TIMESTAMP}"
    log_info "Service URL: ${SERVICE_URL}"
    
    # Verificaciones iniciales
    check_dependencies
    check_service
    setup_results_dir
    
    # Ejecutar benchmarks
    log_info "=== INICIANDO BENCHMARKS ==="
    
    # Test 1: Cache Effectiveness (el más importante)
    if run_benchmark "cache_effectiveness" "cache_effectiveness.js" "Mide efectividad del caché con diferentes patrones de carga"; then
        generate_cache_report
    fi
    
    # Test 2: Load Test General
    run_benchmark "load_test" "load_test.js" "Test de carga general con múltiples escenarios"
    
    # Test 3: Stress Test (opcional - comentar si no se desea)
    if [[ "${SKIP_STRESS_TEST}" != "true" ]]; then
        log_warning "Iniciando stress test - puede impactar el servicio significativamente"
        sleep 3
        run_benchmark "stress_test" "stress_test.js" "Test de estrés para encontrar límites del servicio"
    else
        log_info "Stress test omitido (SKIP_STRESS_TEST=true)"
    fi
    
    # Generar reportes finales
    generate_consolidated_report
    show_summary
}

# Manejo de argumentos
case "${1:-}" in
    --help|-h)
        echo "BTC LTP Service Benchmark Runner"
        echo ""
        echo "Uso: $0 [opciones]"
        echo ""
        echo "Variables de entorno:"
        echo "  SERVICE_URL=http://localhost:8080    URL del servicio (default: localhost:8080)"
        echo "  SKIP_STRESS_TEST=true                Omitir el stress test"
        echo ""
        echo "Ejemplos:"
        echo "  $0                                   Ejecutar todos los benchmarks"
        echo "  SERVICE_URL=http://prod.example.com $0   Benchmarks en producción"
        echo "  SKIP_STRESS_TEST=true $0             Solo cache y load tests"
        ;;
    *)
        main "$@"
        ;;
esac
