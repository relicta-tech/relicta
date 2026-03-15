// Package version provides domain types for semantic versioning.
package version

import "errors"

// Domain errors for version operations.
var (
	// ErrInvalidVersion indicates an invalid version string.
	ErrInvalidVersion = errors.New("invalid semantic version")

	// ErrInvalidBumpType indicates an invalid bump type.
	ErrInvalidBumpType = errors.New("invalid bump type")

	// ErrVersionNotFound indicates a version was not found.
	ErrVersionNotFound = errors.New("version not found")

	// ErrCannotDowngrade indicates an attempt to downgrade a version.
	ErrCannotDowngrade = errors.New("cannot downgrade version")

	// ErrUnknownChannel indicates an unrecognized release channel.
	ErrUnknownChannel = errors.New("unknown release channel")

	// ErrInvalidPromotion indicates an invalid channel promotion.
	ErrInvalidPromotion = errors.New("invalid channel promotion")

	// ErrNoChannelVersions indicates no versions were found for a channel.
	ErrNoChannelVersions = errors.New("no versions found for channel")
)
