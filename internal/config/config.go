package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                  string
	CacheType             string
	CacheTTL              time.Duration
	RedisURL              string
	GlobalRateLimitPerSec int
	PerIPRateLimitPerSec  int
	DatabaseURL           string
	FetchTimeoutSeconds   int
	MaxConcurrentFetches  int
	ServerReadTimeout     time.Duration
	ServerWriteTimeout    time.Duration
	ServerShutdownTimeout time.Duration
}

func Load() *Config {
	// Load .env file if it exists (optional)
	if err := godotenv.Load(); err != nil {
		log.Printf("No .env file found or error loading it: %v", err)
	}

	return &Config{
		Port:                  getEnv("PORT", "8080"),
		CacheType:             getEnv("CACHE_TYPE", "memory"),
		CacheTTL:              getDurationEnv("CACHE_TTL", 3600*time.Second),
		RedisURL:              getEnv("REDIS_URL", "redis://localhost:6379"),
		GlobalRateLimitPerSec: getIntEnv("GLOBAL_RATE_LIMIT_PER_SEC", 100),
		PerIPRateLimitPerSec:  getIntEnv("PER_IP_RATE_LIMIT_PER_SEC", 10),
		DatabaseURL:           getEnv("DATABASE_URL", "postgresql://user:pass@localhost:5432/dbname"),
		FetchTimeoutSeconds:   getIntEnv("FETCH_TIMEOUT_SECONDS", 10),
		MaxConcurrentFetches:  getIntEnv("MAX_CONCURRENT_FETCHES", 10),
		ServerReadTimeout:     getDurationEnv("SERVER_READ_TIMEOUT", 15*time.Second),
		ServerWriteTimeout:    getDurationEnv("SERVER_WRITE_TIMEOUT", 15*time.Second),
		ServerShutdownTimeout: getDurationEnv("SERVER_SHUTDOWN_TIMEOUT", 30*time.Second),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return time.Duration(intVal) * time.Second
		}
	}
	return defaultValue
}
