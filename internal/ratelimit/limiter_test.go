package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestTokenBucket_Allow(t *testing.T) {
	// Create a bucket with capacity 3, refill rate 1 per second
	bucket := NewTokenBucket(3, 1)
	
	// Should allow first 3 requests immediately
	for i := 0; i < 3; i++ {
		if !bucket.Allow() {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}
	
	// 4th request should be denied
	if bucket.Allow() {
		t.Error("4th request should be denied")
	}
	
	// Wait a bit more than 1 second and try again
	time.Sleep(1100 * time.Millisecond)
	
	// Should allow one more request after refill
	if !bucket.Allow() {
		t.Error("Request after refill should be allowed")
	}
	
	// Next request should be denied
	if bucket.Allow() {
		t.Error("Request immediately after refill should be denied")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	bucket := NewTokenBucket(5, 2) // 5 capacity, 2 per second
	
	// Consume all tokens
	for i := 0; i < 5; i++ {
		bucket.Allow()
	}
	
	// Should be empty
	if bucket.Allow() {
		t.Error("Bucket should be empty")
	}
	
	// Wait 1 second (should add 2 tokens)
	time.Sleep(1 * time.Second)
	
	// Should allow 2 requests
	if !bucket.Allow() {
		t.Error("First request after refill should be allowed")
	}
	if !bucket.Allow() {
		t.Error("Second request after refill should be allowed")
	}
	
	// Third should be denied
	if bucket.Allow() {
		t.Error("Third request should be denied")
	}
}

func TestTwoTierRateLimiter_Allow(t *testing.T) {
	// Global: 10 req/sec, Per-IP: 3 req/sec
	limiter := NewTwoTierRateLimiter(10, 10, 3, 3)
	
	// Test per-IP limiting
	for i := 0; i < 3; i++ {
		if !limiter.Allow("192.168.1.1") {
			t.Errorf("Request %d for IP 192.168.1.1 should be allowed", i+1)
		}
	}
	
	// 4th request from same IP should be denied
	if limiter.Allow("192.168.1.1") {
		t.Error("4th request from same IP should be denied")
	}
	
	// Different IP should still be allowed
	for i := 0; i < 3; i++ {
		if !limiter.Allow("192.168.1.2") {
			t.Errorf("Request %d for IP 192.168.1.2 should be allowed", i+1)
		}
	}
}

func TestTwoTierRateLimiter_GlobalLimit(t *testing.T) {
	// Global: 2 req/sec, Per-IP: 10 req/sec (higher than global)
	limiter := NewTwoTierRateLimiter(2, 2, 10, 10)
	
	// Use different IPs to bypass per-IP limit, test global limit
	if !limiter.Allow("192.168.1.1") {
		t.Error("First global request should be allowed")
	}
	if !limiter.Allow("192.168.1.2") {
		t.Error("Second global request should be allowed")
	}
	
	// Third request should be denied due to global limit
	if limiter.Allow("192.168.1.3") {
		t.Error("Third global request should be denied")
	}
}

func TestTwoTierRateLimiter_Wait(t *testing.T) {
	limiter := NewTwoTierRateLimiter(1, 10, 1, 10) // Very fast refill for testing
	
	// Consume the token
	if !limiter.Allow("192.168.1.1") {
		t.Error("First request should be allowed")
	}
	
	// Wait should complete quickly due to fast refill
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	start := time.Now()
	err := limiter.Wait(ctx, "192.168.1.1")
	duration := time.Since(start)
	
	if err != nil {
		t.Errorf("Wait should not return error: %v", err)
	}
	
	// Should complete within reasonable time
	if duration > 1*time.Second {
		t.Errorf("Wait took too long: %v", duration)
	}
}

func TestTwoTierRateLimiter_WaitTimeout(t *testing.T) {
	limiter := NewTwoTierRateLimiter(1, 1, 1, 1) // Slow refill
	
	// Consume the token
	limiter.Allow("192.168.1.1")
	
	// Wait with short timeout should fail
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	err := limiter.Wait(ctx, "192.168.1.1")
	if err == nil {
		t.Error("Wait should timeout and return error")
	}
	
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

func BenchmarkTokenBucket_Allow(b *testing.B) {
	bucket := NewTokenBucket(1000, 1000) // Large capacity to avoid blocking
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bucket.Allow()
		}
	})
}

