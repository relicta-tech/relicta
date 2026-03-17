// Package ciapproval provides CI-safe approval capabilities for non-interactive environments.
// It enables automated approval workflows in CI/CD pipelines while maintaining
// audit trails and governance compliance.
package ciapproval

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config configures CI-safe approval behavior.
// Values can be set via environment variables for CI/CD integration.
type Config struct {
	// Enabled indicates if CI approval mode is active.
	// Env: RELICTA_CI_APPROVAL=true
	Enabled bool `json:"enabled"`

	// AutoApprove enables automatic approval when conditions are met.
	// Env: RELICTA_AUTO_APPROVE=true
	AutoApprove bool `json:"autoApprove"`

	// TrustedActor is the actor ID to use for CI approvals.
	// Env: RELICTA_CI_ACTOR=ci:github-actions
	TrustedActor string `json:"trustedActor"`

	// MaxRiskScore is the maximum risk score for auto-approval (0.0-1.0).
	// Env: RELICTA_MAX_AUTO_APPROVE_RISK=0.3
	MaxRiskScore float64 `json:"maxRiskScore"`

	// AllowedBumpTypes lists bump types that can be auto-approved.
	// Env: RELICTA_ALLOWED_BUMP_TYPES=patch,minor
	AllowedBumpTypes []string `json:"allowedBumpTypes,omitempty"`

	// AllowedBranches lists branches that can trigger auto-approval.
	// Env: RELICTA_ALLOWED_BRANCHES=main,master,release/*
	AllowedBranches []string `json:"allowedBranches,omitempty"`

	// RequireCleanCI requires all CI checks to pass.
	// Env: RELICTA_REQUIRE_CLEAN_CI=true
	RequireCleanCI bool `json:"requireCleanCI"`

	// BlockBreakingChanges prevents auto-approval of breaking changes.
	// Env: RELICTA_BLOCK_BREAKING=true
	BlockBreakingChanges bool `json:"blockBreakingChanges"`

	// BlockSecurityChanges prevents auto-approval of security-related changes.
	// Env: RELICTA_BLOCK_SECURITY=true
	BlockSecurityChanges bool `json:"blockSecurityChanges"`

	// ApprovalTimeout is how long the CI approval is valid.
	// Env: RELICTA_APPROVAL_TIMEOUT=1h
	ApprovalTimeout time.Duration `json:"approvalTimeout"`

	// AuditLogPath is where to write detailed audit logs.
	// Env: RELICTA_CI_AUDIT_LOG=/path/to/audit.json
	AuditLogPath string `json:"auditLogPath,omitempty"`

	// RequireSignedCommits requires all commits to be signed.
	// Env: RELICTA_REQUIRE_SIGNED_COMMITS=true
	RequireSignedCommits bool `json:"requireSignedCommits"`

	// BypassReason is an optional reason for bypassing normal approval.
	// Env: RELICTA_BYPASS_REASON="Emergency hotfix"
	BypassReason string `json:"bypassReason,omitempty"`

	// DryRun simulates approval without persisting.
	// Env: RELICTA_DRY_RUN=true
	DryRun bool `json:"dryRun"`
}

// DefaultConfig returns sensible defaults for CI approval.
func DefaultConfig() *Config {
	return &Config{
		Enabled:              false,
		AutoApprove:          false,
		MaxRiskScore:         0.3,
		AllowedBumpTypes:     []string{"patch"},
		AllowedBranches:      []string{"main", "master"},
		RequireCleanCI:       true,
		BlockBreakingChanges: true,
		BlockSecurityChanges: true,
		ApprovalTimeout:      1 * time.Hour,
		DryRun:               false,
	}
}

