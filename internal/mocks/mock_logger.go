package mocks

import (
	"context"

	"Perion_Assignment/internal/models"

	"github.com/stretchr/testify/mock"
)

// MockLogger is a mock implementation of logger.Service
type MockLogger struct {
	mock.Mock
}

// LogInfo mocks the LogInfo method of logger.Service
func (m *MockLogger) LogInfo(ctx context.Context, operation, message string, metadata map[string]interface{}) {
	m.Called(ctx, operation, message, metadata)
}

// LogSuccess mocks the LogSuccess method of logger.Service
func (m *MockLogger) LogSuccess(ctx context.Context, operation, targetName, message string, metadata map[string]interface{}) {
	m.Called(ctx, operation, targetName, message, metadata)
}

// LogError mocks the LogError method of logger.Service
func (m *MockLogger) LogError(ctx context.Context, operation, targetName, message string, err error, severity models.LogSeverity, metadata map[string]interface{}) {
	m.Called(ctx, operation, targetName, message, err, severity, metadata)
}

// Close mocks the Close method of logger.Service
func (m *MockLogger) Close() error {
	args := m.Called()
	return args.Error(0)
}