package attestation

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatement_JSONSerialization(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	stmt := Statement{
		Type: StatementType,
		Subject: []Subject{
			{
				Name:   "v1.2.3",
				Digest: map[string]string{"sha256": "abc123"},
			},
		},
		PredicateType: PredicateTypeGovernance,
		Predicate: GovernancePredicate{
			Version:      "1.2.3",
			Tag:          "v1.2.3",
			Repository:   "github.com/example/repo",
			CommitSHA:    "abc123def456",
			ReleasedAt:   now,
			RiskScore:    0.25,
			Decision:     "approved",
			AutoApproved: true,
			Rationale:    []string{"Low risk patch release"},
			Approvals: []ApprovalRecord{
				{
					ApproverID:   "ci:github-actions",
					ApproverKind: "ci",
					ApprovedAt:   now,
					AutoApproved: true,
				},
			},
			Initiator: ActorIdentity{
				ID:   "human:alice",
				Kind: "human",
				Name: "Alice",
			},
			AuditChainHash:  "deadbeef",
			AuditEntryCount: 5,
		},
	}

	data, err := json.MarshalIndent(stmt, "", "  ")
	require.NoError(t, err)

	// Verify key fields are present
	assert.Contains(t, string(data), `"_type": "https://in-toto.io/Statement/v1"`)
	assert.Contains(t, string(data), `"predicateType": "https://relicta.dev/attestation/governance/v1"`)
	assert.Contains(t, string(data), `"sha256": "abc123"`)
	assert.Contains(t, string(data), `"riskScore": 0.25`)
	assert.Contains(t, string(data), `"decision": "approved"`)
	assert.Contains(t, string(data), `"auditChainHash": "deadbeef"`)

	// Verify round-trip
	var decoded Statement
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, StatementType, decoded.Type)
	assert.Equal(t, PredicateTypeGovernance, decoded.PredicateType)
	assert.Len(t, decoded.Subject, 1)
	assert.Equal(t, "v1.2.3", decoded.Subject[0].Name)
}

func TestGovernancePredicate_JSONRoundTrip(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	pred := GovernancePredicate{
		Version:      "2.0.0",
		Tag:          "v2.0.0",
		Repository:   "github.com/relicta-tech/relicta",
		CommitSHA:    "1234567890abcdef",
		ReleasedAt:   now,
		RiskScore:    0.72,
		Decision:     "approval_required",
		AutoApproved: false,
		Rationale:    []string{"Major version bump", "Breaking changes detected"},
		Approvals: []ApprovalRecord{
			{
				ApproverID:   "human:bob",
				ApproverKind: "human",
				ApprovedAt:   now,
				AutoApproved: false,
				Level:        "technical",
			},
			{
				ApproverID:   "human:carol",
				ApproverKind: "human",
				ApprovedAt:   now,
				AutoApproved: false,
				Level:        "release",
			},
		},
		Initiator: ActorIdentity{
			ID:   "agent:cursor",
			Kind: "agent",
		},
		AuditChainHash:  "abcdef1234567890",
		AuditEntryCount: 12,
	}

	data, err := json.Marshal(pred)
	require.NoError(t, err)

	var decoded GovernancePredicate
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, pred.Version, decoded.Version)
	assert.Equal(t, pred.RiskScore, decoded.RiskScore)
	assert.Equal(t, pred.Decision, decoded.Decision)
	assert.False(t, decoded.AutoApproved)
	assert.Len(t, decoded.Approvals, 2)
	assert.Equal(t, "technical", decoded.Approvals[0].Level)
	assert.Equal(t, "release", decoded.Approvals[1].Level)
	assert.Equal(t, "agent", decoded.Initiator.Kind)
	assert.Len(t, decoded.Rationale, 2)
}

func TestSignedAttestation_JSONSerialization(t *testing.T) {
	att := SignedAttestation{
		Payload:     []byte(`{"_type":"test"}`),
		PayloadType: PayloadType,
		Signatures: []Signature{
			{
				KeyID: "key-1",
				Sig:   "c2lnbmF0dXJl",
			},
		},
	}

	data, err := json.Marshal(att)
	require.NoError(t, err)

	var decoded SignedAttestation
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, PayloadType, decoded.PayloadType)
	assert.Len(t, decoded.Signatures, 1)
	assert.Equal(t, "key-1", decoded.Signatures[0].KeyID)
	assert.Equal(t, "c2lnbmF0dXJl", decoded.Signatures[0].Sig)
}

func TestSignedAttestation_EmptySignatures(t *testing.T) {
	att := SignedAttestation{
		Payload:     []byte(`{}`),
		PayloadType: PayloadType,
	}

	data, err := json.Marshal(att)
	require.NoError(t, err)

	// Verify omitempty works for empty signatures
	assert.NotContains(t, string(data), `"signatures"`)
}

func TestGovernancePredicate_EmptyRationale(t *testing.T) {
	pred := GovernancePredicate{
		Version:  "1.0.0",
		Decision: "approved",
	}

	data, err := json.Marshal(pred)
	require.NoError(t, err)

	// Rationale should be omitted when empty
	assert.NotContains(t, string(data), `"rationale"`)
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "https://in-toto.io/Statement/v1", StatementType)
	assert.Equal(t, "https://relicta.dev/attestation/governance/v1", PredicateTypeGovernance)
	assert.Equal(t, "application/vnd.in-toto+json", PayloadType)
}
