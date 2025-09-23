#!/bin/bash
# Demo de Validación con Tipos Inválidos - Fail Fast
# Este script demuestra cómo el sistema maneja strings y valores inválidos en configuración

set -e
echo "🔍 Demo: Validación de Tipos Inválidos en Configuración"
echo "====================================================="

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo ""
echo -e "${BLUE}📚 Teoría: ¿Qué pasa con tipos inválidos?${NC}"
echo "1. ENV vars con strings inválidos → Viper parsing error ANTES de validación"
echo "2. YAML con valores inválidos → Viper usa defaults o falla silenciosamente"
echo "3. Nuestra validación de integridad detecta estos casos edge"
echo ""

# Función para probar casos de ENV vars
test_env_case() {
    local test_name="$1"
    local env_var="$2"
    local expected="$3"
    
    echo -e "${YELLOW}🧪 Test ENV: $test_name${NC}"
    echo "Command: $env_var go run cmd/api/main.go"
    echo ""
    
    # Ejecutar y capturar resultado
    if eval "$env_var timeout 3s go run cmd/api/main.go 2>&1" | grep -q "$expected"; then
        echo -e "${RED}✅ ¡CORRECTO! Error detectado: $expected${NC}"
    else
        echo -e "${GREEN}❌ ERROR: No se detectó el error esperado${NC}"
    fi
    echo ""
    echo "=" | head -c 50; echo ""
}

# Función para probar casos de archivos YAML
test_yaml_case() {
    local test_name="$1"
    local config_file="$2"
    local expected="$3"
    
    echo -e "${YELLOW}🧪 Test YAML: $test_name${NC}"
    echo "Config: $config_file"
    echo ""
    
    # Ejecutar y capturar resultado
    if timeout 3s go run cmd/api/main.go -config "$config_file" 2>&1 | grep -q "$expected"; then
        echo -e "${RED}✅ ¡CORRECTO! Error detectado: $expected${NC}"
    else
        echo -e "${GREEN}❌ INFO: Error no detectado (usando defaults)${NC}"
    fi
    echo ""
    echo "=" | head -c 50; echo ""
}

echo -e "${BLUE}🔬 CASOS DE TEST: Variables de Entorno${NC}"
echo ""

# Test 1: String completamente inválido en TTL
test_env_case "TTL con string inválido" \
             "CACHE_TTL='abc-invalid-string'" \
             "time: invalid duration"

# Test 2: Número sin unidad en TTL
test_env_case "TTL sin unidad de tiempo" \
             "CACHE_TTL='123'" \
             "time: missing unit in duration"

# Test 3: String parcialmente válido
test_env_case "TTL con formato malo" \
             "CACHE_TTL='30minutes'" \
             "time: unknown unit"

# Test 4: Caracteres especiales
test_env_case "TTL con caracteres especiales" \
             "CACHE_TTL='30s@#$'" \
             "time: invalid duration"

# Test 5: String vacío
test_env_case "TTL string vacío" \
             "CACHE_TTL=''" \
             "time: invalid duration"

echo ""
echo -e "${BLUE}🔬 CASOS DE TEST: Archivos YAML${NC}"
echo ""

# Test archivos YAML con valores inválidos
test_yaml_case "TTL con string inválido en YAML" \
              "configs/config.test-invalid-types.yaml" \
              "Configuration loaded"

test_yaml_case "TTL con valor 0 en YAML" \
              "configs/config.test-zero-values.yaml" \
              "cache TTL parsed as 0"

echo ""
echo -e "${GREEN}📊 RESUMEN DE COMPORTAMIENTOS:${NC}"
echo ""
echo -e "${YELLOW}ENV Variables:${NC}"
echo "• Strings inválidos → ${RED}Viper parsing error INMEDIATO${NC}"
echo "• Números sin unidad → ${RED}Viper parsing error INMEDIATO${NC}" 
echo "• Caracteres especiales → ${RED}Viper parsing error INMEDIATO${NC}"
echo ""
echo -e "${YELLOW}YAML Files:${NC}"
echo "• Strings inválidos → ${GREEN}Viper usa defaults, servicio inicia${NC}"
echo "• Valores 0 explícitos → ${RED}Nuestra validación detecta y falla${NC}"
echo "• Valores faltantes → ${GREEN}Viper usa defaults del código${NC}"
echo ""
echo -e "${BLUE}🎯 CONCLUSIÓN:${NC}"
echo "• ${RED}ENV vars fallan FAST en parsing (nivel Viper)${NC}"
echo "• ${GREEN}YAML es más permisivo, usa defaults silenciosamente${NC}"
echo "• ${YELLOW}Nuestra validación detecta casos edge (valores 0, vacíos)${NC}"
echo ""

# Mostrar ejemplo práctico final
echo -e "${BLUE}🔍 EJEMPLO PRÁCTICO:${NC}"
echo ""
echo "Para demostrar parsing error:"
echo -e "${YELLOW}$ CACHE_TTL='abc' go run cmd/api/main.go${NC}"
echo "Result: time: invalid duration"
echo ""
echo "Para demostrar detección de valores zero:"
echo -e "${YELLOW}$ go run cmd/api/main.go -config configs/config.test-zero-values.yaml${NC}"  
echo "Result: cache TTL parsed as 0, likely due to invalid duration format"
echo ""

echo -e "${GREEN}✅ Demo de tipos inválidos completado!${NC}"
