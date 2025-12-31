package http

import (
	"context"
	"net/http"
	"time"

	"Perion_Assignment/internal/logger"
	"Perion_Assignment/internal/ratelimit"

	"github.com/gorilla/mux"
)

// Server represents the HTTP server with all dependencies
type Server struct {
	handler *Handler
	logger  logger.Service
	server  *http.Server
}

// NewServer creates a new HTTP server
func NewServer(
	addr string,
	handler *Handler,
	logger logger.Service,
	rateLimiter ratelimit.Service,
	readTimeout, writeTimeout time.Duration,
) *Server {
	// Create Gorilla mux router
	router := mux.NewRouter()

	// Create server
	srv := &Server{
		handler: handler,
		logger:  logger,
		server: &http.Server{
			Addr:         addr,
			Handler:      router,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
		},
	}

	// Register middleware (order matters: logging -> rate limiting -> cors -> recovery)
	router.Use(loggingMiddleware(logger))
	router.Use(rateLimitingMiddleware(rateLimiter, logger))
	router.Use(corsMiddleware())
	router.Use(recoveryMiddleware(logger))

	// Register routes
	srv.registerRoutes(router)

	return srv
}

// registerRoutes sets up all API routes
func (s *Server) registerRoutes(router *mux.Router) {
	// Health check
	router.HandleFunc("/health", s.handler.HealthCheck).Methods("GET")

	// API routes
	router.HandleFunc("/api/analyze/{domain}", s.handler.AnalyzeSingleDomain).Methods("GET")
	router.HandleFunc("/api/batch-analysis", s.handler.AnalyzeBatchDomains).Methods("POST")

	// Root handler
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"AdsTxt Analysis API","version":"1.0.0","endpoints":["/health","/api/analyze/{domain}","/api/batch-analysis"]}`))
	}).Methods("GET")
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.LogInfo(context.Background(), logger.OpServerStart, "Starting HTTP server", map[string]interface{}{
		"addr": s.server.Addr,
	})

	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.LogInfo(ctx, logger.OpServerShutdown, "Shutting down HTTP server", nil)
	return s.server.Shutdown(ctx)
}
