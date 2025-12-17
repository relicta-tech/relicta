package cgp

import (
	"strings"
	"testing"
	"time"
)

func TestExecutionStep_String(t *testing.T) {
	tests := []struct {
		name     string
		step     ExecutionStep
		expected string
	}{
		{"tag", ExecutionStepTag, "tag"},
		{"changelog", ExecutionStepChangelog, "changelog"},
		{"release_notes", ExecutionStepReleaseNotes, "release_notes"},
		{"publish", ExecutionStepPublish, "publish"},
		{"notify", ExecutionStepNotify, "notify"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.step.String(); got != tt.expected {
				t.Errorf("ExecutionStep.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAllExecutionSteps(t *testing.T) {
	steps := AllExecutionSteps()
	if len(steps) != 5 {
		t.Errorf("AllExecutionSteps() returned %d steps, want 5", len(steps))
	}

	expected := map[ExecutionStep]bool{
		ExecutionStepTag:          true,
		ExecutionStepChangelog:    true,
		ExecutionStepReleaseNotes: true,
		ExecutionStepPublish:      true,
		ExecutionStepNotify:       true,
	}

	for _, step := range steps {
		if !expected[step] {
			t.Errorf("Unexpected execution step: %v", step)
		}
	}
}

func TestNewAuthorization(t *testing.T) {
	approver := NewHumanActor("john@example.com", "John Doe")
	auth := NewAuthorization("dec_123", "prop_456", approver, "1.2.0")

	if auth.CGPVersion != Version {
		t.Errorf("NewAuthorization().CGPVersion = %v, want %v", auth.CGPVersion, Version)
	}
	if auth.Type != MessageTypeAuthorization {
		t.Errorf("NewAuthorization().Type = %v, want %v", auth.Type, MessageTypeAuthorization)
	}
	if !strings.HasPrefix(auth.ID, "auth_") {
		t.Errorf("NewAuthorization().ID = %v, should start with 'auth_'", auth.ID)
	}
	if auth.DecisionID != "dec_123" {
		t.Errorf("NewAuthorization().DecisionID = %v, want dec_123", auth.DecisionID)
	}
	if auth.ProposalID != "prop_456" {
		t.Errorf("NewAuthorization().ProposalID = %v, want prop_456", auth.ProposalID)
	}
	if auth.ApprovedBy.ID != approver.ID {
		t.Errorf("NewAuthorization().ApprovedBy.ID = %v, want %v", auth.ApprovedBy.ID, approver.ID)
	}
	if auth.Version != "1.2.0" {
		t.Errorf("NewAuthorization().Version = %v, want 1.2.0", auth.Version)
	}
	if auth.Tag != "v1.2.0" {
		t.Errorf("NewAuthorization().Tag = %v, want v1.2.0", auth.Tag)
	}
	if auth.Timestamp.IsZero() {
		t.Error("NewAuthorization().Timestamp should not be zero")
	}
	if auth.ApprovedAt.IsZero() {
		t.Error("NewAuthorization().ApprovedAt should not be zero")
	}
	if auth.ValidUntil.IsZero() {
		t.Error("NewAuthorization().ValidUntil should not be zero")
	}
	if len(auth.AllowedSteps) != 5 {
		t.Errorf("NewAuthorization().AllowedSteps should have 5 steps, got %d", len(auth.AllowedSteps))
	}
	if auth.ApprovalChain == nil {
		t.Error("NewAuthorization().ApprovalChain should not be nil")
	}
}

func TestGenerateAuthorizationID(t *testing.T) {
	id1 := GenerateAuthorizationID()
	id2 := GenerateAuthorizationID()

	if !strings.HasPrefix(id1, "auth_") {
		t.Errorf("GenerateAuthorizationID() = %v, should start with 'auth_'", id1)
	}
	if id1 == id2 {
		t.Error("GenerateAuthorizationID() should generate unique IDs")
	}
}

func TestExecutionAuthorization_Validate(t *testing.T) {
	validApprover := NewHumanActor("john@example.com", "John Doe")

	tests := []struct {
		name      string
		auth      *ExecutionAuthorization
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid authorization",
			auth:      NewAuthorization("dec_123", "prop_456", validApprover, "1.2.0"),
			expectErr: false,
		},
		{
			name: "missing CGP version",
			auth: &ExecutionAuthorization{
				Type:         MessageTypeAuthorization,
				ID:           "auth_123",
				DecisionID:   "dec_123",
				ProposalID:   "prop_456",
				ApprovedBy:   validApprover,
				Version:      "1.0.0",
				AllowedSteps: AllExecutionSteps(),
			},
			expectErr: true,
			errMsg:    "CGP version is required",
		},
		{
			name: "invalid message type",
			auth: &ExecutionAuthorization{
				CGPVersion:   Version,
				Type:         MessageTypeDecision,
				ID:           "auth_123",
				DecisionID:   "dec_123",
				ProposalID:   "prop_456",
				ApprovedBy:   validApprover,
				Version:      "1.0.0",
				AllowedSteps: AllExecutionSteps(),
			},
			expectErr: true,
			errMsg:    "invalid message type",
		},
		{
			name: "missing ID",
			auth: &ExecutionAuthorization{
				CGPVersion:   Version,
				Type:         MessageTypeAuthorization,
				DecisionID:   "dec_123",
				ProposalID:   "prop_456",
				ApprovedBy:   validApprover,
				Version:      "1.0.0",
				AllowedSteps: AllExecutionSteps(),
			},
			expectErr: true,
			errMsg:    "authorization ID is required",
		},
		{
			name: "missing decision ID",
			auth: &ExecutionAuthorization{
				CGPVersion:   Version,
				Type:         MessageTypeAuthorization,
				ID:           "auth_123",
				ProposalID:   "prop_456",
				ApprovedBy:   validApprover,
				Version:      "1.0.0",
				AllowedSteps: AllExecutionSteps(),
			},
			expectErr: true,
			errMsg:    "decision ID is required",
		},
		{
			name: "missing proposal ID",
			auth: &ExecutionAuthorization{
				CGPVersion:   Version,
				Type:         MessageTypeAuthorization,
				ID:           "auth_123",
				DecisionID:   "dec_123",
				ApprovedBy:   validApprover,
				Version:      "1.0.0",
				AllowedSteps: AllExecutionSteps(),
			},
			expectErr: true,
			errMsg:    "proposal ID is required",
		},
		{
			name: "invalid approver",
			auth: &ExecutionAuthorization{
				CGPVersion:   Version,
				Type:         MessageTypeAuthorization,
				ID:           "auth_123",
				DecisionID:   "dec_123",
				ProposalID:   "prop_456",
				ApprovedBy:   Actor{Kind: ActorKind("invalid"), ID: "test"},
				Version:      "1.0.0",
				AllowedSteps: AllExecutionSteps(),
			},
			expectErr: true,
			errMsg:    "invalid approver",
		},
		{
			name: "missing version",
			auth: &ExecutionAuthorization{
				CGPVersion:   Version,
				Type:         MessageTypeAuthorization,
				ID:           "auth_123",
				DecisionID:   "dec_123",
				ProposalID:   "prop_456",
				ApprovedBy:   validApprover,
				AllowedSteps: AllExecutionSteps(),
			},
			expectErr: true,
			errMsg:    "version is required",
		},
		{
			name: "no allowed steps",
			auth: &ExecutionAuthorization{
				CGPVersion:   Version,
				Type:         MessageTypeAuthorization,
				ID:           "auth_123",
				DecisionID:   "dec_123",
				ProposalID:   "prop_456",
				ApprovedBy:   validApprover,
				Version:      "1.0.0",
				AllowedSteps: []ExecutionStep{},
			},
			expectErr: true,
			errMsg:    "at least one allowed step",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.auth.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("ExecutionAuthorization.Validate() error = %v, expectErr %v", err, tt.expectErr)
			}
			if tt.expectErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ExecutionAuthorization.Validate() error = %v, should contain %v", err, tt.errMsg)
			}
		})
	}
}

