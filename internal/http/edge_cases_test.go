package http

import (
	"Perion_Assignment/internal/mocks"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpMocks "Perion_Assignment/internal/http/mocks"
	"Perion_Assignment/internal/logger"
	"Perion_Assignment/internal/models"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// failingResponseWriter simulates a ResponseWriter that fails on Write
type failingResponseWriter struct {
	http.ResponseWriter
	failOnWrite bool
	headers     http.Header
}

func (f *failingResponseWriter) Header() http.Header {
	if f.headers == nil {
		f.headers = make(http.Header)
	}
	return f.headers
}

func (f *failingResponseWriter) Write([]byte) (int, error) {
	if f.failOnWrite {
		return 0, errors.New("write failed")
	}
	return f.ResponseWriter.Write([]byte{})
}

func (f *failingResponseWriter) WriteHeader(code int) {
	if f.ResponseWriter != nil {
		f.ResponseWriter.WriteHeader(code)
	}
}

func TestWriteErrorResponse_EncodingFailure(t *testing.T) {
	// This test is tricky because writeErrorResponse doesn't return errors
	// We need to test that it doesn't panic when JSON encoding fails
	// But JSON encoding of ErrorResponse should rarely fail...

	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}

	handler := NewHandler(mockAnalysisService, mockLogger)

	// Create a ResponseWriter that fails on Write
	baseW := httptest.NewRecorder()
	failingW := &failingResponseWriter{
		ResponseWriter: baseW,
		failOnWrite:    true,
	}

	// Expect the encoding error to be logged when the response write fails
	mockLogger.On("LogError", mock.Anything, "response_encoding", "", "Failed to encode error response", mock.AnythingOfType("*errors.errorString"), models.LogSeverityLow, mock.Anything).Return()

	// Act - trigger writeErrorResponse through a handler
	req := httptest.NewRequest(http.MethodGet, "/api/analyze/", nil)

	// Add LogEvent to context (needed for X-Request-ID header)
	logEvent := logger.NewRequestLogEvent("192.168.1.1")
	ctx := logger.WithLogEvent(req.Context(), logEvent)
	req = req.WithContext(ctx)

	req = mux.SetURLVars(req, map[string]string{}) // Empty domain triggers error

	// This should call writeErrorResponse internally
	handler.AnalyzeSingleDomain(failingW, req)

	// Assert - main thing is that it doesn't panic
	// The error response write will fail, but the handler should handle it gracefully
	assert.True(t, true) // If we get here, no panic occurred
	mockLogger.AssertExpectations(t)
}

func TestGetClientIP_MalformedRemoteAddr(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		expected   string
	}{
		{
			name:       "malformed address without port",
			remoteAddr: "192.168.1.1",
			expected:   "192.168.1.1", // Should fallback to full address
		},
		{
			name:       "empty address",
			remoteAddr: "",
			expected:   "", // Should return empty
		},
		{
			name:       "malformed with extra colons",
			remoteAddr: "192.168.1.1:8080:extra",
			expected:   "192.168.1.1:8080:extra", // Should return full address on parse error
		},
		{
			name:       "IPv6 address",
			remoteAddr: "[::1]:8080",
			expected:   "::1", // Should extract IPv6 correctly
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr

			ip := getClientIP(req)
			assert.Equal(t, tt.expected, ip)
		})
	}
}

func TestLoggingMiddleware_ContextEdgeCases(t *testing.T) {
	// Arrange
	mockLogger := &mocks.MockLogger{}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify log event exists even with edge cases
		logEvent := logger.GetLogEvent(r.Context())
		assert.NotNil(t, logEvent)
		w.WriteHeader(http.StatusOK)
	})

	// Setup mock - expect both start and complete log calls
	mockLogger.On("LogInfo", mock.Anything, "http_request_start", "HTTP request received", mock.Anything).Return()
	mockLogger.On("LogInfo", mock.Anything, "http_request_complete", "HTTP request processed", mock.Anything).Return()

	middleware := loggingMiddleware(mockLogger)
	handler := middleware(testHandler)

	// Test with already cancelled context
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel() // Cancel before request
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	mockLogger.AssertExpectations(t)
}

func TestRateLimitingMiddleware_NoLogEventInContext(t *testing.T) {
	// Arrange
	mockRateLimiter := &httpMocks.MockRateLimiter{}
	mockLogger := &mocks.MockLogger{}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Setup mock - even without log event, rate limiter should be called with some IP
	mockRateLimiter.On("Allow", mock.AnythingOfType("string")).Return(true)

	middleware := rateLimitingMiddleware(mockRateLimiter, mockLogger)
	handler := middleware(testHandler)

	// Create request WITHOUT log event in context (edge case)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	// Act - this should handle the case where GetLogEvent returns nil/default
	handler.ServeHTTP(w, req)

	// Assert - should work even without proper log event
	assert.Equal(t, http.StatusOK, w.Code)
	mockRateLimiter.AssertExpectations(t)
}

