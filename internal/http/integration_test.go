package http

import (
	"Perion_Assignment/internal/mocks"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpMocks "Perion_Assignment/internal/http/mocks"
	"Perion_Assignment/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// createTestServer creates a test server with all dependencies mocked
func createTestServer(t *testing.T, mockAnalysisService *httpMocks.MockAnalysisService, mockLogger *mocks.MockLogger, mockRateLimiter *httpMocks.MockRateLimiter) *Server {
	handler := NewHandler(mockAnalysisService, mockLogger)

	// Create server with test timeouts
	return NewServer(
		"localhost:0", // Random port for testing
		handler,
		mockLogger,
		mockRateLimiter,
		10*time.Second, // readTimeout
		10*time.Second, // writeTimeout
	)
}

func TestIntegration_HealthCheck(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	server := createTestServer(t, mockAnalysisService, mockLogger, mockRateLimiter)

	// Setup mocks - expect middleware calls
	mockLogger.On("LogInfo", mock.Anything, "http_request_start", "HTTP request received", mock.Anything).Return(); mockLogger.On("LogInfo", mock.Anything, "http_request_complete", "HTTP request processed", mock.Anything).Return()
	mockRateLimiter.On("Allow", mock.AnythingOfType("string")).Return(true)
	mockLogger.On("LogInfo", mock.Anything, "health_check", "Health check performed successfully", mock.Anything).Return()

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	// Act
	server.server.Handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))

	var response HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "healthy", response.Status)
	assert.Equal(t, "1.0.0", response.Version)

	// Verify mocks
	mockLogger.AssertExpectations(t)
	mockRateLimiter.AssertExpectations(t)
}

func TestIntegration_RootEndpoint(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	server := createTestServer(t, mockAnalysisService, mockLogger, mockRateLimiter)

	// Setup mocks
	mockLogger.On("LogInfo", mock.Anything, "http_request_start", "HTTP request received", mock.Anything).Return(); mockLogger.On("LogInfo", mock.Anything, "http_request_complete", "HTTP request processed", mock.Anything).Return()
	mockRateLimiter.On("Allow", mock.AnythingOfType("string")).Return(true)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	// Act
	server.server.Handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "AdsTxt Analysis API", response["message"])
	assert.Equal(t, "1.0.0", response["version"])
	assert.NotNil(t, response["endpoints"])

	// Verify mocks
	mockLogger.AssertExpectations(t)
	mockRateLimiter.AssertExpectations(t)
}

func TestIntegration_AnalyzeSingleDomain_Success(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	server := createTestServer(t, mockAnalysisService, mockLogger, mockRateLimiter)

	domain := "example.com"
	expectedAnalysis := &models.DomainAnalysis{
		Domain:           domain,
		TotalAdvertisers: 3,
		Advertisers: []models.AdvertiserInfo{
			{Domain: "google.com", Count: 2},
			{Domain: "facebook.com", Count: 1},
		},
		Cached:    true,
		Timestamp: time.Now().UTC(),
	}

	// Setup mocks
	mockLogger.On("LogInfo", mock.Anything, "http_request_start", "HTTP request received", mock.Anything).Return(); mockLogger.On("LogInfo", mock.Anything, "http_request_complete", "HTTP request processed", mock.Anything).Return()
	mockRateLimiter.On("Allow", mock.AnythingOfType("string")).Return(true)
	mockLogger.On("LogInfo", mock.Anything, "domain_analysis", mock.AnythingOfType("string"), mock.Anything).Return()
	mockAnalysisService.On("AnalyzeDomain", mock.Anything, domain).Return(expectedAnalysis, nil)
	mockLogger.On("LogSuccess", mock.Anything, "domain_analysis", domain, "Successfully analyzed domain", mock.Anything).Return()

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/analyze/"+domain, nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	// Act
	server.server.Handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))

	var response models.DomainAnalysis
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, domain, response.Domain)
	assert.Equal(t, 3, response.TotalAdvertisers)
	assert.True(t, response.Cached)
	assert.Len(t, response.Advertisers, 2)

	// Verify mocks
	mockAnalysisService.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
	mockRateLimiter.AssertExpectations(t)
}

