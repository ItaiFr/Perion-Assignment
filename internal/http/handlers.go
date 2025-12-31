package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"Perion_Assignment/internal/domainAnalysis"
	"Perion_Assignment/internal/logger"
	"Perion_Assignment/internal/models"
	"github.com/gorilla/mux"
)

// Handler contains the HTTP handlers for the API
type Handler struct {
	analysisService domainAnalysis.AnalysisService
	logger          logger.Service
}

// NewHandler creates a new HTTP handler
func NewHandler(
	analysisService domainAnalysis.AnalysisService,
	logger logger.Service,
) *Handler {
	return &Handler{
		analysisService: analysisService,
		logger:          logger,
	}
}


// ErrorResponse represents an error response
type ErrorResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

// writeJSONResponse writes a JSON response with standard headers including X-Request-ID
func (h *Handler) writeJSONResponse(w http.ResponseWriter, r *http.Request, statusCode int, data interface{}) error {
	// Extract LogEvent from context to get ProcessID for X-Request-ID header
	logEvent := logger.GetLogEvent(r.Context())

	// Set headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", logEvent.ProcessID)
	w.WriteHeader(statusCode)

	// Encode and send response
	return json.NewEncoder(w).Encode(data)
}

// AnalyzeSingleDomain handles GET /api/analyze/{domain}
func (h *Handler) AnalyzeSingleDomain(w http.ResponseWriter, r *http.Request) {
	// LogEvent is automatically created by logging middleware
	ctx := r.Context()

	// Extract domain from URL path parameters
	vars := mux.Vars(r)
	domain := vars["domain"]
	if domain == "" {
		h.writeErrorResponse(w, r, http.StatusBadRequest, "domain is required", "")
		return
	}

	h.logger.LogInfo(ctx, logger.OpDomainAnalysis, fmt.Sprintf("Starting analysis for domain: %s", domain), map[string]interface{}{
		"domain": domain,
	})

	// Perform analysis
	analysis, err := h.analysisService.AnalyzeDomain(ctx, domain)
	if err != nil {
		h.logger.LogError(ctx, logger.OpDomainAnalysis, domain, "Domain analysis failed", err, models.LogSeverityMedium, nil)

		// Determine appropriate status code based on error type
		statusCode := h.getStatusCodeForError(err)
		h.writeErrorResponse(w, r, statusCode, "analysis failed", err.Error())
		return
	}

	// Write successful response using centralized function
	if err := h.writeJSONResponse(w, r, http.StatusOK, analysis); err != nil {
		// Response already sent with 200, but log the encoding error
		h.logger.LogError(ctx, logger.OpDomainAnalysis, domain, "Failed to encode response", err, models.LogSeverityLow, nil)
		return
	}

	// Log success only after successful encoding
	h.logger.LogSuccess(ctx, logger.OpDomainAnalysis, domain, "Successfully analyzed domain", nil)
}

// AnalyzeBatchDomains handles POST /api/batch-analysis
func (h *Handler) AnalyzeBatchDomains(w http.ResponseWriter, r *http.Request) {
	// LogEvent is automatically created by logging middleware
	ctx := r.Context()


	// Parse request body
	var request models.BatchAnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.logger.LogError(ctx, logger.OpBatchAnalysis, "", "Invalid request body", err, models.LogSeverityLow, nil)
		h.writeErrorResponse(w, r, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	// Validate request
	if len(request.Domains) == 0 {
		h.writeErrorResponse(w, r, http.StatusBadRequest, "domains array cannot be empty", "")
		return
	}

	if len(request.Domains) > 100 { // Limit batch size
		h.writeErrorResponse(w, r, http.StatusBadRequest, "too many domains", "Maximum 100 domains per batch")
		return
	}

	h.logger.LogInfo(ctx, logger.OpBatchAnalysis, fmt.Sprintf("Starting batch analysis for %d domains", len(request.Domains)), map[string]interface{}{
		"domains_count": len(request.Domains),
		"domains":       request.Domains,
	})

	// Perform batch analysis
	response, err := h.analysisService.AnalyzeDomains(ctx, request.Domains)
	if err != nil {
		h.logger.LogError(ctx, logger.OpBatchAnalysis, "", "Batch analysis failed", err, models.LogSeverityMedium, nil)
		h.writeErrorResponse(w, r, http.StatusInternalServerError, "batch analysis failed", err.Error())
		return
	}

	// Determine status code based on results
	statusCode := h.getBatchStatusCode(response)

	// Write successful response using centralized function
	if err := h.writeJSONResponse(w, r, statusCode, response); err != nil {
		// Response already sent with status code, but log the encoding error
		h.logger.LogError(ctx, logger.OpBatchAnalysis, "", "Failed to encode batch response", err, models.LogSeverityLow, nil)
		return
	}

	// Log success only after successful encoding
	h.logger.LogSuccess(ctx, logger.OpBatchAnalysis, "", fmt.Sprintf("Completed batch analysis: %d succeeded, %d failed", response.Summary.Succeeded, response.Summary.Failed), map[string]interface{}{
		"total":     response.Summary.Total,
		"succeeded": response.Summary.Succeeded,
		"failed":    response.Summary.Failed,
	})
}

// HealthCheck handles GET /health
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	// LogEvent is automatically created by logging middleware
	ctx := r.Context()

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Version:   "1.0.0",
	}

	// Write response using centralized function
	if err := h.writeJSONResponse(w, r, http.StatusOK, response); err != nil {
		// Response already sent with 200, but log the encoding error
		h.logger.LogError(ctx, logger.OpHealthCheck, "", "Failed to encode health response", err, models.LogSeverityLow, nil)
		return
	}

	// Log success only after successful encoding
	h.logger.LogInfo(ctx, logger.OpHealthCheck, "Health check performed successfully", nil)
}

// writeErrorResponse writes a standardized error response
func (h *Handler) writeErrorResponse(w http.ResponseWriter, r *http.Request, statusCode int, error, message string) {
	response := ErrorResponse{
		Error:     error,
		Message:   message,
		Timestamp: time.Now().UTC(),
	}

	// Use centralized response function to ensure consistent headers including X-Request-ID
	if err := h.writeJSONResponse(w, r, statusCode, response); err != nil {
		// Encoding failed - response already sent with status code, but log the error
		h.logger.LogError(r.Context(), "response_encoding", "", "Failed to encode error response", err, models.LogSeverityLow, nil)
	}
}


// getStatusCodeForError determines the appropriate HTTP status code for an error
func (h *Handler) getStatusCodeForError(err error) int {
	switch {
	case strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404"):
		return http.StatusNotFound
	case strings.Contains(err.Error(), "timeout"):
		return http.StatusRequestTimeout
	case strings.Contains(err.Error(), "invalid domain"):
		return http.StatusBadRequest
	case strings.Contains(err.Error(), "rate limit"):
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

// getBatchStatusCode determines the status code for batch responses
func (h *Handler) getBatchStatusCode(response *models.BatchAnalysisResponse) int {
	if response.Summary.Failed == 0 {
		// All succeeded
		return http.StatusOK
	} else if response.Summary.Succeeded == 0 {
		// All failed
		return http.StatusBadRequest
	} else {
		// Partial success - use 207 Multi-Status
		return http.StatusMultiStatus
	}
}