func TestExecutionAuthorization_Validity(t *testing.T) {
	approver := NewHumanActor("john@example.com", "John Doe")

	// Create an authorization that is valid
	validAuth := NewAuthorization("dec_123", "prop_456", approver, "1.2.0")
	if !validAuth.IsValid() {
		t.Error("New authorization should be valid")
	}
	if validAuth.IsExpired() {
		t.Error("New authorization should not be expired")
	}
	if validAuth.TimeToExpiry() <= 0 {
		t.Error("New authorization should have positive time to expiry")
	}

	// Create an expired authorization
	expiredAuth := NewAuthorization("dec_123", "prop_456", approver, "1.2.0")
	expiredAuth.ValidUntil = time.Now().Add(-time.Hour)
	if expiredAuth.IsValid() {
		t.Error("Expired authorization should not be valid")
	}
	if !expiredAuth.IsExpired() {
		t.Error("Expired authorization should be expired")
	}
}

func TestExecutionAuthorization_IsStepAllowed(t *testing.T) {
	approver := NewHumanActor("john@example.com", "John Doe")
	auth := NewAuthorization("dec_123", "prop_456", approver, "1.2.0").
		WithAllowedSteps(ExecutionStepTag, ExecutionStepPublish)

	tests := []struct {
		name     string
		step     ExecutionStep
		expected bool
	}{
		{"tag is allowed", ExecutionStepTag, true},
		{"publish is allowed", ExecutionStepPublish, true},
		{"changelog is not allowed", ExecutionStepChangelog, false},
		{"release_notes is not allowed", ExecutionStepReleaseNotes, false},
		{"notify is not allowed", ExecutionStepNotify, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := auth.IsStepAllowed(tt.step); got != tt.expected {
				t.Errorf("IsStepAllowed(%v) = %v, want %v", tt.step, got, tt.expected)
			}
		})
	}
}

