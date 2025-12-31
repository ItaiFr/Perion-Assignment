package logger

import (
	"context"

	"Perion_Assignment/internal/models"
)

// Service defines the interface for application logging
// External packages should use this interface, not the concrete implementations
type Service interface {
	LogInfo(ctx context.Context, operation, message string, metadata map[string]interface{})
	LogSuccess(ctx context.Context, operation, targetName, message string, metadata map[string]interface{})
	LogError(ctx context.Context, operation, targetName, message string, err error, severity models.LogSeverity, metadata map[string]interface{})
	Close() error
}

// DatabaseConnection defines the interface for database operations used by logger implementations
type DatabaseConnection interface {
	InsertLog(ctx context.Context, entry *models.LogEntry) error
	Close() error
	Ping(ctx context.Context) error
}