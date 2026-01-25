package attribution

import (
	"testing"
)

func TestCapability_CategorySubcategory(t *testing.T) {
	tests := []struct {
		cap        Capability
		wantCat    string
		wantSubcat string
	}{
		{CapabilityPropose, "propose", ""},
		{CapabilityProposeMinor, "propose", "minor"},
		{CapabilityProposeMajor, "propose", "major"},
		{CapabilityApproveAll, "approve", "all"},
		{CapabilityExecuteTag, "execute", "tag"},
		{CapabilityConfigurePolicy, "configure", "policy"},
	}

	for _, tt := range tests {
		if got := tt.cap.Category(); got != tt.wantCat {
			t.Errorf("%s.Category() = %s, want %s", tt.cap, got, tt.wantCat)
		}
		if got := tt.cap.Subcategory(); got != tt.wantSubcat {
			t.Errorf("%s.Subcategory() = %s, want %s", tt.cap, got, tt.wantSubcat)
		}
	}
}

func TestCapability_Implies(t *testing.T) {
	tests := []struct {
		cap     Capability
		other   Capability
		implies bool
	}{
		// Self-implication
		{CapabilityApprove, CapabilityApprove, true},

		// ApproveAll implies lower approval levels
		{CapabilityApproveAll, CapabilityApprove, true},
		{CapabilityApproveAll, CapabilityApproveOwn, true},
		{CapabilityApproveLowRisk, CapabilityApproveAll, false},

		// ProposeMajor implies lower proposal levels
		{CapabilityProposeMajor, CapabilityProposeMinor, true},
		{CapabilityProposeMajor, CapabilityProposePatch, true},
		{CapabilityProposeMinor, CapabilityProposeMajor, false},

		// Execute implies specific execution capabilities
		{CapabilityExecute, CapabilityExecuteTag, true},
		{CapabilityExecute, CapabilityExecutePublish, true},
		{CapabilityExecuteTag, CapabilityExecute, false},

		// Unrelated capabilities don't imply each other
		{CapabilityApprove, CapabilityExecute, false},
		{CapabilityPropose, CapabilityApprove, false},
	}

	for _, tt := range tests {
		if got := tt.cap.Implies(tt.other); got != tt.implies {
			t.Errorf("%s.Implies(%s) = %v, want %v", tt.cap, tt.other, got, tt.implies)
		}
	}
}

func TestCapabilitySet_Basic(t *testing.T) {
	cs := NewCapabilitySet()

	cs.Add(CapabilityPropose)
	cs.Add(CapabilityExecuteTag)

	if !cs.Has(CapabilityPropose) {
		t.Error("Set should contain CapabilityPropose")
	}
	if !cs.Has(CapabilityExecuteTag) {
		t.Error("Set should contain CapabilityExecuteTag")
	}
	if cs.Has(CapabilityApprove) {
		t.Error("Set should not contain CapabilityApprove")
	}
	if cs.Len() != 2 {
		t.Errorf("Len() = %d, want 2", cs.Len())
	}

	cs.Remove(CapabilityPropose)
	if cs.Has(CapabilityPropose) {
		t.Error("Set should not contain CapabilityPropose after removal")
	}
	if cs.Len() != 1 {
		t.Errorf("Len() = %d, want 1", cs.Len())
	}
}

func TestCapabilitySet_From(t *testing.T) {
	cs := CapabilitySetFrom(
		CapabilityPropose,
		CapabilityApprove,
		CapabilityExecute,
	)

	if cs.Len() != 3 {
		t.Errorf("Len() = %d, want 3", cs.Len())
	}
	if !cs.Has(CapabilityPropose) {
		t.Error("Set should contain CapabilityPropose")
	}
	if !cs.Has(CapabilityApprove) {
		t.Error("Set should contain CapabilityApprove")
	}
	if !cs.Has(CapabilityExecute) {
		t.Error("Set should contain CapabilityExecute")
	}
}

func TestCapabilitySet_HasAny(t *testing.T) {
	cs := CapabilitySetFrom(CapabilityPropose, CapabilityApprove)

	if !cs.HasAny(CapabilityPropose, CapabilityExecute) {
		t.Error("HasAny should return true when any capability matches")
	}
	if cs.HasAny(CapabilityExecute, CapabilityConfigurePolicy) {
		t.Error("HasAny should return false when no capability matches")
	}
}

