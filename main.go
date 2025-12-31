package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"Perion_Assignment/internal/cache"
	"Perion_Assignment/internal/cache/domainCache"
	"Perion_Assignment/internal/config"
	"Perion_Assignment/internal/domainAnalysis"
	"Perion_Assignment/internal/models"
	"Perion_Assignment/internal/fetcher"
	"Perion_Assignment/internal/http"
	"Perion_Assignment/internal/logger"
	"Perion_Assignment/internal/parser"
	"Perion_Assignment/internal/ratelimit"
)

func main() {
	// Load configuration
	cfg := config.Load()
	
	// Initialize database connection for logging (Supabase)
	db, err := logger.NewSupabaseConnection(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to Supabase: %v", err)
	}
	defer db.Close()
	
	// Initialize logger
	appLogger := logger.NewDatabaseLogger(db)
	defer appLogger.Close()
	
	// Create internal log event for startup
	startupCtx := logger.WithLogEvent(context.Background(), logger.NewInternalLogEvent())
	
	appLogger.LogInfo(startupCtx, logger.OpServerStart, "Starting AdsTxt Analysis API", map[string]interface{}{
		"version": "1.0.0",
		"config":  map[string]interface{}{
			"port":        cfg.Port,
			"cache_type": cfg.CacheType,
			"cache_ttl":  cfg.CacheTTL.Seconds(),
		},
	})
	
	// Initialize cache and domain cache
	cacheService, err := initializeCache(cfg)
	if err != nil {
		appLogger.LogError(
			startupCtx,
			"cache_init",
			"",
			"Failed to initialize cache",
			err,
			models.LogSeverityHigh,
			nil,
		)
		log.Fatalf("Failed to initialize cache: %v", err)
	}

	// Initialize domain cache
	domainCacheService := domainCache.New(cacheService, cfg.CacheTTL)
	
	// Initialize components
	adsTxtParser := parser.NewParser()
	adsTxtFetcher := fetcher.NewHTTPFetcher(time.Duration(cfg.FetchTimeoutSeconds) * time.Second)
	
	// Debug: Print rate limit configuration
	fmt.Printf("🔧 Rate Limit Config: Global=%d/sec, Per-IP=%d/sec\n", cfg.GlobalRateLimitPerSec, cfg.PerIPRateLimitPerSec)
	
	rateLimiter := ratelimit.NewTwoTierRateLimiter(
		int64(cfg.GlobalRateLimitPerSec),
		int64(cfg.GlobalRateLimitPerSec),
		int64(cfg.PerIPRateLimitPerSec),
		int64(cfg.PerIPRateLimitPerSec),
	)
	
	// Initialize service
	analysisService := domainAnalysis.NewService(
		adsTxtParser,
		adsTxtFetcher,
		domainCacheService,
		appLogger,
		cfg.MaxConcurrentFetches,
	)
	
	// Initialize HTTP handler
	handler := http.NewHandler(analysisService, appLogger)
	
	// Initialize server
	addr := ":" + cfg.Port
	server := http.NewServer(
		addr,
		handler,
		appLogger,
		rateLimiter,
		cfg.ServerReadTimeout,
		cfg.ServerWriteTimeout,
	)
	
	// Start server in goroutine
	go func() {
		if err := server.Start(); err != nil {
			appLogger.LogError(
				context.Background(),
				logger.OpServerStart,
				"",
				"Server failed to start",
				err,
				models.LogSeverityHigh,
				map[string]interface{}{"addr": addr},
			)
			log.Fatalf("Server failed to start: %v", err)
		}
	}()
	
	fmt.Printf("🚀 AdsTxt Analysis API server started on %s\n", addr)
	fmt.Println("📋 Available endpoints:")
	fmt.Println("  GET  /health                    - Health check")
	fmt.Println("  GET  /api/analyze/{domain}      - Analyze single domain")
	fmt.Println("  POST /api/batch-analysis        - Analyze multiple domains")
	
	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	fmt.Println("\n🛑 Shutting down server...")
	
	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ServerShutdownTimeout)
	defer cancel()
	
	// Shutdown server gracefully
	if err := server.Shutdown(ctx); err != nil {
		appLogger.LogError(
			ctx,
			logger.OpServerShutdown,
			"",
			"Server shutdown error",
			err,
			models.LogSeverityMedium,
			nil,
		)
		log.Printf("Server shutdown error: %v", err)
	} else {
		appLogger.LogInfo(ctx, logger.OpServerShutdown, "Server shutdown completed successfully", nil)
		fmt.Println("✅ Server shutdown completed")
	}
}

func initializeCache(cfg *config.Config) (cache.Service, error) {
	switch cfg.CacheType {
	case "redis":
		return cache.NewRedisCache(cfg.RedisURL)
	case "memory":
		return cache.NewMemoryCache(), nil
	default:
		return nil, fmt.Errorf("unsupported cache type: %s", cfg.CacheType)
	}
}
