package middleware

import (
	"btc-ltp-service/internal/infrastructure/config"
	"btc-ltp-service/internal/infrastructure/logging"
	"encoding/json"
	"net/http"
	"strings"
)

// AuthMiddleware provides API key authentication functionality
type AuthMiddleware struct {
	config config.AuthConfig
}

// NewAuthMiddleware creates a new auth middleware instance
func NewAuthMiddleware(config config.AuthConfig) *AuthMiddleware {
	return &AuthMiddleware{
		config: config,
	}
}

// AuthResponse represents the authentication error response
type AuthResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// Handler wraps the given handler with API key authentication
func (am *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Debug log para verificar que el middleware se está ejecutando
		logging.Debug(r.Context(), "Auth middleware executing", logging.Fields{
			"enabled":     am.config.Enabled,
			"api_key_set": am.config.APIKey != "",
			"path":        r.URL.Path,
		})

		// Si la autenticación está deshabilitada, continuar sin verificar
		if !am.config.Enabled {
			logging.Debug(r.Context(), "Auth disabled, skipping authentication", nil)
			next.ServeHTTP(w, r)
			return
		}

		// Verificar si la ruta está en la lista de rutas no autenticadas
		if am.isUnauthenticatedPath(r.URL.Path) {
			logging.Debug(r.Context(), "Path is in unauth paths, skipping auth", logging.Fields{
				"path": r.URL.Path,
			})
			next.ServeHTTP(w, r)
			return
		}

		logging.Debug(r.Context(), "Path requires authentication", logging.Fields{
			"path":           r.URL.Path,
			"api_key_header": am.config.HeaderName,
		})

		// Obtener la API key del header
		apiKey := r.Header.Get(am.config.HeaderName)
		if apiKey == "" {
			am.respondWithAuthError(w, r, "API key missing", "API_KEY_MISSING")
			return
		}

		// Verificar la API key
		if !am.isValidAPIKey(apiKey) {
			am.respondWithAuthError(w, r, "Invalid API key", "API_KEY_INVALID")
			return
		}

		// Log successful authentication
		logging.Info(r.Context(), "API key authentication successful", logging.Fields{
			"path":       r.URL.Path,
			"method":     r.Method,
			"remote_ip":  getClientIP(r),
			"user_agent": r.Header.Get("User-Agent"),
		})

		// Continuar con el siguiente handler
		next.ServeHTTP(w, r)
	})
}

// isUnauthenticatedPath verifica si la ruta debe estar exenta de autenticación
func (am *AuthMiddleware) isUnauthenticatedPath(path string) bool {
	for _, unauthPath := range am.config.UnauthPaths {
		// Soporte para rutas exactas y prefijos
		if path == unauthPath || strings.HasPrefix(path, unauthPath) {
			return true
		}
	}
	return false
}

// isValidAPIKey verifica si la API key es válida
func (am *AuthMiddleware) isValidAPIKey(providedKey string) bool {
	// Simple string comparison - en producción podríamos usar hashing
	// o múltiples keys almacenadas en base de datos
	return providedKey == am.config.APIKey
}

// respondWithAuthError envía una respuesta de error de autenticación
func (am *AuthMiddleware) respondWithAuthError(w http.ResponseWriter, r *http.Request, message, code string) {
	// Log del intento de acceso no autorizado
	logging.Warn(r.Context(), "API key authentication failed", logging.Fields{
		"path":       r.URL.Path,
		"method":     r.Method,
		"remote_ip":  getClientIP(r),
		"user_agent": r.Header.Get("User-Agent"),
		"error_code": code,
		"reason":     message,
	})

	// Configurar headers de respuesta
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="api"`)
	w.WriteHeader(http.StatusUnauthorized)

	// Crear respuesta JSON estructurada
	response := AuthResponse{
		Error:   "Authentication Failed",
		Message: message,
		Code:    code,
	}

	// Enviar respuesta JSON
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logging.Error(r.Context(), "Error encoding auth error response", logging.Fields{
			"error": err.Error(),
		})
	}
}

// getClientIP extrae la IP real del cliente considerando proxies
func getClientIP(r *http.Request) string {
	// Verificar headers de proxy más comunes
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP != "" {
		// X-Forwarded-For puede contener múltiples IPs, tomar la primera
		if idx := strings.Index(clientIP, ","); idx != -1 {
			clientIP = strings.TrimSpace(clientIP[:idx])
		}
		return clientIP
	}

	clientIP = r.Header.Get("X-Real-IP")
	if clientIP != "" {
		return clientIP
	}

	// Fallback a RemoteAddr
	return r.RemoteAddr
}
