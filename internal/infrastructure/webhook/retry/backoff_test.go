package retry

import (
	"math"
	"net/http"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxRetries != DefaultMaxRetries {
		t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, DefaultMaxRetries)
	}
	if cfg.BaseDelay != DefaultBaseDelay {
		t.Errorf("BaseDelay = %v, want %v", cfg.BaseDelay, DefaultBaseDelay)
	}
	if cfg.MaxDelay != DefaultMaxDelay {
		t.Errorf("MaxDelay = %v, want %v", cfg.MaxDelay, DefaultMaxDelay)
	}
	if cfg.JitterFraction != DefaultJitterFraction {
		t.Errorf("JitterFraction = %f, want %f", cfg.JitterFraction, DefaultJitterFraction)
	}
}

func TestNextDelay_ExponentialGrowth(t *testing.T) {
	cfg := Config{
		BaseDelay:      1 * time.Second,
		MaxDelay:       10 * time.Minute,
		JitterFraction: 0, // No jitter for deterministic test
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 32 * time.Second},
	}

	for _, tt := range tests {
		got := cfg.NextDelay(tt.attempt)
		if got != tt.expected {
			t.Errorf("NextDelay(%d) = %v, want %v", tt.attempt, got, tt.expected)
		}
	}
}

func TestNextDelay_CapsAtMaxDelay(t *testing.T) {
	cfg := Config{
		BaseDelay:      1 * time.Second,
		MaxDelay:       30 * time.Second,
		JitterFraction: 0,
	}

	// Attempt 10 would be 1024s without cap
	got := cfg.NextDelay(10)
	if got != 30*time.Second {
		t.Errorf("NextDelay(10) = %v, want %v (capped)", got, 30*time.Second)
	}
}

func TestNextDelay_WithJitter(t *testing.T) {
	cfg := Config{
		BaseDelay:      1 * time.Second,
		MaxDelay:       10 * time.Minute,
		JitterFraction: 0.25,
	}

	// Run multiple times to verify jitter range
	base := float64(1 * time.Second) // attempt 0
	minExpected := base * (1 - 0.25)
	maxExpected := base * (1 + 0.25)

	for i := 0; i < 100; i++ {
		got := float64(cfg.NextDelay(0))
		if got < minExpected || got > maxExpected {
			t.Errorf("NextDelay(0) = %v, want in range [%v, %v]",
				time.Duration(got), time.Duration(minExpected), time.Duration(maxExpected))
		}
	}

	// Verify that values are not all the same (jitter is working)
	seen := make(map[time.Duration]bool)
	for i := 0; i < 50; i++ {
		seen[cfg.NextDelay(0)] = true
	}
	if len(seen) < 2 {
		t.Error("NextDelay with jitter should produce varying values")
	}
}

func TestNextDelay_NegativeAttempt(t *testing.T) {
	cfg := Config{
		BaseDelay:      1 * time.Second,
		MaxDelay:       10 * time.Minute,
		JitterFraction: 0,
	}

	got := cfg.NextDelay(-1)
	if got != 1*time.Second {
		t.Errorf("NextDelay(-1) = %v, want %v", got, 1*time.Second)
	}
}

