# 🛡️ Validación de Configuración - Fail Fast

Este documento explica las mejoras implementadas en el sistema de validación de configuración para cumplir con los requisitos de **fail-fast validation** y demostración de **precedencia de configuración**.

## 🎯 Características Implementadas

### ✅ 1. Validación TTL Mejorada (Bad TTL Detection)

La validación de TTL ahora detecta casos edge específicos:

```yaml
# ❌ Casos que fallan fast:
cache:
  ttl: 50ms      # Error: "TTL too short: 50ms, minimum 100ms (causes excessive cache churn)"
  ttl: 0s        # Error: "TTL must be positive, got: 0s"  
  ttl: -10s      # Error: "TTL must be positive, got: -10s"
  ttl: 25h       # Error: "TTL too long: 25h, maximum 24h (stale data risk)"
  ttl: 500ms     # Warning: "TTL potentially inefficient: 500ms, recommended minimum 1s"
  ttl: 2h        # Warning: "TTL potentially stale: 2h, recommended maximum 1h for financial data"

# ✅ Casos válidos:
cache:
  ttl: 30s       # ✅ Óptimo
  ttl: 5m        # ✅ Bueno 
  ttl: 1h        # ✅ Límite recomendado
```

### ✅ 2. Validación de Pares Desconocidos (Unknown Pair Detection)

Validación contra lista conocida de pares de Kraken:

```yaml
# ❌ Casos que fallan fast:
business:
  supported_pairs:
    - "DOGE/MOON"     # Error: "unknown trading pairs: [DOGE/MOON]"
    - "INVALID"       # Error: "invalid pair format: [INVALID], expected BASE/QUOTE"
    - "BTC//USD"      # Error: "invalid pair format: [BTC//USD]"
    - "FAKE/COIN"     # Error: "unknown trading pairs: [FAKE/COIN]"

# ✅ Casos válidos (pares conocidos en Kraken):
business:
  supported_pairs:
    - "BTC/USD"       # ✅ Válido
    - "ETH/EUR"       # ✅ Válido
    - "LTC/BTC"       # ✅ Válido
    - "XRP/USD"       # ✅ Válido
```

**Pares Soportados**: BTC, ETH, LTC, XRP, ADA, DOT, LINK, UNI, SOL con USD, EUR, BTC, CHF, GBP.

### ✅ 3. Demostración de Precedencia YAML + ENV

**Orden de Precedencia**: `defaults` → `config.yaml` → `config.{environment}.yaml` → `ENV vars`

## 📁 Archivos de Configuración

### Archivos Creados

1. **`configs/config.test-bad-ttl.yaml`** - Configuración con valores inválidos para testing
2. **`configs/config.demo-precedence.yaml`** - Demuestra precedencia con valores base
3. **`configs/config.production.yaml`** - Configuración optimizada para producción
4. **`configs/demo.env`** - Variables de entorno que sobrescriben YAML

## 🚀 Uso y Demostraciones

### Ejecutar Validaciones Fail-Fast

```bash
# 1. Test con TTL inválido
CACHE_TTL=50ms go run cmd/api/main.go
# Error: "cache TTL validation failed: TTL too short: 50ms"

# 2. Test con pares desconocidos  
SUPPORTED_PAIRS="BTC/USD,DOGE/MOON" go run cmd/api/main.go
# Error: "trading pairs validation failed: unknown trading pairs: [DOGE/MOON]"

# 3. Test con configuración inválida completa
go run cmd/api/main.go --config configs/config.test-bad-ttl.yaml
# Múltiples errores de validación
```

### Demostrar Precedencia

```bash
# 1. Usar configuración base con precedencia
export CACHE_BACKEND=memory      # Sobrescribe "redis" del YAML
export PORT=8080                 # Sobrescribe "9000" del YAML  
export LOG_LEVEL=info            # Sobrescribe "debug" del YAML

go run cmd/api/main.go
# Se usa: memory cache, puerto 8080, log level info

# 2. Cargar variables desde archivo
source configs/demo.env
go run cmd/api/main.go

# 3. Demostrar con script automatizado
./scripts/demo-config-validation.sh
```

### Ejecutar Tests de Validación

