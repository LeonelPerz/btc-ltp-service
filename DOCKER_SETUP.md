# 🐳 BTC LTP Service - Configuración Docker

Este documento explica cómo configurar y usar el servicio BTC LTP con Docker Compose.

## 🚀 Inicio Rápido

### 1. Levantar el servicio completo (con Redis)
```bash
# Usar Makefile (recomendado)
make up

# O usando docker-compose directamente
docker-compose up -d
```

### 2. Levantar solo con cache en memoria
```bash
# Para desarrollo/testing sin Redis
make memory

# O usando docker-compose directamente
docker-compose -f docker-compose.memory.yml up -d
```

### 3. Probar el servicio
```bash
# Test automático completo
make test

# Tests manuales
curl http://localhost:8080/health
curl http://localhost:8080/api/v1/status
curl http://localhost:8080/api/v1/ltp
curl "http://localhost:8080/api/v1/ltp?pair=BTC/USD"
```

## ⚙️ Configuración

### Variables de Entorno

El servicio soporta las siguientes variables de entorno:

#### **Server Configuration**
```bash
PORT=8080                          # Puerto del servidor
```

#### **Cache Configuration**
```bash
CACHE_BACKEND=redis               # redis | memory
CACHE_TTL=1m                      # TTL del cache
CACHE_REFRESH_INTERVAL=30s        # Intervalo de refresh
```

#### **Kraken API Configuration**
```bash
KRAKEN_TIMEOUT=10s                     # Timeout para llamadas REST
KRAKEN_BASE_URL=https://api.kraken.com # Base URL para PairMapper
```

#### **WebSocket Configuration**
```bash
KRAKEN_WEBSOCKET_ENABLED=true          # Habilitar WebSocket
KRAKEN_WEBSOCKET_URL=wss://ws.kraken.com/
KRAKEN_WEBSOCKET_TIMEOUT=90s           # Timeout WebSocket
KRAKEN_RECONNECT_DELAY=5s              # Delay entre reconexiones
KRAKEN_MAX_RECONNECT_TRIES=5           # Máx intentos reconexión
```

#### **Rate Limiting Configuration**
```bash
KRAKEN_RATE_LIMIT_ENABLED=true         # Habilitar rate limiting
KRAKEN_RATE_LIMIT_CONSERVATIVE=true    # Modo conservativo
KRAKEN_RATE_LIMIT_CAPACITY=10          # Capacidad del bucket
KRAKEN_RATE_LIMIT_REFILL_RATE=1        # Tokens por período
KRAKEN_RATE_LIMIT_REFILL_PERIOD=2s     # Período de refill
```

#### **Redis Configuration**
```bash
REDIS_ADDR=redis:6379             # Dirección de Redis
REDIS_PASSWORD=                   # Password de Redis (opcional)
REDIS_DB=0                        # Base de datos Redis
```

#### **Application Configuration**
```bash
LOG_LEVEL=info                    # debug | info | warn | error
SUPPORTED_PAIRS=BTC/USD,BTC/EUR,BTC/CAD,ETH/USD,ETH/EUR,LTC/USD,LTC/EUR
```

### Archivo de Configuración Personalizada

1. **Copia el archivo de ejemplo:**
   ```bash
   cp config.example.env .env
   ```

2. **Edita las variables según tus necesidades:**
   ```bash
   # Editar .env con tus configuraciones
   nano .env
   ```

3. **Usar el archivo .env:**
   ```bash
   # Docker Compose automáticamente carga .env
   docker-compose up -d
   ```

## 📁 Archivos de Configuración

### `docker-compose.yml` (Principal)
- ✅ Servicio completo con Redis
- ✅ Cache persistente
- ✅ WebSocket habilitado
- ✅ Rate limiting configurado
- ✅ Health checks

### `docker-compose.memory.yml` (Desarrollo/Testing)
- ✅ Solo cache en memoria
- ✅ Sin dependencias externas
- ✅ Inicio más rápido
- ✅ Ideal para desarrollo

## 🛠️ Comandos Útiles (Makefile)

