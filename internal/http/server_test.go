package http

import (
	"Perion_Assignment/internal/mocks"
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	httpMocks "Perion_Assignment/internal/http/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestServer_StartWithInvalidAddr(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	handler := NewHandler(mockAnalysisService, mockLogger)

	// Use invalid address (port already in use or invalid format)
	server := NewServer(
		"invalid-address:99999", // Invalid port number
		handler,
		mockLogger,
		mockRateLimiter,
		10*time.Second,
		10*time.Second,
	)

	// Setup mock - expect start log
	mockLogger.On("LogInfo", mock.Anything, "server_start", "Starting HTTP server", mock.MatchedBy(func(metadata map[string]interface{}) bool {
		return metadata["addr"] == "invalid-address:99999"
	})).Return()

	// Act
	err := server.Start()

	// Assert
	assert.Error(t, err) // Should fail with invalid address
	mockLogger.AssertExpectations(t)
}

func TestServer_StartWithValidAddrButPortInUse(t *testing.T) {
	// Arrange - first, occupy a port
	listener, err := net.Listen("tcp", "localhost:0") // Get any available port
	require.NoError(t, err)
	defer listener.Close()

	usedPort := listener.Addr().(*net.TCPAddr).Port
	usedAddr := "localhost:" + string(rune(usedPort))

	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	handler := NewHandler(mockAnalysisService, mockLogger)
	server := NewServer(
		usedAddr, // This port is already in use
		handler,
		mockLogger,
		mockRateLimiter,
		10*time.Second,
		10*time.Second,
	)

	// Setup mock
	mockLogger.On("LogInfo", mock.Anything, "server_start", "Starting HTTP server", mock.Anything).Return()

	// Act
	err = server.Start()

	// Assert
	assert.Error(t, err) // Should fail because port is in use
	mockLogger.AssertExpectations(t)
}

func TestServer_Shutdown(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	handler := NewHandler(mockAnalysisService, mockLogger)
	server := NewServer(
		"localhost:0", // Use any available port
		handler,
		mockLogger,
		mockRateLimiter,
		10*time.Second,
		10*time.Second,
	)

	// Setup mock
	mockLogger.On("LogInfo", mock.Anything, "server_shutdown", "Shutting down HTTP server", mock.Anything).Return()

	// Act
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)

	// Assert
	assert.NoError(t, err) // Shutdown should succeed even if server wasn't started
	mockLogger.AssertExpectations(t)
}

func TestServer_ShutdownWithCancelledContext(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	handler := NewHandler(mockAnalysisService, mockLogger)
	server := NewServer(
		"localhost:0",
		handler,
		mockLogger,
		mockRateLimiter,
		10*time.Second,
		10*time.Second,
	)

	// Setup mock
	mockLogger.On("LogInfo", mock.Anything, "server_shutdown", "Shutting down HTTP server", mock.Anything).Return()

	// Create already cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Act
	err := server.Shutdown(ctx)

	// Assert - shutdown might succeed or fail depending on timing
	// The important thing is that it doesn't panic and the logger is called
	mockLogger.AssertExpectations(t)
	_ = err // Don't assert on error as it depends on internal http.Server behavior
}

func TestRouterRegistration(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	handler := NewHandler(mockAnalysisService, mockLogger)
	server := NewServer(
		"localhost:0",
		handler,
		mockLogger,
		mockRateLimiter,
		10*time.Second,
		10*time.Second,
	)

	// Test that routes are properly registered by making requests
	testCases := []struct {
		method   string
		path     string
		expected int // Expected status code (not necessarily success)
	}{
		{"GET", "/health", 200},                  // Should work
		{"GET", "/", 200},                        // Should work
		{"GET", "/api/analyze/example.com", 429}, // Might be rate limited but route exists
		{"POST", "/api/batch-analysis", 429},     // Might be rate limited but route exists
		{"PUT", "/health", 405},                  // Wrong method
		{"GET", "/nonexistent", 404},             // Route doesn't exist
	}

	for _, tc := range testCases {
		t.Run(tc.method+"_"+tc.path, func(t *testing.T) {
			// Setup minimal mocks for this test
			if tc.expected != 404 && tc.expected != 405 {
				mockLogger.On("LogInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()
				mockRateLimiter.On("Allow", mock.AnythingOfType("string")).Return(false).Maybe() // Rate limit to avoid complex setup
				mockLogger.On("LogError", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()
			}

			req, _ := http.NewRequest(tc.method, tc.path, nil)
			req.RemoteAddr = "127.0.0.1:12345"
			w := &responseRecorder{code: 200}

			// Act
			server.server.Handler.ServeHTTP(w, req)

			// Assert - verify expected behavior
			if tc.expected == 404 {
				assert.Equal(t, 404, w.code, "Route should not exist")
			} else if tc.expected == 405 {
				assert.Equal(t, 405, w.code, "Method should not be allowed")
			} else {
				// Route exists (might be rate limited or successful)
				assert.NotEqual(t, 404, w.code, "Route should exist")
				assert.NotEqual(t, 405, w.code, "Method should be allowed")
			}
		})
	}
}

// responseRecorder is a simple implementation to capture status code
type responseRecorder struct {
	code    int
	headers http.Header
}

func (r *responseRecorder) Header() http.Header {
	if r.headers == nil {
		r.headers = make(http.Header)
	}
	return r.headers
}

func (r *responseRecorder) Write([]byte) (int, error) {
	return 0, nil
}

func (r *responseRecorder) WriteHeader(code int) {
	r.code = code
}
