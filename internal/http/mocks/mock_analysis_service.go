package mocks

import (
	"context"

	"Perion_Assignment/internal/models"

	"github.com/stretchr/testify/mock"
)

// MockAnalysisService is a mock implementation of domainAnalysis.AnalysisService
type MockAnalysisService struct {
	mock.Mock
}

// AnalyzeDomain mocks the AnalyzeDomain method of domainAnalysis.AnalysisService
func (m *MockAnalysisService) AnalyzeDomain(ctx context.Context, domain string) (*models.DomainAnalysis, error) {
	args := m.Called(ctx, domain)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DomainAnalysis), args.Error(1)
}

// AnalyzeDomains mocks the AnalyzeDomains method of domainAnalysis.AnalysisService
func (m *MockAnalysisService) AnalyzeDomains(ctx context.Context, domains []string) (*models.BatchAnalysisResponse, error) {
	args := m.Called(ctx, domains)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.BatchAnalysisResponse), args.Error(1)
}