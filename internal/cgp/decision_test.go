package cgp

import (
	"strings"
	"testing"
)

func TestDecisionType_String(t *testing.T) {
	tests := []struct {
		name     string
		decision DecisionType
		expected string
	}{
		{"approved", DecisionApproved, "approved"},
		{"approval_required", DecisionApprovalRequired, "approval_required"},
		{"rejected", DecisionRejected, "rejected"},
		{"deferred", DecisionDeferred, "deferred"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.decision.String(); got != tt.expected {
				t.Errorf("DecisionType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDecisionType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		decision DecisionType
		expected bool
	}{
		{"approved is valid", DecisionApproved, true},
		{"approval_required is valid", DecisionApprovalRequired, true},
		{"rejected is valid", DecisionRejected, true},
		{"deferred is valid", DecisionDeferred, true},
		{"empty is invalid", DecisionType(""), false},
		{"unknown is invalid", DecisionType("pending"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.decision.IsValid(); got != tt.expected {
				t.Errorf("DecisionType.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDecisionType_AllowsExecution(t *testing.T) {
	tests := []struct {
		name     string
		decision DecisionType
		expected bool
	}{
		{"approved allows execution", DecisionApproved, true},
		{"approval_required does not allow", DecisionApprovalRequired, false},
		{"rejected does not allow", DecisionRejected, false},
		{"deferred does not allow", DecisionDeferred, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.decision.AllowsExecution(); got != tt.expected {
				t.Errorf("DecisionType.AllowsExecution() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDecisionType_RequiresHumanAction(t *testing.T) {
	tests := []struct {
		name     string
		decision DecisionType
		expected bool
	}{
		{"approved does not require human", DecisionApproved, false},
		{"approval_required requires human", DecisionApprovalRequired, true},
		{"rejected does not require human", DecisionRejected, false},
		{"deferred requires human", DecisionDeferred, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.decision.RequiresHumanAction(); got != tt.expected {
				t.Errorf("DecisionType.RequiresHumanAction() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDecisionType_IsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		decision DecisionType
		expected bool
	}{
		{"approved is terminal", DecisionApproved, true},
		{"approval_required is not terminal", DecisionApprovalRequired, false},
		{"rejected is terminal", DecisionRejected, true},
		{"deferred is not terminal", DecisionDeferred, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.decision.IsTerminal(); got != tt.expected {
				t.Errorf("DecisionType.IsTerminal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAllDecisionTypes(t *testing.T) {
	types := AllDecisionTypes()
	if len(types) != 4 {
		t.Errorf("AllDecisionTypes() returned %d types, want 4", len(types))
	}

	expected := map[DecisionType]bool{
		DecisionApproved:         true,
		DecisionApprovalRequired: true,
		DecisionRejected:         true,
		DecisionDeferred:         true,
	}

	for _, dt := range types {
		if !expected[dt] {
			t.Errorf("Unexpected decision type: %v", dt)
		}
	}
}

func TestNewDecision(t *testing.T) {
	decision := NewDecision("prop_123", DecisionApproved)

	if decision.CGPVersion != Version {
		t.Errorf("NewDecision().CGPVersion = %v, want %v", decision.CGPVersion, Version)
	}
	if decision.Type != MessageTypeDecision {
		t.Errorf("NewDecision().Type = %v, want %v", decision.Type, MessageTypeDecision)
	}
	if !strings.HasPrefix(decision.ID, "dec_") {
		t.Errorf("NewDecision().ID = %v, should start with 'dec_'", decision.ID)
	}
	if decision.ProposalID != "prop_123" {
		t.Errorf("NewDecision().ProposalID = %v, want prop_123", decision.ProposalID)
	}
	if decision.Decision != DecisionApproved {
		t.Errorf("NewDecision().Decision = %v, want %v", decision.Decision, DecisionApproved)
	}
	if decision.Timestamp.IsZero() {
		t.Error("NewDecision().Timestamp should not be zero")
	}
	if decision.RiskFactors == nil {
		t.Error("NewDecision().RiskFactors should not be nil")
	}
	if decision.Rationale == nil {
		t.Error("NewDecision().Rationale should not be nil")
	}
}

func TestGenerateDecisionID(t *testing.T) {
	id1 := GenerateDecisionID()
	id2 := GenerateDecisionID()

	if !strings.HasPrefix(id1, "dec_") {
		t.Errorf("GenerateDecisionID() = %v, should start with 'dec_'", id1)
	}
	if id1 == id2 {
		t.Error("GenerateDecisionID() should generate unique IDs")
	}
}

func TestGovernanceDecision_Validate(t *testing.T) {
	tests := []struct {
		name      string
		decision  *GovernanceDecision
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid decision",
			decision:  NewDecision("prop_123", DecisionApproved),
			expectErr: false,
		},
		{
			name: "missing CGP version",
			decision: &GovernanceDecision{
				Type:       MessageTypeDecision,
				ID:         "dec_123",
				ProposalID: "prop_123",
				Decision:   DecisionApproved,
			},
			expectErr: true,
			errMsg:    "CGP version is required",
		},
		{
			name: "invalid message type",
			decision: &GovernanceDecision{
				CGPVersion: Version,
				Type:       MessageTypeProposal,
				ID:         "dec_123",
				ProposalID: "prop_123",
				Decision:   DecisionApproved,
			},
			expectErr: true,
			errMsg:    "invalid message type",
		},
		{
			name: "missing ID",
			decision: &GovernanceDecision{
				CGPVersion: Version,
				Type:       MessageTypeDecision,
				ProposalID: "prop_123",
				Decision:   DecisionApproved,
			},
			expectErr: true,
			errMsg:    "decision ID is required",
		},
		{
			name: "missing proposal ID",
			decision: &GovernanceDecision{
				CGPVersion: Version,
				Type:       MessageTypeDecision,
				ID:         "dec_123",
				Decision:   DecisionApproved,
			},
			expectErr: true,
			errMsg:    "proposal ID is required",
		},
		{
			name: "invalid decision type",
			decision: &GovernanceDecision{
				CGPVersion: Version,
				Type:       MessageTypeDecision,
				ID:         "dec_123",
				ProposalID: "prop_123",
				Decision:   DecisionType("invalid"),
			},
			expectErr: true,
			errMsg:    "invalid decision type",
		},
		{
			name: "risk score below 0",
			decision: &GovernanceDecision{
				CGPVersion: Version,
				Type:       MessageTypeDecision,
				ID:         "dec_123",
				ProposalID: "prop_123",
				Decision:   DecisionApproved,
				RiskScore:  -0.1,
			},
			expectErr: true,
			errMsg:    "risk score must be between 0.0 and 1.0",
		},
		{
			name: "risk score above 1",
			decision: &GovernanceDecision{
				CGPVersion: Version,
				Type:       MessageTypeDecision,
				ID:         "dec_123",
				ProposalID: "prop_123",
				Decision:   DecisionApproved,
				RiskScore:  1.1,
			},
			expectErr: true,
			errMsg:    "risk score must be between 0.0 and 1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.decision.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("GovernanceDecision.Validate() error = %v, expectErr %v", err, tt.expectErr)
			}
			if tt.expectErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("GovernanceDecision.Validate() error = %v, should contain %v", err, tt.errMsg)
			}
		})
	}
}

func TestGovernanceDecision_RiskLevels(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		high     bool
		medium   bool
		low      bool
		severity Severity
	}{
		{"critical risk", 0.9, true, false, false, SeverityCritical},
		{"high risk boundary", 0.8, true, false, false, SeverityCritical},
		{"high risk", 0.7, true, false, false, SeverityHigh},
		{"medium risk upper", 0.6, false, true, false, SeverityHigh},
		{"medium risk", 0.5, false, true, false, SeverityMedium},
		{"medium risk lower", 0.4, false, true, false, SeverityMedium},
		{"low risk", 0.3, false, false, true, SeverityLow},
		{"very low risk", 0.1, false, false, true, SeverityLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := NewDecision("prop_123", DecisionApproved).WithRiskScore(tt.score)

			if got := decision.IsHighRisk(); got != tt.high {
				t.Errorf("GovernanceDecision.IsHighRisk() = %v, want %v", got, tt.high)
			}
			if got := decision.IsMediumRisk(); got != tt.medium {
				t.Errorf("GovernanceDecision.IsMediumRisk() = %v, want %v", got, tt.medium)
			}
			if got := decision.IsLowRisk(); got != tt.low {
				t.Errorf("GovernanceDecision.IsLowRisk() = %v, want %v", got, tt.low)
			}
			if got := decision.RiskSeverity(); got != tt.severity {
				t.Errorf("GovernanceDecision.RiskSeverity() = %v, want %v", got, tt.severity)
			}
		})
	}
}

func TestGovernanceDecision_WithRiskScore(t *testing.T) {
	decision := NewDecision("prop_123", DecisionApproved).WithRiskScore(0.5)
	if decision.RiskScore != 0.5 {
		t.Errorf("WithRiskScore() RiskScore = %v, want 0.5", decision.RiskScore)
	}
}

func TestGovernanceDecision_WithRecommendedVersion(t *testing.T) {
	decision := NewDecision("prop_123", DecisionApproved).WithRecommendedVersion("1.2.0")
	if decision.RecommendedVersion != "1.2.0" {
		t.Errorf("WithRecommendedVersion() = %v, want 1.2.0", decision.RecommendedVersion)
	}
}

func TestGovernanceDecision_AddRiskFactor(t *testing.T) {
	decision := NewDecision("prop_123", DecisionApproved).
		AddRiskFactor("api_change", "Breaking API change", 0.3, SeverityHigh)

	if len(decision.RiskFactors) != 1 {
		t.Errorf("AddRiskFactor() should add factor, got %d factors", len(decision.RiskFactors))
	}
	if decision.RiskFactors[0].Category != "api_change" {
		t.Errorf("RiskFactor.Category = %v, want api_change", decision.RiskFactors[0].Category)
	}
	if decision.RiskFactors[0].Score != 0.3 {
		t.Errorf("RiskFactor.Score = %v, want 0.3", decision.RiskFactors[0].Score)
	}
	if decision.RiskFactors[0].Severity != SeverityHigh {
		t.Errorf("RiskFactor.Severity = %v, want %v", decision.RiskFactors[0].Severity, SeverityHigh)
	}
}

func TestGovernanceDecision_AddRationale(t *testing.T) {
	decision := NewDecision("prop_123", DecisionApproved).
		AddRationale("Low risk change").
		AddRationale("No breaking changes")

	if len(decision.Rationale) != 2 {
		t.Errorf("AddRationale() should add rationale, got %d items", len(decision.Rationale))
	}
	if decision.Rationale[0] != "Low risk change" {
		t.Errorf("Rationale[0] = %v, want 'Low risk change'", decision.Rationale[0])
	}
}

func TestGovernanceDecision_AddRequiredAction(t *testing.T) {
	decision := NewDecision("prop_123", DecisionApprovalRequired).
		AddRequiredAction("human_approval", "Needs team lead approval")

	if len(decision.RequiredActions) != 1 {
		t.Errorf("AddRequiredAction() should add action, got %d actions", len(decision.RequiredActions))
	}
	if decision.RequiredActions[0].Type != "human_approval" {
		t.Errorf("RequiredAction.Type = %v, want human_approval", decision.RequiredActions[0].Type)
	}
}

func TestGovernanceDecision_AddCondition(t *testing.T) {
	decision := NewDecision("prop_123", DecisionApproved).
		AddCondition("time_window", "business_hours")

	if len(decision.Conditions) != 1 {
		t.Errorf("AddCondition() should add condition, got %d conditions", len(decision.Conditions))
	}
	if decision.Conditions[0].Type != "time_window" {
		t.Errorf("Condition.Type = %v, want time_window", decision.Conditions[0].Type)
	}
}

func TestGovernanceDecision_WithAnalysis(t *testing.T) {
	analysis := &ChangeAnalysis{
		Features: 2,
		Fixes:    1,
		Breaking: 0,
	}
	decision := NewDecision("prop_123", DecisionApproved).WithAnalysis(analysis)

	if decision.Analysis != analysis {
		t.Error("WithAnalysis() should set analysis")
	}
}

func TestGovernanceDecision_HasBreakingChanges(t *testing.T) {
	tests := []struct {
		name     string
		decision *GovernanceDecision
		expected bool
	}{
		{
			name:     "no analysis",
			decision: NewDecision("prop_123", DecisionApproved),
			expected: false,
		},
		{
			name: "analysis without breaking",
			decision: NewDecision("prop_123", DecisionApproved).
				WithAnalysis(&ChangeAnalysis{Breaking: 0}),
			expected: false,
		},
		{
			name: "analysis with breaking",
			decision: NewDecision("prop_123", DecisionApproved).
				WithAnalysis(&ChangeAnalysis{Breaking: 1}),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.decision.HasBreakingChanges(); got != tt.expected {
				t.Errorf("HasBreakingChanges() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGovernanceDecision_HasSecurityImpact(t *testing.T) {
	tests := []struct {
		name     string
		decision *GovernanceDecision
		expected bool
	}{
		{
			name:     "no analysis",
			decision: NewDecision("prop_123", DecisionApproved),
			expected: false,
		},
		{
			name: "analysis without security",
			decision: NewDecision("prop_123", DecisionApproved).
				WithAnalysis(&ChangeAnalysis{Security: 0}),
			expected: false,
		},
		{
			name: "analysis with security",
			decision: NewDecision("prop_123", DecisionApproved).
				WithAnalysis(&ChangeAnalysis{Security: 2}),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.decision.HasSecurityImpact(); got != tt.expected {
				t.Errorf("HasSecurityImpact() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestChangeAnalysis_TotalChanges(t *testing.T) {
	tests := []struct {
		name     string
		analysis *ChangeAnalysis
		expected int
	}{
		{
			name:     "nil analysis",
			analysis: nil,
			expected: 0,
		},
		{
			name: "sum all changes",
			analysis: &ChangeAnalysis{
				Features:     2,
				Fixes:        3,
				Breaking:     1,
				Security:     1,
				Dependencies: 2,
				Other:        1,
			},
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.analysis.TotalChanges(); got != tt.expected {
				t.Errorf("TotalChanges() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestChangeAnalysis_HasAPIChanges(t *testing.T) {
	tests := []struct {
		name     string
		analysis *ChangeAnalysis
		expected bool
	}{
		{
			name:     "nil analysis",
			analysis: nil,
			expected: false,
		},
		{
			name:     "no API changes",
			analysis: &ChangeAnalysis{APIChanges: []APIChange{}},
			expected: false,
		},
		{
			name: "has API changes",
			analysis: &ChangeAnalysis{
				APIChanges: []APIChange{{Type: "added", Symbol: "NewFunc"}},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.analysis.HasAPIChanges(); got != tt.expected {
				t.Errorf("HasAPIChanges() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestChangeAnalysis_BreakingAPIChanges(t *testing.T) {
	analysis := &ChangeAnalysis{
		APIChanges: []APIChange{
			{Type: "added", Symbol: "NewFunc", Breaking: false},
			{Type: "removed", Symbol: "OldFunc", Breaking: true},
			{Type: "modified", Symbol: "ChangedFunc", Breaking: true},
		},
	}

	breaking := analysis.BreakingAPIChanges()
	if len(breaking) != 2 {
		t.Errorf("BreakingAPIChanges() returned %d changes, want 2", len(breaking))
	}

	// nil analysis should return nil
	var nilAnalysis *ChangeAnalysis
	if nilAnalysis.BreakingAPIChanges() != nil {
		t.Error("BreakingAPIChanges() on nil should return nil")
	}
}
