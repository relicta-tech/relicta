package governance

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/internal/cgp/memory"
	"github.com/relicta-tech/relicta/internal/cgp/policy"
	"github.com/relicta-tech/relicta/internal/cgp/risk"
	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

func createTestService(t *testing.T) (*Service, *memory.InMemoryStore) {
	t.Helper()

	riskCalc := risk.NewCalculatorWithDefaults()
	policyEngine := policy.NewEngine(nil, nil)
	eval := evaluator.New(
		evaluator.WithRiskCalculator(riskCalc),
		evaluator.WithPolicyEngine(policyEngine),
	)

	memStore := memory.NewInMemoryStore()
	svc := NewService(eval, WithMemoryStore(memStore))

	return svc, memStore
}

func createTestRelease(t *testing.T) *release.Release {
	t.Helper()

	// Create release using the actual API
	rel := release.NewRelease("release-123", "main", "owner/repo")

	// Add a plan with a changeset
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	changeSet.AddCommit(changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "add feature"))

	current, _ := version.Parse("1.0.0")
	next, _ := version.Parse("1.1.0")
	plan := release.NewReleasePlan(current, next, changes.ReleaseTypeMinor, changeSet, false)
	_ = release.SetPlan(rel, plan)

	return rel
}

func TestNewService(t *testing.T) {
	riskCalc := risk.NewCalculatorWithDefaults()
	policyEngine := policy.NewEngine(nil, nil)
	eval := evaluator.New(
		evaluator.WithRiskCalculator(riskCalc),
		evaluator.WithPolicyEngine(policyEngine),
	)

	svc := NewService(eval)
	if svc == nil {
		t.Fatal("NewService() returned nil")
	}
	if svc.evaluator == nil {
		t.Error("evaluator is nil")
	}
	if svc.logger == nil {
		t.Error("logger is nil")
	}
}

func TestService_EvaluateRelease(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()
	actor := cgp.NewHumanActor("john@example.com", "John")

	rel := createTestRelease(t)

	input := EvaluateReleaseInput{
		Release:    rel,
		Actor:      actor,
		Repository: "owner/repo",
	}

	result, err := svc.EvaluateRelease(ctx, input)
	if err != nil {
		t.Fatalf("EvaluateRelease() error = %v", err)
	}

	if result == nil {
		t.Fatal("EvaluateRelease() returned nil")
	}

	// Check that we got a valid decision
	validDecisions := []cgp.DecisionType{
		cgp.DecisionApproved,
		cgp.DecisionApprovalRequired,
		cgp.DecisionRejected,
		cgp.DecisionDeferred,
	}
	found := false
	for _, d := range validDecisions {
		if result.Decision == d {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("unexpected decision: %v", result.Decision)
	}

	// Risk score should be valid
	if result.RiskScore < 0 || result.RiskScore > 1 {
		t.Errorf("invalid risk score: %v", result.RiskScore)
	}

	// Severity should be set
	if result.Severity == "" {
		t.Error("severity is empty")
	}
}

func TestService_EvaluateRelease_NilRelease(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()
	actor := cgp.NewHumanActor("john@example.com", "John")

	input := EvaluateReleaseInput{
		Release:    nil,
		Actor:      actor,
		Repository: "owner/repo",
	}

	_, err := svc.EvaluateRelease(ctx, input)
	if err == nil {
		t.Error("EvaluateRelease() should error with nil release")
	}
}

func TestService_EvaluateRelease_WithHistory(t *testing.T) {
	svc, memStore := createTestService(t)
	ctx := context.Background()
	actor := cgp.NewHumanActor("john@example.com", "John")

	// Seed some historical data
	for i := 0; i < 5; i++ {
		memStore.RecordRelease(ctx, &memory.ReleaseRecord{
			ID:         "release-" + string(rune('0'+i)),
			Repository: "owner/repo",
			Actor:      actor,
			RiskScore:  0.3,
			Outcome:    memory.OutcomeSuccess,
			ReleasedAt: time.Now(),
		})
	}

	rel := createTestRelease(t)

	input := EvaluateReleaseInput{
		Release:        rel,
		Actor:          actor,
		Repository:     "owner/repo",
		IncludeHistory: true,
	}

	result, err := svc.EvaluateRelease(ctx, input)
	if err != nil {
		t.Fatalf("EvaluateRelease() error = %v", err)
	}

	if result.HistoricalContext == nil {
		t.Error("expected historical context, got nil")
	} else {
		if result.HistoricalContext.RecentReleases != 5 {
			t.Errorf("RecentReleases = %d, want 5", result.HistoricalContext.RecentReleases)
		}
		if result.HistoricalContext.SuccessRate != 1.0 {
			t.Errorf("SuccessRate = %v, want 1.0", result.HistoricalContext.SuccessRate)
		}
	}
}

func TestService_RecordReleaseOutcome(t *testing.T) {
	svc, memStore := createTestService(t)
	ctx := context.Background()
	actor := cgp.NewHumanActor("john@example.com", "John")

	input := RecordOutcomeInput{
		ReleaseID:       "release-1",
		Repository:      "owner/repo",
		Version:         "v1.1.0",
		Actor:           actor,
		RiskScore:       0.4,
		Decision:        cgp.DecisionApproved,
		BreakingChanges: 0,
		FilesChanged:    10,
		LinesChanged:    100,
		Outcome:         memory.OutcomeSuccess,
		Duration:        5 * time.Minute,
	}

	err := svc.RecordReleaseOutcome(ctx, input)
	if err != nil {
		t.Fatalf("RecordReleaseOutcome() error = %v", err)
	}

	// Verify it was recorded
	releases, err := memStore.GetReleaseHistory(ctx, "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory() error = %v", err)
	}
	if len(releases) != 1 {
		t.Errorf("got %d releases, want 1", len(releases))
	}
}

