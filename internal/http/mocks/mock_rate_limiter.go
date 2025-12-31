package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockRateLimiter is a mock implementation of ratelimit.Service
type MockRateLimiter struct {
	mock.Mock
}

// Allow mocks the Allow method of ratelimit.Service
func (m *MockRateLimiter) Allow(clientIP string) bool {
	args := m.Called(clientIP)
	return args.Bool(0)
}

// Wait mocks the Wait method of ratelimit.Service
func (m *MockRateLimiter) Wait(ctx context.Context, clientIP string) error {
	args := m.Called(ctx, clientIP)
	return args.Error(0)
}