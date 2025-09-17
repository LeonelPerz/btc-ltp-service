package cache

import (
	"btc-ltp-service/internal/domain/interfaces"
	"context"
	"sync"
	"time"
)

// cacheItem representa un elemento en el cache con su valor y tiempo de expiración
type cacheItem struct {
	value     string
	expiresAt time.Time
}

// isExpired verifica si el item ha expirado
func (item *cacheItem) isExpired() bool {
	return time.Now().After(item.expiresAt)
}

// MemoryCache implementa la interfaz Cache usando memoria local
type MemoryCache struct {
	items map[string]*cacheItem
	mu    sync.RWMutex
}

// NewMemoryCache crea una nueva instancia de cache en memoria
func NewMemoryCache() interfaces.Cache {
	return &MemoryCache{
		items: make(map[string]*cacheItem),
	}
}

// Get obtiene un valor del cache
func (c *MemoryCache) Get(ctx context.Context, key string) (string, error) {
	c.mu.RLock()
	item, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		return "", ErrKeyNotFound
	}

	if item.isExpired() {
		// Eliminar clave expirada para evitar fuga de memoria
		_ = c.Delete(ctx, key)
		return "", ErrKeyExpired
	}

	return item.value, nil
}

// Set almacena un valor en el cache con TTL y realiza una limpieza ligera de expirados
func (c *MemoryCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Limpieza rápida de expirados para evitar crecimiento sin control
	now := time.Now()
	for k, item := range c.items {
		if now.After(item.expiresAt) {
			delete(c.items, k)
		}
	}

	expiresAt := now.Add(ttl)
	c.items[key] = &cacheItem{
		value:     value,
		expiresAt: expiresAt,
	}

	return nil
}

// Delete elimina un valor del cache
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	return nil
}

// Size retorna el número de elementos en el cache (método auxiliar para debugging)
func (c *MemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Cleanup elimina elementos expirados del cache (método auxiliar)
func (c *MemoryCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, item := range c.items {
		if now.After(item.expiresAt) {
			delete(c.items, key)
		}
	}
}