func TestService_RecordIncident(t *testing.T) {
	svc, memStore := createTestService(t)
	ctx := context.Background()

	input := RecordIncidentInput{
		ID:            "incident-1",
		Repository:    "owner/repo",
		ReleaseID:     "release-1",
		Version:       "v1.1.0",
		Type:          memory.IncidentRollback,
		Severity:      cgp.SeverityHigh,
		Description:   "Service outage",
		DetectedAt:    time.Now(),
		TimeToDetect:  10 * time.Minute,
		TimeToResolve: 30 * time.Minute,
	}

	err := svc.RecordIncident(ctx, input)
	if err != nil {
		t.Fatalf("RecordIncident() error = %v", err)
	}

	// Verify it was recorded
	incidents, err := memStore.GetIncidentHistory(ctx, "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetIncidentHistory() error = %v", err)
	}
	if len(incidents) != 1 {
		t.Errorf("got %d incidents, want 1", len(incidents))
	}
}

func TestService_QuickRiskCheck(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()
	actor := cgp.NewHumanActor("john@example.com", "John")

	rel := createTestRelease(t)

	input := EvaluateReleaseInput{
		Release:    rel,
		Actor:      actor,
		Repository: "owner/repo",
	}

	riskScore, severity, err := svc.QuickRiskCheck(ctx, input)
	if err != nil {
		t.Fatalf("QuickRiskCheck() error = %v", err)
	}

	if riskScore < 0 || riskScore > 1 {
		t.Errorf("invalid risk score: %v", riskScore)
	}
	if severity == "" {
		t.Error("severity is empty")
	}
}

func TestService_QuickRiskCheck_NilRelease(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()
	actor := cgp.NewHumanActor("john@example.com", "John")

	input := EvaluateReleaseInput{
		Release:    nil,
		Actor:      actor,
		Repository: "owner/repo",
	}

	_, _, err := svc.QuickRiskCheck(ctx, input)
	if err == nil {
		t.Error("QuickRiskCheck() should error with nil release")
	}
}