func TestShouldRetry(t *testing.T) {
	cfg := Config{MaxRetries: 5}

	tests := []struct {
		attempt int
		want    bool
	}{
		{0, true},
		{1, true},
		{4, true},
		{5, false},
		{6, false},
		{100, false},
	}

	for _, tt := range tests {
		got := cfg.ShouldRetry(tt.attempt)
		if got != tt.want {
			t.Errorf("ShouldRetry(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestParseRetryAfter_DeltaSeconds(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{},
	}
	resp.Header.Set("Retry-After", "120")

	got := ParseRetryAfter(resp)
	if got != 120*time.Second {
		t.Errorf("ParseRetryAfter('120') = %v, want %v", got, 120*time.Second)
	}
}

func TestParseRetryAfter_HTTPDate(t *testing.T) {
	future := time.Now().UTC().Add(60 * time.Second)
	resp := &http.Response{
		Header: http.Header{},
	}
	resp.Header.Set("Retry-After", future.Format(http.TimeFormat))

	got := ParseRetryAfter(resp)
	// Allow some tolerance for test execution time
	if got < 55*time.Second || got > 65*time.Second {
		t.Errorf("ParseRetryAfter(HTTP-date) = %v, want ~60s", got)
	}
}

func TestParseRetryAfter_Empty(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{},
	}

	got := ParseRetryAfter(resp)
	if got != 0 {
		t.Errorf("ParseRetryAfter(empty) = %v, want 0", got)
	}
}

func TestParseRetryAfter_NilResponse(t *testing.T) {
	got := ParseRetryAfter(nil)
	if got != 0 {
		t.Errorf("ParseRetryAfter(nil) = %v, want 0", got)
	}
}

func TestParseRetryAfter_Invalid(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{},
	}
	resp.Header.Set("Retry-After", "not-a-number-or-date")

	got := ParseRetryAfter(resp)
	if got != 0 {
		t.Errorf("ParseRetryAfter('invalid') = %v, want 0", got)
	}
}

func TestParseRetryAfter_ZeroSeconds(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{},
	}
	resp.Header.Set("Retry-After", "0")

	got := ParseRetryAfter(resp)
	if got != 0 {
		t.Errorf("ParseRetryAfter('0') = %v, want 0", got)
	}
}

func TestNextAttemptTime_UsesLargerOfBackoffAndRetryAfter(t *testing.T) {
	cfg := Config{
		BaseDelay:      1 * time.Second,
		MaxDelay:       10 * time.Minute,
		JitterFraction: 0,
	}

	// Case 1: Retry-After > backoff
	resp := &http.Response{
		Header: http.Header{},
	}
	resp.Header.Set("Retry-After", "300") // 5 minutes

	before := time.Now().UTC()
	got := cfg.NextAttemptTime(0, resp) // backoff = 1s, Retry-After = 300s
	expected := before.Add(300 * time.Second)

	if got.Before(expected.Add(-1 * time.Second)) {
		t.Errorf("NextAttemptTime should use Retry-After (300s), got %v", got.Sub(before))
	}

	// Case 2: backoff > Retry-After
	resp2 := &http.Response{
		Header: http.Header{},
	}
	resp2.Header.Set("Retry-After", "1") // 1 second

	before2 := time.Now().UTC()
	got2 := cfg.NextAttemptTime(5, resp2) // backoff = 32s, Retry-After = 1s
	minExpected := before2.Add(32 * time.Second)

	if got2.Before(minExpected.Add(-1 * time.Second)) {
		t.Errorf("NextAttemptTime should use backoff (32s), got %v", got2.Sub(before2))
	}
}

func TestNextAttemptTime_NilResponse(t *testing.T) {
	cfg := Config{
		BaseDelay:      1 * time.Second,
		MaxDelay:       10 * time.Minute,
		JitterFraction: 0,
	}

	before := time.Now().UTC()
	got := cfg.NextAttemptTime(0, nil)
	after := time.Now().UTC().Add(2 * time.Second)

	if got.Before(before) || got.After(after) {
		t.Errorf("NextAttemptTime with nil response should use backoff only, got %v", got)
	}
}

func TestExponentialGrowthPattern(t *testing.T) {
	cfg := Config{
		BaseDelay:      500 * time.Millisecond,
		MaxDelay:       1 * time.Hour,
		JitterFraction: 0,
	}

	// Verify each step is exactly 2x the previous
	for i := 0; i < 10; i++ {
		delay := cfg.NextDelay(i)
		expected := time.Duration(float64(500*time.Millisecond) * math.Pow(2, float64(i)))
		if delay != expected {
			t.Errorf("attempt %d: delay = %v, want %v", i, delay, expected)
		}
	}
}
