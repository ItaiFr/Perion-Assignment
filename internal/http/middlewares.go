package http

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"Perion_Assignment/internal/logger"
	"Perion_Assignment/internal/models"
	"Perion_Assignment/internal/ratelimit"
	"github.com/gorilla/mux"
)

// loggingMiddleware creates LogEvent and logs HTTP requests
func loggingMiddleware(loggerService logger.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract client IP and create LogEvent for this request
			clientIP := getClientIP(r)
			logEvent := logger.NewRequestLogEvent(clientIP)

			// Add LogEvent to request context
			ctx := logger.WithLogEvent(r.Context(), logEvent)
			r = r.WithContext(ctx)

			// Read and log request body (safely)
			var bodyBytes []byte
			var bodyStr string
			if r.Body != nil {
				// Read the body
				bodyBytes, _ = io.ReadAll(r.Body)
				r.Body.Close()

				// Restore the body for subsequent handlers
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				// Convert to string for logging (limit to first 1000 chars to avoid huge logs)
				if len(bodyBytes) > 1000 {
					bodyStr = string(bodyBytes[:1000]) + "... (truncated)"
				} else {
					bodyStr = string(bodyBytes)
				}
			}

			// Extract URL parameters from Gorilla mux router
			urlParams := mux.Vars(r)

			// Log the initial request with full details
			requestMetadata := map[string]interface{}{
				"method":      r.Method,
				"path":        r.URL.Path,
				"query":       r.URL.RawQuery,
				"user_agent":  r.UserAgent(),
				"client_ip":   clientIP,
			}

			// Add URL params if any
			if len(urlParams) > 0 {
				requestMetadata["url_params"] = urlParams
			}

			// Add body if present (and not too large)
			if bodyStr != "" {
				requestMetadata["body"] = bodyStr
			}

			loggerService.LogInfo(ctx, "http_request_start", "HTTP request received", requestMetadata)

			// Wrap ResponseWriter to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Calculate total request duration from LogEvent start time
			duration := time.Since(logEvent.StartTime)

			// Log the completed request with all context
			loggerService.LogInfo(ctx, "http_request_complete", "HTTP request processed", map[string]interface{}{
				"method":      r.Method,
				"path":        r.URL.Path,
				"status_code": wrapped.statusCode,
				"duration_ms": duration.Milliseconds(),
				"user_agent":  r.UserAgent(),
				"client_ip":   clientIP,
			})
		})
	}
}

// corsMiddleware adds CORS headers
func corsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// recoveryMiddleware recovers from panics
func recoveryMiddleware(loggerService logger.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					loggerService.LogError(
						r.Context(),
						"panic_recovery",
						"",
						"Panic recovered in HTTP handler",
						fmt.Errorf("panic: %v", err),
						models.LogSeverityHigh,
						map[string]interface{}{
							"panic":  err,
							"path":   r.URL.Path,
							"method": r.Method,
						},
					)

					// Get LogEvent from context for request ID
					logEvent := logger.GetLogEvent(r.Context())

					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("X-Request-ID", logEvent.ProcessID)
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error":"internal server error","message":"An unexpected error occurred"}`))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// rateLimitingMiddleware applies rate limiting to requests
// Expects LogEvent to already be in context from logging middleware
func rateLimitingMiddleware(rateLimiter ratelimit.Service, loggerService logger.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			
			// Get LogEvent from context (created by logging middleware)
			logEvent := logger.GetLogEvent(ctx)
			clientIP := logEvent.ClientIP
			
			// Check rate limiting
			if !rateLimiter.Allow(clientIP) {
				loggerService.LogError(ctx, logger.OpRateLimited, "", "Rate limit exceeded", models.ErrRateLimitExceeded, models.LogSeverityMedium, map[string]interface{}{
					"path":   r.URL.Path,
					"method": r.Method,
				})

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Request-ID", logEvent.ProcessID)
				w.Header().Set("X-RateLimit-Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate limit exceeded","message":"Please try again later"}`))
				return
			}
			
			// Continue to next middleware/handler
			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for load balancers/proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// Fall back to remote address
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	
	return host
}