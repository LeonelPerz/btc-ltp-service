# Implementación de Rate Limiting para API de Kraken

## Resumen

Se ha implementado un sistema completo de rate limiting usando el patrón **Token Bucket** para prevenir baneos de la API de Kraken. La implementación incluye:

## Componentes Implementados

### 1. Token Bucket (`internal/ratelimit/token_bucket.go`)

- **Algoritmo**: Token Bucket con refill automático basado en tiempo
- **Funcionalidades**:
  - Capacidad configurable (burst size)
  - Tasa de refill configurable (tokens por período)
  - Thread-safe con mutexes
  - Método `Allow()` para verificación no bloqueante
  - Método `WaitForToken()` para espera bloqueante
  - Estadísticas detalladas (`GetStats()`)

### 2. Rate Limiter de Kraken (`internal/ratelimit/kraken_limiter.go`)

- **Configuraciones Predefinidas**:
  - **Conservadora**: 10 tokens, 1 token/2s (30 req/min)
  - **Por defecto**: 15 tokens, 1 token/1s (60 req/min)
  - **Personalizada**: Configurable vía parámetros
- **Control de Estado**: Enable/disable dinámico
- **Modos**: Conservative, Default, Custom

### 3. Configuración (`internal/config/config.go`)

```yaml
kraken:
  rate_limit:
    enabled: true
    conservative: true
    capacity: 10
    refill_rate: 1
    refill_period: "2s"
```

**Variables de Entorno**:
- `KRAKEN_RATE_LIMIT_ENABLED`
- `KRAKEN_RATE_LIMIT_CONSERVATIVE` 
- `KRAKEN_RATE_LIMIT_CAPACITY`
- `KRAKEN_RATE_LIMIT_REFILL_RATE`
- `KRAKEN_RATE_LIMIT_REFILL_PERIOD`

### 4. Integración en Clientes

#### Cliente REST (`internal/client/kraken/kraken_client.go`)
- Rate limiting aplicado antes de cada petición HTTP
- Constructores con diferentes configuraciones:
  - `NewClient()` - Configuración conservadora
  - `NewClientWithRateLimit()` - Configuración personalizable
  - `NewClientWithoutRateLimit()` - Para testing
- Métodos de control:
  - `EnableRateLimit(bool)`
  - `GetRateLimitStats()`
  - `GetRateLimitMode()`

#### Cliente Híbrido (`internal/client/kraken/hybrid_client.go`)
- Rate limiting solo aplica a fallback REST
- WebSocket no afectado (conexión persistente)
- Configuración automática desde config
- Métodos de control expuestos

## Características Clave

### 1. **Request Collapsing**
- Las peticiones concurrentes comparten el mismo token bucket
- Previene ráfagas excesivas de peticiones

### 2. **Configuración Flexible**
- Múltiples niveles de configuración (conservador/default/custom)
- Control dinámico enable/disable
- Configuración vía archivo o variables de entorno

### 3. **Observabilidad**
- Logging detallado con niveles apropiados
- Estadísticas en tiempo real del token bucket
- Métricas de utilización y estado

### 4. **Thread Safety**
- Implementación thread-safe con mutexes
- Manejo concurrente seguro

### 5. **Testing Comprehensivo**
- Tests unitarios para token bucket
- Tests de integración con servidor mock
- Benchmarks de performance
- Tests de configuración

## Configuraciones Recomendadas

### Producción Conservadora
```yaml
kraken:
  rate_limit:
    enabled: true
    conservative: true    # 30 req/min, burst 10
```

### Producción Estándar  
```yaml
kraken:
  rate_limit:
    enabled: true
    conservative: false   # 60 req/min, burst 15
```

### Desarrollo/Testing
```yaml
kraken:
  rate_limit:
    enabled: false        # Sin límites
```

## Comportamiento en Tiempo de Ejecución

### 1. **Petición Permitida**
```
[INFO] Token granted by rate limiter remaining_tokens=9 capacity=10
```

### 2. **Petición Rechazada**
```
[DEBUG] Token denied by rate limiter - bucket empty remaining_tokens=0
[DEBUG] Rate limiter caused request delay wait_time=150ms
```

### 3. **Refill Automático**
```
[DEBUG] Token bucket refilled tokens_added=1 current_tokens=5 capacity=10
```

## Ventajas de la Implementación

1. **Previene Baneos**: Rate limiting conservador previene exceder límites de Kraken
2. **Performance**: Overhead mínimo en operaciones normales  
3. **Flexibilidad**: Múltiples configuraciones según necesidades
4. **Observabilidad**: Logging y métricas detalladas
5. **Mantenibilidad**: Código modular y bien testeado
6. **Compatibilidad**: No rompe funcionalidad existente

## Flujo de Integración

1. **Inicialización**: Rate limiter se crea con configuración al start
2. **Pre-Request**: Antes de cada petición HTTP se solicita token
3. **Token Available**: Petición procede inmediatamente
4. **Token Unavailable**: Espera hasta que token esté disponible
5. **Background Refill**: Tokens se reabastecen automáticamente según configuración

Esta implementación asegura que el servicio respete los límites de la API de Kraken mientras mantiene la mejor performance posible.
