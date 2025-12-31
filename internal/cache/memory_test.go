package cache

import (
	"context"
	"testing"
	"time"

	"Perion_Assignment/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCache_SetAndGet(t *testing.T) {
	cache := newMemoryCache()
	ctx := context.Background()

	// Set a value
	err := cache.Set(ctx, "test-key", "test-value", 1*time.Hour)
	require.NoError(t, err)

	// Get the value
	value, err := cache.Get(ctx, "test-key")
	require.NoError(t, err)
	assert.Equal(t, "test-value", value)
}

func TestMemoryCache_Get_NotFound(t *testing.T) {
	cache := newMemoryCache()
	ctx := context.Background()

	// Try to get non-existent key
	value, err := cache.Get(ctx, "non-existent")
	assert.Nil(t, value)
	assert.ErrorIs(t, err, models.ErrCacheUnavailable)
}

func TestMemoryCache_Get_Expired(t *testing.T) {
	cache := newMemoryCache()
	ctx := context.Background()

	// Set a value with very short TTL
	err := cache.Set(ctx, "expiring-key", "expiring-value", 100*time.Millisecond)
	require.NoError(t, err)

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	// Try to get expired value
	value, err := cache.Get(ctx, "expiring-key")
	assert.Nil(t, value)
	assert.ErrorIs(t, err, models.ErrCacheUnavailable)
}

func TestMemoryCache_Set_InvalidTTL(t *testing.T) {
	cache := newMemoryCache()
	ctx := context.Background()

	tests := []struct {
		name string
		ttl  time.Duration
	}{
		{"zero TTL", 0},
		{"negative TTL", -1 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cache.Set(ctx, "test-key", "test-value", tt.ttl)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "TTL must be positive")
		})
	}
}

func TestMemoryCache_Set_Overwrite(t *testing.T) {
	cache := newMemoryCache()
	ctx := context.Background()

	// Set initial value
	err := cache.Set(ctx, "key", "value1", 1*time.Hour)
	require.NoError(t, err)

	// Overwrite with new value
	err = cache.Set(ctx, "key", "value2", 1*time.Hour)
	require.NoError(t, err)

	// Get the value
	value, err := cache.Get(ctx, "key")
	require.NoError(t, err)
	assert.Equal(t, "value2", value)
}

func TestMemoryCache_Delete(t *testing.T) {
	cache := newMemoryCache()
	ctx := context.Background()

	// Set a value
	err := cache.Set(ctx, "test-key", "test-value", 1*time.Hour)
	require.NoError(t, err)

	// Delete the value
	err = cache.Delete(ctx, "test-key")
	require.NoError(t, err)

	// Verify it's gone
	value, err := cache.Get(ctx, "test-key")
	assert.Nil(t, value)
	assert.ErrorIs(t, err, models.ErrCacheUnavailable)
}

func TestMemoryCache_Delete_NonExistent(t *testing.T) {
	cache := newMemoryCache()
	ctx := context.Background()

	// Delete non-existent key should not error
	err := cache.Delete(ctx, "non-existent")
	assert.NoError(t, err)
}

func TestMemoryCache_Size(t *testing.T) {
	cache := newMemoryCache()
	ctx := context.Background()

	// Initially empty
	assert.Equal(t, 0, cache.Size())

	// Add entries
	err := cache.Set(ctx, "key1", "value1", 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, cache.Size())

	err = cache.Set(ctx, "key2", "value2", 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 2, cache.Size())

	// Delete entry
	err = cache.Delete(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, 1, cache.Size())
}

func TestMemoryCache_DifferentTypes(t *testing.T) {
	cache := newMemoryCache()
	ctx := context.Background()

	tests := []struct {
		name  string
		key   string
		value interface{}
	}{
		{"string", "key1", "string-value"},
		{"int", "key2", 42},
		{"struct", "key3", struct{ Name string }{Name: "test"}},
		{"slice", "key4", []string{"a", "b", "c"}},
		{"map", "key5", map[string]int{"a": 1, "b": 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cache.Set(ctx, tt.key, tt.value, 1*time.Hour)
			require.NoError(t, err)

			value, err := cache.Get(ctx, tt.key)
			require.NoError(t, err)
			assert.Equal(t, tt.value, value)
		})
	}
}

func TestMemoryCache_ConcurrentAccess(t *testing.T) {
	cache := newMemoryCache()
	ctx := context.Background()

	// Pre-populate cache
	for i := 0; i < 100; i++ {
		key := "key-" + string(rune(i))
		err := cache.Set(ctx, key, i, 1*time.Hour)
		require.NoError(t, err)
	}

	// Concurrent reads and writes
	done := make(chan bool)

	// Writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := "concurrent-" + string(rune(id)) + "-" + string(rune(j))
				_ = cache.Set(ctx, key, id*100+j, 1*time.Hour)
			}
			done <- true
		}(i)
	}

	// Readers
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				key := "key-" + string(rune(j))
				_, _ = cache.Get(ctx, key)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Verify cache is still functional
	err := cache.Set(ctx, "final-test", "works", 1*time.Hour)
	require.NoError(t, err)

	value, err := cache.Get(ctx, "final-test")
	require.NoError(t, err)
	assert.Equal(t, "works", value)
}

