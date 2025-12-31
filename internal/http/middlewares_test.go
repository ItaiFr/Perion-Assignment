package http

import (
	"Perion_Assignment/internal/mocks"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpMocks "Perion_Assignment/internal/http/mocks"
	"Perion_Assignment/internal/logger"
	"Perion_Assignment/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestLoggingMiddleware_Success(t *testing.T) {
	// Arrange
	mockLogger := &mocks.MockLogger{}

	// Create a simple test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify log event exists in context
		logEvent := logger.GetLogEvent(r.Context())
		assert.NotNil(t, logEvent)
		assert.Equal(t, models.ProcessTypeRequest, logEvent.ProcessType)
		assert.Equal(t, "192.168.1.1", logEvent.ClientIP)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Setup mock - expect HTTP request log
	mockLogger.On("LogInfo", mock.Anything, "http_request_start", "HTTP request received", mock.Anything).Return(); mockLogger.On("LogInfo", mock.Anything, "http_request_complete", "HTTP request processed", mock.MatchedBy(func(metadata map[string]interface{}) bool {
		return metadata["method"] == "GET" &&
			metadata["path"] == "/test" &&
			metadata["status_code"] == 200 &&
			metadata["client_ip"] == "192.168.1.1"
	})).Return()

	// Create middleware
	middleware := loggingMiddleware(mockLogger)
	handler := middleware(testHandler)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())

	// Verify mock
	mockLogger.AssertExpectations(t)
}

func TestLoggingMiddleware_WithHeaders(t *testing.T) {
	// Arrange
	mockLogger := &mocks.MockLogger{}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logEvent := logger.GetLogEvent(r.Context())
		assert.Equal(t, "10.0.0.5", logEvent.ClientIP) // From X-Forwarded-For
		w.WriteHeader(http.StatusCreated)
	})

	// Setup mock
	mockLogger.On("LogInfo", mock.Anything, "http_request_start", "HTTP request received", mock.Anything).Return(); mockLogger.On("LogInfo", mock.Anything, "http_request_complete", "HTTP request processed", mock.MatchedBy(func(metadata map[string]interface{}) bool {
		return metadata["method"] == "POST" &&
			metadata["status_code"] == 201 &&
			metadata["client_ip"] == "10.0.0.5" &&
			metadata["user_agent"] == "test-agent"
	})).Return()

	middleware := loggingMiddleware(mockLogger)
	handler := middleware(testHandler)

	// Create request with headers
	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.5, 192.168.1.1")
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "192.168.1.100:8080"
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusCreated, w.Code)
	mockLogger.AssertExpectations(t)
}

func TestCorsMiddleware_Preflight(t *testing.T) {
	// Arrange
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should not be called for OPTIONS request
		t.Error("Handler should not be called for OPTIONS request")
	})

	middleware := corsMiddleware()
	handler := middleware(testHandler)

	// Create OPTIONS request
	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization", w.Header().Get("Access-Control-Allow-Headers"))
}

func TestCorsMiddleware_RegularRequest(t *testing.T) {
	// Arrange
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	middleware := corsMiddleware()
	handler := middleware(testHandler)

	// Create regular request
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization", w.Header().Get("Access-Control-Allow-Headers"))
}

func TestRecoveryMiddleware_Panic(t *testing.T) {
	// Arrange
	mockLogger := &mocks.MockLogger{}

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Setup mock - expect panic log
	mockLogger.On("LogError", mock.Anything, "panic_recovery", "", "Panic recovered in HTTP handler", mock.MatchedBy(func(err error) bool {
		return err.Error() == "panic: test panic"
	}), models.LogSeverityHigh, mock.MatchedBy(func(metadata map[string]interface{}) bool {
		return metadata["panic"] == "test panic" &&
			metadata["path"] == "/test" &&
			metadata["method"] == "GET"
	})).Return()

	middleware := recoveryMiddleware(mockLogger)
	handler := middleware(panicHandler)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "internal server error", response["error"])
	assert.Equal(t, "An unexpected error occurred", response["message"])

	// Verify mock
	mockLogger.AssertExpectations(t)
}

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	// Arrange
	mockLogger := &mocks.MockLogger{}

	normalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	middleware := recoveryMiddleware(mockLogger)
	handler := middleware(normalHandler)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())

	// Verify no logging calls were made (no panic)
	mockLogger.AssertNotCalled(t, "LogError")
}

func TestRateLimitingMiddleware_Allowed(t *testing.T) {
	// Arrange
	mockRateLimiter := &httpMocks.MockRateLimiter{}
	mockLogger := &mocks.MockLogger{}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	clientIP := "192.168.1.1"

	// Setup mocks
	mockRateLimiter.On("Allow", clientIP).Return(true)

	middleware := rateLimitingMiddleware(mockRateLimiter, mockLogger)
	handler := middleware(testHandler)

	// Create request with context containing log event
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	logEvent := &models.LogEvent{
		ProcessID:   "test-123",
		ProcessType: models.ProcessTypeRequest,
		StartTime:   time.Now().UTC(),
		ClientIP:    clientIP,
	}
	ctx := logger.WithLogEvent(req.Context(), logEvent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())

	// Verify mocks
	mockRateLimiter.AssertExpectations(t)
	mockLogger.AssertNotCalled(t, "LogError") // No rate limiting error
}

