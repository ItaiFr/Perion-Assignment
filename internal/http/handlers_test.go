package http

import (
	"Perion_Assignment/internal/mocks"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	httpMocks "Perion_Assignment/internal/http/mocks"
	"Perion_Assignment/internal/models"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHandler_AnalyzeSingleDomain_Success(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}

	handler := NewHandler(mockAnalysisService, mockLogger)

	domain := "example.com"
	expectedAnalysis := &models.DomainAnalysis{
		Domain:           domain,
		TotalAdvertisers: 2,
		Advertisers: []models.AdvertiserInfo{
			{Domain: "google.com", Count: 2},
		},
		Cached:    false,
		Timestamp: time.Now().UTC(),
	}

	// Setup mocks
	mockLogger.On("LogInfo", mock.Anything, "domain_analysis", mock.AnythingOfType("string"), mock.Anything).Return()
	mockAnalysisService.On("AnalyzeDomain", mock.Anything, domain).Return(expectedAnalysis, nil)
	mockLogger.On("LogSuccess", mock.Anything, "domain_analysis", domain, "Successfully analyzed domain", mock.Anything).Return()

	// Create request with Gorilla Mux context
	req := httptest.NewRequest(http.MethodGet, "/api/analyze/"+domain, nil)
	req = mux.SetURLVars(req, map[string]string{"domain": domain})

	w := httptest.NewRecorder()

	// Act
	handler.AnalyzeSingleDomain(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response models.DomainAnalysis
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, domain, response.Domain)
	assert.Equal(t, 2, response.TotalAdvertisers)
	assert.False(t, response.Cached)
	assert.Len(t, response.Advertisers, 1)

	// Verify mocks
	mockAnalysisService.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestHandler_AnalyzeSingleDomain_MissingDomain(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}

	handler := NewHandler(mockAnalysisService, mockLogger)

	// Create request without domain parameter
	req := httptest.NewRequest(http.MethodGet, "/api/analyze/", nil)
	req = mux.SetURLVars(req, map[string]string{}) // Empty vars

	w := httptest.NewRecorder()

	// Act
	handler.AnalyzeSingleDomain(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "domain is required", response.Error)
	assert.Empty(t, response.Message)

	// Verify no service calls were made
	mockAnalysisService.AssertNotCalled(t, "AnalyzeDomain")
}

