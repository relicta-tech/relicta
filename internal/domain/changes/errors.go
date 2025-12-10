// Package changes provides domain types for analyzing commit changes.
package changes

import "errors"

// Domain errors for changes operations.
var (
	// ErrInvalidCommitMessage indicates an invalid conventional commit message.
	ErrInvalidCommitMessage = errors.New("invalid conventional commit message")

	// ErrEmptyChangeSet indicates an empty changeset.
	ErrEmptyChangeSet = errors.New("changeset is empty")

	// ErrNoCommitsFound indicates no commits were found in the specified range.
	ErrNoCommitsFound = errors.New("no commits found in the specified range")

	// ErrInvalidCommitType indicates an unrecognized commit type.
	ErrInvalidCommitType = errors.New("invalid commit type")

	// ErrInvalidReleaseType indicates an unrecognized release type.
	ErrInvalidReleaseType = errors.New("invalid release type")
)