func TestRateLimitingMiddleware_RateLimited(t *testing.T) {
	// Arrange
	mockRateLimiter := &httpMocks.MockRateLimiter{}
	mockLogger := &mocks.MockLogger{}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should not be called when rate limited
		t.Error("Handler should not be called when rate limited")
	})

	clientIP := "192.168.1.1"

	// Setup mocks
	mockRateLimiter.On("Allow", clientIP).Return(false)
	mockLogger.On("LogError", mock.Anything, "rate_limited", "", "Rate limit exceeded", models.ErrRateLimitExceeded, models.LogSeverityMedium, mock.MatchedBy(func(metadata map[string]interface{}) bool {
		return metadata["path"] == "/test" && metadata["method"] == "POST"
	})).Return()

	middleware := rateLimitingMiddleware(mockRateLimiter, mockLogger)
	handler := middleware(testHandler)

	// Create request with context containing log event
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	logEvent := &models.LogEvent{
		ProcessID:   "test-456",
		ProcessType: models.ProcessTypeRequest,
		StartTime:   time.Now().UTC(),
		ClientIP:    clientIP,
	}
	ctx := logger.WithLogEvent(req.Context(), logEvent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "1", w.Header().Get("X-RateLimit-Retry-After"))

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "rate limit exceeded", response["error"])
	assert.Equal(t, "Please try again later", response["message"])

	// Verify mocks
	mockRateLimiter.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	tests := []struct {
		name          string
		xForwardedFor string
		expectedIP    string
	}{
		{"single IP", "192.168.1.1", "192.168.1.1"},
		{"multiple IPs", "10.0.0.1, 192.168.1.1, 172.16.0.1", "10.0.0.1"},
		{"with spaces", "  203.0.113.195  , 192.168.1.1", "203.0.113.195"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			req.RemoteAddr = "192.168.1.100:8080"

			ip := getClientIP(req)
			assert.Equal(t, tt.expectedIP, ip)
		})
	}
}

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "203.0.113.195")
	req.RemoteAddr = "192.168.1.100:8080"

	ip := getClientIP(req)
	assert.Equal(t, "203.0.113.195", ip)
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100:8080"

	ip := getClientIP(req)
	assert.Equal(t, "192.168.1.100", ip)
}

func TestGetClientIP_RemoteAddrWithoutPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100"

	ip := getClientIP(req)
	assert.Equal(t, "192.168.1.100", ip) // Falls back to full RemoteAddr when SplitHostPort fails
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	// Arrange
	w := httptest.NewRecorder()
	wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

	// Act
	wrapped.WriteHeader(http.StatusCreated)

	// Assert
	assert.Equal(t, http.StatusCreated, wrapped.statusCode)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestResponseWriter_DefaultStatusCode(t *testing.T) {
	// Arrange
	w := httptest.NewRecorder()
	wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

	// Act - write without explicit WriteHeader
	_, _ = wrapped.Write([]byte("test"))

	// Assert
	assert.Equal(t, http.StatusOK, wrapped.statusCode) // Should remain default
	assert.Equal(t, http.StatusOK, w.Code)
}

// Integration test: multiple middlewares working together
func TestMiddlewareChain_Integration(t *testing.T) {
	// Arrange
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify log event exists
		logEvent := logger.GetLogEvent(r.Context())
		assert.NotNil(t, logEvent)
		assert.Equal(t, "192.168.1.1", logEvent.ClientIP)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	clientIP := "192.168.1.1"

	// Setup mocks
	mockLogger.On("LogInfo", mock.Anything, "http_request_start", "HTTP request received", mock.Anything).Return(); mockLogger.On("LogInfo", mock.Anything, "http_request_complete", "HTTP request processed", mock.MatchedBy(func(metadata map[string]interface{}) bool {
		return metadata["status_code"] == 200
	})).Return()
	mockRateLimiter.On("Allow", clientIP).Return(true)

	// Build middleware chain: logging -> cors -> recovery -> rate limiting -> handler
	handler := loggingMiddleware(mockLogger)(
		corsMiddleware()(
			recoveryMiddleware(mockLogger)(
				rateLimitingMiddleware(mockRateLimiter, mockLogger)(
					finalHandler,
				),
			),
		),
	)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/analyze/example.com", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))

	// Verify mocks
	mockLogger.AssertExpectations(t)
	mockRateLimiter.AssertExpectations(t)
}

func TestMiddlewareChain_WithPanicAndRateLimit(t *testing.T) {
	// Arrange
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("unexpected error")
	})

	clientIP := "192.168.1.1"

	// Setup mocks
	mockLogger.On("LogInfo", mock.Anything, "http_request_start", "HTTP request received", mock.Anything).Return(); mockLogger.On("LogInfo", mock.Anything, "http_request_complete", "HTTP request processed", mock.MatchedBy(func(metadata map[string]interface{}) bool {
		return metadata["status_code"] == 500 // Panic results in 500
	})).Return()
	mockRateLimiter.On("Allow", clientIP).Return(true)
	mockLogger.On("LogError", mock.Anything, "panic_recovery", "", "Panic recovered in HTTP handler", mock.AnythingOfType("*errors.errorString"), models.LogSeverityHigh, mock.Anything).Return()

	// Build middleware chain
	handler := loggingMiddleware(mockLogger)(
		rateLimitingMiddleware(mockRateLimiter, mockLogger)(
			recoveryMiddleware(mockLogger)(
				panicHandler,
			),
		),
	)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/api/batch-analysis", nil)
	req.RemoteAddr = "192.168.1.1:8080"
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "internal server error", response["error"])

	// Verify mocks
	mockLogger.AssertExpectations(t)
	mockRateLimiter.AssertExpectations(t)
}
