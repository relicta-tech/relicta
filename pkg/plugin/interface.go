// Package plugin provides the public interface for Relicta plugins.
// Plugin authors should implement this interface to create compatible plugins.
package plugin

import (
	"context"
)

// Hook represents a point in the release workflow where plugins can execute.
type Hook string

const (
	// HookPreInit runs before initialization.
	HookPreInit Hook = "pre-init"
	// HookPostInit runs after initialization.
	HookPostInit Hook = "post-init"
	// HookPrePlan runs before planning.
	HookPrePlan Hook = "pre-plan"
	// HookPostPlan runs after planning.
	HookPostPlan Hook = "post-plan"
	// HookPreVersion runs before version bump.
	HookPreVersion Hook = "pre-version"
	// HookPostVersion runs after version bump.
	HookPostVersion Hook = "post-version"
	// HookPreNotes runs before notes generation.
	HookPreNotes Hook = "pre-notes"
	// HookPostNotes runs after notes generation.
	HookPostNotes Hook = "post-notes"
	// HookPreApprove runs before approval.
	HookPreApprove Hook = "pre-approve"
	// HookPostApprove runs after approval.
	HookPostApprove Hook = "post-approve"
	// HookPrePublish runs before publishing.
	HookPrePublish Hook = "pre-publish"
	// HookPostPublish runs after publishing.
	HookPostPublish Hook = "post-publish"
	// HookOnSuccess runs when release succeeds.
	HookOnSuccess Hook = "on-success"
	// HookOnError runs when release fails.
	HookOnError Hook = "on-error"
)

// AllHooks returns all available hooks in execution order.
func AllHooks() []Hook {
	return []Hook{
		HookPreInit, HookPostInit,
		HookPrePlan, HookPostPlan,
		HookPreVersion, HookPostVersion,
		HookPreNotes, HookPostNotes,
		HookPreApprove, HookPostApprove,
		HookPrePublish, HookPostPublish,
		HookOnSuccess, HookOnError,
	}
}

// Plugin is the interface that all plugins must implement.
type Plugin interface {
	// GetInfo returns metadata about the plugin.
	GetInfo() Info

	// Execute runs the plugin for the given hook.
	Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error)

	// Validate validates the plugin configuration.
	// The context allows for cancellation of long-running validation operations.
	Validate(ctx context.Context, config map[string]any) (*ValidateResponse, error)
}

// Info contains metadata about a plugin.
type Info struct {
	// Name is the plugin name.
	Name string `json:"name"`
	// Version is the plugin version.
	Version string `json:"version"`
	// Description is a short description of the plugin.
	Description string `json:"description"`
	// Author is the plugin author.
	Author string `json:"author"`
	// Hooks lists the hooks this plugin supports.
	Hooks []Hook `json:"hooks"`
	// ConfigSchema is a JSON schema for the plugin configuration.
	ConfigSchema string `json:"config_schema,omitempty"`
}

// ExecuteRequest contains the context for plugin execution.
type ExecuteRequest struct {
	// Hook is the hook being executed.
	Hook Hook `json:"hook"`
	// Config is the plugin-specific configuration.
	Config map[string]any `json:"config"`
	// Context contains the release context.
	Context ReleaseContext `json:"context"`
	// DryRun indicates if this is a dry run.
	DryRun bool `json:"dry_run"`
}

// ExecuteResponse contains the result of plugin execution.
type ExecuteResponse struct {
	// Success indicates if the execution was successful.
	Success bool `json:"success"`
	// Message is an optional message about the execution.
	Message string `json:"message,omitempty"`
	// Error is the error message if execution failed.
	Error string `json:"error,omitempty"`
	// Outputs contains any outputs from the plugin.
	Outputs map[string]any `json:"outputs,omitempty"`
	// Artifacts lists any artifacts created by the plugin.
	Artifacts []Artifact `json:"artifacts,omitempty"`
}