func TestHandler_AnalyzeSingleDomain_ServiceError(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}

	handler := NewHandler(mockAnalysisService, mockLogger)

	domain := "example.com"
	serviceError := models.NewDomainError(domain, "failed to fetch ads.txt", errors.New("network timeout"))

	// Setup mocks
	mockLogger.On("LogInfo", mock.Anything, "domain_analysis", mock.AnythingOfType("string"), mock.Anything).Return()
	mockAnalysisService.On("AnalyzeDomain", mock.Anything, domain).Return(nil, serviceError)
	mockLogger.On("LogError", mock.Anything, "domain_analysis", domain, "Domain analysis failed", serviceError, models.LogSeverityMedium, mock.Anything).Return()

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/analyze/"+domain, nil)
	req = mux.SetURLVars(req, map[string]string{"domain": domain})

	w := httptest.NewRecorder()

	// Act
	handler.AnalyzeSingleDomain(w, req)

	// Assert
	assert.Equal(t, http.StatusRequestTimeout, w.Code) // timeout error maps to 408
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "analysis failed", response.Error)
	assert.Contains(t, response.Message, "failed to fetch ads.txt")

	// Verify mocks
	mockAnalysisService.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestHandler_AnalyzeBatchDomains_Success(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}

	handler := NewHandler(mockAnalysisService, mockLogger)

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
	mockLogger.On("LogInfo", mock.Anything, "batch_analysis", mock.AnythingOfType("string"), mock.Anything).Return()
	mockAnalysisService.On("AnalyzeDomains", mock.Anything, domains).Return(expectedResponse, nil)
	mockLogger.On("LogSuccess", mock.Anything, "batch_analysis", "", mock.AnythingOfType("string"), mock.Anything).Return()

	// Create request
	bodyBytes, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/batch-analysis", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	// Act
	handler.AnalyzeBatchDomains(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code) // All succeeded = 200
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response models.BatchAnalysisResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 2, response.Summary.Total)
	assert.Equal(t, 2, response.Summary.Succeeded)
	assert.Equal(t, 0, response.Summary.Failed)
	assert.Equal(t, 3, response.TotalAdvertisers)

	// Verify mocks
	mockAnalysisService.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestHandler_AnalyzeBatchDomains_PartialSuccess(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}

	handler := NewHandler(mockAnalysisService, mockLogger)

	domains := []string{"example.com", "fail.com"}
	requestBody := models.BatchAnalysisRequest{
		Domains: domains,
	}

	expectedResponse := &models.BatchAnalysisResponse{
		Results: []models.DomainResult{
			{Domain: "example.com", Success: true, TotalAdvertisers: 2, Cached: false},
			{Domain: "fail.com", Success: false, Error: "failed to fetch ads.txt", Cached: false},
		},
		Summary: models.BatchSummary{
			Total:     2,
			Succeeded: 1,
			Failed:    1,
		},
		Advertisers: []models.AdvertiserInfo{
			{Domain: "google.com", Count: 2},
		},
		TotalAdvertisers: 2,
		Timestamp:        time.Now().UTC(),
	}

	// Setup mocks
	mockLogger.On("LogInfo", mock.Anything, "batch_analysis", mock.AnythingOfType("string"), mock.Anything).Return()
	mockAnalysisService.On("AnalyzeDomains", mock.Anything, domains).Return(expectedResponse, nil)
	mockLogger.On("LogSuccess", mock.Anything, "batch_analysis", "", mock.AnythingOfType("string"), mock.Anything).Return()

	// Create request
	bodyBytes, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/batch-analysis", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	// Act
	handler.AnalyzeBatchDomains(w, req)

	// Assert
	assert.Equal(t, http.StatusMultiStatus, w.Code) // Partial success = 207
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response models.BatchAnalysisResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 2, response.Summary.Total)
	assert.Equal(t, 1, response.Summary.Succeeded)
	assert.Equal(t, 1, response.Summary.Failed)

	// Verify mocks
	mockAnalysisService.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestHandler_AnalyzeBatchDomains_InvalidJSON(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}

	handler := NewHandler(mockAnalysisService, mockLogger)

	// Setup mocks
	mockLogger.On("LogError", mock.Anything, "batch_analysis", "", "Invalid request body", mock.AnythingOfType("*json.SyntaxError"), models.LogSeverityLow, mock.Anything).Return()

	// Create request with invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/api/batch-analysis", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	// Act
	handler.AnalyzeBatchDomains(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "invalid request body", response.Error)

	// Verify no service calls were made
	mockAnalysisService.AssertNotCalled(t, "AnalyzeDomains")
	mockLogger.AssertExpectations(t)
}

func TestHandler_AnalyzeBatchDomains_EmptyDomains(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}

	handler := NewHandler(mockAnalysisService, mockLogger)

	requestBody := models.BatchAnalysisRequest{
		Domains: []string{}, // Empty array
	}

	// Create request
	bodyBytes, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/batch-analysis", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	// Act
	handler.AnalyzeBatchDomains(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "domains array cannot be empty", response.Error)

	// Verify no service calls were made
	mockAnalysisService.AssertNotCalled(t, "AnalyzeDomains")
}

func TestHandler_AnalyzeBatchDomains_TooManyDomains(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}

	handler := NewHandler(mockAnalysisService, mockLogger)

	// Create request with 101 domains (exceeds limit of 100)
	domains := make([]string, 101)
	for i := 0; i < 101; i++ {
		domains[i] = "example" + string(rune(i)) + ".com"
	}

	requestBody := models.BatchAnalysisRequest{
		Domains: domains,
	}

	// Create request
	bodyBytes, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/batch-analysis", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	// Act
	handler.AnalyzeBatchDomains(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "too many domains", response.Error)
	assert.Equal(t, "Maximum 100 domains per batch", response.Message)

	// Verify no service calls were made
	mockAnalysisService.AssertNotCalled(t, "AnalyzeDomains")
}

