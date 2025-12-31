package domainCache

import (
	"context"
	"time"

	"Perion_Assignment/internal/models"
)

// Service defines the interface for domain analysis cache operations
type Service interface {
	Get(ctx context.Context, domain string) (*models.DomainAnalysis, error)
	Set(ctx context.Context, domain string, analysis *models.DomainAnalysis, ttl time.Duration) error
	Delete(ctx context.Context, domain string) error
}