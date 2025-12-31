package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_DefaultValues(t *testing.T) {
	// Clear all relevant environment variables
	envVars := []string{
		"PORT", "CACHE_TYPE", "CACHE_TTL", "REDIS_URL",
		"GLOBAL_RATE_LIMIT_PER_SEC", "PER_IP_RATE_LIMIT_PER_SEC",
		"DATABASE_URL", "FETCH_TIMEOUT_SECONDS",
		"MAX_CONCURRENT_FETCHES", "SERVER_READ_TIMEOUT",
		"SERVER_WRITE_TIMEOUT", "SERVER_SHUTDOWN_TIMEOUT",
	}

	for _, key := range envVars {
		os.Unsetenv(key)
	}

	cfg := Load()

	// Verify default values
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "memory", cfg.CacheType)
	assert.Equal(t, 3600*time.Second, cfg.CacheTTL)
	assert.Equal(t, "redis://localhost:6379", cfg.RedisURL)
	assert.Equal(t, 100, cfg.GlobalRateLimitPerSec)
	assert.Equal(t, 10, cfg.PerIPRateLimitPerSec)
	assert.Equal(t, "postgresql://user:pass@localhost:5432/dbname", cfg.DatabaseURL)
	assert.Equal(t, 10, cfg.FetchTimeoutSeconds)
	assert.Equal(t, 10, cfg.MaxConcurrentFetches)
	assert.Equal(t, 15*time.Second, cfg.ServerReadTimeout)
	assert.Equal(t, 15*time.Second, cfg.ServerWriteTimeout)
	assert.Equal(t, 30*time.Second, cfg.ServerShutdownTimeout)
}