func TestIntegration_AnalyzeSingleDomain_RateLimited(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	server := createTestServer(t, mockAnalysisService, mockLogger, mockRateLimiter)

	domain := "example.com"

	// Setup mocks - rate limiter denies request
	mockLogger.On("LogInfo", mock.Anything, "http_request_start", "HTTP request received", mock.Anything).Return(); mockLogger.On("LogInfo", mock.Anything, "http_request_complete", "HTTP request processed", mock.Anything).Return()
	mockRateLimiter.On("Allow", mock.AnythingOfType("string")).Return(false)
	mockLogger.On("LogError", mock.Anything, "rate_limited", "", "Rate limit exceeded", models.ErrRateLimitExceeded, models.LogSeverityMedium, mock.Anything).Return()

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/analyze/"+domain, nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	// Act
	server.server.Handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "1", w.Header().Get("X-RateLimit-Retry-After"))

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "rate limit exceeded", response.Error)
	assert.Equal(t, "Please try again later", response.Message)

	// Verify analysis service was not called
	mockAnalysisService.AssertNotCalled(t, "AnalyzeDomain")
	mockLogger.AssertExpectations(t)
	mockRateLimiter.AssertExpectations(t)
}

func TestIntegration_AnalyzeBatchDomains_Success(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	server := createTestServer(t, mockAnalysisService, mockLogger, mockRateLimiter)

	domains := []string{"example.com", "test.com"}
	requestBody := models.BatchAnalysisRequest{
		Domains: domains,
	}

	expectedResponse := &models.BatchAnalysisResponse{
		Results: []models.DomainResult{
			{Domain: "example.com", Success: true, TotalAdvertisers: 2, Cached: false},
			{Domain: "test.com", Success: true, TotalAdvertisers: 1, Cached: true},
		},
		Summary: models.BatchSummary{
			Total:     2,
			Succeeded: 2,
			Failed:    0,
		},
		Advertisers: []models.AdvertiserInfo{
			{Domain: "google.com", Count: 3},
		},
		TotalAdvertisers: 3,
		Timestamp:        time.Now().UTC(),
	}

	// Setup mocks
	mockLogger.On("LogInfo", mock.Anything, "http_request_start", "HTTP request received", mock.Anything).Return(); mockLogger.On("LogInfo", mock.Anything, "http_request_complete", "HTTP request processed", mock.Anything).Return()
	mockRateLimiter.On("Allow", mock.AnythingOfType("string")).Return(true)
	mockLogger.On("LogInfo", mock.Anything, "batch_analysis", mock.AnythingOfType("string"), mock.Anything).Return()
	mockAnalysisService.On("AnalyzeDomains", mock.Anything, domains).Return(expectedResponse, nil)
	mockLogger.On("LogSuccess", mock.Anything, "batch_analysis", "", mock.AnythingOfType("string"), mock.Anything).Return()

	// Create request
	bodyBytes, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/batch-analysis", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	// Act
	server.server.Handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code) // All succeeded
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))

	var response models.BatchAnalysisResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 2, response.Summary.Total)
	assert.Equal(t, 2, response.Summary.Succeeded)
	assert.Equal(t, 0, response.Summary.Failed)

	// Verify mocks
	mockAnalysisService.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
	mockRateLimiter.AssertExpectations(t)
}

func TestIntegration_CorsHeaders(t *testing.T) {
	// Test that CORS headers are applied to regular requests
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	server := createTestServer(t, mockAnalysisService, mockLogger, mockRateLimiter)

	// Setup mocks for middleware
	mockLogger.On("LogInfo", mock.Anything, "http_request_start", "HTTP request received", mock.Anything).Return(); mockLogger.On("LogInfo", mock.Anything, "http_request_complete", "HTTP request processed", mock.Anything).Return()
	mockRateLimiter.On("Allow", mock.AnythingOfType("string")).Return(true)
	mockLogger.On("LogInfo", mock.Anything, "health_check", "Health check performed successfully", mock.Anything).Return()

	// Create regular GET request
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	// Act
	server.server.Handler.ServeHTTP(w, req)

	// Assert CORS headers are applied
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization", w.Header().Get("Access-Control-Allow-Headers"))

	// Verify mocks
	mockLogger.AssertExpectations(t)
	mockRateLimiter.AssertExpectations(t)
}

