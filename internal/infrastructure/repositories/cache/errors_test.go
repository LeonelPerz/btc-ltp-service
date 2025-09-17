package cache

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrors_ErrKeyNotFound(t *testing.T) {
	tests := []struct {
		name        string
		error       error
		wantMessage string
		wantString  string
	}{
		{
			name:        "ErrKeyNotFound message",
			error:       ErrKeyNotFound,
			wantMessage: "key not found",
			wantString:  "key not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, tt.error)
			assert.Equal(t, tt.wantMessage, tt.error.Error())
			assert.Equal(t, tt.wantString, tt.error.Error())
		})
	}
}

func TestErrors_ErrKeyExpired(t *testing.T) {
	tests := []struct {
		name        string
		error       error
		wantMessage string
		wantString  string
	}{
		{
			name:        "ErrKeyExpired message",
			error:       ErrKeyExpired,
			wantMessage: "key expired",
			wantString:  "key expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, tt.error)
			assert.Equal(t, tt.wantMessage, tt.error.Error())
			assert.Equal(t, tt.wantString, tt.error.Error())
		})
	}
}

func TestErrors_ErrorComparison(t *testing.T) {
	tests := []struct {
		name      string
		error1    error
		error2    error
		wantEqual bool
	}{
		{
			name:      "same ErrKeyNotFound instances",
			error1:    ErrKeyNotFound,
			error2:    ErrKeyNotFound,
			wantEqual: true,
		},
		{
			name:      "same ErrKeyExpired instances",
			error1:    ErrKeyExpired,
			error2:    ErrKeyExpired,
			wantEqual: true,
		},
		{
			name:      "different error types",
			error1:    ErrKeyNotFound,
			error2:    ErrKeyExpired,
			wantEqual: false,
		},
		{
			name:      "ErrKeyNotFound vs nil",
			error1:    ErrKeyNotFound,
			error2:    nil,
			wantEqual: false,
		},
		{
			name:      "ErrKeyExpired vs nil",
			error1:    ErrKeyExpired,
			error2:    nil,
			wantEqual: false,
		},
		{
			name:      "ErrKeyNotFound vs generic error with same message",
			error1:    ErrKeyNotFound,
			error2:    errors.New("key not found"),
			wantEqual: true, // Mismo mensaje, Go los considera iguales
		},
		{
			name:      "ErrKeyExpired vs generic error with same message",
			error1:    ErrKeyExpired,
			error2:    errors.New("key expired"),
			wantEqual: true, // Mismo mensaje, Go los considera iguales
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantEqual {
				assert.Equal(t, tt.error1, tt.error2)
				assert.True(t, tt.error1 == tt.error2)
			} else {
				assert.NotEqual(t, tt.error1, tt.error2)
				assert.False(t, tt.error1 == tt.error2)
			}
		})
	}
}

func TestErrors_ErrorWrapping(t *testing.T) {
	tests := []struct {
		name          string
		baseError     error
		wrapperFormat string
		wantContains  string
		wantUnwrapped error
	}{
		{
			name:          "wrap ErrKeyNotFound",
			baseError:     ErrKeyNotFound,
			wrapperFormat: "cache operation failed: %w",
			wantContains:  "cache operation failed: key not found",
			wantUnwrapped: ErrKeyNotFound,
		},
		{
			name:          "wrap ErrKeyExpired",
			baseError:     ErrKeyExpired,
			wrapperFormat: "get operation failed: %w",
			wantContains:  "get operation failed: key expired",
			wantUnwrapped: ErrKeyExpired,
		},
		{
			name:          "multiple wrapping ErrKeyNotFound",
			baseError:     ErrKeyNotFound,
			wrapperFormat: "redis error: %w",
			wantContains:  "redis error: key not found",
			wantUnwrapped: ErrKeyNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrappedError := fmt.Errorf(tt.wrapperFormat, tt.baseError)

			assert.Error(t, wrappedError)
			assert.Contains(t, wrappedError.Error(), tt.wantContains)

			// Test unwrapping
			unwrapped := errors.Unwrap(wrappedError)
			assert.Equal(t, tt.wantUnwrapped, unwrapped)

			// Test errors.Is
			assert.True(t, errors.Is(wrappedError, tt.baseError))
		})
	}
}

func TestErrors_ErrorsIs(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		target error
		wantIs bool
	}{
		{
			name:   "ErrKeyNotFound is ErrKeyNotFound",
			err:    ErrKeyNotFound,
			target: ErrKeyNotFound,
			wantIs: true,
		},
		{
			name:   "ErrKeyExpired is ErrKeyExpired",
			err:    ErrKeyExpired,
			target: ErrKeyExpired,
			wantIs: true,
		},
		{
			name:   "ErrKeyNotFound is not ErrKeyExpired",
			err:    ErrKeyNotFound,
			target: ErrKeyExpired,
			wantIs: false,
		},
		{
			name:   "ErrKeyExpired is not ErrKeyNotFound",
			err:    ErrKeyExpired,
			target: ErrKeyNotFound,
			wantIs: false,
		},
		{
			name:   "wrapped ErrKeyNotFound is ErrKeyNotFound",
			err:    fmt.Errorf("cache error: %w", ErrKeyNotFound),
			target: ErrKeyNotFound,
			wantIs: true,
		},
		{
			name:   "wrapped ErrKeyExpired is ErrKeyExpired",
			err:    fmt.Errorf("cache error: %w", ErrKeyExpired),
			target: ErrKeyExpired,
			wantIs: true,
		},
		{
			name:   "generic error is not ErrKeyNotFound",
			err:    errors.New("some other error"),
			target: ErrKeyNotFound,
			wantIs: false,
		},
		{
			name:   "nil error is not ErrKeyNotFound",
			err:    nil,
			target: ErrKeyNotFound,
			wantIs: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := errors.Is(tt.err, tt.target)
			assert.Equal(t, tt.wantIs, result)
		})
	}
}

