package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockFetcher is a mock implementation of fetcher.Service
type MockFetcher struct {
	mock.Mock
}

// Fetch mocks the Fetch method of fetcher.Service
func (m *MockFetcher) Fetch(ctx context.Context, domain string) (string, error) {
	args := m.Called(ctx, domain)
	return args.String(0), args.Error(1)
}