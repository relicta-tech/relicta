package cgp

import (
	"encoding/json"
	"fmt"
)

// Envelope is the versioned wire format wrapper for all CGP messages.
// Every CGP message on the wire is wrapped in this envelope.
type Envelope struct {
	CGPVersion string          `json:"cgpVersion"`
	Type       MessageType     `json:"type"`
	Payload    json.RawMessage `json:"payload"`
}

// Marshal serializes a CGP message into the versioned envelope format.
// The msg must be one of: *ChangeProposal, *GovernanceEvaluation,
// *GovernanceDecision, or *ExecutionAuthorization.
func Marshal(msg any) ([]byte, error) {
	var msgType MessageType

	switch m := msg.(type) {
	case *ChangeProposal:
		msgType = m.Type
		if msgType == "" {
			msgType = TypeChangeProposal
		}
	case *GovernanceEvaluation:
		msgType = m.Type
		if msgType == "" {
			msgType = TypeGovernanceEvaluation
		}
	case *GovernanceDecision:
		msgType = m.Type
		if msgType == "" {
			msgType = TypeGovernanceDecision
		}
	case *ExecutionAuthorization:
		msgType = m.Type
		if msgType == "" {
			msgType = TypeExecutionAuthorization
		}
	default:
		return nil, fmt.Errorf("unsupported CGP message type: %T", msg)
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	env := Envelope{
		CGPVersion: ProtocolVersion,
		Type:       msgType,
		Payload:    payload,
	}

	data, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal envelope: %w", err)
	}

	return data, nil
}

// Unmarshal deserializes a CGP envelope and returns the typed message.
// The returned value is one of: *ChangeProposal, *GovernanceEvaluation,
// *GovernanceDecision, or *ExecutionAuthorization.
func Unmarshal(data []byte) (any, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("failed to decode envelope: %w", err)
	}

	if env.CGPVersion == "" {
		return nil, fmt.Errorf("missing cgpVersion in envelope")
	}

	if !env.Type.IsValid() {
		return nil, fmt.Errorf("unknown CGP message type: %q", env.Type)
	}

	switch env.Type {
	case TypeChangeProposal:
		var p ChangeProposal
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return nil, fmt.Errorf("failed to decode proposal payload: %w", err)
		}
		return &p, nil

	case TypeGovernanceEvaluation:
		var e GovernanceEvaluation
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, fmt.Errorf("failed to decode evaluation payload: %w", err)
		}
		return &e, nil

	case TypeGovernanceDecision:
		var d GovernanceDecision
		if err := json.Unmarshal(env.Payload, &d); err != nil {
			return nil, fmt.Errorf("failed to decode decision payload: %w", err)
		}
		return &d, nil

	case TypeExecutionAuthorization:
		var a ExecutionAuthorization
		if err := json.Unmarshal(env.Payload, &a); err != nil {
			return nil, fmt.Errorf("failed to decode authorization payload: %w", err)
		}
		return &a, nil

	default:
		return nil, fmt.Errorf("unsupported CGP message type: %q", env.Type)
	}
}

// UnmarshalEnvelope decodes only the envelope without parsing the payload.
// This is useful for routing or inspecting messages before full deserialization.
func UnmarshalEnvelope(data []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("failed to decode envelope: %w", err)
	}
	return &env, nil
}