func TestLoad_WithEnvironmentVariables(t *testing.T) {
	// Set environment variables
	os.Setenv("PORT", "9090")
	os.Setenv("CACHE_TYPE", "redis")
	os.Setenv("CACHE_TTL", "7200")
	os.Setenv("REDIS_URL", "redis://custom:6380")
	os.Setenv("GLOBAL_RATE_LIMIT_PER_SEC", "200")
	os.Setenv("PER_IP_RATE_LIMIT_PER_SEC", "20")
	os.Setenv("DATABASE_URL", "postgresql://custom-db")
	os.Setenv("FETCH_TIMEOUT_SECONDS", "15")
	os.Setenv("MAX_CONCURRENT_FETCHES", "20")
	os.Setenv("SERVER_READ_TIMEOUT", "30")
	os.Setenv("SERVER_WRITE_TIMEOUT", "30")
	os.Setenv("SERVER_SHUTDOWN_TIMEOUT", "60")

	// Defer cleanup
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("CACHE_TYPE")
		os.Unsetenv("CACHE_TTL")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("GLOBAL_RATE_LIMIT_PER_SEC")
		os.Unsetenv("PER_IP_RATE_LIMIT_PER_SEC")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("FETCH_TIMEOUT_SECONDS")
		os.Unsetenv("MAX_CONCURRENT_FETCHES")
		os.Unsetenv("SERVER_READ_TIMEOUT")
		os.Unsetenv("SERVER_WRITE_TIMEOUT")
		os.Unsetenv("SERVER_SHUTDOWN_TIMEOUT")
	}()

	cfg := Load()

	// Verify environment variable values are used
	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "redis", cfg.CacheType)
	assert.Equal(t, 7200*time.Second, cfg.CacheTTL)
	assert.Equal(t, "redis://custom:6380", cfg.RedisURL)
	assert.Equal(t, 200, cfg.GlobalRateLimitPerSec)
	assert.Equal(t, 20, cfg.PerIPRateLimitPerSec)
	assert.Equal(t, "postgresql://custom-db", cfg.DatabaseURL)
	assert.Equal(t, 15, cfg.FetchTimeoutSeconds)
	assert.Equal(t, 20, cfg.MaxConcurrentFetches)
	assert.Equal(t, 30*time.Second, cfg.ServerReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.ServerWriteTimeout)
	assert.Equal(t, 60*time.Second, cfg.ServerShutdownTimeout)
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "uses default when env not set",
			key:          "TEST_VAR_1",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
		{
			name:         "uses env value when set",
			key:          "TEST_VAR_2",
			defaultValue: "default",
			envValue:     "custom",
			expected:     "custom",
		},
		{
			name:         "uses env value even if empty string",
			key:          "TEST_VAR_3",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetIntEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int
		envValue     string
		expected     int
	}{
		{
			name:         "uses default when env not set",
			key:          "TEST_INT_1",
			defaultValue: 42,
			envValue:     "",
			expected:     42,
		},
		{
			name:         "uses env value when valid int",
			key:          "TEST_INT_2",
			defaultValue: 42,
			envValue:     "100",
			expected:     100,
		},
		{
			name:         "uses default when env value is invalid",
			key:          "TEST_INT_3",
			defaultValue: 42,
			envValue:     "not-a-number",
			expected:     42,
		},
		{
			name:         "handles negative numbers",
			key:          "TEST_INT_4",
			defaultValue: 42,
			envValue:     "-10",
			expected:     -10,
		},
		{
			name:         "handles zero",
			key:          "TEST_INT_5",
			defaultValue: 42,
			envValue:     "0",
			expected:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getIntEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDurationEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue time.Duration
		envValue     string
		expected     time.Duration
	}{
		{
			name:         "uses default when env not set",
			key:          "TEST_DURATION_1",
			defaultValue: 10 * time.Second,
			envValue:     "",
			expected:     10 * time.Second,
		},
		{
			name:         "uses env value when valid int (seconds)",
			key:          "TEST_DURATION_2",
			defaultValue: 10 * time.Second,
			envValue:     "30",
			expected:     30 * time.Second,
		},
		{
			name:         "uses default when env value is invalid",
			key:          "TEST_DURATION_3",
			defaultValue: 10 * time.Second,
			envValue:     "not-a-number",
			expected:     10 * time.Second,
		},
		{
			name:         "handles zero",
			key:          "TEST_DURATION_4",
			defaultValue: 10 * time.Second,
			envValue:     "0",
			expected:     0 * time.Second,
		},
		{
			name:         "handles large numbers",
			key:          "TEST_DURATION_5",
			defaultValue: 10 * time.Second,
			envValue:     "3600",
			expected:     3600 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getDurationEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoad_PartialEnvironmentVariables(t *testing.T) {
	// Set only some environment variables
	os.Setenv("PORT", "3000")
	os.Setenv("CACHE_TYPE", "redis")
	defer os.Unsetenv("PORT")
	defer os.Unsetenv("CACHE_TYPE")

	// Ensure others are not set
	os.Unsetenv("GLOBAL_RATE_LIMIT_PER_SEC")

	cfg := Load()

	// Custom values
	assert.Equal(t, "3000", cfg.Port)
	assert.Equal(t, "redis", cfg.CacheType)

	// Default values
	assert.Equal(t, 100, cfg.GlobalRateLimitPerSec)
}

func TestLoad_InvalidIntegerEnvironmentVariables(t *testing.T) {
	// Set invalid integer values
	os.Setenv("GLOBAL_RATE_LIMIT_PER_SEC", "invalid")
	os.Setenv("PER_IP_RATE_LIMIT_PER_SEC", "also-invalid")
	os.Setenv("FETCH_TIMEOUT_SECONDS", "not-a-number")

	defer func() {
		os.Unsetenv("GLOBAL_RATE_LIMIT_PER_SEC")
		os.Unsetenv("PER_IP_RATE_LIMIT_PER_SEC")
		os.Unsetenv("FETCH_TIMEOUT_SECONDS")
	}()

	cfg := Load()

	// Should fall back to defaults
	assert.Equal(t, 100, cfg.GlobalRateLimitPerSec)
	assert.Equal(t, 10, cfg.PerIPRateLimitPerSec)
	assert.Equal(t, 10, cfg.FetchTimeoutSeconds)
}

func TestLoad_InvalidDurationEnvironmentVariables(t *testing.T) {
	// Set invalid duration values
	os.Setenv("CACHE_TTL", "invalid")
	os.Setenv("SERVER_READ_TIMEOUT", "not-a-number")

	defer func() {
		os.Unsetenv("CACHE_TTL")
		os.Unsetenv("SERVER_READ_TIMEOUT")
	}()

	cfg := Load()

	// Should fall back to defaults
	assert.Equal(t, 3600*time.Second, cfg.CacheTTL)
	assert.Equal(t, 15*time.Second, cfg.ServerReadTimeout)
}

func TestLoad_EmptyStringEnvironmentVariables(t *testing.T) {
	// Set empty string values - should use defaults
	os.Setenv("PORT", "")
	os.Setenv("CACHE_TYPE", "")

	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("CACHE_TYPE")
	}()

	cfg := Load()

	// Should use defaults when env is empty string
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "memory", cfg.CacheType)
}

func TestConfig_StructureAndFields(t *testing.T) {
	cfg := &Config{
		Port:                   "8080",
		CacheType:             "memory",
		CacheTTL:              1 * time.Hour,
		RedisURL:              "redis://localhost:6379",
		GlobalRateLimitPerSec: 100,
		PerIPRateLimitPerSec:  10,
		DatabaseURL:           "postgresql://localhost",
		FetchTimeoutSeconds:   10,
		MaxConcurrentFetches:  10,
		ServerReadTimeout:     15 * time.Second,
		ServerWriteTimeout:    15 * time.Second,
		ServerShutdownTimeout: 30 * time.Second,
	}

	// Verify all fields are accessible
	require.NotNil(t, cfg)
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "memory", cfg.CacheType)
	assert.Equal(t, 1*time.Hour, cfg.CacheTTL)
	assert.Equal(t, "redis://localhost:6379", cfg.RedisURL)
	assert.Equal(t, 100, cfg.GlobalRateLimitPerSec)
	assert.Equal(t, 10, cfg.PerIPRateLimitPerSec)
	assert.Equal(t, "postgresql://localhost", cfg.DatabaseURL)
	assert.Equal(t, 10, cfg.FetchTimeoutSeconds)
	assert.Equal(t, 10, cfg.MaxConcurrentFetches)
	assert.Equal(t, 15*time.Second, cfg.ServerReadTimeout)
	assert.Equal(t, 15*time.Second, cfg.ServerWriteTimeout)
	assert.Equal(t, 30*time.Second, cfg.ServerShutdownTimeout)
}