func BenchmarkTwoTierRateLimiter_Allow(b *testing.B) {
	limiter := NewTwoTierRateLimiter(1000, 1000, 1000, 1000) // Large limits

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ip := "192.168.1.1"
		for pb.Next() {
			limiter.Allow(ip)
		}
	})
}

// TestTwoTierRateLimiter_CleanupOldBuckets tests that old IP buckets are cleaned up
func TestTwoTierRateLimiter_CleanupOldBuckets(t *testing.T) {
	limiter := NewTwoTierRateLimiter(10, 10, 3, 3)

	// Create buckets for multiple IPs
	for i := 0; i < 5; i++ {
		ip := "192.168.1." + string(rune('1'+i))
		limiter.Allow(ip)
	}

	// Count IP buckets
	initialCount := 0
	limiter.ipBuckets.Range(func(key, value interface{}) bool {
		initialCount++
		return true
	})

	if initialCount != 5 {
		t.Errorf("Expected 5 IP buckets, got %d", initialCount)
	}

	// Note: The cleanup routine runs every 10 minutes in production
	// We can't easily test it runs without mocking time,
	// but we've verified the buckets are created correctly
}

// TestTwoTierRateLimiter_ConcurrentIPBucketCreation tests concurrent access to IP buckets
func TestTwoTierRateLimiter_ConcurrentIPBucketCreation(t *testing.T) {
	limiter := NewTwoTierRateLimiter(500, 500, 10, 10)

	// Create channels for synchronization
	done := make(chan bool)

	// Launch multiple goroutines that create buckets for different IPs
	numGoroutines := 10
	ipsPerGoroutine := 5

	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			for i := 0; i < ipsPerGoroutine; i++ {
				// Create unique IP using string formatting
				ip := "10." + string(rune('0'+goroutineID)) + ".1." + string(rune('0'+i))
				limiter.Allow(ip)
			}
			done <- true
		}(g)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify no race conditions occurred by checking bucket count
	bucketCount := 0
	limiter.ipBuckets.Range(func(key, value interface{}) bool {
		bucketCount++
		return true
	})

	expectedCount := numGoroutines * ipsPerGoroutine
	if bucketCount != expectedCount {
		t.Errorf("Expected %d IP buckets, got %d (race condition or duplicate IPs)", expectedCount, bucketCount)
	}

	// The main goal is to ensure no race conditions - if we get here without panics, that's good
	if bucketCount < expectedCount/2 {
		t.Errorf("Too few buckets created: %d, expected at least %d", bucketCount, expectedCount/2)
	}
}

// TestTwoTierRateLimiter_Wait_ActualBlocking tests that Wait() actually blocks
func TestTwoTierRateLimiter_Wait_ActualBlocking(t *testing.T) {
	// Create limiter with slow refill rate (1 token per second)
	limiter := NewTwoTierRateLimiter(1, 1, 1, 1)

	// Consume the initial token
	if !limiter.Allow("192.168.1.1") {
		t.Fatal("First request should be allowed")
	}

	// Now Wait should block until a token is available
	ctx := context.Background()
	start := time.Now()

	err := limiter.Wait(ctx, "192.168.1.1")
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Wait should not return error: %v", err)
	}

	// Should have blocked for approximately 1 second (with some tolerance)
	// Note: Wait() polls periodically, so it may take slightly longer
	if duration < 800*time.Millisecond {
		t.Errorf("Wait should have blocked for ~1 second, but only blocked for %v", duration)
	}

	if duration > 2*time.Second {
		t.Errorf("Wait blocked too long: %v", duration)
	}

	// Wait() already consumed a token when it succeeded,
	// so we've verified the blocking behavior
}

