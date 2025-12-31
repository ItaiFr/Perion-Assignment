package ratelimit

import "context"

// Service defines the interface for rate limiting
// External packages should use this interface, not the concrete implementations
type Service interface {
	Allow(clientIP string) bool
	Wait(ctx context.Context, clientIP string) error
}