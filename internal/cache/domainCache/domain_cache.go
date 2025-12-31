package domainCache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"Perion_Assignment/internal/cache"
	"Perion_Assignment/internal/models"
)

// domainCache implements Service using a generic cache
type domainCache struct {
	cache cache.Service
	ttl   time.Duration
}

// New creates a new domain analysis cache
func New(cache cache.Service, ttl time.Duration) Service {
	return &domainCache{
		cache: cache,
		ttl:   ttl,
	}
}

// Get retrieves a domain analysis from the cache
func (d *domainCache) Get(ctx context.Context, domain string) (*models.DomainAnalysis, error) {
	cacheKey := fmt.Sprintf("domain:%s", domain)
	value, err := d.cache.Get(ctx, cacheKey)
	if err != nil {
		return nil, err
	}
	
	// Handle type conversion
	switch v := value.(type) {
	case *models.DomainAnalysis:
		// Memory cache returns the actual object
		return v, nil
	case models.DomainAnalysis:
		// Handle value type
		return &v, nil
	case string:
		// Redis cache returns JSON string, unmarshal it
		var analysis models.DomainAnalysis
		if err := json.Unmarshal([]byte(v), &analysis); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cached domain analysis: %w", err)
		}
		return &analysis, nil
	default:
		return nil, fmt.Errorf("unexpected type in cache: %T", v)
	}
}

// Set stores a domain analysis in the cache
func (d *domainCache) Set(ctx context.Context, domain string, analysis *models.DomainAnalysis, ttl time.Duration) error {
	cacheKey := fmt.Sprintf("domain:%s", domain)
	
	// Use provided TTL or default from domainCache
	cacheTTL := ttl
	if cacheTTL == 0 {
		cacheTTL = d.ttl
	}
	
	return d.cache.Set(ctx, cacheKey, analysis, cacheTTL)
}

// Delete removes a domain analysis from the cache
func (d *domainCache) Delete(ctx context.Context, domain string) error {
	cacheKey := fmt.Sprintf("domain:%s", domain)
	return d.cache.Delete(ctx, cacheKey)
}