```bash
# Gestión de servicios
make up          # Levantar con Redis
make memory      # Levantar con memory cache
make down        # Bajar servicios
make restart     # Reiniciar servicios
make build       # Construir imágenes

# Monitoreo
make logs        # Ver logs del servicio
make logs-all    # Ver logs de todos los servicios
make status      # Ver estado de containers
make test        # Probar API completa

# Desarrollo
make dev-restart  # Reinicio rápido del servicio
make dev-rebuild  # Rebuild del servicio

# Utilidades
make redis-cli   # Conectar a Redis CLI
make config      # Mostrar configuración
make clean       # Limpiar containers
make clean-all   # Limpiar todo (incluye volúmenes)
```

## 🔍 Monitoreo y Debugging

### Ver logs en tiempo real
```bash
# Logs del servicio principal
make logs

# Logs de todos los servicios
make logs-all

# Logs específicos con docker-compose
docker-compose logs -f btc-ltp-service
docker-compose logs -f redis
```

### Health Checks
```bash
# Estado de containers
make status

# Health check manual
curl http://localhost:8080/health

# Estado detallado de conexiones
curl http://localhost:8080/api/v1/status
```

### Redis Debugging
```bash
# Conectar a Redis
make redis-cli

# Comandos útiles en Redis CLI
> keys *              # Ver todas las keys
> get "BTC/USD"       # Ver precio específico
> ttl "BTC/USD"       # Ver TTL de una key
> monitor             # Monitor en tiempo real
```

## 🚀 Arquitectura del Servicio

### Componentes
- **API REST**: Endpoints para obtener precios LTP
- **WebSocket Client**: Conexión en tiempo real a Kraken WebSocket
- **PairMapper**: Mapeo dinámico de nomenclaturas Kraken (REST vs WebSocket)
- **Cache**: Redis o Memory cache para optimizar performance
- **Rate Limiter**: Control de llamadas a API externa
- **Health Checks**: Monitoreo automático del servicio

### Flujo de Datos
1. **Cliente** hace request → **API REST**
2. **API** verifica **Cache** → Si hit: retorna datos
3. Si miss: **API** obtiene datos de **WebSocket** (tiempo real)
4. Si WebSocket falla: **Fallback** a **Kraken REST API**
5. **PairMapper** convierte nomenclaturas automáticamente
6. **Cache** almacena datos frescos
7. **Rate Limiter** controla llamadas externas

## 🔧 Troubleshooting

### Problemas Comunes

#### 1. Error 500 "Failed to retrieve price data"
```bash
# Verificar configuración
curl http://localhost:8080/api/v1/status

# Ver logs detallados
make logs

# Verificar conectividad
docker-compose exec btc-ltp-service wget -q -O- http://api.kraken.com/0/public/AssetPairs
```

#### 2. WebSocket no conecta
```bash
# Verificar configuración WebSocket
docker-compose logs btc-ltp-service | grep -i websocket

# Test manual de conectividad
docker-compose exec btc-ltp-service wget -q -O- https://ws.kraken.com/
```

#### 3. Redis no disponible
```bash
# Verificar estado de Redis
make status

# Reconectar Redis
docker-compose restart redis

# Verificar conectividad
make redis-cli
```

#### 4. Pares no soportados
```bash
# Ver pares configurados
curl http://localhost:8080/api/v1/pairs

# Verificar configuración
docker-compose exec btc-ltp-service env | grep SUPPORTED_PAIRS

# Ver logs del PairMapper
docker-compose logs btc-ltp-service | grep -i "pair"
```

### Logs Útiles para Debug
```bash
# WebSocket issues
docker-compose logs btc-ltp-service | grep -i "websocket\|ws\|reconnect"

# Rate limiting
docker-compose logs btc-ltp-service | grep -i "rate\|limit"

# Cache issues
docker-compose logs btc-ltp-service | grep -i "cache\|redis"

# API calls
docker-compose logs btc-ltp-service | grep -i "http\|api\|kraken"
```

## 📊 Métricas y Monitoreo

### Endpoints de Monitoreo
```bash
GET /health           # Health check básico
GET /api/v1/status    # Estado detallado de conexiones
GET /metrics          # Métricas de Prometheus
GET /api/v1/pairs     # Pares soportados
```

### Métricas Disponibles
- Conexiones WebSocket activas
- Calls por minuto a Kraken API  
- Cache hit/miss ratios
- Response times
- Error rates

## 🔐 Seguridad

### Configuración de Producción
- ✅ Containers no-root
- ✅ Health checks configurados
- ✅ Rate limiting habilitado
- ✅ Timeouts apropiados
- ✅ Logs estructurados (no secrets)
- ✅ Restart policies configuradas
