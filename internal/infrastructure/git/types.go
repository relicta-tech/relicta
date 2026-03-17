// Package git provides git operations for Relicta.
package git

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
)

// CommitType is an alias to the domain CommitType for backward compatibility.
// New code should prefer using changes.CommitType directly.
type CommitType = changes.CommitType

// CommitType constants - aliases to domain layer constants.
const (
	CommitTypeFeat     = changes.CommitTypeFeat
	CommitTypeFix      = changes.CommitTypeFix
	CommitTypeDocs     = changes.CommitTypeDocs
	CommitTypeStyle    = changes.CommitTypeStyle
	CommitTypeRefactor = changes.CommitTypeRefactor
	CommitTypePerf     = changes.CommitTypePerf
	CommitTypeTest     = changes.CommitTypeTest
	CommitTypeBuild    = changes.CommitTypeBuild
	CommitTypeCI       = changes.CommitTypeCI
	CommitTypeChore    = changes.CommitTypeChore
	CommitTypeRevert   = changes.CommitTypeRevert
	// CommitTypeUnknown represents an unknown commit type.
	CommitTypeUnknown CommitType = ""
)

// ReleaseType is an alias to the domain ReleaseType for backward compatibility.
// New code should prefer using changes.ReleaseType directly.
type ReleaseType = changes.ReleaseType

// ReleaseType constants - aliases to domain layer constants.
const (
	ReleaseTypeMajor = changes.ReleaseTypeMajor
	ReleaseTypeMinor = changes.ReleaseTypeMinor
	ReleaseTypePatch = changes.ReleaseTypePatch
	ReleaseTypeNone  = changes.ReleaseTypeNone
)

// Commit represents a git commit.
type Commit struct {
	// Hash is the commit SHA.
	Hash string `json:"hash"`
	// ShortHash is the abbreviated commit SHA (7 characters).
	ShortHash string `json:"short_hash"`
	// Message is the full commit message.
	Message string `json:"message"`
	// Subject is the first line of the commit message.
	Subject string `json:"subject"`
	// Body is the commit message body (everything after the first line).
	Body string `json:"body"`
	// Author is the commit author.
	Author Author `json:"author"`
	// Committer is the person who made the commit.
	Committer Author `json:"committer"`
	// Date is the commit date.
	Date time.Time `json:"date"`
	// Parents are the parent commit hashes.
	Parents []string `json:"parents"`
}

// Author represents a git author or committer.
type Author struct {
	// Name is the author's name.
	Name string `json:"name"`
	// Email is the author's email.
	Email string `json:"email"`
}

// ConventionalCommit represents a parsed conventional commit.
type ConventionalCommit struct {
	// Commit is the underlying git commit.
	Commit Commit `json:"commit"`
	// Type is the commit type (feat, fix, etc.).
	Type CommitType `json:"type"`
	// Scope is the optional scope of the commit.
	Scope string `json:"scope,omitempty"`
	// Description is the commit description (after type and scope).
	Description string `json:"description"`
	// Body is the commit body.
	Body string `json:"body,omitempty"`
	// Footer is the commit footer (contains breaking change info, references, etc.).
	Footer string `json:"footer,omitempty"`
	// Breaking indicates if this is a breaking change.
	Breaking bool `json:"breaking"`
	// BreakingDescription is the description of the breaking change.
	BreakingDescription string `json:"breaking_description,omitempty"`
	// References are issue/PR references found in the commit.
	References []Reference `json:"references,omitempty"`
	// IsConventional indicates if the commit follows conventional commit format.
	IsConventional bool `json:"is_conventional"`
}

// Reference represents a reference to an issue or PR.
type Reference struct {
	// Type is the reference type (e.g., "issue", "pr", "closes", "fixes").
	Type string `json:"type"`
	// ID is the reference ID (issue/PR number).
	ID string `json:"id"`
	// Raw is the raw reference string as found in the commit.
	Raw string `json:"raw"`
}

// Tag represents a git tag.
type Tag struct {
	// Name is the tag name.
	Name string `json:"name"`
	// Hash is the commit hash the tag points to.
	Hash string `json:"hash"`
	// Message is the tag message (for annotated tags).
	Message string `json:"message,omitempty"`
	// Tagger is the person who created the tag (for annotated tags).
	Tagger *Author `json:"tagger,omitempty"`
	// Date is the tag date.
	Date time.Time `json:"date"`
	// IsAnnotated indicates if this is an annotated tag.
	IsAnnotated bool `json:"is_annotated"`
}

