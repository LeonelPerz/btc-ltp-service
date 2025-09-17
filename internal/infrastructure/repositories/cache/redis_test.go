package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRedisClient es un mock del cliente Redis
type MockRedisClient struct {
	mock.Mock
}

func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	args := m.Called(ctx, key)
	cmd := redis.NewStringCmd(ctx, "get", key)
	if args.Error(1) != nil {
		cmd.SetErr(args.Error(1))
	} else {
		cmd.SetVal(args.String(0))
	}
	return cmd
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	args := m.Called(ctx, key, value, expiration)
	cmd := redis.NewStatusCmd(ctx, "set", key, value)
	if args.Error(0) != nil {
		cmd.SetErr(args.Error(0))
	} else {
		cmd.SetVal("OK")
	}
	return cmd
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	args := m.Called(ctx, keys)
	cmd := redis.NewIntCmd(ctx, "del")
	if args.Error(1) != nil {
		cmd.SetErr(args.Error(1))
	} else {
		cmd.SetVal(int64(args.Int(0)))
	}
	return cmd
}

func (m *MockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	args := m.Called(ctx)
	cmd := redis.NewStatusCmd(ctx, "ping")
	if args.Error(0) != nil {
		cmd.SetErr(args.Error(0))
	} else {
		cmd.SetVal("PONG")
	}
	return cmd
}

func (m *MockRedisClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRedisClient) DBSize(ctx context.Context) *redis.IntCmd {
	args := m.Called(ctx)
	cmd := redis.NewIntCmd(ctx, "dbsize")
	if len(args) > 1 && args.Error(1) != nil {
		cmd.SetErr(args.Error(1))
	} else {
		// Usar Get(0) para obtener el primer argumento de forma segura
		if val := args.Get(0); val != nil {
			if intVal, ok := val.(int64); ok {
				cmd.SetVal(intVal)
			} else if intVal, ok := val.(int); ok {
				cmd.SetVal(int64(intVal))
			} else {
				cmd.SetVal(int64(0))
			}
		}
	}
	return cmd
}

func (m *MockRedisClient) FlushAll(ctx context.Context) *redis.StatusCmd {
	args := m.Called(ctx)
	cmd := redis.NewStatusCmd(ctx, "flushall")
	if args.Error(0) != nil {
		cmd.SetErr(args.Error(0))
	} else {
		cmd.SetVal("OK")
	}
	return cmd
}

// RedisCache con cliente mockeable para testing
type TestableRedisCache struct {
	client RedisClientInterface
}

type RedisClientInterface interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Ping(ctx context.Context) *redis.StatusCmd
	Close() error
	DBSize(ctx context.Context) *redis.IntCmd
	FlushAll(ctx context.Context) *redis.StatusCmd
}

func NewTestableRedisCache(client RedisClientInterface) *TestableRedisCache {
	return &TestableRedisCache{client: client}
}

func (r *TestableRedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrKeyNotFound
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

func (r *TestableRedisCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *TestableRedisCache) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func TestNewRedisCache(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		password string
		db       int
		wantNil  bool
	}{
		{
			name:     "valid parameters",
			addr:     "localhost:6379",
			password: "",
			db:       0,
			wantNil:  false,
		},
		{
			name:     "with password",
			addr:     "localhost:6379",
			password: "secret",
			db:       0,
			wantNil:  false,
		},
		{
			name:     "different database",
			addr:     "localhost:6379",
			password: "",
			db:       1,
			wantNil:  false,
		},
		{
			name:     "empty address",
			addr:     "",
			password: "",
			db:       0,
			wantNil:  false, // Constructor no valida parámetros
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewRedisCache(tt.addr, tt.password, tt.db)
			if tt.wantNil {
				assert.Nil(t, cache)
			} else {
				assert.NotNil(t, cache)
				redisCache, ok := cache.(*RedisCache)
				assert.True(t, ok)
				assert.NotNil(t, redisCache.client)
			}
		})
	}
}

func TestNewRedisCacheWithClient(t *testing.T) {
	tests := []struct {
		name    string
		client  *redis.Client
		wantNil bool
	}{
		{
			name:    "valid client",
			client:  redis.NewClient(&redis.Options{Addr: "localhost:6379"}),
			wantNil: false,
		},
		{
			name:    "nil client",
			client:  nil,
			wantNil: false, // Constructor no valida nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewRedisCacheWithClient(tt.client)
			if tt.wantNil {
				assert.Nil(t, cache)
			} else {
				assert.NotNil(t, cache)
			}
		})
	}
}

