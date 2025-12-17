package cgp

import (
	"strings"
	"testing"
)

func TestBumpType_String(t *testing.T) {
	tests := []struct {
		name     string
		bump     BumpType
		expected string
	}{
		{"major", BumpTypeMajor, "major"},
		{"minor", BumpTypeMinor, "minor"},
		{"patch", BumpTypePatch, "patch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bump.String(); got != tt.expected {
				t.Errorf("BumpType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBumpType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		bump     BumpType
		expected bool
	}{
		{"major is valid", BumpTypeMajor, true},
		{"minor is valid", BumpTypeMinor, true},
		{"patch is valid", BumpTypePatch, true},
		{"empty is invalid", BumpType(""), false},
		{"unknown is invalid", BumpType("prerelease"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bump.IsValid(); got != tt.expected {
				t.Errorf("BumpType.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewProposal(t *testing.T) {
	actor := NewAgentActor("cursor", "Cursor", "gpt-4")
	scope := ProposalScope{
		Repository:  "owner/repo",
		CommitRange: "abc123..def456",
	}
	intent := ProposalIntent{
		Summary:       "Add new feature",
		SuggestedBump: BumpTypeMinor,
		Confidence:    0.9,
	}

	proposal := NewProposal(actor, scope, intent)

	if proposal.CGPVersion != Version {
		t.Errorf("NewProposal().CGPVersion = %v, want %v", proposal.CGPVersion, Version)
	}
	if proposal.Type != MessageTypeProposal {
		t.Errorf("NewProposal().Type = %v, want %v", proposal.Type, MessageTypeProposal)
	}
	if !strings.HasPrefix(proposal.ID, "prop_") {
		t.Errorf("NewProposal().ID = %v, should start with 'prop_'", proposal.ID)
	}
	if proposal.Timestamp.IsZero() {
		t.Error("NewProposal().Timestamp should not be zero")
	}
	if proposal.Actor.ID != actor.ID {
		t.Errorf("NewProposal().Actor.ID = %v, want %v", proposal.Actor.ID, actor.ID)
	}
	if proposal.Scope.Repository != scope.Repository {
		t.Errorf("NewProposal().Scope.Repository = %v, want %v", proposal.Scope.Repository, scope.Repository)
	}
	if proposal.Intent.Summary != intent.Summary {
		t.Errorf("NewProposal().Intent.Summary = %v, want %v", proposal.Intent.Summary, intent.Summary)
	}
}

func TestGenerateProposalID(t *testing.T) {
	id1 := GenerateProposalID()
	id2 := GenerateProposalID()

	if !strings.HasPrefix(id1, "prop_") {
		t.Errorf("GenerateProposalID() = %v, should start with 'prop_'", id1)
	}
	if id1 == id2 {
		t.Error("GenerateProposalID() should generate unique IDs")
	}
}

func TestChangeProposal_Validate(t *testing.T) {
	validActor := NewAgentActor("cursor", "Cursor", "gpt-4")
	validScope := ProposalScope{
		Repository:  "owner/repo",
		CommitRange: "abc123..def456",
	}
	validIntent := ProposalIntent{
		Summary:    "Add new feature",
		Confidence: 0.9,
	}

	tests := []struct {
		name      string
		proposal  *ChangeProposal
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid proposal",
			proposal:  NewProposal(validActor, validScope, validIntent),
			expectErr: false,
		},
		{
			name: "missing CGP version",
			proposal: &ChangeProposal{
				Type:   MessageTypeProposal,
				ID:     "prop_123",
				Actor:  validActor,
				Scope:  validScope,
				Intent: validIntent,
			},
			expectErr: true,
			errMsg:    "CGP version is required",
		},
		{
			name: "invalid message type",
			proposal: &ChangeProposal{
				CGPVersion: Version,
				Type:       MessageTypeDecision,
				ID:         "prop_123",
				Actor:      validActor,
				Scope:      validScope,
				Intent:     validIntent,
			},
			expectErr: true,
			errMsg:    "invalid message type",
		},
		{
			name: "missing ID",
			proposal: &ChangeProposal{
				CGPVersion: Version,
				Type:       MessageTypeProposal,
				ID:         "",
				Actor:      validActor,
				Scope:      validScope,
				Intent:     validIntent,
			},
			expectErr: true,
			errMsg:    "proposal ID is required",
		},
		{
			name: "invalid actor",
			proposal: &ChangeProposal{
				CGPVersion: Version,
				Type:       MessageTypeProposal,
				ID:         "prop_123",
				Actor:      Actor{Kind: ActorKind("invalid"), ID: "test"},
				Scope:      validScope,
				Intent:     validIntent,
			},
			expectErr: true,
			errMsg:    "invalid actor",
		},
		{
			name: "invalid scope - missing repository",
			proposal: &ChangeProposal{
				CGPVersion: Version,
				Type:       MessageTypeProposal,
				ID:         "prop_123",
				Actor:      validActor,
				Scope:      ProposalScope{CommitRange: "abc..def"},
				Intent:     validIntent,
			},
			expectErr: true,
			errMsg:    "repository is required",
		},
		{
			name: "invalid intent - missing summary",
			proposal: &ChangeProposal{
				CGPVersion: Version,
				Type:       MessageTypeProposal,
				ID:         "prop_123",
				Actor:      validActor,
				Scope:      validScope,
				Intent:     ProposalIntent{Confidence: 0.5},
			},
			expectErr: true,
			errMsg:    "summary is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.proposal.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("ChangeProposal.Validate() error = %v, expectErr %v", err, tt.expectErr)
			}
			if tt.expectErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ChangeProposal.Validate() error = %v, should contain %v", err, tt.errMsg)
			}
		})
	}
}

func TestProposalScope_Validate(t *testing.T) {
	tests := []struct {
		name      string
		scope     ProposalScope
		expectErr bool
	}{
		{
			name: "valid with commit range",
			scope: ProposalScope{
				Repository:  "owner/repo",
				CommitRange: "abc..def",
			},
			expectErr: false,
		},
		{
			name: "valid with commits",
			scope: ProposalScope{
				Repository: "owner/repo",
				Commits:    []string{"abc123", "def456"},
			},
			expectErr: false,
		},
		{
			name: "missing repository",
			scope: ProposalScope{
				CommitRange: "abc..def",
			},
			expectErr: true,
		},
		{
			name: "missing both commit range and commits",
			scope: ProposalScope{
				Repository: "owner/repo",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.scope.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("ProposalScope.Validate() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestProposalIntent_Validate(t *testing.T) {
	tests := []struct {
		name      string
		intent    ProposalIntent
		expectErr bool
	}{
		{
			name: "valid intent",
			intent: ProposalIntent{
				Summary:    "Add new feature",
				Confidence: 0.9,
			},
			expectErr: false,
		},
		{
			name: "valid with bump type",
			intent: ProposalIntent{
				Summary:       "Fix bug",
				SuggestedBump: BumpTypePatch,
				Confidence:    0.8,
			},
			expectErr: false,
		},
		{
			name: "missing summary",
			intent: ProposalIntent{
				Confidence: 0.5,
			},
			expectErr: true,
		},
		{
			name: "confidence below 0",
			intent: ProposalIntent{
				Summary:    "Test",
				Confidence: -0.1,
			},
			expectErr: true,
		},
		{
			name: "confidence above 1",
			intent: ProposalIntent{
				Summary:    "Test",
				Confidence: 1.1,
			},
			expectErr: true,
		},
		{
			name: "invalid bump type",
			intent: ProposalIntent{
				Summary:       "Test",
				SuggestedBump: BumpType("invalid"),
				Confidence:    0.5,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.intent.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("ProposalIntent.Validate() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestProposalIntent_HasBreakingChanges(t *testing.T) {
	tests := []struct {
		name     string
		intent   ProposalIntent
		expected bool
	}{
		{
			name:     "no breaking changes",
			intent:   ProposalIntent{Summary: "Test"},
			expected: false,
		},
		{
			name: "has breaking changes",
			intent: ProposalIntent{
				Summary:         "Test",
				BreakingChanges: []string{"Removed deprecated API"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.intent.HasBreakingChanges(); got != tt.expected {
				t.Errorf("ProposalIntent.HasBreakingChanges() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProposalIntent_ConfidenceLevels(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
		high       bool
		medium     bool
		low        bool
	}{
		{"high confidence", 0.9, true, false, false},
		{"high boundary", 0.8, true, false, false},
		{"medium confidence", 0.6, false, true, false},
		{"medium lower boundary", 0.5, false, true, false},
		{"low confidence", 0.3, false, false, true},
		{"low boundary", 0.49, false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := ProposalIntent{Summary: "Test", Confidence: tt.confidence}
			if got := intent.IsHighConfidence(); got != tt.high {
				t.Errorf("ProposalIntent.IsHighConfidence() = %v, want %v", got, tt.high)
			}
			if got := intent.IsMediumConfidence(); got != tt.medium {
				t.Errorf("ProposalIntent.IsMediumConfidence() = %v, want %v", got, tt.medium)
			}
			if got := intent.IsLowConfidence(); got != tt.low {
				t.Errorf("ProposalIntent.IsLowConfidence() = %v, want %v", got, tt.low)
			}
		})
	}
}

func TestChangeProposal_WithContext(t *testing.T) {
	proposal := NewProposal(
		NewAgentActor("cursor", "Cursor", "gpt-4"),
		ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		ProposalIntent{Summary: "Test", Confidence: 0.9},
	)

	ctx := &ProposalContext{
		AgentSession: "session-123",
	}

	result := proposal.WithContext(ctx)
	if result.Context != ctx {
		t.Error("WithContext should set context")
	}
	if result != proposal {
		t.Error("WithContext should return the same proposal for chaining")
	}
}

func TestChangeProposal_AddIssue(t *testing.T) {
	proposal := NewProposal(
		NewAgentActor("cursor", "Cursor", "gpt-4"),
		ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		ProposalIntent{Summary: "Test", Confidence: 0.9},
	)

	result := proposal.AddIssue("github", "123", "https://github.com/owner/repo/issues/123")

	if proposal.Context == nil {
		t.Error("AddIssue should create context if nil")
	}
	if len(proposal.Context.Issues) != 1 {
		t.Errorf("AddIssue should add issue, got %d issues", len(proposal.Context.Issues))
	}
	if proposal.Context.Issues[0].Provider != "github" {
		t.Errorf("Issue provider = %v, want github", proposal.Context.Issues[0].Provider)
	}
	if result != proposal {
		t.Error("AddIssue should return the same proposal for chaining")
	}
}

func TestChangeProposal_AddMetadata(t *testing.T) {
	proposal := NewProposal(
		NewAgentActor("cursor", "Cursor", "gpt-4"),
		ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		ProposalIntent{Summary: "Test", Confidence: 0.9},
	)

	result := proposal.AddMetadata("key", "value")

	if proposal.Context == nil {
		t.Error("AddMetadata should create context if nil")
	}
	if proposal.Context.Metadata == nil {
		t.Error("AddMetadata should create metadata map if nil")
	}
	if proposal.Context.Metadata["key"] != "value" {
		t.Errorf("Metadata[key] = %v, want value", proposal.Context.Metadata["key"])
	}
	if result != proposal {
		t.Error("AddMetadata should return the same proposal for chaining")
	}
}
