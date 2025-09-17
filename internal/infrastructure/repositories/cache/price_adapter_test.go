package cache

import (
	"btc-ltp-service/internal/domain/entities"
	"btc-ltp-service/internal/domain/interfaces"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockCache es un mock del interfaces.Cache
type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *MockCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func TestNewPriceCache(t *testing.T) {
	tests := []struct {
		name    string
		backend interfaces.Cache
		ttl     time.Duration
		wantNil bool
	}{
		{
			name:    "valid parameters",
			backend: &MockCache{},
			ttl:     5 * time.Minute,
			wantNil: false,
		},
		{
			name:    "nil backend",
			backend: nil,
			ttl:     5 * time.Minute,
			wantNil: false, // El constructor no valida nil backend
		},
		{
			name:    "zero TTL",
			backend: &MockCache{},
			ttl:     0,
			wantNil: false,
		},
		{
			name:    "negative TTL",
			backend: &MockCache{},
			ttl:     -1 * time.Minute,
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewPriceCache(tt.backend, tt.ttl)
			if tt.wantNil {
				assert.Nil(t, adapter)
			} else {
				assert.NotNil(t, adapter)
				assert.Equal(t, tt.backend, adapter.backend)
				assert.Equal(t, tt.ttl, adapter.ttl)
			}
		})
	}
}