func TestService_NoMemoryStore(t *testing.T) {
	riskCalc := risk.NewCalculatorWithDefaults()
	policyEngine := policy.NewEngine(nil, nil)
	eval := evaluator.New(
		evaluator.WithRiskCalculator(riskCalc),
		evaluator.WithPolicyEngine(policyEngine),
	)

	// Service without memory store
	svc := NewService(eval)
	ctx := context.Background()
	actor := cgp.NewHumanActor("john@example.com", "John")

	// RecordReleaseOutcome should not error without memory store
	err := svc.RecordReleaseOutcome(ctx, RecordOutcomeInput{
		ReleaseID:  "release-1",
		Repository: "owner/repo",
		Actor:      actor,
		Outcome:    memory.OutcomeSuccess,
	})
	if err != nil {
		t.Errorf("RecordReleaseOutcome() error = %v, want nil", err)
	}

	// RecordIncident should not error without memory store
	err = svc.RecordIncident(ctx, RecordIncidentInput{
		ID:         "incident-1",
		Repository: "owner/repo",
		Type:       memory.IncidentRollback,
		Severity:   cgp.SeverityHigh,
		DetectedAt: time.Now(),
	})
	if err != nil {
		t.Errorf("RecordIncident() error = %v, want nil", err)
	}
}

// Security detection tests