func TestHandlers_CancelledContext(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}

	handler := NewHandler(mockAnalysisService, mockLogger)

	domain := "example.com"

	// Setup mocks
	mockLogger.On("LogInfo", mock.Anything, "domain_analysis", mock.AnythingOfType("string"), mock.Anything).Return()
	mockAnalysisService.On("AnalyzeDomain", mock.Anything, domain).Return(nil, context.Canceled)
	mockLogger.On("LogError", mock.Anything, "domain_analysis", domain, "Domain analysis failed", context.Canceled, models.LogSeverityMedium, mock.Anything).Return()

	// Create request with cancelled context
	req := httptest.NewRequest(http.MethodGet, "/api/analyze/"+domain, nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel() // Cancel context
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"domain": domain})

	w := httptest.NewRecorder()

	// Act
	handler.AnalyzeSingleDomain(w, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code) // Context cancelled maps to 500

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "analysis failed", response.Error)

	mockAnalysisService.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestBatchHandlers_CancelledContext(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}

	handler := NewHandler(mockAnalysisService, mockLogger)

	domains := []string{"example.com"}
	requestBody := models.BatchAnalysisRequest{
		Domains: domains,
	}

	// Setup mocks
	mockLogger.On("LogInfo", mock.Anything, "batch_analysis", mock.AnythingOfType("string"), mock.Anything).Return()
	mockAnalysisService.On("AnalyzeDomains", mock.Anything, domains).Return(nil, context.Canceled)
	mockLogger.On("LogError", mock.Anything, "batch_analysis", "", "Batch analysis failed", context.Canceled, models.LogSeverityMedium, mock.Anything).Return()

	// Create request with cancelled context
	bodyBytes, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/batch-analysis", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	ctx, cancel := context.WithCancel(req.Context())
	cancel() // Cancel context
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	// Act
	handler.AnalyzeBatchDomains(w, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "batch analysis failed", response.Error)

	mockAnalysisService.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestResponseWriter_EdgeCases(t *testing.T) {
	// Test responseWriter with multiple WriteHeader calls
	baseW := httptest.NewRecorder()
	wrapped := &responseWriter{ResponseWriter: baseW, statusCode: http.StatusOK}

	// First WriteHeader call
	wrapped.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusCreated, wrapped.statusCode)

	// Second WriteHeader call (should be ignored by http package)
	wrapped.WriteHeader(http.StatusBadRequest)
	// Our wrapper will update, but the underlying ResponseWriter will ignore
	assert.Equal(t, http.StatusBadRequest, wrapped.statusCode)
	assert.Equal(t, http.StatusCreated, baseW.Code) // Underlying should keep first value
}

func TestMiddlewareChain_ComplexErrorScenario(t *testing.T) {
	// Test middleware chain with multiple error conditions
	// Arrange
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	// Handler that panics AND has rate limiting issues
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("multiple errors test")
	})

	clientIP := "192.168.1.1"

	// Setup mocks
	mockLogger.On("LogInfo", mock.Anything, "http_request_start", "HTTP request received", mock.Anything).Return(); mockLogger.On("LogInfo", mock.Anything, "http_request_complete", "HTTP request processed", mock.Anything).Return()
	mockRateLimiter.On("Allow", clientIP).Return(true) // Allow, but handler will panic
	mockLogger.On("LogError", mock.Anything, "panic_recovery", "", "Panic recovered in HTTP handler", mock.AnythingOfType("*errors.errorString"), models.LogSeverityHigh, mock.Anything).Return()

	// Build middleware chain
	handler := loggingMiddleware(mockLogger)(
		rateLimitingMiddleware(mockRateLimiter, mockLogger)(
			corsMiddleware()(
				recoveryMiddleware(mockLogger)(
					panicHandler,
				),
			),
		),
	)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:8080"
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert - panic should be recovered, request should be logged
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "internal server error", response["error"])

	// Verify mocks
	mockLogger.AssertExpectations(t)
	mockRateLimiter.AssertExpectations(t)
}

func TestGetClientIP_IPv6EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		expected   string
	}{
		{
			name:       "IPv6 with port",
			remoteAddr: "[2001:db8::1]:8080",
			expected:   "2001:db8::1",
		},
		{
			name:       "IPv6 localhost with port",
			remoteAddr: "[::1]:8080",
			expected:   "::1",
		},
		{
			name:       "malformed IPv6",
			remoteAddr: "[2001:db8::1",
			expected:   "[2001:db8::1", // Should fallback to full address
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr

			ip := getClientIP(req)
			assert.Equal(t, tt.expected, ip)
		})
	}
}

func TestServer_NewServerConfiguration(t *testing.T) {
	// Test that NewServer properly configures all components
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	handler := NewHandler(mockAnalysisService, mockLogger)

	readTimeout := 5 * time.Second
	writeTimeout := 10 * time.Second
	addr := "localhost:8080"

	server := NewServer(
		addr,
		handler,
		mockLogger,
		mockRateLimiter,
		readTimeout,
		writeTimeout,
	)

	// Verify server configuration
	assert.Equal(t, addr, server.server.Addr)
	assert.Equal(t, readTimeout, server.server.ReadTimeout)
	assert.Equal(t, writeTimeout, server.server.WriteTimeout)
	assert.NotNil(t, server.server.Handler)
	assert.Equal(t, handler, server.handler)
	assert.Equal(t, mockLogger, server.logger)
}