func TestErrors_InSwitchStatements(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCase string
	}{
		{
			name:         "ErrKeyNotFound in switch",
			err:          ErrKeyNotFound,
			expectedCase: "not_found",
		},
		{
			name:         "ErrKeyExpired in switch",
			err:          ErrKeyExpired,
			expectedCase: "expired",
		},
		{
			name:         "other error in switch",
			err:          errors.New("some other error"),
			expectedCase: "other",
		},
		{
			name:         "nil error in switch",
			err:          nil,
			expectedCase: "no_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string

			switch {
			case tt.err == nil:
				result = "no_error"
			case errors.Is(tt.err, ErrKeyNotFound):
				result = "not_found"
			case errors.Is(tt.err, ErrKeyExpired):
				result = "expired"
			default:
				result = "other"
			}

			assert.Equal(t, tt.expectedCase, result)
		})
	}
}

func TestErrors_TypeAssertions(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantType string
	}{
		{
			name:     "ErrKeyNotFound type",
			err:      ErrKeyNotFound,
			wantType: "*errors.errorString",
		},
		{
			name:     "ErrKeyExpired type",
			err:      ErrKeyExpired,
			wantType: "*errors.errorString",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verificar que son errores válidos
			assert.Error(t, tt.err)

			// Verificar que implementan la interfaz error
			_, ok := tt.err.(error)
			assert.True(t, ok)

			// Verificar que no son nil
			assert.NotNil(t, tt.err)
		})
	}
}

func TestErrors_ConcurrentAccess(t *testing.T) {
	// Test que los errores pueden ser accedidos concurrentemente sin problemas
	t.Run("concurrent access to ErrKeyNotFound", func(t *testing.T) {
		done := make(chan bool, 100)

		for i := 0; i < 100; i++ {
			go func() {
				defer func() { done <- true }()

				// Acceder al error desde múltiples goroutines
				err := ErrKeyNotFound
				msg := err.Error()

				assert.Equal(t, "key not found", msg)
				assert.Error(t, err)
			}()
		}

		// Esperar a que todas las goroutines terminen
		for i := 0; i < 100; i++ {
			<-done
		}
	})

	t.Run("concurrent access to ErrKeyExpired", func(t *testing.T) {
		done := make(chan bool, 100)

		for i := 0; i < 100; i++ {
			go func() {
				defer func() { done <- true }()

				// Acceder al error desde múltiples goroutines
				err := ErrKeyExpired
				msg := err.Error()

				assert.Equal(t, "key expired", msg)
				assert.Error(t, err)
			}()
		}

		// Esperar a que todas las goroutines terminen
		for i := 0; i < 100; i++ {
			<-done
		}
	})
}

func TestErrors_StringRepresentation(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantStr string
	}{
		{
			name:    "ErrKeyNotFound string representation",
			err:     ErrKeyNotFound,
			wantStr: "key not found",
		},
		{
			name:    "ErrKeyExpired string representation",
			err:     ErrKeyExpired,
			wantStr: "key expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Error() method
			assert.Equal(t, tt.wantStr, tt.err.Error())

			// Test string conversion
			assert.Equal(t, tt.wantStr, fmt.Sprintf("%s", tt.err))

			// Test with %v verb
			assert.Equal(t, tt.wantStr, fmt.Sprintf("%v", tt.err))

			// Test with %+v verb (detailed)
			assert.Contains(t, fmt.Sprintf("%+v", tt.err), tt.wantStr)
		})
	}
}

func TestErrors_NilComparisons(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		isNil bool
	}{
		{
			name:  "ErrKeyNotFound is not nil",
			err:   ErrKeyNotFound,
			isNil: false,
		},
		{
			name:  "ErrKeyExpired is not nil",
			err:   ErrKeyExpired,
			isNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.isNil {
				assert.Nil(t, tt.err)
			} else {
				assert.NotNil(t, tt.err)
			}
		})
	}
}

// Test de uso práctico de los errores en contexto de cache
func TestErrors_PracticalUsage(t *testing.T) {
	t.Run("error handling in cache operations", func(t *testing.T) {
		// Simular función que retorna errores de cache
		getCacheValue := func(key string) (string, error) {
			switch key {
			case "missing":
				return "", ErrKeyNotFound
			case "expired":
				return "", ErrKeyExpired
			case "valid":
				return "value", nil
			default:
				return "", errors.New("unknown error")
			}
		}

		// Test casos específicos
		tests := []struct {
			key           string
			expectedValue string
			expectedError error
		}{
			{"missing", "", ErrKeyNotFound},
			{"expired", "", ErrKeyExpired},
			{"valid", "value", nil},
			{"unknown", "", nil}, // Error genérico
		}

		for _, tt := range tests {
			value, err := getCacheValue(tt.key)
			assert.Equal(t, tt.expectedValue, value)

			if tt.expectedError != nil {
				assert.True(t, errors.Is(err, tt.expectedError))
			} else if tt.key == "unknown" {
				assert.Error(t, err)
				assert.False(t, errors.Is(err, ErrKeyNotFound))
				assert.False(t, errors.Is(err, ErrKeyExpired))
			} else {
				assert.NoError(t, err)
			}
		}
	})
}