// FromEnvironment creates a Config from environment variables.
// Environment variables override default values.
func FromEnvironment() *Config {
	cfg := DefaultConfig()

	// Check if we're in a CI environment
	if isCI() {
		cfg.Enabled = true
	}

	// Override from environment variables
	if v := os.Getenv("RELICTA_CI_APPROVAL"); v != "" {
		cfg.Enabled = parseBool(v)
	}
	if v := os.Getenv("RELICTA_AUTO_APPROVE"); v != "" {
		cfg.AutoApprove = parseBool(v)
	}
	if v := os.Getenv("RELICTA_CI_ACTOR"); v != "" {
		cfg.TrustedActor = v
	} else {
		cfg.TrustedActor = detectCIActor()
	}
	if v := os.Getenv("RELICTA_MAX_AUTO_APPROVE_RISK"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.MaxRiskScore = f
		}
	}
	if v := os.Getenv("RELICTA_ALLOWED_BUMP_TYPES"); v != "" {
		cfg.AllowedBumpTypes = parseList(v)
	}
	if v := os.Getenv("RELICTA_ALLOWED_BRANCHES"); v != "" {
		cfg.AllowedBranches = parseList(v)
	}
	if v := os.Getenv("RELICTA_REQUIRE_CLEAN_CI"); v != "" {
		cfg.RequireCleanCI = parseBool(v)
	}
	if v := os.Getenv("RELICTA_BLOCK_BREAKING"); v != "" {
		cfg.BlockBreakingChanges = parseBool(v)
	}
	if v := os.Getenv("RELICTA_BLOCK_SECURITY"); v != "" {
		cfg.BlockSecurityChanges = parseBool(v)
	}
	if v := os.Getenv("RELICTA_APPROVAL_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ApprovalTimeout = d
		}
	}
	if v := os.Getenv("RELICTA_CI_AUDIT_LOG"); v != "" {
		cfg.AuditLogPath = v
	}
	if v := os.Getenv("RELICTA_REQUIRE_SIGNED_COMMITS"); v != "" {
		cfg.RequireSignedCommits = parseBool(v)
	}
	if v := os.Getenv("RELICTA_BYPASS_REASON"); v != "" {
		cfg.BypassReason = v
	}
	if v := os.Getenv("RELICTA_DRY_RUN"); v != "" {
		cfg.DryRun = parseBool(v)
	}

	return cfg
}

// isCI detects if running in a CI environment.
func isCI() bool {
	// Check common CI environment indicators
	ciEnvVars := []string{
		"CI",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"JENKINS_URL",
		"CIRCLECI",
		"TRAVIS",
		"BUILDKITE",
		"AZURE_PIPELINES",
		"TEAMCITY_VERSION",
		"BITBUCKET_BUILD_NUMBER",
	}

	for _, env := range ciEnvVars {
		if v := os.Getenv(env); v != "" && v != "false" {
			return true
		}
	}

	return false
}

// detectCIActor attempts to determine the CI actor from environment.
func detectCIActor() string {
	// GitHub Actions
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		actor := os.Getenv("GITHUB_ACTOR")
		if actor == "" {
			actor = "github-actions"
		}
		workflow := os.Getenv("GITHUB_WORKFLOW")
		if workflow != "" {
			return "ci:github-actions:" + workflow
		}
		return "ci:github-actions:" + actor
	}

	// GitLab CI
	if os.Getenv("GITLAB_CI") == "true" {
		user := os.Getenv("GITLAB_USER_LOGIN")
		if user == "" {
			user = "gitlab-ci"
		}
		pipeline := os.Getenv("CI_PIPELINE_NAME")
		if pipeline != "" {
			return "ci:gitlab:" + pipeline
		}
		return "ci:gitlab:" + user
	}

	// Jenkins
	if os.Getenv("JENKINS_URL") != "" {
		user := os.Getenv("BUILD_USER")
		if user == "" {
			user = os.Getenv("BUILD_USER_ID")
		}
		if user == "" {
			user = "jenkins"
		}
		job := os.Getenv("JOB_NAME")
		if job != "" {
			return "ci:jenkins:" + job
		}
		return "ci:jenkins:" + user
	}

	// CircleCI
	if os.Getenv("CIRCLECI") == "true" {
		workflow := os.Getenv("CIRCLE_WORKFLOW_ID")
		if workflow != "" {
			return "ci:circleci:" + workflow
		}
		return "ci:circleci"
	}

	// Azure Pipelines
	if os.Getenv("AZURE_PIPELINES") == "true" || os.Getenv("TF_BUILD") == "True" {
		pipeline := os.Getenv("BUILD_DEFINITIONNAME")
		if pipeline != "" {
			return "ci:azure:" + pipeline
		}
		return "ci:azure"
	}

	// Buildkite
	if os.Getenv("BUILDKITE") == "true" {
		pipeline := os.Getenv("BUILDKITE_PIPELINE_SLUG")
		if pipeline != "" {
			return "ci:buildkite:" + pipeline
		}
		return "ci:buildkite"
	}

	// Generic CI
	if os.Getenv("CI") == "true" {
		return "ci:generic"
	}

	return "ci:unknown"
}