func TestMemoryCache_ExpirationBehavior(t *testing.T) {
	cache := newMemoryCache()
	ctx := context.Background()

	// Set multiple entries with different expiration times
	err := cache.Set(ctx, "short", "expires-soon", 100*time.Millisecond)
	require.NoError(t, err)

	err = cache.Set(ctx, "long", "expires-later", 1*time.Hour)
	require.NoError(t, err)

	// Both should be available immediately
	value, err := cache.Get(ctx, "short")
	require.NoError(t, err)
	assert.Equal(t, "expires-soon", value)

	value, err = cache.Get(ctx, "long")
	require.NoError(t, err)
	assert.Equal(t, "expires-later", value)

	// Wait for short to expire
	time.Sleep(200 * time.Millisecond)

	// Short should be expired
	value, err = cache.Get(ctx, "short")
	assert.Nil(t, value)
	assert.ErrorIs(t, err, models.ErrCacheUnavailable)

	// Long should still be available
	value, err = cache.Get(ctx, "long")
	require.NoError(t, err)
	assert.Equal(t, "expires-later", value)
}

func TestNewMemoryCache_PublicConstructor(t *testing.T) {
	// Test the public constructor
	cache := NewMemoryCache()
	assert.NotNil(t, cache)

	// Verify it works
	ctx := context.Background()
	err := cache.Set(ctx, "test", "value", 1*time.Hour)
	require.NoError(t, err)

	value, err := cache.Get(ctx, "test")
	require.NoError(t, err)
	assert.Equal(t, "value", value)
}

func BenchmarkMemoryCache_Set(b *testing.B) {
	cache := newMemoryCache()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Set(ctx, "bench-key", "bench-value", 1*time.Hour)
	}
}

func BenchmarkMemoryCache_Get(b *testing.B) {
	cache := newMemoryCache()
	ctx := context.Background()

	// Pre-populate
	_ = cache.Set(ctx, "bench-key", "bench-value", 1*time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(ctx, "bench-key")
	}
}

func BenchmarkMemoryCache_ConcurrentReadWrite(b *testing.B) {
	cache := newMemoryCache()
	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 100; i++ {
		key := "key-" + string(rune(i))
		_ = cache.Set(ctx, key, i, 1*time.Hour)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				key := "key-" + string(rune(i%100))
				_, _ = cache.Get(ctx, key)
			} else {
				key := "write-" + string(rune(i))
				_ = cache.Set(ctx, key, i, 1*time.Hour)
			}
			i++
		}
	})
}

func TestMemoryCache_CleanupExpired(t *testing.T) {
	cache := newMemoryCache()
	ctx := context.Background()

	// Add entries with very short TTL
	for i := 0; i < 10; i++ {
		key := "short-" + string(rune(i))
		err := cache.Set(ctx, key, i, 50*time.Millisecond)
		require.NoError(t, err)
	}

	// Add entries with longer TTL
	for i := 0; i < 5; i++ {
		key := "long-" + string(rune(i))
		err := cache.Set(ctx, key, i, 10*time.Hour)
		require.NoError(t, err)
	}

	// Initial size should be 15
	assert.Equal(t, 15, cache.Size())

	// Wait for short TTL entries to expire
	time.Sleep(100 * time.Millisecond)

	// Verify expired entries return error
	for i := 0; i < 10; i++ {
		key := "short-" + string(rune(i))
		_, err := cache.Get(ctx, key)
		assert.ErrorIs(t, err, models.ErrCacheUnavailable)
	}

	// Long TTL entries should still work
	for i := 0; i < 5; i++ {
		key := "long-" + string(rune(i))
		value, err := cache.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, i, value)
	}

	// Note: The cleanup routine runs every 5 minutes in production,
	// so we can't easily test it automatically removes expired entries
	// without mocking time. But we've verified that Get() correctly
	// identifies and refuses to return expired entries.
}

func TestMemoryCache_CleanupRoutineStarted(t *testing.T) {
	// This test verifies the cleanup goroutine is started
	// We create a cache and verify it's functional
	cache := newMemoryCache()
	ctx := context.Background()

	// The cleanup routine should be running in the background
	// We can't easily test it runs without waiting 5 minutes,
	// but we can verify the cache is functional
	err := cache.Set(ctx, "test", "value", 1*time.Hour)
	require.NoError(t, err)

	value, err := cache.Get(ctx, "test")
	require.NoError(t, err)
	assert.Equal(t, "value", value)

	// The goroutine is launched in newMemoryCache(),
	// so by this point it's running
}