// Branch represents a git branch.
type Branch struct {
	// Name is the branch name.
	Name string `json:"name"`
	// Hash is the commit hash the branch points to.
	Hash string `json:"hash"`
	// IsRemote indicates if this is a remote branch.
	IsRemote bool `json:"is_remote"`
	// Remote is the remote name (for remote branches).
	Remote string `json:"remote,omitempty"`
	// Upstream is the upstream branch name.
	Upstream string `json:"upstream,omitempty"`
}

// CategorizedChanges groups commits by their type for changelog generation.
type CategorizedChanges struct {
	// Features contains feat commits.
	Features []ConventionalCommit `json:"features,omitempty"`
	// Fixes contains fix commits.
	Fixes []ConventionalCommit `json:"fixes,omitempty"`
	// Performance contains perf commits.
	Performance []ConventionalCommit `json:"performance,omitempty"`
	// Documentation contains docs commits.
	Documentation []ConventionalCommit `json:"documentation,omitempty"`
	// Refactoring contains refactor commits.
	Refactoring []ConventionalCommit `json:"refactoring,omitempty"`
	// Breaking contains breaking change commits.
	Breaking []ConventionalCommit `json:"breaking,omitempty"`
	// Other contains all other commits.
	Other []ConventionalCommit `json:"other,omitempty"`
	// All contains all commits in order.
	All []ConventionalCommit `json:"all"`
}

// HasChanges returns true if there are any categorized changes.
func (c *CategorizedChanges) HasChanges() bool {
	return len(c.All) > 0
}

// HasBreakingChanges returns true if there are any breaking changes.
func (c *CategorizedChanges) HasBreakingChanges() bool {
	return len(c.Breaking) > 0
}

// TotalCount returns the total number of commits.
func (c *CategorizedChanges) TotalCount() int {
	return len(c.All)
}

// DetermineReleaseType determines the release type based on the changes.
func (c *CategorizedChanges) DetermineReleaseType() ReleaseType {
	if c.HasBreakingChanges() {
		return ReleaseTypeMajor
	}
	if len(c.Features) > 0 {
		return ReleaseTypeMinor
	}
	if len(c.Fixes) > 0 || len(c.Performance) > 0 {
		return ReleaseTypePatch
	}
	if len(c.All) > 0 {
		return ReleaseTypePatch
	}
	return ReleaseTypeNone
}

// Regular expressions for parsing conventional commits.
var (
	// ConventionalCommitRegex matches the conventional commit format.
	// Format: <type>(<scope>)!?: <description>
	ConventionalCommitRegex = regexp.MustCompile(
		`^(?P<type>feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)` +
			`(?:\((?P<scope>[^)]+)\))?` +
			`(?P<breaking>!)?` +
			`:\s*` +
			`(?P<description>.+)$`,
	)

	// BreakingChangeRegex matches BREAKING CHANGE footer.
	BreakingChangeRegex = regexp.MustCompile(`(?i)^BREAKING[ -]CHANGE:\s*(.+)$`)

	// ReferenceRegex matches issue/PR references.
	// Matches: #123, GH-123, fixes #123, closes #123, etc.
	ReferenceRegex = regexp.MustCompile(`(?i)(?:(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+)?(?:#|GH-)(\d+)`)
)

// ParseOptions configures commit parsing behavior.
type ParseOptions struct {
	// StrictMode requires commits to follow conventional commit format exactly.
	StrictMode bool
	// ParseReferences enables parsing of issue/PR references.
	ParseReferences bool
	// ParseCoAuthors enables parsing of co-author information.
	ParseCoAuthors bool
}

// DefaultParseOptions returns the default parsing options.
func DefaultParseOptions() ParseOptions {
	return ParseOptions{
		StrictMode:      false,
		ParseReferences: true,
		ParseCoAuthors:  true,
	}
}

// CommitFilter defines criteria for filtering commits.
type CommitFilter struct {
	// Types filters commits by type.
	Types []CommitType
	// Authors filters commits by author email.
	Authors []string
	// ExcludeAuthors excludes commits by author email.
	ExcludeAuthors []string
	// Scopes filters commits by scope.
	Scopes []string
	// ExcludeScopes excludes commits by scope.
	ExcludeScopes []string
	// IncludeNonConventional includes non-conventional commits.
	IncludeNonConventional bool
	// OnlyBreaking only includes breaking changes.
	OnlyBreaking bool
	// Since filters commits after this date.
	Since *time.Time
	// Until filters commits before this date.
	Until *time.Time
	// PathFilter filters commits that touch these paths.
	PathFilter []string
}

// TagFilter defines criteria for filtering tags.
type TagFilter struct {
	// Prefix filters tags by prefix (e.g., "v").
	Prefix string
	// Pattern filters tags by glob pattern.
	Pattern string
	// Since filters tags after this date.
	Since *time.Time
	// Until filters tags before this date.
	Until *time.Time
}

