// Package ai provides AI-powered content generation for ReleasePilot.
package ai

import (
	"context"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	tests := []struct {
		name string
		rpm  int
		want bool // whether limiter should be created (non-nil)
	}{
		{"positive rpm", 60, true},
		{"zero rpm", 0, false},
		{"negative rpm", -1, false},
		{"small rpm", 1, true},
		{"large rpm", 1000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewRateLimiter(tt.rpm)
			if tt.want && limiter == nil {
				t.Error("Expected limiter to be created, got nil")
			}
			if !tt.want && limiter != nil {
				t.Error("Expected limiter to be nil")
			}
		})
	}
}

func TestRateLimiter_Wait_NilLimiter(t *testing.T) {
	var limiter *RateLimiter = nil
	ctx := context.Background()

	err := limiter.Wait(ctx)
	if err != nil {
		t.Errorf("Wait on nil limiter should return nil, got %v", err)
	}
}

func TestRateLimiter_Wait_ImmediateAvailability(t *testing.T) {
	limiter := NewRateLimiter(60)
	ctx := context.Background()

	// First few requests should be immediate
	for i := 0; i < 5; i++ {
		start := time.Now()
		err := limiter.Wait(ctx)
		if err != nil {
			t.Errorf("Wait failed: %v", err)
		}
		elapsed := time.Since(start)
		if elapsed > 100*time.Millisecond {
			t.Errorf("Wait took too long: %v", elapsed)
		}
	}
}

func TestRateLimiter_Wait_ContextCanceled(t *testing.T) {
	// Create a limiter with very low rate
	limiter := NewRateLimiter(1)
	ctx, cancel := context.WithCancel(context.Background())

	// Exhaust the token
	err := limiter.Wait(ctx)
	if err != nil {
		t.Fatalf("First wait failed: %v", err)
	}

	// Cancel context before second wait completes
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// This should be canceled
	err = limiter.Wait(ctx)
	if err == nil {
		t.Error("Expected context canceled error")
	}
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestRateLimiter_Wait_ContextTimeout(t *testing.T) {
	// Create a limiter with very low rate
	limiter := NewRateLimiter(1)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Exhaust the token
	err := limiter.Wait(ctx)
	if err != nil {
		t.Fatalf("First wait failed: %v", err)
	}

	// This should timeout
	err = limiter.Wait(ctx)
	if err == nil {
		t.Error("Expected timeout error")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

func TestRateLimiter_Available_NilLimiter(t *testing.T) {
	var limiter *RateLimiter = nil

	available := limiter.Available()
	if available != 1 {
		t.Errorf("Available on nil limiter should return 1, got %v", available)
	}
}

func TestRateLimiter_Available_InitialTokens(t *testing.T) {
	limiter := NewRateLimiter(60)

	available := limiter.Available()
	if available != 60 {
		t.Errorf("Initial available tokens should be 60, got %v", available)
	}
}

func TestRateLimiter_Available_AfterWait(t *testing.T) {
	limiter := NewRateLimiter(60)
	ctx := context.Background()

	// Consume one token
	err := limiter.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	available := limiter.Available()
	// Should be less than 60 (minus 1 consumed, plus any refill)
	if available >= 60 {
		t.Errorf("Available tokens should be less than 60 after Wait, got %v", available)
	}
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	// Create a limiter with 60 rpm (1 token per second)
	limiter := NewRateLimiter(60)
	ctx := context.Background()

	// Consume all tokens
	initialTokens := int(limiter.Available())
	for i := 0; i < initialTokens; i++ {
		err := limiter.Wait(ctx)
		if err != nil {
			t.Fatalf("Wait failed at iteration %d: %v", i, err)
		}
	}

	// Wait a bit for refill
	time.Sleep(100 * time.Millisecond)

	// Should have some tokens again
	available := limiter.Available()
	if available <= 0 {
		t.Errorf("Tokens should refill over time, got %v", available)
	}
}

func TestRateLimiter_MaxTokensCap(t *testing.T) {
	limiter := NewRateLimiter(10)

	// Wait to allow refill
	time.Sleep(200 * time.Millisecond)

	// Available should not exceed max
	available := limiter.Available()
	if available > 10 {
		t.Errorf("Available tokens should not exceed max (10), got %v", available)
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewRateLimiter(100)
	ctx := context.Background()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 5; j++ {
				err := limiter.Wait(ctx)
				if err != nil {
					t.Errorf("Concurrent Wait failed: %v", err)
				}
				_ = limiter.Available()
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestRateLimiter_Wait_WithTokenRefill(t *testing.T) {
	// Create a rate limiter with 1 request per second (slow enough to test waiting)
	limiter := NewRateLimiter(1)
	ctx := context.Background()

	// Use all tokens
	err := limiter.Wait(ctx)
	if err != nil {
		t.Fatalf("First Wait failed: %v", err)
	}

	// Second wait should have to wait for refill (but should complete quickly due to refill)
	start := time.Now()
	err = limiter.Wait(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Second Wait failed: %v", err)
	}

	// Should have had to wait at least a bit (but not the full second due to refill logic)
	if elapsed < 10*time.Millisecond {
		t.Errorf("Expected some wait time, but got %v", elapsed)
	}
}

func TestRateLimiter_Wait_MinimumWaitTime(t *testing.T) {
	// Create a limiter with very low rate to ensure minimum wait time is enforced
	limiter := NewRateLimiter(1)
	ctx := context.Background()

	// Consume initial token
	_ = limiter.Wait(ctx)

	// Next wait should enforce minimum wait time of 10ms
	start := time.Now()
	err := limiter.Wait(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Should have waited at least the minimum
	if elapsed < 10*time.Millisecond {
		t.Errorf("Expected minimum wait time of 10ms, got %v", elapsed)
	}
}
