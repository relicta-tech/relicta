// Package protocol provides CGP message handling and validation.
//
// This package implements the message layer of the Change Governance Protocol,
// providing a unified interface for encoding, decoding, and validating
// CGP messages across different transport mechanisms.
package protocol

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
)

// Message is a wrapper for any CGP message with common envelope fields.
type Message struct {
	// Header contains routing and metadata.
	Header Header `json:"header"`

	// Payload is the actual message content.
	Payload json.RawMessage `json:"payload"`

	// parsed holds the decoded payload for convenience.
	parsed any
}

// Header contains common message metadata.
type Header struct {
	// MessageID is a unique identifier for this message.
	MessageID string `json:"messageId"`

	// Type is the CGP message type.
	Type cgp.MessageType `json:"type"`

	// CGPVersion is the protocol version.
	CGPVersion string `json:"cgpVersion"`

	// Timestamp is when the message was created.
	Timestamp time.Time `json:"timestamp"`

	// CorrelationID links related messages (e.g., proposal -> decision).
	CorrelationID string `json:"correlationId,omitempty"`

	// Source identifies where this message came from.
	Source string `json:"source,omitempty"`

	// Destination indicates where this message should go.
	Destination string `json:"destination,omitempty"`
}

// NewMessage creates a new message with the given payload.
func NewMessage(msgType cgp.MessageType, payload any) (*Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	return &Message{
		Header: Header{
			MessageID:  generateMessageID(),
			Type:       msgType,
			CGPVersion: cgp.Version,
			Timestamp:  time.Now().UTC(),
		},
		Payload: payloadBytes,
		parsed:  payload,
	}, nil
}

// NewProposalMessage creates a message from a proposal.
func NewProposalMessage(proposal *cgp.ChangeProposal) (*Message, error) {
	msg, err := NewMessage(cgp.MessageTypeProposal, proposal)
	if err != nil {
		return nil, err
	}
	msg.Header.CorrelationID = proposal.ID
	return msg, nil
}

// NewEvaluationMessage creates a message from an evaluation.
func NewEvaluationMessage(evaluation *cgp.GovernanceEvaluation) (*Message, error) {
	msg, err := NewMessage(cgp.MessageTypeEvaluation, evaluation)
	if err != nil {
		return nil, err
	}
	msg.Header.CorrelationID = evaluation.ProposalID
	return msg, nil
}

// NewDecisionMessage creates a message from a decision.
func NewDecisionMessage(decision *cgp.GovernanceDecision) (*Message, error) {
	msg, err := NewMessage(cgp.MessageTypeDecision, decision)
	if err != nil {
		return nil, err
	}
	msg.Header.CorrelationID = decision.ProposalID
	return msg, nil
}

// NewAuthorizationMessage creates a message from an authorization.
func NewAuthorizationMessage(auth *cgp.ExecutionAuthorization) (*Message, error) {
	msg, err := NewMessage(cgp.MessageTypeAuthorization, auth)
	if err != nil {
		return nil, err
	}
	msg.Header.CorrelationID = auth.ProposalID
	return msg, nil
}

// WithCorrelationID sets the correlation ID.
func (m *Message) WithCorrelationID(id string) *Message {
	m.Header.CorrelationID = id
	return m
}

// WithSource sets the source identifier.
func (m *Message) WithSource(source string) *Message {
	m.Header.Source = source
	return m
}

// WithDestination sets the destination.
func (m *Message) WithDestination(dest string) *Message {
	m.Header.Destination = dest
	return m
}

// Encode serializes the message to JSON.
func (m *Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// EncodePretty serializes the message to indented JSON.
func (m *Message) EncodePretty() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// DecodeMessage deserializes a message from JSON.
func DecodeMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to decode message: %w", err)
	}
	return &msg, nil
}

// DecodePayload decodes the payload into a specific type.
func (m *Message) DecodePayload(v any) error {
	return json.Unmarshal(m.Payload, v)
}

// AsProposal decodes the payload as a proposal.
func (m *Message) AsProposal() (*cgp.ChangeProposal, error) {
	if m.Header.Type != cgp.MessageTypeProposal {
		return nil, fmt.Errorf("message type is %s, not proposal", m.Header.Type)
	}
	var p cgp.ChangeProposal
	if err := m.DecodePayload(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

// AsEvaluation decodes the payload as an evaluation.
func (m *Message) AsEvaluation() (*cgp.GovernanceEvaluation, error) {
	if m.Header.Type != cgp.MessageTypeEvaluation {
		return nil, fmt.Errorf("message type is %s, not evaluation", m.Header.Type)
	}
	var e cgp.GovernanceEvaluation
	if err := m.DecodePayload(&e); err != nil {
		return nil, err
	}
	return &e, nil
}

// AsDecision decodes the payload as a decision.
func (m *Message) AsDecision() (*cgp.GovernanceDecision, error) {
	if m.Header.Type != cgp.MessageTypeDecision {
		return nil, fmt.Errorf("message type is %s, not decision", m.Header.Type)
	}
	var d cgp.GovernanceDecision
	if err := m.DecodePayload(&d); err != nil {
		return nil, err
	}
	return &d, nil
}

// AsAuthorization decodes the payload as an authorization.
func (m *Message) AsAuthorization() (*cgp.ExecutionAuthorization, error) {
	if m.Header.Type != cgp.MessageTypeAuthorization {
		return nil, fmt.Errorf("message type is %s, not authorization", m.Header.Type)
	}
	var a cgp.ExecutionAuthorization
	if err := m.DecodePayload(&a); err != nil {
		return nil, err
	}
	return &a, nil
}

// generateMessageID creates a unique message ID.
func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// MessageChain represents a sequence of related messages.
type MessageChain struct {
	// CorrelationID links all messages in this chain.
	CorrelationID string `json:"correlationId"`

	// Messages are the messages in order.
	Messages []*Message `json:"messages"`
}

// NewMessageChain creates a new message chain.
func NewMessageChain(correlationID string) *MessageChain {
	return &MessageChain{
		CorrelationID: correlationID,
		Messages:      []*Message{},
	}
}

// Add adds a message to the chain.
func (c *MessageChain) Add(msg *Message) {
	msg.Header.CorrelationID = c.CorrelationID
	c.Messages = append(c.Messages, msg)
}

// GetByType returns messages of a specific type.
func (c *MessageChain) GetByType(msgType cgp.MessageType) []*Message {
	var result []*Message
	for _, m := range c.Messages {
		if m.Header.Type == msgType {
			result = append(result, m)
		}
	}
	return result
}

// GetProposal returns the proposal from the chain, if any.
func (c *MessageChain) GetProposal() (*cgp.ChangeProposal, error) {
	msgs := c.GetByType(cgp.MessageTypeProposal)
	if len(msgs) == 0 {
		return nil, fmt.Errorf("no proposal in chain")
	}
	return msgs[0].AsProposal()
}

// GetDecision returns the decision from the chain, if any.
func (c *MessageChain) GetDecision() (*cgp.GovernanceDecision, error) {
	msgs := c.GetByType(cgp.MessageTypeDecision)
	if len(msgs) == 0 {
		return nil, fmt.Errorf("no decision in chain")
	}
	return msgs[len(msgs)-1].AsDecision()
}

// Len returns the number of messages in the chain.
func (c *MessageChain) Len() int {
	return len(c.Messages)
}
