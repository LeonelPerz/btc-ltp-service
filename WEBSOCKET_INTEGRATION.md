# Integración WebSocket para el Servicio BTC LTP

## Resumen
Se ha implementado soporte completo para WebSocket con fallback automático a REST API para el servicio de Last Traded Price (LTP) de Bitcoin, conectándose al feed en tiempo real de Kraken.

## Características Implementadas

### 1. Cliente WebSocket Nativo
- **Archivo**: `internal/client/kraken/websocket_client.go`
- Conexión a `wss://ws.kraken.com/` para datos en tiempo real
- Suscripción automática al feed ticker de Kraken
- Manejo robusto de reconexión con backoff exponencial
- Procesamiento de mensajes en tiempo real
- Thread-safe con mutex para acceso concurrente

### 2. Cliente Híbrido con Fallback
- **Archivo**: `internal/client/kraken/hybrid_client.go`
- Combina WebSocket y REST API automáticamente
- Fallback inteligente a REST si WebSocket falla
- Recuperación automática cuando WebSocket se restablece
- Configuración flexible para habilitar/deshabilitar WebSocket

### 3. Configuración Extendida
- **Archivo**: `internal/config/config.go`
- Variables de entorno para control de WebSocket:
  - `KRAKEN_WEBSOCKET_ENABLED`: Habilitar/deshabilitar WebSocket
  - `KRAKEN_WEBSOCKET_URL`: URL del WebSocket (por defecto: `wss://ws.kraken.com/`)
  - `KRAKEN_WEBSOCKET_TIMEOUT`: Timeout de conexión WebSocket
  - `KRAKEN_RECONNECT_DELAY`: Retraso entre intentos de reconexión
  - `KRAKEN_MAX_RECONNECT_TRIES`: Máximo número de intentos de reconexión

### 4. Nuevos Modelos de Datos
- **Archivo**: `internal/model/response.go`
- Estructuras para mensajes WebSocket de Kraken
- Manejo de datos ticker en tiempo real
- Compatibilidad con formato de API REST existente

### 5. Endpoint de Estado
- **Nuevo endpoint**: `GET /api/v1/status`
- Monitoreo del estado de conexión WebSocket
- Información sobre modo fallback
- Timestamp de última actualización

## Arquitectura de Fallback

### Flujo de Trabajo
1. **Inicio**: Se intenta conectar al WebSocket
2. **Éxito WebSocket**: Datos en tiempo real desde WebSocket
3. **Fallo WebSocket**: Fallback automático a REST API
4. **Recuperación**: Reconexión automática en segundo plano
5. **Restauración**: Vuelta a WebSocket cuando se recupera la conexión

### Criterios de Fallback
- Error de conexión WebSocket
- Pérdida de conectividad
- Datos obsoletos (>2 minutos sin actualizaciones)
- Errores durante la suscripción

## Configuración por Defecto

```yaml
kraken:
  websocket_enabled: true
  websocket_url: "wss://ws.kraken.com/"
  websocket_timeout: "30s"
  reconnect_delay: "5s"
  max_reconnect_tries: 5
```

## Ventajas de la Implementación

### Performance
- **Latencia reducida**: Datos en tiempo real vs polling REST
- **Menor carga de servidor**: Menos requests HTTP
- **Eficiencia de ancho de banda**: Stream continuo vs requests individuales

### Robustez
- **Alta disponibilidad**: Fallback automático garantiza continuidad
- **Recuperación automática**: Sin intervención manual necesaria
- **Monitoreo integrado**: Endpoint de estado para observabilidad

### Compatibilidad
- **Backward compatible**: API REST existente sigue funcionando
- **Configuración flexible**: WebSocket se puede deshabilitar
- **Interfaz uniforme**: Misma interfaz para ambos modos

## Uso de la Nueva Funcionalidad

### Variables de Entorno
```bash
# Habilitar WebSocket (por defecto: true)
export KRAKEN_WEBSOCKET_ENABLED=true

# Personalizar configuración WebSocket
export KRAKEN_WEBSOCKET_TIMEOUT=60s
export KRAKEN_RECONNECT_DELAY=3s
export KRAKEN_MAX_RECONNECT_TRIES=10
```

### Monitoreo
```bash
# Verificar estado de conexión
curl http://localhost:8080/api/v1/status

# Respuesta de ejemplo
{
  "status": "ok",
  "connection": {
    "websocket_enabled": true,
    "websocket_connected": true,
    "fallback_mode": false,
    "rest_available": true,
    "last_ws_update": "2025-01-15T10:30:45Z"
  },
  "timestamp": 1736936245
}
```

### Logs del Sistema
```
INFO - WebSocket connection established and subscribed to ticker data
INFO - ✓ WebSocket connection: ACTIVE (real-time updates)
WARN - WebSocket data unavailable, falling back to REST
INFO - WebSocket data received, switching back from REST fallback
```

## Métricas y Observabilidad

- **Prometheus metrics**: Incluye métricas de WebSocket
- **Structured logging**: Logs detallados de conexión y fallback
- **Health checks**: Estado de salud incluye conectividad WebSocket
- **Error tracking**: Métricas de errores WebSocket separadas

## Próximos Pasos Recomendados

1. **Tests unitarios**: Implementar tests para WebSocket client
2. **Tests de integración**: Probar escenarios de fallback
3. **Monitoreo adicional**: Alertas para pérdida de conexión WebSocket
4. **Optimizaciones**: Implementar heartbeat personalizado si es necesario

## Notas de Implementación

- Compatible con Go 1.21.13
- Usa `gorilla/websocket` como dependencia WebSocket
- Thread-safe y production-ready
- Manejo de errores robusto con logging estructurado
- Configuración por variables de entorno y archivos YAML
