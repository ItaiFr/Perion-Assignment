package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"Perion_Assignment/internal/models"
)

// MemoryCache implements Service using in-memory storage
type MemoryCache struct {
	data  map[string]*cacheEntry
	mutex sync.RWMutex
}

// cacheEntry represents a single cache entry with expiration
type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache() Service {
	return newMemoryCache()
}

// newMemoryCache creates the concrete implementation
func newMemoryCache() *MemoryCache {
	cache := &MemoryCache{
		data: make(map[string]*cacheEntry),
	}
	
	// Start cleanup routine
	go cache.cleanupExpired()
	
	return cache
}

// Get retrieves a cached value for the given key
func (m *MemoryCache) Get(ctx context.Context, key string) (interface{}, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	entry, exists := m.data[key]
	if !exists {
		return nil, models.ErrCacheUnavailable
	}
	
	// Check if entry has expired
	if time.Now().After(entry.expiresAt) {
		// Remove expired entry (will be cleaned up by background routine)
		return nil, models.ErrCacheUnavailable
	}
	
	return entry.value, nil
}

// Set stores a value in the cache with the specified TTL
func (m *MemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if ttl <= 0 {
		return fmt.Errorf("TTL must be positive, got: %v", ttl)
	}
	
	m.data[key] = &cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	
	return nil
}

// Delete removes an entry from the cache
func (m *MemoryCache) Delete(ctx context.Context, key string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	delete(m.data, key)
	return nil
}

// cleanupExpired removes expired entries from the cache
func (m *MemoryCache) cleanupExpired() {
	ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	defer ticker.Stop()
	
	for range ticker.C {
		now := time.Now()
		
		m.mutex.Lock()
		for key, entry := range m.data {
			if now.After(entry.expiresAt) {
				delete(m.data, key)
			}
		}
		m.mutex.Unlock()
	}
}

// Size returns the current number of cached entries (for monitoring)
func (m *MemoryCache) Size() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.data)
}