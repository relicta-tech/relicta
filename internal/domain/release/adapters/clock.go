// Package adapters provides infrastructure implementations for the release governance domain.
package adapters

import (
	"time"

	"github.com/relicta-tech/relicta/internal/domain/release/ports"
)

// RealClock implements ports.Clock using the system time.
type RealClock struct{}

// Ensure RealClock implements the interface.
var _ ports.Clock = (*RealClock)(nil)

// Now returns the current system time.
func (RealClock) Now() time.Time {
	return time.Now()
}

// NewRealClock creates a new RealClock instance.
func NewRealClock() *RealClock {
	return &RealClock{}
}
