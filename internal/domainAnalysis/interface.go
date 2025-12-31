package domainAnalysis

import (
	"context"

	"Perion_Assignment/internal/models"
)

// AnalysisService defines the interface for domain analysis operations
// External packages should use this interface, not the concrete implementations
type AnalysisService interface {
	AnalyzeDomain(ctx context.Context, domain string) (*models.DomainAnalysis, error)
	AnalyzeDomains(ctx context.Context, domains []string) (*models.BatchAnalysisResponse, error)
}