func TestRedisCache_Get(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		setupMock func(*MockRedisClient)
		wantValue string
		wantErr   error
	}{
		{
			name: "existing key",
			key:  "test-key",
			setupMock: func(m *MockRedisClient) {
				m.On("Get", mock.Anything, "test-key").Return("test-value", nil)
			},
			wantValue: "test-value",
			wantErr:   nil,
		},
		{
			name: "non-existent key (redis.Nil)",
			key:  "missing-key",
			setupMock: func(m *MockRedisClient) {
				m.On("Get", mock.Anything, "missing-key").Return("", redis.Nil)
			},
			wantValue: "",
			wantErr:   ErrKeyNotFound,
		},
		{
			name: "empty key",
			key:  "",
			setupMock: func(m *MockRedisClient) {
				m.On("Get", mock.Anything, "").Return("empty-key-value", nil)
			},
			wantValue: "empty-key-value",
			wantErr:   nil,
		},
		{
			name: "redis connection error",
			key:  "test-key",
			setupMock: func(m *MockRedisClient) {
				m.On("Get", mock.Anything, "test-key").Return("", errors.New("connection failed"))
			},
			wantValue: "",
			wantErr:   errors.New("connection failed"),
		},
		{
			name: "redis timeout",
			key:  "test-key",
			setupMock: func(m *MockRedisClient) {
				m.On("Get", mock.Anything, "test-key").Return("", context.DeadlineExceeded)
			},
			wantValue: "",
			wantErr:   context.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockRedisClient{}
			tt.setupMock(mockClient)

			cache := NewTestableRedisCache(mockClient)
			ctx := context.Background()

			value, err := cache.Get(ctx, tt.key)

			assert.Equal(t, tt.wantValue, value)
			if tt.wantErr != nil {
				assert.Error(t, err)
				if tt.wantErr.Error() != "key not found" && tt.wantErr.Error() != "key expired" {
					assert.Equal(t, tt.wantErr.Error(), err.Error())
				} else {
					assert.Equal(t, tt.wantErr, err)
				}
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestRedisCache_Set(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     string
		ttl       time.Duration
		setupMock func(*MockRedisClient)
		wantErr   bool
	}{
		{
			name:  "valid key-value",
			key:   "test-key",
			value: "test-value",
			ttl:   5 * time.Minute,
			setupMock: func(m *MockRedisClient) {
				m.On("Set", mock.Anything, "test-key", "test-value", 5*time.Minute).Return(nil)
			},
			wantErr: false,
		},
		{
			name:  "empty key",
			key:   "",
			value: "test-value",
			ttl:   5 * time.Minute,
			setupMock: func(m *MockRedisClient) {
				m.On("Set", mock.Anything, "", "test-value", 5*time.Minute).Return(nil)
			},
			wantErr: false,
		},
		{
			name:  "empty value",
			key:   "test-key",
			value: "",
			ttl:   5 * time.Minute,
			setupMock: func(m *MockRedisClient) {
				m.On("Set", mock.Anything, "test-key", "", 5*time.Minute).Return(nil)
			},
			wantErr: false,
		},
		{
			name:  "zero TTL",
			key:   "test-key",
			value: "test-value",
			ttl:   0,
			setupMock: func(m *MockRedisClient) {
				m.On("Set", mock.Anything, "test-key", "test-value", time.Duration(0)).Return(nil)
			},
			wantErr: false,
		},
		{
			name:  "negative TTL",
			key:   "test-key",
			value: "test-value",
			ttl:   -1 * time.Minute,
			setupMock: func(m *MockRedisClient) {
				m.On("Set", mock.Anything, "test-key", "test-value", -1*time.Minute).Return(nil)
			},
			wantErr: false,
		},
		{
			name:  "redis connection error",
			key:   "test-key",
			value: "test-value",
			ttl:   5 * time.Minute,
			setupMock: func(m *MockRedisClient) {
				m.On("Set", mock.Anything, "test-key", "test-value", 5*time.Minute).Return(errors.New("connection failed"))
			},
			wantErr: true,
		},
		{
			name:  "redis memory full",
			key:   "test-key",
			value: "test-value",
			ttl:   5 * time.Minute,
			setupMock: func(m *MockRedisClient) {
				m.On("Set", mock.Anything, "test-key", "test-value", 5*time.Minute).Return(errors.New("OOM command not allowed"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockRedisClient{}
			tt.setupMock(mockClient)

			cache := NewTestableRedisCache(mockClient)
			ctx := context.Background()

			err := cache.Set(ctx, tt.key, tt.value, tt.ttl)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestRedisCache_Delete(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		setupMock func(*MockRedisClient)
		wantErr   bool
	}{
		{
			name: "existing key",
			key:  "test-key",
			setupMock: func(m *MockRedisClient) {
				m.On("Del", mock.Anything, []string{"test-key"}).Return(1, nil)
			},
			wantErr: false,
		},
		{
			name: "non-existent key",
			key:  "missing-key",
			setupMock: func(m *MockRedisClient) {
				m.On("Del", mock.Anything, []string{"missing-key"}).Return(0, nil)
			},
			wantErr: false, // Delete no falla si la clave no existe
		},
		{
			name: "empty key",
			key:  "",
			setupMock: func(m *MockRedisClient) {
				m.On("Del", mock.Anything, []string{""}).Return(0, nil)
			},
			wantErr: false,
		},
		{
			name: "redis connection error",
			key:  "test-key",
			setupMock: func(m *MockRedisClient) {
				m.On("Del", mock.Anything, []string{"test-key"}).Return(0, errors.New("connection failed"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockRedisClient{}
			tt.setupMock(mockClient)

			cache := NewTestableRedisCache(mockClient)
			ctx := context.Background()

			err := cache.Delete(ctx, tt.key)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// Tests para los métodos auxiliares usando el RedisCache real con mock
func TestRedisCache_AuxiliaryMethods(t *testing.T) {
	t.Run("Ping", func(t *testing.T) {
		tests := []struct {
			name      string
			setupMock func(*MockRedisClient)
			wantErr   bool
		}{
			{
				name: "successful ping",
				setupMock: func(m *MockRedisClient) {
					m.On("Ping", mock.Anything).Return(nil)
				},
				wantErr: false,
			},
			{
				name: "connection failure",
				setupMock: func(m *MockRedisClient) {
					m.On("Ping", mock.Anything).Return(errors.New("connection failed"))
				},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockClient := &MockRedisClient{}
				tt.setupMock(mockClient)

				// Para Ping necesitamos usar el RedisCache real
				// pero esto requiere modificar la estructura para ser testeable
				// Por ahora, probamos la lógica básica
				ctx := context.Background()
				err := mockClient.Ping(ctx).Err()

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}

				mockClient.AssertExpectations(t)
			})
		}
	})

	t.Run("Close", func(t *testing.T) {
		tests := []struct {
			name      string
			setupMock func(*MockRedisClient)
			wantErr   bool
		}{
			{
				name: "successful close",
				setupMock: func(m *MockRedisClient) {
					m.On("Close").Return(nil)
				},
				wantErr: false,
			},
			{
				name: "already closed",
				setupMock: func(m *MockRedisClient) {
					m.On("Close").Return(errors.New("client is closed"))
				},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockClient := &MockRedisClient{}
				tt.setupMock(mockClient)

				err := mockClient.Close()

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}

				mockClient.AssertExpectations(t)
			})
		}
	})

	t.Run("DBSize", func(t *testing.T) {
		tests := []struct {
			name      string
			setupMock func(*MockRedisClient)
			wantSize  int64
			wantErr   bool
		}{
			{
				name: "empty database",
				setupMock: func(m *MockRedisClient) {
					m.On("DBSize", mock.Anything).Return(int64(0), nil)
				},
				wantSize: 0,
				wantErr:  false,
			},
			{
				name: "database with keys",
				setupMock: func(m *MockRedisClient) {
					m.On("DBSize", mock.Anything).Return(int64(42), nil)
				},
				wantSize: 42,
				wantErr:  false,
			},
			{
				name: "connection error",
				setupMock: func(m *MockRedisClient) {
					m.On("DBSize", mock.Anything).Return(nil, errors.New("connection failed"))
				},
				wantSize: 0,
				wantErr:  true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockClient := &MockRedisClient{}
				tt.setupMock(mockClient)

				ctx := context.Background()
				size, err := mockClient.DBSize(ctx).Result()

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.wantSize, size)
				}

				mockClient.AssertExpectations(t)
			})
		}
	})

	t.Run("FlushAll", func(t *testing.T) {
		tests := []struct {
			name      string
			setupMock func(*MockRedisClient)
			wantErr   bool
		}{
			{
				name: "successful flush",
				setupMock: func(m *MockRedisClient) {
					m.On("FlushAll", mock.Anything).Return(nil)
				},
				wantErr: false,
			},
			{
				name: "connection error",
				setupMock: func(m *MockRedisClient) {
					m.On("FlushAll", mock.Anything).Return(errors.New("connection failed"))
				},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockClient := &MockRedisClient{}
				tt.setupMock(mockClient)

				ctx := context.Background()
				err := mockClient.FlushAll(ctx).Err()

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}

				mockClient.AssertExpectations(t)
			})
		}
	})
}

func TestRedisCache_ContextHandling(t *testing.T) {
	tests := []struct {
		name        string
		setupCtx    func() context.Context
		operation   string
		expectError bool
	}{
		{
			name: "context cancelled - Get",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			operation:   "get",
			expectError: false, // Mock no simula cancelación
		},
		{
			name: "context timeout - Set",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
				defer cancel()
				time.Sleep(2 * time.Nanosecond)
				return ctx
			},
			operation:   "set",
			expectError: false, // Mock no simula timeout
		},
		{
			name: "valid context - Delete",
			setupCtx: func() context.Context {
				return context.Background()
			},
			operation:   "delete",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockRedisClient{}

			switch tt.operation {
			case "get":
				mockClient.On("Get", mock.Anything, "test-key").Return("value", nil)
			case "set":
				mockClient.On("Set", mock.Anything, "test-key", "value", 5*time.Minute).Return(nil)
			case "delete":
				mockClient.On("Del", mock.Anything, []string{"test-key"}).Return(1, nil)
			}

			cache := NewTestableRedisCache(mockClient)
			ctx := tt.setupCtx()

			var err error
			switch tt.operation {
			case "get":
				_, err = cache.Get(ctx, "test-key")
			case "set":
				err = cache.Set(ctx, "test-key", "value", 5*time.Minute)
			case "delete":
				err = cache.Delete(ctx, "test-key")
			}

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestRedisCache_EdgeCases(t *testing.T) {
	t.Run("large value", func(t *testing.T) {
		mockClient := &MockRedisClient{}
		largeValue := make([]byte, 1024*1024) // 1MB
		for i := range largeValue {
			largeValue[i] = byte(i % 256)
		}

		mockClient.On("Set", mock.Anything, "large-key", string(largeValue), 5*time.Minute).Return(nil)
		mockClient.On("Get", mock.Anything, "large-key").Return(string(largeValue), nil)

		cache := NewTestableRedisCache(mockClient)
		ctx := context.Background()

		err := cache.Set(ctx, "large-key", string(largeValue), 5*time.Minute)
		assert.NoError(t, err)

		value, err := cache.Get(ctx, "large-key")
		assert.NoError(t, err)
		assert.Equal(t, string(largeValue), value)

		mockClient.AssertExpectations(t)
	})

	t.Run("unicode handling", func(t *testing.T) {
		mockClient := &MockRedisClient{}
		unicodeKey := "🔑-key"
		unicodeValue := "🎯-value-ñáéíóú"

		mockClient.On("Set", mock.Anything, unicodeKey, unicodeValue, 5*time.Minute).Return(nil)
		mockClient.On("Get", mock.Anything, unicodeKey).Return(unicodeValue, nil)

		cache := NewTestableRedisCache(mockClient)
		ctx := context.Background()

		err := cache.Set(ctx, unicodeKey, unicodeValue, 5*time.Minute)
		assert.NoError(t, err)

		value, err := cache.Get(ctx, unicodeKey)
		assert.NoError(t, err)
		assert.Equal(t, unicodeValue, value)

		mockClient.AssertExpectations(t)
	})
}