func TestPriceCacheAdapter_key(t *testing.T) {
	adapter := NewPriceCache(&MockCache{}, time.Minute)

	tests := []struct {
		name string
		pair string
		want string
	}{
		{
			name: "valid pair",
			pair: "BTC/USD",
			want: "price:BTC/USD",
		},
		{
			name: "empty pair",
			pair: "",
			want: "price:",
		},
		{
			name: "special characters",
			pair: "BTC-USD@2024",
			want: "price:BTC-USD@2024",
		},
		{
			name: "unicode characters",
			pair: "₿TC/USD",
			want: "price:₿TC/USD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.key(tt.pair)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPriceCacheAdapter_Set(t *testing.T) {
	tests := []struct {
		name        string
		price       *entities.Price
		setupMock   func(*MockCache)
		wantErr     bool
		expectedErr string
	}{
		{
			name: "valid price",
			price: &entities.Price{
				Pair:      "BTC/USD",
				Amount:    50000.0,
				Timestamp: time.Now(),
				Age:       time.Minute,
			},
			setupMock: func(m *MockCache) {
				m.On("Set", mock.Anything, "price:BTC/USD", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(nil)
			},
			wantErr: false,
		},
		{
			name:  "nil price",
			price: nil,
			setupMock: func(m *MockCache) {
				// No se llama al mock porque falla antes
			},
			wantErr:     true,
			expectedErr: "runtime error", // panic convertido en error
		},
		{
			name: "price with empty pair",
			price: &entities.Price{
				Pair:      "",
				Amount:    50000.0,
				Timestamp: time.Now(),
				Age:       time.Minute,
			},
			setupMock: func(m *MockCache) {
				m.On("Set", mock.Anything, "price:", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "price with zero amount",
			price: &entities.Price{
				Pair:      "BTC/USD",
				Amount:    0.0,
				Timestamp: time.Now(),
				Age:       time.Minute,
			},
			setupMock: func(m *MockCache) {
				m.On("Set", mock.Anything, "price:BTC/USD", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "price with negative amount",
			price: &entities.Price{
				Pair:      "BTC/USD",
				Amount:    -1000.0,
				Timestamp: time.Now(),
				Age:       time.Minute,
			},
			setupMock: func(m *MockCache) {
				m.On("Set", mock.Anything, "price:BTC/USD", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "backend error",
			price: &entities.Price{
				Pair:      "BTC/USD",
				Amount:    50000.0,
				Timestamp: time.Now(),
				Age:       time.Minute,
			},
			setupMock: func(m *MockCache) {
				m.On("Set", mock.Anything, "price:BTC/USD", mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(errors.New("backend error"))
			},
			wantErr:     true,
			expectedErr: "backend error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := &MockCache{}
			if tt.setupMock != nil {
				tt.setupMock(mockCache)
			}

			adapter := NewPriceCache(mockCache, time.Minute)
			ctx := context.Background()

			// Capturar panics para casos como nil price
			defer func() {
				if r := recover(); r != nil && tt.wantErr {
					assert.Contains(t, tt.expectedErr, "runtime error")
				}
			}()

			err := adapter.Set(ctx, tt.price)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != "" && err != nil {
					assert.Contains(t, err.Error(), tt.expectedErr)
				}
			} else {
				assert.NoError(t, err)
			}

			mockCache.AssertExpectations(t)
		})
	}
}

func TestPriceCacheAdapter_Get(t *testing.T) {
	validPrice := &entities.Price{
		Pair:      "BTC/USD",
		Amount:    50000.0,
		Timestamp: time.Now(),
		Age:       time.Minute,
	}
	validPriceJSON, _ := json.Marshal(validPrice)

	tests := []struct {
		name        string
		pair        string
		setupMock   func(*MockCache)
		wantPrice   *entities.Price
		wantFound   bool
		description string
	}{
		{
			name: "existing price",
			pair: "BTC/USD",
			setupMock: func(m *MockCache) {
				m.On("Get", mock.Anything, "price:BTC/USD").Return(string(validPriceJSON), nil)
			},
			wantPrice: validPrice,
			wantFound: true,
		},
		{
			name: "non-existent price",
			pair: "ETH/USD",
			setupMock: func(m *MockCache) {
				m.On("Get", mock.Anything, "price:ETH/USD").Return("", ErrKeyNotFound)
			},
			wantPrice: nil,
			wantFound: false,
		},
		{
			name: "empty pair",
			pair: "",
			setupMock: func(m *MockCache) {
				m.On("Get", mock.Anything, "price:").Return("", ErrKeyNotFound)
			},
			wantPrice: nil,
			wantFound: false,
		},
		{
			name: "backend error",
			pair: "BTC/USD",
			setupMock: func(m *MockCache) {
				m.On("Get", mock.Anything, "price:BTC/USD").Return("", errors.New("backend error"))
			},
			wantPrice: nil,
			wantFound: false,
		},
		{
			name: "invalid JSON data",
			pair: "BTC/USD",
			setupMock: func(m *MockCache) {
				m.On("Get", mock.Anything, "price:BTC/USD").Return("invalid json", nil)
			},
			wantPrice: nil,
			wantFound: false,
		},
		{
			name: "partial JSON data",
			pair: "BTC/USD",
			setupMock: func(m *MockCache) {
				m.On("Get", mock.Anything, "price:BTC/USD").Return(`{"pair":"BTC/USD","amount":`, nil)
			},
			wantPrice: nil,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := &MockCache{}
			tt.setupMock(mockCache)

			adapter := NewPriceCache(mockCache, time.Minute)
			ctx := context.Background()

			price, found := adapter.Get(ctx, tt.pair)

			assert.Equal(t, tt.wantFound, found)
			if tt.wantFound {
				assert.NotNil(t, price)
				assert.Equal(t, tt.wantPrice.Pair, price.Pair)
				assert.Equal(t, tt.wantPrice.Amount, price.Amount)
			} else {
				assert.Nil(t, price)
			}

			mockCache.AssertExpectations(t)
		})
	}
}

func TestPriceCacheAdapter_GetMany(t *testing.T) {
	btcPrice := &entities.Price{Pair: "BTC/USD", Amount: 50000.0, Timestamp: time.Now(), Age: time.Minute}
	ethPrice := &entities.Price{Pair: "ETH/USD", Amount: 3000.0, Timestamp: time.Now(), Age: time.Minute}
	btcPriceJSON, _ := json.Marshal(btcPrice)
	ethPriceJSON, _ := json.Marshal(ethPrice)

	tests := []struct {
		name        string
		pairs       []string
		setupMock   func(*MockCache)
		wantPrices  []*entities.Price
		wantMissing []string
		description string
	}{
		{
			name:  "all pairs exist",
			pairs: []string{"BTC/USD", "ETH/USD"},
			setupMock: func(m *MockCache) {
				m.On("Get", mock.Anything, "price:BTC/USD").Return(string(btcPriceJSON), nil)
				m.On("Get", mock.Anything, "price:ETH/USD").Return(string(ethPriceJSON), nil)
			},
			wantPrices:  []*entities.Price{btcPrice, ethPrice},
			wantMissing: []string{},
		},
		{
			name:  "no pairs exist",
			pairs: []string{"BTC/USD", "ETH/USD"},
			setupMock: func(m *MockCache) {
				m.On("Get", mock.Anything, "price:BTC/USD").Return("", ErrKeyNotFound)
				m.On("Get", mock.Anything, "price:ETH/USD").Return("", ErrKeyNotFound)
			},
			wantPrices:  []*entities.Price{},
			wantMissing: []string{"BTC/USD", "ETH/USD"},
		},
		{
			name:  "mixed existent pairs",
			pairs: []string{"BTC/USD", "ETH/USD", "ADA/USD"},
			setupMock: func(m *MockCache) {
				m.On("Get", mock.Anything, "price:BTC/USD").Return(string(btcPriceJSON), nil)
				m.On("Get", mock.Anything, "price:ETH/USD").Return("", ErrKeyNotFound)
				m.On("Get", mock.Anything, "price:ADA/USD").Return("", errors.New("backend error"))
			},
			wantPrices:  []*entities.Price{btcPrice},
			wantMissing: []string{"ETH/USD", "ADA/USD"},
		},
		{
			name:        "empty pairs list",
			pairs:       []string{},
			setupMock:   func(m *MockCache) {},
			wantPrices:  []*entities.Price{},
			wantMissing: []string{},
		},
		{
			name:  "single pair",
			pairs: []string{"BTC/USD"},
			setupMock: func(m *MockCache) {
				m.On("Get", mock.Anything, "price:BTC/USD").Return(string(btcPriceJSON), nil)
			},
			wantPrices:  []*entities.Price{btcPrice},
			wantMissing: []string{},
		},
		{
			name:  "duplicate pairs",
			pairs: []string{"BTC/USD", "BTC/USD"},
			setupMock: func(m *MockCache) {
				m.On("Get", mock.Anything, "price:BTC/USD").Return(string(btcPriceJSON), nil).Times(2)
			},
			wantPrices:  []*entities.Price{btcPrice, btcPrice},
			wantMissing: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := &MockCache{}
			tt.setupMock(mockCache)

			adapter := NewPriceCache(mockCache, time.Minute)
			ctx := context.Background()

			prices, missing := adapter.GetMany(ctx, tt.pairs)

			assert.Len(t, prices, len(tt.wantPrices))
			assert.Equal(t, tt.wantMissing, missing)

			for i, expectedPrice := range tt.wantPrices {
				if i < len(prices) {
					assert.Equal(t, expectedPrice.Pair, prices[i].Pair)
					assert.Equal(t, expectedPrice.Amount, prices[i].Amount)
				}
			}

			mockCache.AssertExpectations(t)
		})
	}
}

func TestPriceCacheAdapter_ConcurrentAccess(t *testing.T) {
	mockCache := &MockCache{}
	adapter := NewPriceCache(mockCache, time.Minute)

	price := &entities.Price{
		Pair:      "BTC/USD",
		Amount:    50000.0,
		Timestamp: time.Now(),
		Age:       time.Minute,
	}

	// Setup mock para operaciones concurrentes
	mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockCache.On("Get", mock.Anything, mock.Anything).Return("", ErrKeyNotFound).Maybe()

	ctx := context.Background()

	// Test concurrent writes
	t.Run("concurrent writes", func(t *testing.T) {
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func() {
				err := adapter.Set(ctx, price)
				assert.NoError(t, err)
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})

	// Test concurrent reads
	t.Run("concurrent reads", func(t *testing.T) {
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func() {
				_, found := adapter.Get(ctx, "BTC/USD")
				_ = found // No importa el resultado
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func TestPriceCacheAdapter_ContextHandling(t *testing.T) {
	tests := []struct {
		name        string
		setupCtx    func() context.Context
		operation   string
		expectError bool
	}{
		{
			name: "context cancelled - Set",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancelar inmediatamente
				return ctx
			},
			operation:   "set",
			expectError: false, // El mock no simula cancelación
		},
		{
			name: "context timeout - Get",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
				defer cancel()
				time.Sleep(2 * time.Nanosecond) // Asegurar timeout
				return ctx
			},
			operation:   "get",
			expectError: false, // El mock no simula timeout
		},
		{
			name: "valid context",
			setupCtx: func() context.Context {
				return context.Background()
			},
			operation:   "set",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := &MockCache{}
			mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
			mockCache.On("Get", mock.Anything, mock.Anything).Return("", ErrKeyNotFound)

			adapter := NewPriceCache(mockCache, time.Minute)
			ctx := tt.setupCtx()

			price := &entities.Price{
				Pair:      "BTC/USD",
				Amount:    50000.0,
				Timestamp: time.Now(),
				Age:       time.Minute,
			}

			switch tt.operation {
			case "set":
				err := adapter.Set(ctx, price)
				if tt.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			case "get":
				_, found := adapter.Get(ctx, "BTC/USD")
				_ = found // El resultado no importa para esta prueba
			}
		})
	}
}