func TestExecutionAuthorization_WithValidity(t *testing.T) {
	approver := NewHumanActor("john@example.com", "John Doe")
	auth := NewAuthorization("dec_123", "prop_456", approver, "1.2.0")
	originalApprovedAt := auth.ApprovedAt

	auth.WithValidity(48 * time.Hour)

	expectedValidUntil := originalApprovedAt.Add(48 * time.Hour)
	if !auth.ValidUntil.Equal(expectedValidUntil) {
		t.Errorf("WithValidity() ValidUntil = %v, want %v", auth.ValidUntil, expectedValidUntil)
	}
}

func TestExecutionAuthorization_WithReleaseNotes(t *testing.T) {
	approver := NewHumanActor("john@example.com", "John Doe")
	auth := NewAuthorization("dec_123", "prop_456", approver, "1.2.0").
		WithReleaseNotes("## Release Notes\n- Feature 1")

	if auth.ReleaseNotes != "## Release Notes\n- Feature 1" {
		t.Errorf("WithReleaseNotes() = %v, want release notes", auth.ReleaseNotes)
	}
}

func TestExecutionAuthorization_WithChangelog(t *testing.T) {
	approver := NewHumanActor("john@example.com", "John Doe")
	auth := NewAuthorization("dec_123", "prop_456", approver, "1.2.0").
		WithChangelog("## [1.2.0] - 2024-01-15\n- Added feature")

	if auth.Changelog != "## [1.2.0] - 2024-01-15\n- Added feature" {
		t.Errorf("WithChangelog() = %v, want changelog", auth.Changelog)
	}
}

func TestExecutionAuthorization_WithAllowedSteps(t *testing.T) {
	approver := NewHumanActor("john@example.com", "John Doe")
	auth := NewAuthorization("dec_123", "prop_456", approver, "1.2.0").
		WithAllowedSteps(ExecutionStepTag, ExecutionStepPublish)

	if len(auth.AllowedSteps) != 2 {
		t.Errorf("WithAllowedSteps() should have 2 steps, got %d", len(auth.AllowedSteps))
	}
}

func TestExecutionAuthorization_AddRestriction(t *testing.T) {
	approver := NewHumanActor("john@example.com", "John Doe")
	auth := NewAuthorization("dec_123", "prop_456", approver, "1.2.0").
		AddRestriction("No weekend deployments").
		AddRestriction("Requires security review")

	if len(auth.Restrictions) != 2 {
		t.Errorf("AddRestriction() should have 2 restrictions, got %d", len(auth.Restrictions))
	}
	if auth.Restrictions[0] != "No weekend deployments" {
		t.Errorf("Restriction[0] = %v, want 'No weekend deployments'", auth.Restrictions[0])
	}
}

func TestExecutionAuthorization_RecordApproval(t *testing.T) {
	approver := NewHumanActor("john@example.com", "John Doe")
	reviewer := NewHumanActor("jane@example.com", "Jane Doe")
	auth := NewAuthorization("dec_123", "prop_456", approver, "1.2.0").
		RecordApproval(reviewer, ApprovalActionApprove, "LGTM")

	if len(auth.ApprovalChain) != 1 {
		t.Errorf("RecordApproval() should have 1 record, got %d", len(auth.ApprovalChain))
	}
	if auth.ApprovalChain[0].Actor.ID != reviewer.ID {
		t.Errorf("ApprovalChain[0].Actor.ID = %v, want %v", auth.ApprovalChain[0].Actor.ID, reviewer.ID)
	}
	if auth.ApprovalChain[0].Action != ApprovalActionApprove {
		t.Errorf("ApprovalChain[0].Action = %v, want %v", auth.ApprovalChain[0].Action, ApprovalActionApprove)
	}
	if auth.ApprovalChain[0].Comment != "LGTM" {
		t.Errorf("ApprovalChain[0].Comment = %v, want LGTM", auth.ApprovalChain[0].Comment)
	}
	if auth.ApprovalChain[0].Timestamp.IsZero() {
		t.Error("ApprovalChain[0].Timestamp should not be zero")
	}
}