func TestIsSecurityCommit(t *testing.T) {
	tests := []struct {
		name     string
		commit   *changes.ConventionalCommit
		expected bool
	}{
		{
			name:     "nil commit",
			commit:   nil,
			expected: false,
		},
		{
			name:     "regular feature commit",
			commit:   changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "add new button"),
			expected: false,
		},
		{
			name:     "security scope",
			commit:   changes.NewConventionalCommit("abc123", changes.CommitTypeFix, "fix issue", changes.WithScope("security")),
			expected: true,
		},
		{
			name:     "auth scope",
			commit:   changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "add login", changes.WithScope("auth")),
			expected: true,
		},
		{
			name:     "crypto scope",
			commit:   changes.NewConventionalCommit("abc123", changes.CommitTypeFix, "update algo", changes.WithScope("crypto")),
			expected: true,
		},
		{
			name:     "oauth scope",
			commit:   changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "add oauth", changes.WithScope("oauth")),
			expected: true,
		},
		{
			name:     "cve keyword in subject",
			commit:   changes.NewConventionalCommit("abc123", changes.CommitTypeFix, "fix CVE-2024-1234"),
			expected: true,
		},
		{
			name:     "vulnerability keyword",
			commit:   changes.NewConventionalCommit("abc123", changes.CommitTypeFix, "patch vulnerability in parser"),
			expected: true,
		},
		{
			name:     "xss keyword",
			commit:   changes.NewConventionalCommit("abc123", changes.CommitTypeFix, "prevent XSS attack"),
			expected: true,
		},
		{
			name:     "injection keyword",
			commit:   changes.NewConventionalCommit("abc123", changes.CommitTypeFix, "fix sql injection"),
			expected: true,
		},
		{
			name:     "sanitize keyword",
			commit:   changes.NewConventionalCommit("abc123", changes.CommitTypeFix, "sanitize user input"),
			expected: true,
		},
		{
			name:     "security fix keyword",
			commit:   changes.NewConventionalCommit("abc123", changes.CommitTypeFix, "security fix for login"),
			expected: true,
		},
		{
			name:     "password keyword",
			commit:   changes.NewConventionalCommit("abc123", changes.CommitTypeFix, "improve password hashing"),
			expected: true,
		},
		{
			name:     "encrypt keyword",
			commit:   changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "encrypt sensitive data"),
			expected: true,
		},
		{
			name:     "authentication scope partial match",
			commit:   changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "add 2fa", changes.WithScope("authentication")),
			expected: true,
		},
		{
			name:     "unrelated scope with security in message",
			commit:   changes.NewConventionalCommit("abc123", changes.CommitTypeDocs, "update security docs", changes.WithScope("docs")),
			expected: true, // keyword "security" in subject
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSecurityCommit(tt.commit)
			if got != tt.expected {
				t.Errorf("isSecurityCommit() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCountSecurityChanges(t *testing.T) {
	tests := []struct {
		name     string
		commits  []*changes.ConventionalCommit
		expected int
	}{
		{
			name:     "nil changeset",
			commits:  nil,
			expected: 0,
		},
		{
			name: "no security commits",
			commits: []*changes.ConventionalCommit{
				changes.NewConventionalCommit("1", changes.CommitTypeFeat, "add feature"),
				changes.NewConventionalCommit("2", changes.CommitTypeFix, "fix bug"),
				changes.NewConventionalCommit("3", changes.CommitTypeDocs, "update docs"),
			},
			expected: 0,
		},
		{
			name: "one security commit",
			commits: []*changes.ConventionalCommit{
				changes.NewConventionalCommit("1", changes.CommitTypeFeat, "add feature"),
				changes.NewConventionalCommit("2", changes.CommitTypeFix, "fix vulnerability"),
				changes.NewConventionalCommit("3", changes.CommitTypeDocs, "update docs"),
			},
			expected: 1,
		},
		{
			name: "multiple security commits",
			commits: []*changes.ConventionalCommit{
				changes.NewConventionalCommit("1", changes.CommitTypeFix, "fix CVE-2024-001"),
				changes.NewConventionalCommit("2", changes.CommitTypeFeat, "add oauth", changes.WithScope("auth")),
				changes.NewConventionalCommit("3", changes.CommitTypeFix, "sanitize input"),
			},
			expected: 3,
		},
		{
			name: "mixed commits",
			commits: []*changes.ConventionalCommit{
				changes.NewConventionalCommit("1", changes.CommitTypeFeat, "add button"),
				changes.NewConventionalCommit("2", changes.CommitTypeFix, "fix XSS issue"),
				changes.NewConventionalCommit("3", changes.CommitTypeRefactor, "clean up code"),
				changes.NewConventionalCommit("4", changes.CommitTypeFix, "fix auth bypass", changes.WithScope("security")),
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cs *changes.ChangeSet
			if tt.commits != nil {
				cs = changes.NewChangeSet("test-cs", "v1.0.0", "HEAD")
				cs.AddCommits(tt.commits)
			}

			got := countSecurityChanges(cs)
			if got != tt.expected {
				t.Errorf("countSecurityChanges() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSecurityDetectionInEvaluateRelease(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()
	actor := cgp.NewHumanActor("john@example.com", "John")

	// Create release with security commits
	rel := release.NewRelease("release-security", "main", "owner/repo")
	changeSet := changes.NewChangeSet("cs-sec", "v1.0.0", "HEAD")
	changeSet.AddCommit(changes.NewConventionalCommit("1", changes.CommitTypeFeat, "add feature"))
	changeSet.AddCommit(changes.NewConventionalCommit("2", changes.CommitTypeFix, "fix CVE-2024-1234"))
	changeSet.AddCommit(changes.NewConventionalCommit("3", changes.CommitTypeFix, "patch auth bypass", changes.WithScope("security")))

	current, _ := version.Parse("1.0.0")
	next, _ := version.Parse("1.1.0")
	plan := release.NewReleasePlan(current, next, changes.ReleaseTypeMinor, changeSet, false)
	_ = release.SetPlan(rel, plan)

	input := EvaluateReleaseInput{
		Release:    rel,
		Actor:      actor,
		Repository: "owner/repo",
	}

	output, err := svc.EvaluateRelease(ctx, input)
	if err != nil {
		t.Fatalf("EvaluateRelease() error = %v", err)
	}

	// With security changes, risk should be elevated
	if output.RiskScore <= 0 {
		t.Error("expected elevated risk score with security changes")
	}

	// Security changes should trigger require_human_for_security rule
	// (depending on config, may require human approval)
	t.Logf("Risk score with security changes: %.2f, Severity: %s", output.RiskScore, output.Severity)
}