func TestIntegration_InvalidRoute(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	server := createTestServer(t, mockAnalysisService, mockLogger, mockRateLimiter)

	// Create request to invalid route
	req := httptest.NewRequest(http.MethodGet, "/invalid/route", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	// Act
	server.server.Handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Verify no analysis service calls for 404
	mockAnalysisService.AssertNotCalled(t, "AnalyzeDomain")
	mockAnalysisService.AssertNotCalled(t, "AnalyzeDomains")
}

func TestIntegration_InvalidMethod(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	server := createTestServer(t, mockAnalysisService, mockLogger, mockRateLimiter)

	// Create POST request to GET-only endpoint
	req := httptest.NewRequest(http.MethodPost, "/api/analyze/example.com", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	// Act
	server.server.Handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

	// Verify no analysis service calls for wrong method
	mockAnalysisService.AssertNotCalled(t, "AnalyzeDomain")
}

func TestIntegration_WithDifferentClientIPs(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expectedIP string
	}{
		{
			name:       "X-Forwarded-For header",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.195"},
			remoteAddr: "192.168.1.1:8080",
			expectedIP: "203.0.113.195",
		},
		{
			name:       "X-Real-IP header",
			headers:    map[string]string{"X-Real-IP": "203.0.113.200"},
			remoteAddr: "192.168.1.1:8080",
			expectedIP: "203.0.113.200",
		},
		{
			name:       "RemoteAddr fallback",
			headers:    map[string]string{},
			remoteAddr: "10.0.0.5:12345",
			expectedIP: "10.0.0.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockAnalysisService := &httpMocks.MockAnalysisService{}
			mockLogger := &mocks.MockLogger{}
			mockRateLimiter := &httpMocks.MockRateLimiter{}

			server := createTestServer(t, mockAnalysisService, mockLogger, mockRateLimiter)

			// Setup mocks - verify the correct IP is passed to rate limiter
			mockLogger.On("LogInfo", mock.Anything, "http_request_start", "HTTP request received", mock.Anything).Return(); mockLogger.On("LogInfo", mock.Anything, "http_request_complete", "HTTP request processed", mock.Anything).Return()
			mockRateLimiter.On("Allow", tt.expectedIP).Return(true)
			mockLogger.On("LogInfo", mock.Anything, "health_check", "Health check performed successfully", mock.Anything).Return()

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}
			req.RemoteAddr = tt.remoteAddr
			w := httptest.NewRecorder()

			// Act
			server.server.Handler.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, http.StatusOK, w.Code)

			// Verify mocks (especially that correct IP was used)
			mockLogger.AssertExpectations(t)
			mockRateLimiter.AssertExpectations(t)
		})
	}
}

func TestIntegration_ErrorScenarios(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	server := createTestServer(t, mockAnalysisService, mockLogger, mockRateLimiter)

	domain := "example.com"
	serviceError := models.NewDomainError(domain, "failed to fetch ads.txt", models.ErrRateLimitExceeded)

	// Setup mocks
	mockLogger.On("LogInfo", mock.Anything, "http_request_start", "HTTP request received", mock.Anything).Return(); mockLogger.On("LogInfo", mock.Anything, "http_request_complete", "HTTP request processed", mock.Anything).Return()
	mockRateLimiter.On("Allow", mock.AnythingOfType("string")).Return(true)
	mockLogger.On("LogInfo", mock.Anything, "domain_analysis", mock.AnythingOfType("string"), mock.Anything).Return()
	mockAnalysisService.On("AnalyzeDomain", mock.Anything, domain).Return(nil, serviceError)
	mockLogger.On("LogError", mock.Anything, "domain_analysis", domain, "Domain analysis failed", serviceError, models.LogSeverityMedium, mock.Anything).Return()

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/analyze/"+domain, nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	// Act
	server.server.Handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusTooManyRequests, w.Code) // Rate limit error maps to 429
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "analysis failed", response.Error)

	// Verify mocks
	mockAnalysisService.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
	mockRateLimiter.AssertExpectations(t)
}

// Test that middleware order is correct (logging -> rate limiting -> cors -> recovery)
func TestIntegration_MiddlewareOrder(t *testing.T) {
	// This test verifies middleware order by checking that rate limiting happens
	// before reaching the handler but after logging

	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}
	mockRateLimiter := &httpMocks.MockRateLimiter{}

	server := createTestServer(t, mockAnalysisService, mockLogger, mockRateLimiter)

	// Setup mocks - rate limiter denies request
	mockLogger.On("LogInfo", mock.Anything, "http_request_start", "HTTP request received", mock.Anything).Return(); mockLogger.On("LogInfo", mock.Anything, "http_request_complete", "HTTP request processed", mock.Anything).Return()
	mockRateLimiter.On("Allow", mock.AnythingOfType("string")).Return(false)
	mockLogger.On("LogError", mock.Anything, "rate_limited", "", "Rate limit exceeded", models.ErrRateLimitExceeded, models.LogSeverityMedium, mock.Anything).Return()

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/analyze/example.com", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	// Act
	server.server.Handler.ServeHTTP(w, req)

	// Assert - rate limited response
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Verify that handler was never called due to rate limiting
	mockAnalysisService.AssertNotCalled(t, "AnalyzeDomain")

	// Verify mocks
	mockLogger.AssertExpectations(t)
	mockRateLimiter.AssertExpectations(t)
}