func TestExecutionAuthorization_LastApproval(t *testing.T) {
	approver := NewHumanActor("john@example.com", "John Doe")
	reviewer1 := NewHumanActor("jane@example.com", "Jane Doe")
	reviewer2 := NewHumanActor("bob@example.com", "Bob Smith")

	// Empty chain
	auth := NewAuthorization("dec_123", "prop_456", approver, "1.2.0")
	if auth.LastApproval() != nil {
		t.Error("LastApproval() should return nil for empty chain")
	}

	// With approvals
	auth.RecordApproval(reviewer1, ApprovalActionComment, "Looking good").
		RecordApproval(reviewer2, ApprovalActionApprove, "Approved!")

	last := auth.LastApproval()
	if last == nil {
		t.Error("LastApproval() should not return nil")
	}
	if last.Actor.ID != reviewer2.ID {
		t.Errorf("LastApproval().Actor.ID = %v, want %v", last.Actor.ID, reviewer2.ID)
	}
}

func TestExecutionAuthorization_ApprovalCount(t *testing.T) {
	approver := NewHumanActor("john@example.com", "John Doe")
	reviewer1 := NewHumanActor("jane@example.com", "Jane Doe")
	reviewer2 := NewHumanActor("bob@example.com", "Bob Smith")

	auth := NewAuthorization("dec_123", "prop_456", approver, "1.2.0").
		RecordApproval(reviewer1, ApprovalActionComment, "Comment").
		RecordApproval(reviewer1, ApprovalActionApprove, "Approved").
		RecordApproval(reviewer2, ApprovalActionRequestChanges, "Needs work").
		RecordApproval(reviewer2, ApprovalActionApprove, "Now approved")

	// Should count only "approve" actions
	if count := auth.ApprovalCount(); count != 2 {
		t.Errorf("ApprovalCount() = %d, want 2", count)
	}
}

func TestExecutionAuthorization_HasApprovalFrom(t *testing.T) {
	approver := NewHumanActor("john@example.com", "John Doe")
	reviewer1 := NewHumanActor("jane@example.com", "Jane Doe")
	reviewer2 := NewHumanActor("bob@example.com", "Bob Smith")

	auth := NewAuthorization("dec_123", "prop_456", approver, "1.2.0").
		RecordApproval(reviewer1, ApprovalActionApprove, "Approved").
		RecordApproval(reviewer2, ApprovalActionComment, "Comment only")

	if !auth.HasApprovalFrom(reviewer1.ID) {
		t.Error("HasApprovalFrom() should return true for reviewer1")
	}
	if auth.HasApprovalFrom(reviewer2.ID) {
		t.Error("HasApprovalFrom() should return false for reviewer2 (only commented)")
	}
	if auth.HasApprovalFrom("unknown-id") {
		t.Error("HasApprovalFrom() should return false for unknown actor")
	}
}

func TestExecutionAuthorization_HasHumanApproval(t *testing.T) {
	humanApprover := NewHumanActor("john@example.com", "John Doe")
	agentApprover := NewAgentActor("cursor", "Cursor", "gpt-4")
	ciApprover := NewCIActor("github-actions", "release", "123")

	tests := []struct {
		name     string
		auth     *ExecutionAuthorization
		expected bool
	}{
		{
			name: "human approval",
			auth: NewAuthorization("dec_123", "prop_456", humanApprover, "1.2.0").
				RecordApproval(humanApprover, ApprovalActionApprove, "Approved"),
			expected: true,
		},
		{
			name: "agent approval only",
			auth: NewAuthorization("dec_123", "prop_456", agentApprover, "1.2.0").
				RecordApproval(agentApprover, ApprovalActionApprove, "Auto-approved"),
			expected: false,
		},
		{
			name: "ci approval only",
			auth: NewAuthorization("dec_123", "prop_456", ciApprover, "1.2.0").
				RecordApproval(ciApprover, ApprovalActionApprove, "CI approved"),
			expected: false,
		},
		{
			name: "mixed with human approval",
			auth: NewAuthorization("dec_123", "prop_456", agentApprover, "1.2.0").
				RecordApproval(agentApprover, ApprovalActionComment, "Looks good").
				RecordApproval(humanApprover, ApprovalActionApprove, "Human approved"),
			expected: true,
		},
		{
			name:     "no approvals",
			auth:     NewAuthorization("dec_123", "prop_456", humanApprover, "1.2.0"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.auth.HasHumanApproval(); got != tt.expected {
				t.Errorf("HasHumanApproval() = %v, want %v", got, tt.expected)
			}
		})
	}
}