func TestCapabilitySet_HasAll(t *testing.T) {
	cs := CapabilitySetFrom(CapabilityPropose, CapabilityApprove, CapabilityExecute)

	if !cs.HasAll(CapabilityPropose, CapabilityApprove) {
		t.Error("HasAll should return true when all capabilities present")
	}
	if cs.HasAll(CapabilityPropose, CapabilityConfigurePolicy) {
		t.Error("HasAll should return false when not all capabilities present")
	}
}

func TestCapabilitySet_Allows(t *testing.T) {
	cs := CapabilitySetFrom(CapabilityApproveAll, CapabilityExecute)

	// Direct membership
	if !cs.Allows(CapabilityApproveAll) {
		t.Error("Should allow directly included capability")
	}

	// Implied capabilities
	if !cs.Allows(CapabilityApprove) {
		t.Error("ApproveAll should imply Approve")
	}
	if !cs.Allows(CapabilityApproveLowRisk) {
		t.Error("ApproveAll should imply ApproveLowRisk")
	}
	if !cs.Allows(CapabilityExecuteTag) {
		t.Error("Execute should imply ExecuteTag")
	}

	// Not allowed
	if cs.Allows(CapabilityPropose) {
		t.Error("Should not allow unrelated capability")
	}
	if cs.Allows(CapabilityConfigurePolicy) {
		t.Error("Should not allow unrelated capability")
	}
}

func TestCapabilitySet_Merge(t *testing.T) {
	cs1 := CapabilitySetFrom(CapabilityPropose, CapabilityApprove)
	cs2 := CapabilitySetFrom(CapabilityExecute, CapabilityApprove) // Approve is duplicate

	cs1.Merge(cs2)

	if cs1.Len() != 3 {
		t.Errorf("Merged set Len() = %d, want 3", cs1.Len())
	}
	if !cs1.Has(CapabilityPropose) {
		t.Error("Merged set should have Propose")
	}
	if !cs1.Has(CapabilityApprove) {
		t.Error("Merged set should have Approve")
	}
	if !cs1.Has(CapabilityExecute) {
		t.Error("Merged set should have Execute")
	}
}

func TestCapabilitySet_List(t *testing.T) {
	cs := CapabilitySetFrom(CapabilityPropose, CapabilityApprove)

	list := cs.List()
	if len(list) != 2 {
		t.Errorf("List() length = %d, want 2", len(list))
	}

	// Check that both are present (order may vary)
	found := make(map[Capability]bool)
	for _, c := range list {
		found[c] = true
	}
	if !found[CapabilityPropose] || !found[CapabilityApprove] {
		t.Error("List() should contain both capabilities")
	}
}

func TestCapabilitySet_Validate(t *testing.T) {
	valid := CapabilitySetFrom(CapabilityPropose, CapabilityApprove)
	if err := valid.Validate(); err != nil {
		t.Errorf("Valid set should not return error: %v", err)
	}

	invalid := NewCapabilitySet()
	invalid.Add("") // Empty capability
	if err := invalid.Validate(); err == nil {
		t.Error("Set with empty capability should return error")
	}
}

func TestPresetCapabilitySets(t *testing.T) {
	proposal := ProposalCapabilities()
	if !proposal.Has(CapabilityPropose) {
		t.Error("ProposalCapabilities should include Propose")
	}
	if !proposal.Has(CapabilityProposeMinor) {
		t.Error("ProposalCapabilities should include ProposeMinor")
	}

	approval := ApprovalCapabilities()
	if !approval.Has(CapabilityApprove) {
		t.Error("ApprovalCapabilities should include Approve")
	}

	execution := ExecutionCapabilities()
	if !execution.Has(CapabilityExecute) {
		t.Error("ExecutionCapabilities should include Execute")
	}
	if !execution.Has(CapabilityExecuteTag) {
		t.Error("ExecutionCapabilities should include ExecuteTag")
	}

	full := FullCapabilities()
	for _, c := range AllCapabilities() {
		if !full.Has(c) {
			t.Errorf("FullCapabilities should include %s", c)
		}
	}
}

func TestAllCapabilities(t *testing.T) {
	all := AllCapabilities()
	if len(all) < 10 {
		t.Errorf("AllCapabilities() returned %d, expected at least 10", len(all))
	}

	// Check for expected capabilities
	expected := []Capability{
		CapabilityPropose,
		CapabilityApprove,
		CapabilityExecute,
		CapabilityConfigurePolicy,
		CapabilityManageAgents,
	}

	allSet := CapabilitySetFrom(all...)
	for _, c := range expected {
		if !allSet.Has(c) {
			t.Errorf("AllCapabilities should include %s", c)
		}
	}
}