// GetCIContext returns contextual information about the CI environment.
func GetCIContext() map[string]string {
	ctx := make(map[string]string)

	// Common CI metadata
	if v := os.Getenv("CI"); v != "" {
		ctx["ci"] = v
	}

	// GitHub Actions
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		ctx["provider"] = "github-actions"
		ctx["repository"] = os.Getenv("GITHUB_REPOSITORY")
		ctx["ref"] = os.Getenv("GITHUB_REF")
		ctx["sha"] = os.Getenv("GITHUB_SHA")
		ctx["workflow"] = os.Getenv("GITHUB_WORKFLOW")
		ctx["run_id"] = os.Getenv("GITHUB_RUN_ID")
		ctx["run_number"] = os.Getenv("GITHUB_RUN_NUMBER")
		ctx["actor"] = os.Getenv("GITHUB_ACTOR")
		ctx["event_name"] = os.Getenv("GITHUB_EVENT_NAME")
	}

	// GitLab CI
	if os.Getenv("GITLAB_CI") == "true" {
		ctx["provider"] = "gitlab"
		ctx["repository"] = os.Getenv("CI_PROJECT_PATH")
		ctx["ref"] = os.Getenv("CI_COMMIT_REF_NAME")
		ctx["sha"] = os.Getenv("CI_COMMIT_SHA")
		ctx["pipeline_id"] = os.Getenv("CI_PIPELINE_ID")
		ctx["job_id"] = os.Getenv("CI_JOB_ID")
		ctx["user"] = os.Getenv("GITLAB_USER_LOGIN")
	}

	// Jenkins
	if os.Getenv("JENKINS_URL") != "" {
		ctx["provider"] = "jenkins"
		ctx["job_name"] = os.Getenv("JOB_NAME")
		ctx["build_number"] = os.Getenv("BUILD_NUMBER")
		ctx["build_url"] = os.Getenv("BUILD_URL")
		ctx["git_branch"] = os.Getenv("GIT_BRANCH")
		ctx["git_commit"] = os.Getenv("GIT_COMMIT")
	}

	// CircleCI
	if os.Getenv("CIRCLECI") == "true" {
		ctx["provider"] = "circleci"
		ctx["repository"] = os.Getenv("CIRCLE_PROJECT_REPONAME")
		ctx["ref"] = os.Getenv("CIRCLE_BRANCH")
		ctx["sha"] = os.Getenv("CIRCLE_SHA1")
		ctx["build_num"] = os.Getenv("CIRCLE_BUILD_NUM")
		ctx["workflow_id"] = os.Getenv("CIRCLE_WORKFLOW_ID")
	}

	return ctx
}

// parseBool parses a boolean from string, handling various formats.
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes" || s == "on"
}

// parseList parses a comma-separated list from string.
func parseList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
