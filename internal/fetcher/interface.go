package fetcher

import "context"

// Service defines the interface for fetching ads.txt files
// External packages should use this interface, not the concrete implementations
type Service interface {
	Fetch(ctx context.Context, domain string) (string, error)
}