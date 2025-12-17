package cgp

import (
	"testing"
)

func TestMessageType_String(t *testing.T) {
	tests := []struct {
		name     string
		msgType  MessageType
		expected string
	}{
		{"proposal", MessageTypeProposal, "change.proposal"},
		{"evaluation", MessageTypeEvaluation, "change.evaluation"},
		{"decision", MessageTypeDecision, "change.decision"},
		{"authorization", MessageTypeAuthorization, "change.execution_authorized"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msgType.String(); got != tt.expected {
				t.Errorf("MessageType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMessageType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		msgType  MessageType
		expected bool
	}{
		{"proposal is valid", MessageTypeProposal, true},
		{"evaluation is valid", MessageTypeEvaluation, true},
		{"decision is valid", MessageTypeDecision, true},
		{"authorization is valid", MessageTypeAuthorization, true},
		{"empty is invalid", MessageType(""), false},
		{"unknown is invalid", MessageType("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msgType.IsValid(); got != tt.expected {
				t.Errorf("MessageType.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	if Version == "" {
		t.Error("Version should not be empty")
	}
	if Version != "0.1" {
		t.Errorf("Version = %v, want 0.1", Version)
	}
}