// RepositoryInfo contains information about the git repository.
type RepositoryInfo struct {
	// Root is the repository root directory.
	Root string `json:"root"`
	// CurrentBranch is the current checked out branch.
	CurrentBranch string `json:"current_branch"`
	// DefaultBranch is the default branch (main/master).
	DefaultBranch string `json:"default_branch"`
	// Remotes is the list of configured remotes.
	Remotes []RemoteInfo `json:"remotes"`
	// IsDirty indicates if the working tree has uncommitted changes.
	IsDirty bool `json:"is_dirty"`
	// HeadCommit is the current HEAD commit hash.
	HeadCommit string `json:"head_commit"`
}

// RemoteInfo contains information about a git remote.
type RemoteInfo struct {
	// Name is the remote name.
	Name string `json:"name"`
	// URL is the remote URL.
	URL string `json:"url"`
	// PushURL is the push URL (if different from fetch URL).
	PushURL string `json:"push_url,omitempty"`
}

// DiffStats contains statistics about changes between two refs.
type DiffStats struct {
	// FilesChanged is the number of files changed.
	FilesChanged int `json:"files_changed"`
	// Insertions is the number of lines inserted.
	Insertions int `json:"insertions"`
	// Deletions is the number of lines deleted.
	Deletions int `json:"deletions"`
	// Files is the list of changed files with their stats.
	Files []FileStats `json:"files,omitempty"`
}

// FileStats contains statistics about changes to a single file.
type FileStats struct {
	// Path is the file path.
	Path string `json:"path"`
	// Insertions is the number of lines inserted.
	Insertions int `json:"insertions"`
	// Deletions is the number of lines deleted.
	Deletions int `json:"deletions"`
	// Status is the file status (added, modified, deleted, renamed).
	Status string `json:"status"`
	// OldPath is the old path (for renamed files).
	OldPath string `json:"old_path,omitempty"`
}

// gitRefPattern validates safe git reference names.
// Allows: alphanumeric, ., -, _, /, ^, ~, and numbers for relative refs.
// This follows git-check-ref-format rules with additional security restrictions.
var gitRefPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/~^-]*$`)

// dangerousGitRefPatterns contains patterns that could be used for command injection.
var dangerousGitRefPatterns = []string{
	"--",  // Option prefix
	";",   // Command separator
	"|",   // Pipe
	"&",   // Background/AND
	"`",   // Command substitution
	"$(",  // Command substitution
	"${",  // Variable expansion
	"\n",  // Newline
	"\r",  // Carriage return
	"$()", // Command substitution
	"..",  // Path traversal at start (.. alone is ok in refs like HEAD^^)
}

// ErrInvalidGitRef is returned when a git reference contains invalid characters.
var ErrInvalidGitRef = errors.New("invalid git reference")

// ValidateGitRef validates that a git reference is safe to use in shell commands.
// It returns an error if the reference contains potentially dangerous characters
// that could be used for command injection.
//
// Valid references include:
// - Branch names: main, feature/my-branch, release-1.0
// - Tag names: v1.0.0, release/v2.0
// - Commit SHAs: abc1234, full 40-char SHA
// - Relative refs: HEAD, HEAD~1, HEAD^2, main~5
// - Remote refs: origin/main, upstream/feature
func ValidateGitRef(ref string) error {
	if ref == "" {
		return nil // Empty ref is allowed (will use defaults)
	}

	// Check for dangerous patterns
	for _, pattern := range dangerousGitRefPatterns {
		if strings.Contains(ref, pattern) {
			return fmt.Errorf("%w: reference %q contains dangerous pattern %q", ErrInvalidGitRef, ref, pattern)
		}
	}

	// Check that ref starts with allowed prefix or matches safe pattern
	// Allow HEAD as a special case
	if ref == "HEAD" {
		return nil
	}

	// Check length (git refs have max length of ~250 chars typically)
	if len(ref) > 250 {
		return fmt.Errorf("%w: reference %q exceeds maximum length", ErrInvalidGitRef, ref)
	}

	// Validate against safe pattern
	if !gitRefPattern.MatchString(ref) {
		return fmt.Errorf("%w: reference %q contains invalid characters", ErrInvalidGitRef, ref)
	}

	return nil
}

// MustValidateGitRef validates a git reference and panics if invalid.
// Use this only in contexts where invalid refs indicate a programming error.
func MustValidateGitRef(ref string) string {
	if err := ValidateGitRef(ref); err != nil {
		panic(err)
	}
	return ref
}
