package ratelimit

import (
	"context"
	"sync"
	"time"
)

// TokenBucket implements a token bucket rate limiter
type TokenBucket struct {
	capacity   int64
	tokens     int64
	refillRate int64 // tokens per second
	lastRefill time.Time
	mutex      sync.Mutex
}

// NewTokenBucket creates a new token bucket with the specified capacity and refill rate
func NewTokenBucket(capacity, refillRate int64) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity, // Start with full bucket
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a token is available and consumes it if so
func (tb *TokenBucket) Allow() bool {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	
	tb.refill()
	
	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	
	return false
}

// refill adds tokens based on time elapsed since last refill
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	
	tokensToAdd := int64(elapsed * float64(tb.refillRate))
	if tokensToAdd > 0 {
		tb.tokens = min(tb.capacity, tb.tokens+tokensToAdd)
		tb.lastRefill = now
	}
}

// TwoTierRateLimiter implements both global and per-IP rate limiting
type TwoTierRateLimiter struct {
	globalBucket  *TokenBucket
	ipBuckets     sync.Map // map[string]*TokenBucket
	perIPCapacity int64
	perIPRate     int64
}

// NewTwoTierRateLimiter creates a new two-tier rate limiter
func NewTwoTierRateLimiter(globalCapacity, globalRate, perIPCapacity, perIPRate int64) *TwoTierRateLimiter {
	limiter := &TwoTierRateLimiter{
		globalBucket:  NewTokenBucket(globalCapacity, globalRate),
		perIPCapacity: perIPCapacity,
		perIPRate:     perIPRate,
	}
	
	// Start cleanup routine for IP buckets
	go limiter.cleanupIPBuckets()
	
	return limiter
}

// Allow checks both global and per-IP rate limits
func (trl *TwoTierRateLimiter) Allow(clientIP string) bool {
	// Check global limit first
	if !trl.globalBucket.Allow() {
		return false
	}
	
	// Check per-IP limit
	ipBucket := trl.getOrCreateIPBucket(clientIP)
	if !ipBucket.Allow() {
		// If per-IP limit exceeded, we should return the global token
		// since we consumed it but can't proceed
		trl.returnGlobalToken()
		return false
	}
	
	return true
}

// Wait blocks until a token becomes available for the given IP
func (trl *TwoTierRateLimiter) Wait(ctx context.Context, clientIP string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if trl.Allow(clientIP) {
				return nil
			}
		}
	}
}

// getOrCreateIPBucket gets or creates a token bucket for the given IP
func (trl *TwoTierRateLimiter) getOrCreateIPBucket(clientIP string) *TokenBucket {
	if bucket, ok := trl.ipBuckets.Load(clientIP); ok {
		return bucket.(*TokenBucket)
	}
	
	// Create new bucket
	newBucket := NewTokenBucket(trl.perIPCapacity, trl.perIPRate)
	actual, _ := trl.ipBuckets.LoadOrStore(clientIP, newBucket)
	
	return actual.(*TokenBucket)
}

// returnGlobalToken returns a token to the global bucket (compensation for per-IP limit)
func (trl *TwoTierRateLimiter) returnGlobalToken() {
	trl.globalBucket.mutex.Lock()
	defer trl.globalBucket.mutex.Unlock()
	
	if trl.globalBucket.tokens < trl.globalBucket.capacity {
		trl.globalBucket.tokens++
	}
}

// cleanupIPBuckets removes old IP buckets to prevent memory leaks
func (trl *TwoTierRateLimiter) cleanupIPBuckets() {
	ticker := time.NewTicker(10 * time.Minute) // Cleanup every 10 minutes
	defer ticker.Stop()
	
	for range ticker.C {
		cutoff := time.Now().Add(-30 * time.Minute) // Remove buckets older than 30 minutes
		
		trl.ipBuckets.Range(func(key, value interface{}) bool {
			bucket := value.(*TokenBucket)
			bucket.mutex.Lock()
			lastActivity := bucket.lastRefill
			bucket.mutex.Unlock()
			
			if lastActivity.Before(cutoff) {
				trl.ipBuckets.Delete(key)
			}
			return true
		})
	}
}

// Helper function for min
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}