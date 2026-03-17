package attribution

import (
	"fmt"
	"strings"
)

// Capability represents a specific action an agent is allowed to perform.
type Capability string

// Predefined capabilities for common actions.
const (
	// Proposal capabilities
	CapabilityPropose      Capability = "propose"       // Can propose changes
	CapabilityProposeMinor Capability = "propose:minor" // Can propose minor changes
	CapabilityProposeMajor Capability = "propose:major" // Can propose major changes
	CapabilityProposePatch Capability = "propose:patch" // Can propose patch changes

	// Approval capabilities
	CapabilityApprove        Capability = "approve"     // Can approve changes
	CapabilityApproveOwn     Capability = "approve:own" // Can approve own changes
	CapabilityApproveLowRisk Capability = "approve:low" // Can approve low-risk changes
	CapabilityApproveAll     Capability = "approve:all" // Can approve any changes

	// Execution capabilities
	CapabilityExecute        Capability = "execute"         // Can execute releases
	CapabilityExecuteTag     Capability = "execute:tag"     // Can create tags
	CapabilityExecutePublish Capability = "execute:publish" // Can publish releases
	CapabilityExecuteNotify  Capability = "execute:notify"  // Can send notifications

	// Administrative capabilities
	CapabilityConfigurePolicy Capability = "configure:policy" // Can modify policies
	CapabilityManageAgents    Capability = "manage:agents"    // Can manage agent registry
	CapabilityViewAudit       Capability = "view:audit"       // Can view audit trail
	CapabilityExportData      Capability = "export:data"      // Can export data
)

// AllCapabilities returns all predefined capabilities.
func AllCapabilities() []Capability {
	return []Capability{
		CapabilityPropose,
		CapabilityProposeMinor,
		CapabilityProposeMajor,
		CapabilityProposePatch,
		CapabilityApprove,
		CapabilityApproveOwn,
		CapabilityApproveLowRisk,
		CapabilityApproveAll,
		CapabilityExecute,
		CapabilityExecuteTag,
		CapabilityExecutePublish,
		CapabilityExecuteNotify,
		CapabilityConfigurePolicy,
		CapabilityManageAgents,
		CapabilityViewAudit,
		CapabilityExportData,
	}
}

// String returns the string representation.
func (c Capability) String() string {
	return string(c)
}

// Category returns the capability category (before the colon).
func (c Capability) Category() string {
	s := string(c)
	if idx := strings.Index(s, ":"); idx > 0 {
		return s[:idx]
	}
	return s
}

// Subcategory returns the capability subcategory (after the colon).
func (c Capability) Subcategory() string {
	s := string(c)
	if idx := strings.Index(s, ":"); idx > 0 && idx < len(s)-1 {
		return s[idx+1:]
	}
	return ""
}

// Implies returns true if this capability implies the other.
// For example, "approve:all" implies "approve:low".
// Parent capabilities imply their children, not vice versa.
func (c Capability) Implies(other Capability) bool {
	if c == other {
		return true
	}

	// Base category implies all its subcategories
	// e.g., "execute" implies "execute:tag"
	if c.Subcategory() == "" && other.Category() == string(c) {
		return true
	}

	// Special hierarchical implications
	switch c {
	case CapabilityApproveAll:
		return other == CapabilityApprove || other == CapabilityApproveOwn || other == CapabilityApproveLowRisk
	case CapabilityProposeMajor:
		return other == CapabilityPropose || other == CapabilityProposeMinor || other == CapabilityProposePatch
	case CapabilityExecute:
		return other == CapabilityExecuteTag || other == CapabilityExecutePublish || other == CapabilityExecuteNotify
	}

	return false
}

// CapabilitySet manages a set of capabilities.
type CapabilitySet struct {
	capabilities map[Capability]bool
}

// NewCapabilitySet creates an empty capability set.
func NewCapabilitySet() *CapabilitySet {
	return &CapabilitySet{
		capabilities: make(map[Capability]bool),
	}
}

// CapabilitySetFrom creates a capability set from a list of capabilities.
func CapabilitySetFrom(caps ...Capability) *CapabilitySet {
	cs := NewCapabilitySet()
	for _, c := range caps {
		cs.Add(c)
	}
	return cs
}

// Add adds a capability to the set.
func (cs *CapabilitySet) Add(capability Capability) {
	cs.capabilities[capability] = true
}

// Remove removes a capability from the set.
func (cs *CapabilitySet) Remove(capability Capability) {
	delete(cs.capabilities, capability)
}

// Has returns true if the set contains the capability.
func (cs *CapabilitySet) Has(capability Capability) bool {
	return cs.capabilities[capability]
}

// HasAny returns true if the set contains any of the given capabilities.
func (cs *CapabilitySet) HasAny(caps ...Capability) bool {
	for _, c := range caps {
		if cs.Has(c) {
			return true
		}
	}
	return false
}

// HasAll returns true if the set contains all of the given capabilities.
func (cs *CapabilitySet) HasAll(caps ...Capability) bool {
	for _, c := range caps {
		if !cs.Has(c) {
			return false
		}
	}
	return true
}

// Allows returns true if the set allows the given capability.
// This checks both direct membership and implied capabilities.
func (cs *CapabilitySet) Allows(capability Capability) bool {
	if cs.Has(capability) {
		return true
	}
	// Check if any capability in the set implies this one
	for c := range cs.capabilities {
		if c.Implies(capability) {
			return true
		}
	}
	return false
}

// List returns all capabilities in the set.
func (cs *CapabilitySet) List() []Capability {
	result := make([]Capability, 0, len(cs.capabilities))
	for c := range cs.capabilities {
		result = append(result, c)
	}
	return result
}

// Len returns the number of capabilities in the set.
func (cs *CapabilitySet) Len() int {
	return len(cs.capabilities)
}

// Merge adds all capabilities from another set.
func (cs *CapabilitySet) Merge(other *CapabilitySet) {
	for c := range other.capabilities {
		cs.Add(c)
	}
}

// Validate checks if the capability set is valid.
func (cs *CapabilitySet) Validate() error {
	for c := range cs.capabilities {
		if c == "" {
			return fmt.Errorf("empty capability not allowed")
		}
	}
	return nil
}

// ProposalCapabilities returns the standard capabilities for proposing changes.
func ProposalCapabilities() *CapabilitySet {
	return CapabilitySetFrom(
		CapabilityPropose,
		CapabilityProposeMinor,
		CapabilityProposePatch,
	)
}

// ApprovalCapabilities returns the standard capabilities for approving changes.
func ApprovalCapabilities() *CapabilitySet {
	return CapabilitySetFrom(
		CapabilityApprove,
		CapabilityApproveLowRisk,
	)
}

// ExecutionCapabilities returns the standard capabilities for executing releases.
func ExecutionCapabilities() *CapabilitySet {
	return CapabilitySetFrom(
		CapabilityExecute,
		CapabilityExecuteTag,
		CapabilityExecutePublish,
		CapabilityExecuteNotify,
	)
}

// FullCapabilities returns a set with all capabilities.
func FullCapabilities() *CapabilitySet {
	return CapabilitySetFrom(AllCapabilities()...)
}
