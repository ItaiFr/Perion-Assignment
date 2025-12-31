package cache

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"Perion_Assignment/internal/models"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupMiniRedis creates a mini redis server for testing
func setupMiniRedis(t *testing.T) (*miniredis.Miniredis, *RedisCache) {
	mr := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cache := &RedisCache{
		client: client,
	}

	return mr, cache
}

func TestRedisCache_NewRedisCache_Success(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	redisURL := "redis://" + mr.Addr()
	cache, err := NewRedisCache(redisURL)

	require.NoError(t, err)
	assert.NotNil(t, cache)
}

func TestRedisCache_NewRedisCache_InvalidURL(t *testing.T) {
	cache, err := NewRedisCache("invalid://url::")

	assert.Error(t, err)
	assert.Nil(t, cache)
	assert.Contains(t, err.Error(), "failed to parse redis URL")
}

func TestRedisCache_NewRedisCache_ConnectionFailed(t *testing.T) {
	// Use invalid address that won't connect
	cache, err := NewRedisCache("redis://localhost:99999")

	assert.Error(t, err)
	assert.Nil(t, cache)
	assert.Contains(t, err.Error(), "failed to connect to redis")
}

func TestRedisCache_SetAndGet_String(t *testing.T) {
	mr, cache := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Set a string value
	err := cache.Set(ctx, "test-key", "test-value", 1*time.Hour)
	require.NoError(t, err)

	// Get the value
	value, err := cache.Get(ctx, "test-key")
	require.NoError(t, err)

	// Redis stores JSON, so we get back the JSON string
	assert.Equal(t, `"test-value"`, value)
}

func TestRedisCache_SetAndGet_Struct(t *testing.T) {
	mr, cache := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	type TestStruct struct {
		Name  string
		Count int
	}

	original := TestStruct{Name: "test", Count: 42}

	// Set the struct
	err := cache.Set(ctx, "struct-key", original, 1*time.Hour)
	require.NoError(t, err)

	// Get the value
	value, err := cache.Get(ctx, "struct-key")
	require.NoError(t, err)

	// Unmarshal the JSON
	var retrieved TestStruct
	err = json.Unmarshal([]byte(value.(string)), &retrieved)
	require.NoError(t, err)

	assert.Equal(t, original, retrieved)
}

func TestRedisCache_Get_NotFound(t *testing.T) {
	mr, cache := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Try to get non-existent key
	value, err := cache.Get(ctx, "non-existent")

	assert.Nil(t, value)
	assert.ErrorIs(t, err, models.ErrCacheUnavailable)
}

