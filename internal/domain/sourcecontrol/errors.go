// Package sourcecontrol provides domain types for source control operations.
package sourcecontrol

import "errors"

// Domain errors for source control operations.
var (
	// ErrRepositoryNotFound indicates the repository was not found.
	ErrRepositoryNotFound = errors.New("repository not found")

	// ErrNotARepository indicates the path is not a git repository.
	ErrNotARepository = errors.New("not a git repository")

	// ErrCommitNotFound indicates the commit was not found.
	ErrCommitNotFound = errors.New("commit not found")

	// ErrTagNotFound indicates the tag was not found.
	ErrTagNotFound = errors.New("tag not found")

	// ErrTagAlreadyExists indicates the tag already exists.
	ErrTagAlreadyExists = errors.New("tag already exists")

	// ErrBranchNotFound indicates the branch was not found.
	ErrBranchNotFound = errors.New("branch not found")

	// ErrRemoteNotFound indicates the remote was not found.
	ErrRemoteNotFound = errors.New("remote not found")

	// ErrWorkingTreeDirty indicates the working tree has uncommitted changes.
	ErrWorkingTreeDirty = errors.New("working tree has uncommitted changes")

	// ErrNoCommits indicates no commits were found.
	ErrNoCommits = errors.New("no commits found")

	// ErrNoTags indicates no tags were found.
	ErrNoTags = errors.New("no tags found")

	// ErrPushFailed indicates a push operation failed.
	ErrPushFailed = errors.New("push failed")

	// ErrFetchFailed indicates a fetch operation failed.
	ErrFetchFailed = errors.New("fetch failed")

	// ErrAuthenticationRequired indicates authentication is required.
	ErrAuthenticationRequired = errors.New("authentication required")
)
