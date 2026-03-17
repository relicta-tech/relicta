// Package middleware provides HTTP middleware for the dashboard.
package middleware

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter provides simple IP-based rate limiting using a token bucket algorithm.
type RateLimiter struct {
	mu       sync.Mutex
	clients  map[string]*bucket
	rate     int           // tokens added per interval
	burst    int           // max tokens in bucket
	interval time.Duration // refill interval
	cleanup  time.Duration // cleanup interval for stale entries
}

type bucket struct {
	tokens    int
	lastCheck time.Time
}

// RateLimiterConfig configures the rate limiter.
type RateLimiterConfig struct {
	// Rate is the number of requests allowed per interval.
	Rate int
	// Burst is the maximum number of requests allowed in a burst.
	Burst int
	// Interval is the time window for rate limiting.
	Interval time.Duration
}

// DefaultRateLimiterConfig returns sensible defaults for rate limiting.
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		Rate:     100,             // 100 requests per interval
		Burst:    20,              // Allow burst of 20
		Interval: 1 * time.Minute, // Per minute
	}
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		clients:  make(map[string]*bucket),
		rate:     cfg.Rate,
		burst:    cfg.Burst,
		interval: cfg.Interval,
		cleanup:  5 * time.Minute,
	}

	// Start background cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// cleanupLoop periodically removes stale entries.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, b := range rl.clients {
			// Remove entries that haven't been used for 2 cleanup intervals
			if now.Sub(b.lastCheck) > 2*rl.cleanup {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Allow checks if a request from the given IP is allowed.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.clients[ip]
	if !exists {
		rl.clients[ip] = &bucket{
			tokens:    rl.burst - 1, // Consume one token for this request
			lastCheck: now,
		}
		return true
	}

	// Calculate tokens to add based on time elapsed
	elapsed := now.Sub(b.lastCheck)
	tokensToAdd := int(elapsed.Seconds() / rl.interval.Seconds() * float64(rl.rate))
	b.tokens += tokensToAdd
	if b.tokens > rl.burst {
		b.tokens = rl.burst
	}
	b.lastCheck = now

	// Check if we have tokens available
	if b.tokens > 0 {
		b.tokens--
		return true
	}

	return false
}

// RateLimit returns middleware that applies rate limiting.
// If cfg is nil, uses DefaultRateLimiterConfig().
func RateLimit(cfg *RateLimiterConfig) func(http.Handler) http.Handler {
	if cfg == nil {
		defaultCfg := DefaultRateLimiterConfig()
		cfg = &defaultCfg
	}
	rl := NewRateLimiter(*cfg)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get client IP (RealIP middleware should run before this)
			ip := r.RemoteAddr

			if !rl.Allow(ip) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