func TestRedisCache_Set_InvalidTTL(t *testing.T) {
	mr, cache := setupMiniRedis(t)
	defer mr.Close()

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

func TestRedisCache_Set_MarshalError(t *testing.T) {
	mr, cache := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Channels cannot be marshaled to JSON
	invalidValue := make(chan int)

	err := cache.Set(ctx, "test-key", invalidValue, 1*time.Hour)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal value")
}

func TestRedisCache_Set_Overwrite(t *testing.T) {
	mr, cache := setupMiniRedis(t)
	defer mr.Close()

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
	assert.Equal(t, `"value2"`, value)
}

func TestRedisCache_Delete(t *testing.T) {
	mr, cache := setupMiniRedis(t)
	defer mr.Close()

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

func TestRedisCache_Delete_NonExistent(t *testing.T) {
	mr, cache := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Delete non-existent key should not error
	err := cache.Delete(ctx, "non-existent")
	assert.NoError(t, err)
}

func TestRedisCache_TTL_Expiration(t *testing.T) {
	mr, cache := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Set a value with short TTL
	err := cache.Set(ctx, "expiring-key", "expiring-value", 1*time.Second)
	require.NoError(t, err)

	// Value should be available immediately
	value, err := cache.Get(ctx, "expiring-key")
	require.NoError(t, err)
	assert.Equal(t, `"expiring-value"`, value)

	// Fast-forward time in miniredis
	mr.FastForward(2 * time.Second)

	// Value should be expired
	value, err = cache.Get(ctx, "expiring-key")
	assert.Nil(t, value)
	assert.ErrorIs(t, err, models.ErrCacheUnavailable)
}

func TestRedisCache_DifferentTypes(t *testing.T) {
	mr, cache := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	tests := []struct {
		name  string
		key   string
		value interface{}
	}{
		{"string", "key1", "string-value"},
		{"int", "key2", 42},
		{"float", "key3", 3.14},
		{"bool", "key4", true},
		{"slice", "key5", []string{"a", "b", "c"}},
		{"map", "key6", map[string]int{"a": 1, "b": 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cache.Set(ctx, tt.key, tt.value, 1*time.Hour)
			require.NoError(t, err)

			value, err := cache.Get(ctx, tt.key)
			require.NoError(t, err)

			// Unmarshal and verify
			var retrieved interface{}
			switch tt.value.(type) {
			case string:
				var s string
				err = json.Unmarshal([]byte(value.(string)), &s)
				require.NoError(t, err)
				retrieved = s
			case int:
				var i int
				err = json.Unmarshal([]byte(value.(string)), &i)
				require.NoError(t, err)
				retrieved = i
			case float64:
				var f float64
				err = json.Unmarshal([]byte(value.(string)), &f)
				require.NoError(t, err)
				retrieved = f
			case bool:
				var b bool
				err = json.Unmarshal([]byte(value.(string)), &b)
				require.NoError(t, err)
				retrieved = b
			case []string:
				var sl []string
				err = json.Unmarshal([]byte(value.(string)), &sl)
				require.NoError(t, err)
				retrieved = sl
			case map[string]int:
				var m map[string]int
				err = json.Unmarshal([]byte(value.(string)), &m)
				require.NoError(t, err)
				retrieved = m
			}

			assert.Equal(t, tt.value, retrieved)
		})
	}
}

func TestRedisCache_Close(t *testing.T) {
	mr, cache := setupMiniRedis(t)
	defer mr.Close()

	// Close the cache
	err := cache.Close()
	assert.NoError(t, err)
}

func TestRedisCache_ContextCancellation(t *testing.T) {
	mr, cache := setupMiniRedis(t)
	defer mr.Close()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Operations should fail with context error
	err := cache.Set(ctx, "test-key", "test-value", 1*time.Hour)
	assert.Error(t, err)

	_, err = cache.Get(ctx, "test-key")
	assert.Error(t, err)
}

func TestRedisCache_MultipleOperations(t *testing.T) {
	mr, cache := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Set multiple keys
	for i := 0; i < 10; i++ {
		key := "key-" + string(rune(i))
		err := cache.Set(ctx, key, i, 1*time.Hour)
		require.NoError(t, err)
	}

	// Retrieve all keys
	for i := 0; i < 10; i++ {
		key := "key-" + string(rune(i))
		value, err := cache.Get(ctx, key)
		require.NoError(t, err)

		var retrieved int
		err = json.Unmarshal([]byte(value.(string)), &retrieved)
		require.NoError(t, err)
		assert.Equal(t, i, retrieved)
	}

	// Delete half
	for i := 0; i < 5; i++ {
		key := "key-" + string(rune(i))
		err := cache.Delete(ctx, key)
		require.NoError(t, err)
	}

	// Verify deletions
	for i := 0; i < 5; i++ {
		key := "key-" + string(rune(i))
		value, err := cache.Get(ctx, key)
		assert.Nil(t, value)
		assert.ErrorIs(t, err, models.ErrCacheUnavailable)
	}

	// Verify remaining keys still exist
	for i := 5; i < 10; i++ {
		key := "key-" + string(rune(i))
		value, err := cache.Get(ctx, key)
		require.NoError(t, err)
		assert.NotNil(t, value)
	}
}

func BenchmarkRedisCache_Set(b *testing.B) {
	mr := miniredis.RunT(b)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cache := &RedisCache{
		client: client,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Set(ctx, "bench-key", "bench-value", 1*time.Hour)
	}
}

func BenchmarkRedisCache_Get(b *testing.B) {
	mr := miniredis.RunT(b)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cache := &RedisCache{
		client: client,
	}

	ctx := context.Background()

	// Pre-populate
	_ = cache.Set(ctx, "bench-key", "bench-value", 1*time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(ctx, "bench-key")
	}
}

func TestRedisCache_Delete_WithError(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cache := &RedisCache{
		client: client,
	}

	ctx := context.Background()

	// Set a key
	err := cache.Set(ctx, "test-key", "test-value", 1*time.Hour)
	require.NoError(t, err)

	// Close the miniredis server to force error
	mr.Close()

	// Try to delete - should get error
	err = cache.Delete(ctx, "test-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis delete failed")
}

func TestRedisCache_Get_WithError(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cache := &RedisCache{
		client: client,
	}

	ctx := context.Background()

	// Close the miniredis server to force error
	mr.Close()

	// Try to get - should get error (not cache unavailable)
	_, err := cache.Get(ctx, "test-key")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, models.ErrCacheUnavailable)
	assert.Contains(t, err.Error(), "redis get failed")
}

func TestRedisCache_Set_WithError(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cache := &RedisCache{
		client: client,
	}

	ctx := context.Background()

	// Close the miniredis server to force error
	mr.Close()

	// Try to set - should get error
	err := cache.Set(ctx, "test-key", "test-value", 1*time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis set failed")
}
