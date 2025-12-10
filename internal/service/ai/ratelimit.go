// Package ai provides AI-powered content generation for ReleasePilot.
package ai

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewRateLimiter creates a new rate limiter with the specified requests per minute.
// If rpm is 0 or negative, returns nil (no rate limiting).
func NewRateLimiter(rpm int) *RateLimiter {
	if rpm <= 0 {
		return nil
	}

	return &RateLimiter{
		tokens:     float64(rpm),
		maxTokens:  float64(rpm),
		refillRate: float64(rpm) / 60.0, // tokens per second
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available or the context is canceled.
// Returns nil if a token was acquired, or an error if the context was canceled.
func (r *RateLimiter) Wait(ctx context.Context) error {
	if r == nil {
		return nil // No rate limiting
	}

	for {
		// Check context cancellation at start of each iteration
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		r.mu.Lock()
		r.refill()

		if r.tokens >= 1.0 {
			r.tokens--
			r.mu.Unlock()
			return nil
		}

		// Calculate wait time until next token
		waitTime := time.Duration((1.0 - r.tokens) / r.refillRate * float64(time.Second))
		// Ensure minimum wait time to prevent busy-loop
		if waitTime < 10*time.Millisecond {
			waitTime = 10 * time.Millisecond
		}
		r.mu.Unlock()

		// Wait with context cancellation support
		// Use time.NewTimer instead of time.After to avoid timer leak
		timer := time.NewTimer(waitTime)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			// Continue to try again
		}
	}
}

// refill adds tokens based on elapsed time since last refill.
// Must be called with mu held.
func (r *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.tokens += elapsed * r.refillRate
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}
	r.lastRefill = now
}

// Available returns the current number of available tokens.
func (r *RateLimiter) Available() float64 {
	if r == nil {
		return 1 // No rate limiting, always available
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.refill()
	return r.tokens
}
