package mocks

import (
	"context"
	"time"

	"Perion_Assignment/internal/models"

	"github.com/stretchr/testify/mock"
)

// MockDomainCache is a mock implementation of domainCache.Service
type MockDomainCache struct {
	mock.Mock
}

// Get mocks the Get method of domainCache.Service
func (m *MockDomainCache) Get(ctx context.Context, domain string) (*models.DomainAnalysis, error) {
	args := m.Called(ctx, domain)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DomainAnalysis), args.Error(1)
}

// Set mocks the Set method of domainCache.Service
func (m *MockDomainCache) Set(ctx context.Context, domain string, analysis *models.DomainAnalysis, ttl time.Duration) error {
	args := m.Called(ctx, domain, analysis, ttl)
	return args.Error(0)
}

// Delete mocks the Delete method of domainCache.Service
func (m *MockDomainCache) Delete(ctx context.Context, domain string) error {
	args := m.Called(ctx, domain)
	return args.Error(0)
}