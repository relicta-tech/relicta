// Package sourcecontrol provides domain types for source control operations.
package sourcecontrol

import (
	"errors"
	"testing"
)

func TestDomainErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{"ErrRepositoryNotFound", ErrRepositoryNotFound, "repository not found"},
		{"ErrNotARepository", ErrNotARepository, "not a git repository"},
		{"ErrCommitNotFound", ErrCommitNotFound, "commit not found"},
		{"ErrTagNotFound", ErrTagNotFound, "tag not found"},
		{"ErrTagAlreadyExists", ErrTagAlreadyExists, "tag already exists"},
		{"ErrBranchNotFound", ErrBranchNotFound, "branch not found"},
		{"ErrRemoteNotFound", ErrRemoteNotFound, "remote not found"},
		{"ErrWorkingTreeDirty", ErrWorkingTreeDirty, "working tree has uncommitted changes"},
		{"ErrNoCommits", ErrNoCommits, "no commits found"},
		{"ErrNoTags", ErrNoTags, "no tags found"},
		{"ErrPushFailed", ErrPushFailed, "push failed"},
		{"ErrFetchFailed", ErrFetchFailed, "fetch failed"},
		{"ErrAuthenticationRequired", ErrAuthenticationRequired, "authentication required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.msg {
				t.Errorf("%s.Error() = %v, want %v", tt.name, tt.err.Error(), tt.msg)
			}
		})
	}
}

func TestDomainErrors_NotNil(t *testing.T) {
	errs := []error{
		ErrRepositoryNotFound,
		ErrNotARepository,
		ErrCommitNotFound,
		ErrTagNotFound,
		ErrTagAlreadyExists,
		ErrBranchNotFound,
		ErrRemoteNotFound,
		ErrWorkingTreeDirty,
		ErrNoCommits,
		ErrNoTags,
		ErrPushFailed,
		ErrFetchFailed,
		ErrAuthenticationRequired,
	}

	for _, err := range errs {
		if err == nil {
			t.Error("Domain error should not be nil")
		}
	}
}

func TestDomainErrors_Unique(t *testing.T) {
	errs := []error{
		ErrRepositoryNotFound,
		ErrNotARepository,
		ErrCommitNotFound,
		ErrTagNotFound,
		ErrTagAlreadyExists,
		ErrBranchNotFound,
		ErrRemoteNotFound,
		ErrWorkingTreeDirty,
		ErrNoCommits,
		ErrNoTags,
		ErrPushFailed,
		ErrFetchFailed,
		ErrAuthenticationRequired,
	}

	seen := make(map[string]bool)
	for _, err := range errs {
		msg := err.Error()
		if seen[msg] {
			t.Errorf("Duplicate error message: %v", msg)
		}
		seen[msg] = true
	}
}

func TestDomainErrors_ErrorsIs(t *testing.T) {
	// Test that errors.Is works correctly with domain errors
	wrappedErr := errors.New("wrapped: " + ErrTagNotFound.Error())

	// Direct comparison should work
	if !errors.Is(ErrTagNotFound, ErrTagNotFound) {
		t.Error("errors.Is should return true for same error")
	}

	// Different errors should not match
	if errors.Is(ErrTagNotFound, ErrCommitNotFound) {
		t.Error("errors.Is should return false for different errors")
	}

	// Wrapped errors won't match with errors.Is unless we use %w
	if errors.Is(wrappedErr, ErrTagNotFound) {
		t.Error("String concatenation does not create an error chain")
	}
}
