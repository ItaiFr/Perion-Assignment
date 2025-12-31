package logger

import (
	"context"
	"time"

	"Perion_Assignment/internal/models"
	"github.com/google/uuid"
)

// contextKey is used for context values to avoid collisions
type contextKey string

const logEventKey contextKey = "log_event"

// NewLogEvent creates a new log event for process tracking
func NewLogEvent(processType models.ProcessType, clientIP string) *models.LogEvent {
	return &models.LogEvent{
		ProcessID:   uuid.New().String(),
		ProcessType: processType,
		StartTime:   time.Now().UTC(),
		ClientIP:    clientIP,
	}
}

// WithLogEvent adds a log event to the context
func WithLogEvent(ctx context.Context, logEvent *models.LogEvent) context.Context {
	return context.WithValue(ctx, logEventKey, logEvent)
}

// GetLogEvent retrieves the log event from context
func GetLogEvent(ctx context.Context) *models.LogEvent {
	if logEvent := ctx.Value(logEventKey); logEvent != nil {
		if le, ok := logEvent.(*models.LogEvent); ok {
			return le
		}
	}
	// Return default internal process if no log event found
	return NewLogEvent(models.ProcessTypeInternal, "")
}

// NewRequestLogEvent creates a log event for HTTP requests
func NewRequestLogEvent(clientIP string) *models.LogEvent {
	return NewLogEvent(models.ProcessTypeRequest, clientIP)
}

// NewInternalLogEvent creates a log event for internal processes  
func NewInternalLogEvent() *models.LogEvent {
	return NewLogEvent(models.ProcessTypeInternal, "")
}