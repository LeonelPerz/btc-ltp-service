# 🔧 Solución al Error WebSocket 1006 "Abnormal Closure"

## ❌ **Problema Original**
```json
{
  "error": "websocket: close 1006 (abnormal closure): unexpected EOF",
  "level": "error", 
  "msg": "Error reading WebSocket message"
}
```

## 📋 **Análisis del Error 1006**

**Código 1006** significa "cierre anormal" - la conexión WebSocket se perdió sin un mensaje de cierre apropiado.

### 🔍 **Causas Identificadas:**

1. **Cloudflare de Kraken**: Kraken usa Cloudflare que cierra conexiones WebSocket inesperadamente
2. **Falta de Heartbeat**: Sin ping/pong, el servidor cierra conexiones "inactivas"
3. **Timeouts inadecuados**: Conexiones muertas no se detectan rápidamente
4. **Reconexión básica**: Reconexión inmediata puede sobrecargar el servidor

## ✅ **Soluciones Implementadas**

### **1. Sistema de Heartbeat (Ping/Pong)**

```go
// Ping cada 30 segundos para mantener conexión viva
pingInterval: 30 * time.Second
pongTimeout:  10 * time.Second

// Función de ping automático
func (w *WebSocketClient) startPingRoutine() {
    ticker := time.NewTicker(w.pingInterval)
    for {
        // Enviar ping periódicamente
        conn.WriteMessage(websocket.PingMessage, []byte("ping"))
    }
}

// Handler de pong automático
conn.SetPongHandler(func(data string) error {
    w.lastPong = time.Now()
    return nil
})
```

### **2. Timeouts Mejorados**

| Configuración | Valor Anterior | Valor Mejorado | Propósito |
|---------------|----------------|----------------|-----------|
| `websocket_timeout` | 30s | **60s** | Más tiempo para operaciones lentas |
| `read_deadline` | Fijo | **Dinámico** | Actualizado en cada mensaje |
| `write_deadline` | Sin límite | **10s** | Detectar escrituras bloqueadas |

### **3. Detección de Conexiones Muertas**

```go
// Verificar que recibimos pong reciente
if time.Since(lastPong) > w.pingInterval+w.pongTimeout {
    logger.GetLogger().Warn("WebSocket ping timeout - connection may be dead")
    conn.Close() // Forzar reconexión
}
```

### **4. Backoff Exponencial**

**Antes**: Reconexión inmediata cada 5s
**Ahora**: Backoff exponencial con límite

| Intento | Retraso |
|---------|---------|
| 1 | 5s |
| 2 | 10s |
| 3 | 20s |
| 4 | 40s |
| 5 | 80s (máx 2min) |

```go
// Backoff exponencial: 5s, 10s, 20s, 40s, 80s
backoffDelay := w.reconnectDelay * time.Duration(1<<(currentTry-1))
if backoffDelay > 2*time.Minute {
    backoffDelay = 2 * time.Minute // Cap at 2 minutes
}
```

### **5. Parsing de Mensajes Robusto**

```go
// Manejo dual: objetos y arrays
var statusMsg model.KrakenWSMessage
if err := json.Unmarshal(message, &statusMsg); err == nil {
    // Manejar mensajes de estado
}

var tickerArray []interface{}
if err := json.Unmarshal(message, &tickerArray); err != nil {
    // Manejar mensajes de objetos también
    var objMsg map[string]interface{}
    json.Unmarshal(message, &objMsg)
}
```

## 📊 **Configuración Final**

### **Variables de Entorno**
```bash
KRAKEN_WEBSOCKET_ENABLED=true
KRAKEN_WEBSOCKET_URL=wss://ws.kraken.com/
KRAKEN_WEBSOCKET_TIMEOUT=60s          # Aumentado de 30s
KRAKEN_RECONNECT_DELAY=5s             # Base para backoff exponencial
KRAKEN_MAX_RECONNECT_TRIES=5          # 5 intentos máximo
```

### **Parámetros Internos**
```go
pingInterval: 30 * time.Second  // Ping cada 30s
pongTimeout:  10 * time.Second  // Esperar pong 10s
readDeadline: 60 * time.Second  // Timeout de lectura
writeDeadline: 10 * time.Second // Timeout de escritura
```

## 🎯 **Resultados Esperados**

### ✅ **Mejoras Implementadas**

1. **Menos desconexiones**: Heartbeat mantiene conexión viva
2. **Detección rápida**: Timeouts dinámicos detectan problemas
3. **Reconexión inteligente**: Backoff exponencial evita spam
4. **Recuperación robusta**: Resubscripción automática
5. **Logs mejorados**: Debug detallado sin spam

### ⚡ **Comportamiento Mejorado**

**Antes**:
```
❌ Desconexión cada 1-2 minutos
❌ Reconexión inmediata → sobrecarga
❌ Errores de parsing constantes
❌ Fallback frecuente a REST
```

**Ahora**:
```
✅ Conexión estable por períodos largos
✅ Reconexión con backoff inteligente
✅ Parsing robusto sin errores
✅ WebSocket primary, REST solo cuando necesario
```

## 🔍 **Monitoreo y Debugging**

### **Logs de Heartbeat**
```bash
# Pings exitosos (nivel DEBUG)
{"level":"debug","msg":"WebSocket ping sent"}
{"level":"debug","msg":"WebSocket pong received"}

# Problemas detectados
{"level":"warn","msg":"WebSocket ping timeout - connection may be dead"}
{"level":"info","msg":"Attempting WebSocket reconnection","attempt":1,"delay":"5s"}
```

### **Verificación de Estado**
```bash
curl http://localhost:8080/api/v1/status
{
  "connection": {
    "websocket_connected": true,
    "fallback_mode": false,        # Debería ser false más seguido
    "last_ws_update": "2025-09-14T18:30:45Z"
  }
}
```

## 🚀 **Resultado Final**

**El error WebSocket 1006 "abnormal closure" debería ser significativamente menos frecuente gracias a:**

- ✅ **Heartbeat activo** mantiene conexión viva
- ✅ **Timeouts optimizados** para entorno Cloudflare
- ✅ **Reconexión inteligente** evita sobrecargar servidor
- ✅ **Detección proactiva** de conexiones problemáticas

**La conexión WebSocket ahora es mucho más estable y resiliente.**