// ReleaseContext contains information about the current release.
type ReleaseContext struct {
	// Version is the release version (e.g., "1.2.3").
	Version string `json:"version"`
	// PreviousVersion is the previous release version.
	PreviousVersion string `json:"previous_version,omitempty"`
	// TagName is the full tag name (e.g., "v1.2.3").
	TagName string `json:"tag_name"`
	// ReleaseType is the type of release (major, minor, patch).
	ReleaseType string `json:"release_type"`
	// RepositoryURL is the repository URL.
	RepositoryURL string `json:"repository_url,omitempty"`
	// RepositoryOwner is the repository owner.
	RepositoryOwner string `json:"repository_owner,omitempty"`
	// RepositoryName is the repository name.
	RepositoryName string `json:"repository_name,omitempty"`
	// Branch is the branch being released from.
	Branch string `json:"branch"`
	// CommitSHA is the HEAD commit SHA.
	CommitSHA string `json:"commit_sha"`
	// Changelog is the generated changelog content.
	Changelog string `json:"changelog,omitempty"`
	// ReleaseNotes is the generated release notes.
	ReleaseNotes string `json:"release_notes,omitempty"`
	// Changes contains the categorized changes.
	Changes *CategorizedChanges `json:"changes,omitempty"`
	// Environment contains filtered environment variables.
	Environment map[string]string `json:"environment,omitempty"`
}

// CategorizedChanges contains commits grouped by category.
type CategorizedChanges struct {
	// Features lists feature commits.
	Features []ConventionalCommit `json:"features,omitempty"`
	// Fixes lists bug fix commits.
	Fixes []ConventionalCommit `json:"fixes,omitempty"`
	// Breaking lists breaking change commits.
	Breaking []ConventionalCommit `json:"breaking,omitempty"`
	// Performance lists performance improvement commits.
	Performance []ConventionalCommit `json:"performance,omitempty"`
	// Refactor lists refactoring commits.
	Refactor []ConventionalCommit `json:"refactor,omitempty"`
	// Docs lists documentation commits.
	Docs []ConventionalCommit `json:"docs,omitempty"`
	// Other lists other commits.
	Other []ConventionalCommit `json:"other,omitempty"`
}

// ConventionalCommit represents a parsed conventional commit.
type ConventionalCommit struct {
	// Hash is the commit hash.
	Hash string `json:"hash"`
	// Type is the commit type (feat, fix, etc.).
	Type string `json:"type"`
	// Scope is the optional scope.
	Scope string `json:"scope,omitempty"`
	// Description is the commit description.
	Description string `json:"description"`
	// Body is the commit body.
	Body string `json:"body,omitempty"`
	// Breaking indicates if this is a breaking change.
	Breaking bool `json:"breaking"`
	// BreakingDescription is the breaking change description.
	BreakingDescription string `json:"breaking_description,omitempty"`
	// Issues lists referenced issues.
	Issues []string `json:"issues,omitempty"`
	// Author is the commit author.
	Author string `json:"author,omitempty"`
	// Date is the commit date.
	Date string `json:"date,omitempty"`
}

// Artifact represents a file or resource created by a plugin.
type Artifact struct {
	// Name is the artifact name.
	Name string `json:"name"`
	// Path is the artifact path (local file or URL).
	Path string `json:"path"`
	// Type is the artifact type (file, url, etc.).
	Type string `json:"type"`
	// Size is the artifact size in bytes.
	Size int64 `json:"size,omitempty"`
	// Checksum is the artifact checksum.
	Checksum string `json:"checksum,omitempty"`
}

// ValidateResponse contains the result of configuration validation.
type ValidateResponse struct {
	// Valid indicates if the configuration is valid.
	Valid bool `json:"valid"`
	// Errors lists validation errors.
	Errors []ValidationError `json:"errors,omitempty"`
}

// ValidationError represents a configuration validation error.
type ValidationError struct {
	// Field is the field that failed validation.
	Field string `json:"field"`
	// Message is the error message.
	Message string `json:"message"`
	// Code is an optional error code.
	Code string `json:"code,omitempty"`
}
