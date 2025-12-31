package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"Perion_Assignment/internal/models"
	"github.com/redis/go-redis/v9"
)

// RedisCache implements Service using Redis
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new Redis-based cache
func NewRedisCache(redisURL string) (Service, error) {
	return newRedisCache(redisURL)
}

// newRedisCache creates the concrete implementation
func newRedisCache(redisURL string) (*RedisCache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}
	
	client := redis.NewClient(opts)
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}
	
	return &RedisCache{
		client: client,
	}, nil
}

// Get retrieves a cached value for the given key
func (r *RedisCache) Get(ctx context.Context, key string) (interface{}, error) {
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, models.ErrCacheUnavailable
		}
		return nil, fmt.Errorf("redis get failed: %w", err)
	}
	
	// Return the raw JSON string, let the domain layer handle unmarshaling
	return data, nil
}

// Set stores a value in Redis with the specified TTL
func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl <= 0 {
		return fmt.Errorf("TTL must be positive, got: %v", ttl)
	}
	
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}
	
	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("redis set failed: %w", err)
	}
	
	return nil
}

// Delete removes an entry from Redis
func (r *RedisCache) Delete(ctx context.Context, key string) error {
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis delete failed: %w", err)
	}
	return nil
}

// Close closes the Redis connection
func (r *RedisCache) Close() error {
	return r.client.Close()
}