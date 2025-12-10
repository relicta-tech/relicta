// Package release provides domain types for release management.
package release

import "errors"

// Domain errors for release operations.
var (
	// ErrReleaseNotFound indicates a release was not found.
	ErrReleaseNotFound = errors.New("release not found")

	// ErrInvalidStateTransition indicates an invalid state transition.
	ErrInvalidStateTransition = errors.New("invalid state transition")

	// ErrReleaseAlreadyExists indicates a release already exists.
	ErrReleaseAlreadyExists = errors.New("release already exists")

	// ErrNilPlan indicates a nil release plan was provided.
	ErrNilPlan = errors.New("release plan cannot be nil")

	// ErrNilNotes indicates nil release notes were provided.
	ErrNilNotes = errors.New("release notes cannot be nil")

	// ErrNotApproved indicates the release is not approved.
	ErrNotApproved = errors.New("release is not approved")

	// ErrAlreadyPublished indicates the release is already published.
	ErrAlreadyPublished = errors.New("release is already published")

	// ErrNoChanges indicates there are no changes to release.
	ErrNoChanges = errors.New("no changes to release")

	// ErrCannotCancel indicates the release cannot be canceled.
	ErrCannotCancel = errors.New("release cannot be canceled in current state")

	// ErrCannotRetry indicates the release cannot be retried.
	ErrCannotRetry = errors.New("release cannot be retried in current state")
)