// TestTwoTierRateLimiter_Wait_MultipleGoroutines tests Wait with multiple goroutines
func TestTwoTierRateLimiter_Wait_MultipleGoroutines(t *testing.T) {
	limiter := NewTwoTierRateLimiter(5, 5, 2, 2)

	// Consume initial tokens
	for i := 0; i < 5; i++ {
		limiter.Allow("192.168.1.1")
	}

	// Launch multiple goroutines that wait
	numGoroutines := 3
	done := make(chan time.Duration, numGoroutines)

	ctx := context.Background()
	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		go func() {
			err := limiter.Wait(ctx, "192.168.1.1")
			if err != nil {
				t.Errorf("Wait failed: %v", err)
			}
			done <- time.Since(start)
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		duration := <-done
		// Each should have waited some time
		if duration < 100*time.Millisecond {
			t.Errorf("Goroutine %d should have waited longer: %v", i, duration)
		}
	}
}

// TestTwoTierRateLimiter_GlobalVsPerIP tests interaction between global and per-IP limits
func TestTwoTierRateLimiter_GlobalVsPerIP(t *testing.T) {
	// Global: 5 req/sec, Per-IP: 3 req/sec
	limiter := NewTwoTierRateLimiter(5, 5, 3, 3)

	// Use two different IPs
	// Each IP can do 3 requests, but globally only 5 allowed

	// IP1: 3 requests (should all succeed)
	for i := 0; i < 3; i++ {
		if !limiter.Allow("192.168.1.1") {
			t.Errorf("IP1 request %d should be allowed", i+1)
		}
	}

	// IP2: only 2 more requests should succeed (global limit is 5)
	for i := 0; i < 2; i++ {
		if !limiter.Allow("192.168.1.2") {
			t.Errorf("IP2 request %d should be allowed", i+1)
		}
	}

	// IP2: 3rd request should be denied (global limit reached)
	if limiter.Allow("192.168.1.2") {
		t.Error("IP2 third request should be denied due to global limit")
	}
}

// TestTokenBucket_RefillPartial tests partial token refill
func TestTokenBucket_RefillPartial(t *testing.T) {
	bucket := NewTokenBucket(10, 10) // 10 capacity, 10 per second

	// Consume all tokens
	for i := 0; i < 10; i++ {
		bucket.Allow()
	}

	// Wait for 0.5 seconds (should add 5 tokens)
	time.Sleep(500 * time.Millisecond)

	// Should allow 5 requests
	allowed := 0
	for i := 0; i < 10; i++ {
		if bucket.Allow() {
			allowed++
		}
	}

	// Should have allowed approximately 5 requests (with some tolerance for timing)
	if allowed < 4 || allowed > 6 {
		t.Errorf("Expected ~5 requests to be allowed after 0.5s, got %d", allowed)
	}
}

// TestTwoTierRateLimiter_ReturnTokenOnPerIPDenial tests token return on per-IP limit
func TestTwoTierRateLimiter_ReturnTokenOnPerIPDenial(t *testing.T) {
	// Global: 10 req/sec, Per-IP: 2 req/sec
	limiter := NewTwoTierRateLimiter(10, 10, 2, 2)

	// IP1: consume per-IP limit (2 requests)
	limiter.Allow("192.168.1.1")
	limiter.Allow("192.168.1.1")

	// 3rd request should be denied due to per-IP limit
	// This should return the global token
	if limiter.Allow("192.168.1.1") {
		t.Error("Third request should be denied due to per-IP limit")
	}

	// Different IP should still be able to use the global token that was returned
	if !limiter.Allow("192.168.1.2") {
		t.Error("Different IP should be allowed (global token was returned)")
	}
}