```bash
# Tests específicos de validación
go test ./internal/infrastructure/config/ -v

# Test específico de TTL
go test ./internal/infrastructure/config/ -run TestValidateTTL_FailFast -v

# Test específico de pares
go test ./internal/infrastructure/config/ -run TestValidateTradingPairs_FailFast -v
```

## 📋 Ejemplos Prácticos

### Ejemplo 1: Configuración Inválida que Falla Fast

```yaml
# configs/config.invalid.yaml
cache:
  ttl: 10ms           # ❌ Muy corto
business:
  supported_pairs:
    - "DOGE/MOON"     # ❌ Par desconocido
    - "INVALID"       # ❌ Formato inválido
```

```bash
$ go run cmd/api/main.go
# Output:
# ERROR: cache TTL validation failed: TTL too short: 10ms, minimum 100ms (causes excessive cache churn)
# ERROR: trading pairs validation failed: unknown trading pairs: [DOGE/MOON], supported pairs: [BTC/USD, ETH/USD, LTC/USD, XRP/USD, BTC/EUR, ETH/EUR]
```

### Ejemplo 2: Precedencia Completa

**Base (config.demo-precedence.yaml):**
```yaml
server:
  port: 9000        # Base value
cache:
  backend: redis    # Base value
  ttl: 60s         # Base value
logging:
  level: debug     # Base value
```

**Environment Override:**
```bash
export PORT=8080          # ENV overrides 9000
export CACHE_BACKEND=memory  # ENV overrides redis
export LOG_LEVEL=info     # ENV overrides debug
# ttl: 60s permanece del YAML (no override)
```

**Resultado Final:**
- Puerto: `8080` (ENV)
- Cache: `memory` (ENV)  
- TTL: `60s` (YAML)
- Log Level: `info` (ENV)

## 🛠️ Arquitectura de Validación

### Flujo de Validación

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Load Config   │───▶│  Parse & Merge   │───▶│   Validate      │
│   (YAML + ENV)  │    │   (Precedence)   │    │   (Fail Fast)   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                                         │
                                                         ▼
                                               ┌─────────────────┐
                                               │ Start Service   │
                                               │ OR Exit w/Error │
                                               └─────────────────┘
```

### Validadores Específicos

- **`validateTTL()`**: Detecta TTL muy corto/largo con mensajes específicos
- **`validateTradingPairs()`**: Valida contra lista conocida de Kraken
- **`validateCache()`**: Integra validación de TTL y backend
- **`validateBusiness()`**: Integra validación de pares

## 📊 Casos de Test Cubiertos

| Caso | Input | Resultado Esperado |
|------|-------|-------------------|
| TTL Negativo | `ttl: -10s` | ❌ "TTL must be positive" |
| TTL Muy Corto | `ttl: 50ms` | ❌ "TTL too short: 50ms, minimum 100ms" |
| TTL Muy Largo | `ttl: 25h` | ❌ "TTL too long: 25h, maximum 24h" |
| Par Desconocido | `DOGE/MOON` | ❌ "unknown trading pairs: [DOGE/MOON]" |
| Formato Inválido | `INVALID` | ❌ "invalid pair format: [INVALID]" |
| Configuración Válida | `ttl: 30s, BTC/USD` | ✅ Validación exitosa |

## 🎨 Mejoras en Mensajes de Error

### Antes
```
Error: invalid configuration
```

### Después  
```
Error: cache TTL validation failed: TTL too short: 50ms, minimum 100ms (causes excessive cache churn)
Error: trading pairs validation failed: unknown trading pairs: [DOGE/MOON], supported pairs: [BTC/USD, ETH/USD, LTC/USD, XRP/USD, BTC/EUR, ETH/EUR]
```

**Ventajas**:
- Contexto específico del problema
- Valores exactos que causan el error  
- Sugerencias de valores válidos
- Razón técnica del problema (cache churn, stale data, etc.)

---

## 🎯 Cumplimiento de Consigna

✅ **Bad TTL Detection**: Implementado con casos edge específicos  
✅ **Unknown Pair Detection**: Validación contra lista conocida de Kraken  
✅ **Fail Fast**: Validación completa antes de iniciar servicios  
✅ **YAML + ENV Precedence**: Demostrado con archivos de ejemplo y scripts  
✅ **Clean Implementation**: Separación de responsabilidades, mensajes claros

La implementación es **production-ready** y puede expandirse fácilmente con más validaciones específicas.
