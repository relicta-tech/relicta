// Package ai provides AI-powered content generation for Relicta.
package ai

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/felixgeelhaar/fortify/circuitbreaker"
	"github.com/felixgeelhaar/fortify/ratelimit"
	"github.com/felixgeelhaar/fortify/retry"
)

// ResilienceConfig configures resilience patterns for AI services.
type ResilienceConfig struct {
	// Rate limiting
	RateLimitRPM int // Requests per minute (0 = disabled)

	// Retry configuration
	RetryAttempts    int
	RetryInitialWait time.Duration
	RetryMaxWait     time.Duration

	// Circuit breaker
	CircuitBreakerEnabled     bool
	CircuitBreakerThreshold   int           // failures before opening
	CircuitBreakerTimeout     time.Duration // how long to stay open
	CircuitBreakerMaxRequests int           // requests allowed in half-open
}

// DefaultResilienceConfig returns sensible defaults for AI services.
func DefaultResilienceConfig() ResilienceConfig {
	return ResilienceConfig{
		RateLimitRPM:              60,
		RetryAttempts:             3,
		RetryInitialWait:          500 * time.Millisecond,
		RetryMaxWait:              10 * time.Second,
		CircuitBreakerEnabled:     true,
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     30 * time.Second,
		CircuitBreakerMaxRequests: 3,
	}
}

// Resilience wraps Fortify resilience patterns for AI operations.
type Resilience struct {
	rateLimiter    ratelimit.RateLimiter
	retrier        retry.Retry[string]
	circuitBreaker circuitbreaker.CircuitBreaker[string]
	config         ResilienceConfig
}

// NewResilience creates a new resilience wrapper with the given configuration.
func NewResilience(cfg ResilienceConfig) *Resilience {
	r := &Resilience{config: cfg}

	// Configure rate limiter if enabled
	if cfg.RateLimitRPM > 0 {
		r.rateLimiter = ratelimit.New(&ratelimit.Config{
			Rate:     cfg.RateLimitRPM,
			Burst:    cfg.RateLimitRPM * 2, // Allow burst up to 2x rate
			Interval: time.Minute,
		})
	}

	// Configure retry with exponential backoff and jitter
	if cfg.RetryAttempts > 0 {
		r.retrier = retry.New[string](retry.Config{
			MaxAttempts:   cfg.RetryAttempts,
			InitialDelay:  cfg.RetryInitialWait,
			MaxDelay:      cfg.RetryMaxWait,
			BackoffPolicy: retry.BackoffExponential,
			Multiplier:    2.0,
			Jitter:        true,
			IsRetryable:   isRetryableError,
		})
	}

	// Configure circuit breaker if enabled
	if cfg.CircuitBreakerEnabled {
		threshold := cfg.CircuitBreakerThreshold
		r.circuitBreaker = circuitbreaker.New[string](circuitbreaker.Config{
			MaxRequests: uint32(cfg.CircuitBreakerMaxRequests), // #nosec G115 -- bounded config value
			Interval:    cfg.CircuitBreakerTimeout,
			Timeout:     cfg.CircuitBreakerTimeout,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= uint32(threshold) // #nosec G115 -- bounded config value
			},
		})
	}

	return r
}

// Execute runs the operation with all configured resilience patterns.
// Order: Rate Limit → Circuit Breaker → Retry → Operation
func (r *Resilience) Execute(ctx context.Context, operation func(context.Context) (string, error)) (string, error) {
	if r == nil {
		return operation(ctx)
	}

	// 1. Rate limiting - wait for token
	if r.rateLimiter != nil {
		if err := r.rateLimiter.Wait(ctx, "ai-operation"); err != nil {
			return "", err
		}
	}

	// 2. Circuit breaker wraps the retryable operation
	if r.circuitBreaker != nil {
		return r.circuitBreaker.Execute(ctx, func(ctx context.Context) (string, error) {
			return r.executeWithRetry(ctx, operation)
		})
	}

	// Without circuit breaker, just use retry
	return r.executeWithRetry(ctx, operation)
}

// executeWithRetry runs the operation with retry logic.
func (r *Resilience) executeWithRetry(ctx context.Context, operation func(context.Context) (string, error)) (string, error) {
	if r.retrier != nil {
		return r.retrier.Do(ctx, operation)
	}
	return operation(ctx)
}

// isRetryableError determines if an error is worth retrying.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Don't retry context errors
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Rate limit errors are retryable
	if strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "429") {
		return true
	}

	// Server errors (5xx) are retryable
	if strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "internal server error") ||
		strings.Contains(errStr, "bad gateway") ||
		strings.Contains(errStr, "service unavailable") ||
		strings.Contains(errStr, "gateway timeout") {
		return true
	}

	// Network errors are retryable
	if strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "temporary") {
		return true
	}

	// Client errors (4xx except rate limit) are not retryable
	if strings.Contains(errStr, "400") ||
		strings.Contains(errStr, "401") ||
		strings.Contains(errStr, "403") ||
		strings.Contains(errStr, "404") {
		return false
	}

	// Default: retry on unknown errors
	return true
}

// IsRetryableHTTPStatus returns true for HTTP status codes worth retrying.
func IsRetryableHTTPStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// CircuitBreakerState returns the current state of the circuit breaker.
// Returns "closed", "half-open", "open", or "disabled".
func (r *Resilience) CircuitBreakerState() string {
	if r == nil || r.circuitBreaker == nil {
		return "disabled"
	}
	return r.circuitBreaker.State().String()
}

// RateLimitAvailable returns true if there are tokens available for rate limiting.
func (r *Resilience) RateLimitAvailable() bool {
	if r == nil || r.rateLimiter == nil {
		return true
	}
	return r.rateLimiter.Allow(context.Background(), "ai-operation")
}

// Close releases resources held by resilience components.
func (r *Resilience) Close() error {
	if r == nil {
		return nil
	}
	if r.rateLimiter != nil {
		return r.rateLimiter.Close()
	}
	return nil
}
