package cache

import (
	"context"
	"time"
)

// Service defines the interface for generic caching operations
// External packages should use this interface, not the concrete implementations
type Service interface {
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}