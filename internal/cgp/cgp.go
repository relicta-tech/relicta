// Package cgp implements the Change Governance Protocol (CGP) for agentic release management.
//
// CGP is a vendor-neutral, model-agnostic protocol that defines how autonomous systems,
// CI pipelines, and human operators propose, evaluate, approve, and execute production
// changes in a controlled, auditable manner.
//
// The protocol defines three actor types:
//   - Proposers: AI agents, CI systems, or humans proposing a change
//   - Governors: Systems implementing CGP (e.g., Relicta) that evaluate proposals
//   - Executors: Systems that carry out approved actions (CI/CD, registries, plugins)
//
// CGP message flow:
//
//	Proposer → CGP Governor → Decision → (Human Review?) → Executor
package cgp

// Version is the current CGP protocol version.
const Version = "0.1"

// MessageType represents the type of CGP message.
type MessageType string

// CGP message types.
const (
	MessageTypeProposal      MessageType = "change.proposal"
	MessageTypeEvaluation    MessageType = "change.evaluation"
	MessageTypeDecision      MessageType = "change.decision"
	MessageTypeAuthorization MessageType = "change.execution_authorized"
)

// String returns the string representation of the message type.
func (t MessageType) String() string {
	return string(t)
}

// IsValid returns true if the message type is recognized.
func (t MessageType) IsValid() bool {
	switch t {
	case MessageTypeProposal, MessageTypeEvaluation, MessageTypeDecision, MessageTypeAuthorization:
		return true
	default:
		return false
	}
}
