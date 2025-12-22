// Package ai provides AI-powered content generation for Relicta.
package ai

import (
	"context"
	"testing"
	"time"
)

func TestIsRetryableHTTPStatus(t *testing.T) {
	if !IsRetryableHTTPStatus(429) {
		t.Fatal("expected 429 to be retryable")
	}
	if IsRetryableHTTPStatus(200) {
		t.Fatal("expected 200 to be non-retryable")
	}
}

func TestResilienceStateAndClose(t *testing.T) {
	cfg := ResilienceConfig{
		RateLimitRPM:          0,
		RetryAttempts:         0,
		CircuitBreakerEnabled: false,
	}
	res := NewResilience(cfg)

	if state := res.CircuitBreakerState(); state != "disabled" {
		t.Fatalf("expected disabled circuit breaker, got %s", state)
	}
	if !res.RateLimitAvailable() {
		t.Fatal("expected rate limit available when limiter disabled")
	}
	if err := res.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

func TestResilienceRateLimiterClose(t *testing.T) {
	cfg := ResilienceConfig{
		RateLimitRPM:          1,
		RetryAttempts:         0,
		CircuitBreakerEnabled: false,
	}
	res := NewResilience(cfg)

	if !res.RateLimitAvailable() {
		t.Fatal("expected rate limit to allow initial request")
	}
	if err := res.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

func TestResilienceExecuteNoRetry(t *testing.T) {
	cfg := ResilienceConfig{
		RateLimitRPM:          0,
		RetryAttempts:         0,
		CircuitBreakerEnabled: false,
	}
	res := NewResilience(cfg)

	called := 0
	result, err := res.Execute(context.Background(), func(ctx context.Context) (string, error) {
		called++
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result != "ok" || called != 1 {
		t.Fatalf("unexpected result %q called=%d", result, called)
	}
}

func TestResilienceExecuteRateLimit(t *testing.T) {
	cfg := ResilienceConfig{
		RateLimitRPM:          1,
		RetryAttempts:         0,
		CircuitBreakerEnabled: false,
	}
	res := NewResilience(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	time.Sleep(1 * time.Millisecond)

	if _, err := res.Execute(ctx, func(ctx context.Context) (string, error) { return "ok", nil }); err == nil {
		t.Fatal("expected rate limit error with expired context")
	}
}
