package cgp

import (
	"encoding/json"
	"testing"
	"time"
)

// =============================================================================
// Message Validation Tests
// =============================================================================

func TestValidateProposal(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name    string
		input   *ChangeProposal
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil proposal",
			input:   nil,
			wantErr: true,
			errMsg:  "proposal is nil",
		},
		{
			name: "valid proposal",
			input: &ChangeProposal{
				CGPVersion: ProtocolVersion,
				Type:       TypeChangeProposal,
				ID:         "prop_abc123",
				Timestamp:  now,
				Actor:      Actor{Kind: "agent", ID: "agent:cursor"},
				Scope:      Scope{Repository: "owner/repo", CommitRange: "abc..def"},
				Intent:     Intent{Summary: "Add login feature", Confidence: 0.9},
			},
			wantErr: false,
		},
		{
			name: "valid proposal with commits instead of range",
			input: &ChangeProposal{
				CGPVersion: ProtocolVersion,
				Type:       TypeChangeProposal,
				ID:         "prop_abc123",
				Timestamp:  now,
				Actor:      Actor{Kind: "ci", ID: "ci:github-actions"},
				Scope:      Scope{Repository: "owner/repo", Commits: []string{"abc123"}},
				Intent:     Intent{Summary: "Fix bug", Confidence: 0.5},
			},
			wantErr: false,
		},
		{
			name: "missing cgpVersion",
			input: &ChangeProposal{
				Type:      TypeChangeProposal,
				ID:        "prop_abc123",
				Timestamp: now,
				Actor:     Actor{Kind: "agent", ID: "agent:cursor"},
				Scope:     Scope{Repository: "owner/repo", CommitRange: "abc..def"},
				Intent:    Intent{Summary: "test", Confidence: 0.5},
			},
			wantErr: true,
			errMsg:  "cgpVersion is required",
		},
		{
			name: "wrong type",
			input: &ChangeProposal{
				CGPVersion: ProtocolVersion,
				Type:       TypeGovernanceDecision,
				ID:         "prop_abc123",
				Timestamp:  now,
				Actor:      Actor{Kind: "agent", ID: "agent:cursor"},
				Scope:      Scope{Repository: "owner/repo", CommitRange: "abc..def"},
				Intent:     Intent{Summary: "test", Confidence: 0.5},
			},
			wantErr: true,
			errMsg:  "type must be",
		},
		{
			name: "missing id",
			input: &ChangeProposal{
				CGPVersion: ProtocolVersion,
				Type:       TypeChangeProposal,
				Timestamp:  now,
				Actor:      Actor{Kind: "agent", ID: "agent:cursor"},
				Scope:      Scope{Repository: "owner/repo", CommitRange: "abc..def"},
				Intent:     Intent{Summary: "test", Confidence: 0.5},
			},
			wantErr: true,
			errMsg:  "id is required",
		},
		{
			name: "missing actor kind",
			input: &ChangeProposal{
				CGPVersion: ProtocolVersion,
				Type:       TypeChangeProposal,
				ID:         "prop_abc123",
				Timestamp:  now,
				Actor:      Actor{ID: "agent:cursor"},
				Scope:      Scope{Repository: "owner/repo", CommitRange: "abc..def"},
				Intent:     Intent{Summary: "test", Confidence: 0.5},
			},
			wantErr: true,
			errMsg:  "actor.kind is required",
		},
		{
			name: "missing repository",
			input: &ChangeProposal{
				CGPVersion: ProtocolVersion,
				Type:       TypeChangeProposal,
				ID:         "prop_abc123",
				Timestamp:  now,
				Actor:      Actor{Kind: "agent", ID: "agent:cursor"},
				Scope:      Scope{CommitRange: "abc..def"},
				Intent:     Intent{Summary: "test", Confidence: 0.5},
			},
			wantErr: true,
			errMsg:  "scope.repository is required",
		},
		{
			name: "missing commit range and commits",
			input: &ChangeProposal{
				CGPVersion: ProtocolVersion,
				Type:       TypeChangeProposal,
				ID:         "prop_abc123",
				Timestamp:  now,
				Actor:      Actor{Kind: "agent", ID: "agent:cursor"},
				Scope:      Scope{Repository: "owner/repo"},
				Intent:     Intent{Summary: "test", Confidence: 0.5},
			},
			wantErr: true,
			errMsg:  "scope requires commitRange or commits",
		},
		{
			name: "missing summary",
			input: &ChangeProposal{
				CGPVersion: ProtocolVersion,
				Type:       TypeChangeProposal,
				ID:         "prop_abc123",
				Timestamp:  now,
				Actor:      Actor{Kind: "agent", ID: "agent:cursor"},
				Scope:      Scope{Repository: "owner/repo", CommitRange: "abc..def"},
				Intent:     Intent{Confidence: 0.5},
			},
			wantErr: true,
			errMsg:  "intent.summary is required",
		},
		{
			name: "confidence out of range negative",
			input: &ChangeProposal{
				CGPVersion: ProtocolVersion,
				Type:       TypeChangeProposal,
				ID:         "prop_abc123",
				Timestamp:  now,
				Actor:      Actor{Kind: "agent", ID: "agent:cursor"},
				Scope:      Scope{Repository: "owner/repo", CommitRange: "abc..def"},
				Intent:     Intent{Summary: "test", Confidence: -0.1},
			},
			wantErr: true,
			errMsg:  "intent.confidence must be between",
		},
		{
			name: "confidence out of range high",
			input: &ChangeProposal{
				CGPVersion: ProtocolVersion,
				Type:       TypeChangeProposal,
				ID:         "prop_abc123",
				Timestamp:  now,
				Actor:      Actor{Kind: "agent", ID: "agent:cursor"},
				Scope:      Scope{Repository: "owner/repo", CommitRange: "abc..def"},
				Intent:     Intent{Summary: "test", Confidence: 1.5},
			},
			wantErr: true,
			errMsg:  "intent.confidence must be between",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProposal(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateDecision(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name    string
		input   *GovernanceDecision
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil decision",
			input:   nil,
			wantErr: true,
		},
		{
			name: "valid approved decision",
			input: &GovernanceDecision{
				CGPVersion: ProtocolVersion,
				Type:       TypeGovernanceDecision,
				ID:         "dec_abc123",
				ProposalID: "prop_abc123",
				Timestamp:  now,
				Decision:   "approved",
				RiskScore:  0.2,
				Rationale:  []string{"Low risk change"},
			},
			wantErr: false,
		},
		{
			name: "valid denied decision",
			input: &GovernanceDecision{
				CGPVersion: ProtocolVersion,
				Type:       TypeGovernanceDecision,
				ID:         "dec_abc123",
				ProposalID: "prop_abc123",
				Timestamp:  now,
				Decision:   "denied",
				RiskScore:  0.9,
				Rationale:  []string{"Policy violation"},
			},
			wantErr: false,
		},
		{
			name: "valid approval_required decision",
			input: &GovernanceDecision{
				CGPVersion: ProtocolVersion,
				Type:       TypeGovernanceDecision,
				ID:         "dec_abc123",
				ProposalID: "prop_abc123",
				Timestamp:  now,
				Decision:   "approval_required",
				RiskScore:  0.6,
				Rationale:  []string{"Human review needed"},
			},
			wantErr: false,
		},
		{
			name: "invalid decision value",
			input: &GovernanceDecision{
				CGPVersion: ProtocolVersion,
				Type:       TypeGovernanceDecision,
				ID:         "dec_abc123",
				ProposalID: "prop_abc123",
				Timestamp:  now,
				Decision:   "maybe",
				RiskScore:  0.5,
			},
			wantErr: true,
			errMsg:  "decision must be one of",
		},
		{
			name: "risk score out of range",
			input: &GovernanceDecision{
				CGPVersion: ProtocolVersion,
				Type:       TypeGovernanceDecision,
				ID:         "dec_abc123",
				ProposalID: "prop_abc123",
				Timestamp:  now,
				Decision:   "approved",
				RiskScore:  1.5,
			},
			wantErr: true,
			errMsg:  "riskScore must be between",
		},
		{
			name: "missing proposalId",
			input: &GovernanceDecision{
				CGPVersion: ProtocolVersion,
				Type:       TypeGovernanceDecision,
				ID:         "dec_abc123",
				Timestamp:  now,
				Decision:   "approved",
				RiskScore:  0.3,
			},
			wantErr: true,
			errMsg:  "proposalId is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDecision(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateEvaluation(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name    string
		input   *GovernanceEvaluation
		wantErr bool
	}{
		{
			name:    "nil evaluation",
			input:   nil,
			wantErr: true,
		},
		{
			name: "valid evaluation",
			input: &GovernanceEvaluation{
				CGPVersion: ProtocolVersion,
				Type:       TypeGovernanceEvaluation,
				ID:         "eval_abc123",
				ProposalID: "prop_abc123",
				Timestamp:  now,
				RiskScore:  0.4,
				Rationale:  []string{"Medium risk"},
			},
			wantErr: false,
		},
		{
			name: "missing proposalId",
			input: &GovernanceEvaluation{
				CGPVersion: ProtocolVersion,
				Type:       TypeGovernanceEvaluation,
				ID:         "eval_abc123",
				Timestamp:  now,
				RiskScore:  0.4,
			},
			wantErr: true,
		},
		{
			name: "risk score negative",
			input: &GovernanceEvaluation{
				CGPVersion: ProtocolVersion,
				Type:       TypeGovernanceEvaluation,
				ID:         "eval_abc123",
				ProposalID: "prop_abc123",
				Timestamp:  now,
				RiskScore:  -0.1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEvaluation(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateAuthorization(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name    string
		input   *ExecutionAuthorization
		wantErr bool
	}{
		{
			name:    "nil authorization",
			input:   nil,
			wantErr: true,
		},
		{
			name: "valid authorization",
			input: &ExecutionAuthorization{
				CGPVersion: ProtocolVersion,
				Type:       TypeExecutionAuthorization,
				ID:         "auth_abc123",
				ProposalID: "prop_abc123",
				DecisionID: "dec_abc123",
				Timestamp:  now,
				ApprovedBy: Actor{Kind: "human", ID: "human:alice@example.com"},
				Version:    "1.2.0",
				ValidUntil: now.Add(24 * time.Hour),
			},
			wantErr: false,
		},
		{
			name: "missing version",
			input: &ExecutionAuthorization{
				CGPVersion: ProtocolVersion,
				Type:       TypeExecutionAuthorization,
				ID:         "auth_abc123",
				ProposalID: "prop_abc123",
				DecisionID: "dec_abc123",
				Timestamp:  now,
				ApprovedBy: Actor{Kind: "human", ID: "human:alice@example.com"},
				ValidUntil: now.Add(24 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "missing decisionId",
			input: &ExecutionAuthorization{
				CGPVersion: ProtocolVersion,
				Type:       TypeExecutionAuthorization,
				ID:         "auth_abc123",
				ProposalID: "prop_abc123",
				Timestamp:  now,
				ApprovedBy: Actor{Kind: "human", ID: "human:alice@example.com"},
				Version:    "1.2.0",
				ValidUntil: now.Add(24 * time.Hour),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAuthorization(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// =============================================================================
// Codec Round-Trip Tests
// =============================================================================

func TestMarshalUnmarshalProposal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := &ChangeProposal{
		CGPVersion: ProtocolVersion,
		Type:       TypeChangeProposal,
		ID:         "prop_roundtrip",
		Timestamp:  now,
		Actor:      Actor{Kind: "agent", ID: "agent:claude", Name: "Claude"},
		Scope:      Scope{Repository: "relicta-tech/relicta", CommitRange: "v1.0.0..HEAD", Files: []string{"main.go"}},
		Intent:     Intent{Summary: "Add authentication", Confidence: 0.85, Categories: []string{"feature", "security"}},
		Metadata:   Metadata{"session": "sess_123"},
	}

	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Verify envelope structure
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("failed to decode envelope: %v", err)
	}
	if env.CGPVersion != ProtocolVersion {
		t.Errorf("envelope cgpVersion = %q, want %q", env.CGPVersion, ProtocolVersion)
	}
	if env.Type != TypeChangeProposal {
		t.Errorf("envelope type = %q, want %q", env.Type, TypeChangeProposal)
	}

	// Unmarshal and verify
	result, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	p, ok := result.(*ChangeProposal)
	if !ok {
		t.Fatalf("expected *ChangeProposal, got %T", result)
	}

	if p.ID != original.ID {
		t.Errorf("ID = %q, want %q", p.ID, original.ID)
	}
	if p.Actor.Kind != original.Actor.Kind {
		t.Errorf("Actor.Kind = %q, want %q", p.Actor.Kind, original.Actor.Kind)
	}
	if p.Actor.Name != original.Actor.Name {
		t.Errorf("Actor.Name = %q, want %q", p.Actor.Name, original.Actor.Name)
	}
	if p.Scope.Repository != original.Scope.Repository {
		t.Errorf("Scope.Repository = %q, want %q", p.Scope.Repository, original.Scope.Repository)
	}
	if p.Intent.Summary != original.Intent.Summary {
		t.Errorf("Intent.Summary = %q, want %q", p.Intent.Summary, original.Intent.Summary)
	}
	if p.Intent.Confidence != original.Intent.Confidence {
		t.Errorf("Intent.Confidence = %f, want %f", p.Intent.Confidence, original.Intent.Confidence)
	}
}

func TestMarshalUnmarshalDecision(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := &GovernanceDecision{
		CGPVersion:         ProtocolVersion,
		Type:               TypeGovernanceDecision,
		ID:                 "dec_roundtrip",
		ProposalID:         "prop_abc",
		Timestamp:          now,
		Decision:           "approval_required",
		RiskScore:          0.65,
		RecommendedVersion: "2.0.0",
		Rationale:          []string{"Breaking changes detected", "High blast radius"},
		RequiredActions: []RequiredAction{
			{Type: "human_approval", Description: "Review breaking API changes"},
		},
		Conditions: []Condition{
			{Type: "time_window", Value: "business_hours"},
		},
	}

	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	result, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	d, ok := result.(*GovernanceDecision)
	if !ok {
		t.Fatalf("expected *GovernanceDecision, got %T", result)
	}

	if d.Decision != original.Decision {
		t.Errorf("Decision = %q, want %q", d.Decision, original.Decision)
	}
	if d.RiskScore != original.RiskScore {
		t.Errorf("RiskScore = %f, want %f", d.RiskScore, original.RiskScore)
	}
	if d.RecommendedVersion != original.RecommendedVersion {
		t.Errorf("RecommendedVersion = %q, want %q", d.RecommendedVersion, original.RecommendedVersion)
	}
	if len(d.Rationale) != len(original.Rationale) {
		t.Errorf("Rationale count = %d, want %d", len(d.Rationale), len(original.Rationale))
	}
	if len(d.RequiredActions) != 1 {
		t.Errorf("RequiredActions count = %d, want 1", len(d.RequiredActions))
	}
	if len(d.Conditions) != 1 {
		t.Errorf("Conditions count = %d, want 1", len(d.Conditions))
	}
}

func TestMarshalUnmarshalEvaluation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := &GovernanceEvaluation{
		CGPVersion:         ProtocolVersion,
		Type:               TypeGovernanceEvaluation,
		ID:                 "eval_roundtrip",
		ProposalID:         "prop_abc",
		Timestamp:          now,
		RiskScore:          0.35,
		RecommendedVersion: "1.3.0",
		Rationale:          []string{"Low risk patch"},
		PolicyResults: []PolicyResult{
			{PolicyID: "pol_1", Name: "max-risk", Matched: true, Decision: "approved", Rationale: "Score within threshold"},
		},
	}

	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	result, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	e, ok := result.(*GovernanceEvaluation)
	if !ok {
		t.Fatalf("expected *GovernanceEvaluation, got %T", result)
	}

	if e.RiskScore != original.RiskScore {
		t.Errorf("RiskScore = %f, want %f", e.RiskScore, original.RiskScore)
	}
	if len(e.PolicyResults) != 1 {
		t.Fatalf("PolicyResults count = %d, want 1", len(e.PolicyResults))
	}
	if e.PolicyResults[0].PolicyID != "pol_1" {
		t.Errorf("PolicyResults[0].PolicyID = %q, want %q", e.PolicyResults[0].PolicyID, "pol_1")
	}
}

func TestMarshalUnmarshalAuthorization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := &ExecutionAuthorization{
		CGPVersion: ProtocolVersion,
		Type:       TypeExecutionAuthorization,
		ID:         "auth_roundtrip",
		ProposalID: "prop_abc",
		DecisionID: "dec_abc",
		Timestamp:  now,
		ApprovedBy: Actor{Kind: "human", ID: "human:alice@example.com", Name: "Alice"},
		Version:    "1.3.0",
		ValidUntil: now.Add(24 * time.Hour),
		Scope:      []string{"tag", "changelog", "publish"},
	}

	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	result, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	a, ok := result.(*ExecutionAuthorization)
	if !ok {
		t.Fatalf("expected *ExecutionAuthorization, got %T", result)
	}

	if a.Version != original.Version {
		t.Errorf("Version = %q, want %q", a.Version, original.Version)
	}
	if a.ApprovedBy.ID != original.ApprovedBy.ID {
		t.Errorf("ApprovedBy.ID = %q, want %q", a.ApprovedBy.ID, original.ApprovedBy.ID)
	}
	if len(a.Scope) != 3 {
		t.Errorf("Scope count = %d, want 3", len(a.Scope))
	}
}

// =============================================================================
// Version Envelope Tests
// =============================================================================

func TestEnvelopeContainsVersion(t *testing.T) {
	proposal := &ChangeProposal{
		CGPVersion: ProtocolVersion,
		Type:       TypeChangeProposal,
		ID:         "prop_env",
		Timestamp:  time.Now().UTC(),
		Actor:      Actor{Kind: "agent", ID: "agent:test"},
		Scope:      Scope{Repository: "test/repo", CommitRange: "a..b"},
		Intent:     Intent{Summary: "test", Confidence: 0.5},
	}

	data, err := Marshal(proposal)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to parse as generic JSON: %v", err)
	}

	// Verify top-level envelope has cgpVersion
	if _, ok := raw["cgpVersion"]; !ok {
		t.Error("envelope missing cgpVersion field")
	}

	// Verify top-level envelope has type
	if _, ok := raw["type"]; !ok {
		t.Error("envelope missing type field")
	}

	// Verify payload is present
	if _, ok := raw["payload"]; !ok {
		t.Error("envelope missing payload field")
	}
}

func TestUnmarshalInvalidEnvelope(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "invalid JSON",
			input: `{not json}`,
		},
		{
			name:  "missing cgpVersion",
			input: `{"type":"change.proposal","payload":{}}`,
		},
		{
			name:  "unknown type",
			input: `{"cgpVersion":"0.1","type":"unknown.type","payload":{}}`,
		},
		{
			name:  "empty envelope",
			input: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Unmarshal([]byte(tt.input))
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestUnmarshalEnvelope(t *testing.T) {
	proposal := &ChangeProposal{
		CGPVersion: ProtocolVersion,
		Type:       TypeChangeProposal,
		ID:         "prop_peek",
		Timestamp:  time.Now().UTC(),
		Actor:      Actor{Kind: "agent", ID: "agent:test"},
		Scope:      Scope{Repository: "test/repo", CommitRange: "a..b"},
		Intent:     Intent{Summary: "test", Confidence: 0.5},
	}

	data, err := Marshal(proposal)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	env, err := UnmarshalEnvelope(data)
	if err != nil {
		t.Fatalf("UnmarshalEnvelope failed: %v", err)
	}

	if env.CGPVersion != ProtocolVersion {
		t.Errorf("CGPVersion = %q, want %q", env.CGPVersion, ProtocolVersion)
	}
	if env.Type != TypeChangeProposal {
		t.Errorf("Type = %q, want %q", env.Type, TypeChangeProposal)
	}
	if len(env.Payload) == 0 {
		t.Error("Payload is empty")
	}
}

func TestMarshalUnsupportedType(t *testing.T) {
	_, err := Marshal("not a CGP message")
	if err == nil {
		t.Error("expected error for unsupported type")
	}
}

// =============================================================================
// Message Type Tests
// =============================================================================

func TestMessageTypeIsValid(t *testing.T) {
	tests := []struct {
		input MessageType
		valid bool
	}{
		{TypeChangeProposal, true},
		{TypeGovernanceEvaluation, true},
		{TypeGovernanceDecision, true},
		{TypeExecutionAuthorization, true},
		{"unknown.type", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := tt.input.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestProtocolVersion(t *testing.T) {
	if ProtocolVersion == "" {
		t.Error("ProtocolVersion must not be empty")
	}
	if ProtocolVersion != "0.1" {
		t.Errorf("ProtocolVersion = %q, want %q", ProtocolVersion, "0.1")
	}
}

// =============================================================================
// Validation Error Type Tests
// =============================================================================

func TestValidationErrorMultipleErrors(t *testing.T) {
	// A proposal with many missing fields should report multiple errors.
	p := &ChangeProposal{
		Type: TypeChangeProposal,
	}
	err := ValidateProposal(p)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}

	if len(ve.Errors) < 3 {
		t.Errorf("expected at least 3 errors, got %d: %v", len(ve.Errors), ve.Errors)
	}
}

// =============================================================================
// Helpers
// =============================================================================

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
