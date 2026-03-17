// Package retry provides exponential backoff with jitter for webhook delivery retries.
// It supports Retry-After header parsing and configurable maximum retry limits.
package retry

import (
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

// DefaultMaxRetries is the default maximum number of retries before dead-lettering.
const DefaultMaxRetries = 5

// DefaultBaseDelay is the initial backoff delay.
const DefaultBaseDelay = 1 * time.Second

// DefaultMaxDelay caps the maximum backoff delay.
const DefaultMaxDelay = 5 * time.Minute

// DefaultJitterFraction is the proportion of jitter added to the delay (0.0-1.0).
const DefaultJitterFraction = 0.25

// Config holds retry configuration.
type Config struct {
	// MaxRetries is the maximum number of retry attempts before dead-lettering.
	MaxRetries int
	// BaseDelay is the initial backoff delay before the first retry.
	BaseDelay time.Duration
	// MaxDelay caps the maximum backoff delay.
	MaxDelay time.Duration
	// JitterFraction is the proportion of jitter (0.0 to 1.0) added to the delay.
	JitterFraction float64
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxRetries:     DefaultMaxRetries,
		BaseDelay:      DefaultBaseDelay,
		MaxDelay:       DefaultMaxDelay,
		JitterFraction: DefaultJitterFraction,
	}
}

// NextDelay computes the backoff delay for the given attempt number (0-indexed).
// The delay follows exponential growth: baseDelay * 2^attempt, capped at maxDelay,
// with random jitter applied.
func (c Config) NextDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	// Exponential: baseDelay * 2^attempt
	delay := float64(c.BaseDelay) * math.Pow(2, float64(attempt))

	// Cap at max delay
	maxDelay := float64(c.MaxDelay)
	if maxDelay > 0 && delay > maxDelay {
		delay = maxDelay
	}

	// Apply jitter: delay +/- (jitterFraction * delay)
	if c.JitterFraction > 0 {
		jitter := delay * c.JitterFraction
		delay = delay - jitter + (rand.Float64() * 2 * jitter)
	}

	if delay < 0 {
		delay = 0
	}

	return time.Duration(delay)
}

// ShouldRetry returns true if the attempt number is below the maximum retries.
func (c Config) ShouldRetry(attempt int) bool {
	return attempt < c.MaxRetries
}

// NextAttemptTime returns the absolute time for the next retry attempt,
// taking into account the Retry-After header from the response (if any).
// If the Retry-After header specifies a longer delay than the computed backoff,
// the Retry-After value is used instead.
func (c Config) NextAttemptTime(attempt int, resp *http.Response) time.Time {
	backoff := c.NextDelay(attempt)

	if resp != nil {
		retryAfter := ParseRetryAfter(resp)
		if retryAfter > backoff {
			backoff = retryAfter
		}
	}

	return time.Now().UTC().Add(backoff)
}

// ParseRetryAfter extracts the retry delay from a Retry-After response header.
// It supports both delta-seconds and HTTP-date formats per RFC 7231 section 7.1.3.
// Returns 0 if the header is absent or unparseable.
func ParseRetryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}

	val := resp.Header.Get("Retry-After")
	if val == "" {
		return 0
	}

	// Try delta-seconds first (e.g., "120")
	if seconds, err := strconv.Atoi(val); err == nil {
		if seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
		return 0
	}

	// Try HTTP-date format (e.g., "Fri, 31 Dec 1999 23:59:59 GMT")
	if t, err := http.ParseTime(val); err == nil {
		delta := time.Until(t)
		if delta > 0 {
			return delta
		}
	}

	return 0
}
