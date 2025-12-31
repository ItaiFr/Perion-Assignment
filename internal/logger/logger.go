package logger

import (
	"context"
	"fmt"
	"time"

	"Perion_Assignment/internal/models"

	"github.com/google/uuid"
)

// DatabaseLogger implements the Service interface using a database backend
type DatabaseLogger struct {
	db DatabaseConnection
}

// NewDatabaseLogger creates a new database logger
func NewDatabaseLogger(db DatabaseConnection) Service {
	return &DatabaseLogger{
		db: db,
	}
}

// LogInfo logs an informational message (no severity)
func (l *DatabaseLogger) LogInfo(ctx context.Context, operation, message string, metadata map[string]interface{}) {
	l.logEntry(ctx, "", operation, "", message, nil, metadata)
}

// LogSuccess logs a successful operation (no severity)
func (l *DatabaseLogger) LogSuccess(ctx context.Context, operation, targetName, message string, metadata map[string]interface{}) {
	l.logEntry(ctx, "", operation, targetName, message, nil, metadata)
}

// LogError logs an error with required severity
func (l *DatabaseLogger) LogError(ctx context.Context, operation, targetName, message string, err error, severity models.LogSeverity, metadata map[string]interface{}) {
	l.logEntry(ctx, severity, operation, targetName, message, err, metadata)
}

// logEntry is the internal method that creates and stores log entries
func (l *DatabaseLogger) logEntry(ctx context.Context, severity models.LogSeverity, operation, targetName, message string, err error, metadata map[string]interface{}) {
	// Get log event from context
	logEvent := GetLogEvent(ctx)

	entry := &models.LogEntry{
		ID:          uuid.New().String(),
		Timestamp:   time.Now().UTC(),
		Severity:    severity,
		Message:     message,
		Operation:   operation,
		TargetName:  targetName,
		ProcessID:   logEvent.ProcessID,
		ProcessType: logEvent.ProcessType,
		ClientIP:    logEvent.ClientIP,
		Metadata:    metadata,
	}

	// Add error details if present
	if err != nil {
		entry.Error = err.Error()
	}

	// Insert log entry asynchronously to avoid blocking the main flow
	go func() {
		// Create a new context with a reasonable timeout for logging
		logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := l.db.InsertLog(logCtx, entry); err != nil {
			// If logging fails, we should handle it gracefully
			// In production, you might want to fall back to file logging or stdout
			fmt.Printf("Failed to insert log entry: %v\n", err)
		}
	}()
}

// Close closes the logger and its database connection
func (l *DatabaseLogger) Close() error {
	return l.db.Close()
}

// LogOperations defines constants for common operations
const (
	OpDomainAnalysis = "domain_analysis"
	OpBatchAnalysis  = "batch_analysis"
	OpCacheHit       = "cache_hit"
	OpCacheMiss      = "cache_miss"
	OpRateLimited    = "rate_limited"
	OpFetchAdsTxt    = "fetch_ads_txt"
	OpParseAdsTxt    = "parse_ads_txt"
	OpServerStart    = "server_start"
	OpServerShutdown = "server_shutdown"
	OpHealthCheck    = "health_check"
)