func TestHandler_HealthCheck_Success(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}

	handler := NewHandler(mockAnalysisService, mockLogger)

	// Setup mocks
	mockLogger.On("LogInfo", mock.Anything, "health_check", "Health check performed successfully", mock.Anything).Return()

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	// Act
	handler.HealthCheck(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response.Status)
	assert.Equal(t, "1.0.0", response.Version)
	assert.WithinDuration(t, time.Now().UTC(), response.Timestamp, 5*time.Second)

	// Verify mocks
	mockLogger.AssertExpectations(t)
}

func TestHandler_getStatusCodeForError(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		name           string
		error          string
		expectedStatus int
	}{
		{"not found", "resource not found", http.StatusNotFound},
		{"404 error", "HTTP 404 error", http.StatusNotFound},
		{"timeout", "request timeout occurred", http.StatusRequestTimeout},
		{"invalid domain", "invalid domain format", http.StatusBadRequest},
		{"rate limit", "rate limit exceeded", http.StatusTooManyRequests},
		{"generic error", "something went wrong", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.error)
			statusCode := handler.getStatusCodeForError(err)
			assert.Equal(t, tt.expectedStatus, statusCode)
		})
	}
}

func TestHandler_getBatchStatusCode(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		name           string
		succeeded      int
		failed         int
		expectedStatus int
	}{
		{"all success", 5, 0, http.StatusOK},
		{"all failed", 0, 3, http.StatusBadRequest},
		{"partial success", 3, 2, http.StatusMultiStatus},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &models.BatchAnalysisResponse{
				Summary: models.BatchSummary{
					Succeeded: tt.succeeded,
					Failed:    tt.failed,
				},
			}
			statusCode := handler.getBatchStatusCode(response)
			assert.Equal(t, tt.expectedStatus, statusCode)
		})
	}
}

// Test with context that includes log event (simulating middleware)
func TestHandler_AnalyzeSingleDomain_WithLogEvent(t *testing.T) {
	// Arrange
	mockAnalysisService := &httpMocks.MockAnalysisService{}
	mockLogger := &mocks.MockLogger{}

	handler := NewHandler(mockAnalysisService, mockLogger)

	domain := "example.com"
	expectedAnalysis := &models.DomainAnalysis{
		Domain:           domain,
		TotalAdvertisers: 1,
		Advertisers:      []models.AdvertiserInfo{{Domain: "google.com", Count: 1}},
		Cached:           true,
		Timestamp:        time.Now().UTC(),
	}

	// Create context with log event (simulating middleware)
	logEvent := &models.LogEvent{
		ProcessID:   "test-process-123",
		ProcessType: models.ProcessTypeRequest,
		StartTime:   time.Now().UTC(),
		ClientIP:    "192.168.1.1",
	}
	ctx := context.WithValue(context.Background(), "logEvent", logEvent)

	// Setup mocks - use mock.Anything for context since mux.SetURLVars modifies it
	mockLogger.On("LogInfo", mock.Anything, "domain_analysis", mock.AnythingOfType("string"), mock.Anything).Return()
	mockAnalysisService.On("AnalyzeDomain", mock.Anything, domain).Return(expectedAnalysis, nil)
	mockLogger.On("LogSuccess", mock.Anything, "domain_analysis", domain, "Successfully analyzed domain", mock.Anything).Return()

	// Create request with context
	req := httptest.NewRequest(http.MethodGet, "/api/analyze/"+domain, nil)
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"domain": domain})

	w := httptest.NewRecorder()

	// Act
	handler.AnalyzeSingleDomain(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.DomainAnalysis
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, domain, response.Domain)
	assert.True(t, response.Cached)

	// Verify mocks
	mockAnalysisService.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}
