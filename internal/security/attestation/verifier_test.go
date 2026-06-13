package attestation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func signedAttestationForTest(t *testing.T, pred GovernancePredicate) (*SignedAttestation, []byte) {
	t.Helper()

	stmt := &Statement{
		Type: StatementType,
		Subject: []Subject{
			{Name: "v1.0.0", Digest: map[string]string{"sha256": "abc"}},
		},
		PredicateType: PredicateTypeGovernance,
		Predicate:     pred,
	}

	privPEM, pubPEM, err := GenerateTestKey()
	require.NoError(t, err)

	dir := t.TempDir()
	keyPath := filepath.Join(dir, "test.key")
	require.NoError(t, os.WriteFile(keyPath, privPEM, 0600))

	signer := NewSigner(SigningModeLocal, WithKeyPath(keyPath))
	att, err := signer.Sign(context.Background(), stmt)
	require.NoError(t, err)

	return att, pubPEM
}

func unsignedAttestationForTest(t *testing.T, pred GovernancePredicate) *SignedAttestation {
	t.Helper()

	stmt := &Statement{
		Type: StatementType,
		Subject: []Subject{
			{Name: "v1.0.0", Digest: map[string]string{"sha256": "abc"}},
		},
		PredicateType: PredicateTypeGovernance,
		Predicate:     pred,
	}

	signer := NewSigner(SigningModeNone)
	att, err := signer.Sign(context.Background(), stmt)
	require.NoError(t, err)
	return att
}

func TestVerifier_Verify_Signed(t *testing.T) {
	pred := GovernancePredicate{
		Version:      "1.0.0",
		Decision:     "approved",
		AutoApproved: true,
		Approvals: []ApprovalRecord{
			{ApproverID: "ci:gh", ApproverKind: "ci", AutoApproved: true, ApprovedAt: time.Now()},
		},
	}

	att, pubPEM := signedAttestationForTest(t, pred)

	verifier := NewVerifier(WithPublicKey(pubPEM))
	result, err := verifier.Verify(context.Background(), att)
	require.NoError(t, err)

	assert.True(t, result.Valid)
	assert.False(t, result.Unsigned)
	assert.NotEmpty(t, result.SignedBy)
	assert.NotNil(t, result.Predicate)
	assert.Equal(t, "1.0.0", result.Predicate.Version)
}

func TestVerifier_Verify_Unsigned_Allowed(t *testing.T) {
	pred := GovernancePredicate{
		Version:  "1.0.0",
		Decision: "approved",
	}

	att := unsignedAttestationForTest(t, pred)

	verifier := NewVerifier(WithAllowUnsigned(true))
	result, err := verifier.Verify(context.Background(), att)
	require.NoError(t, err)

	assert.True(t, result.Valid)
	assert.True(t, result.Unsigned)
	assert.Equal(t, "1.0.0", result.Predicate.Version)
}

func TestVerifier_Verify_Unsigned_Rejected(t *testing.T) {
	pred := GovernancePredicate{
		Version:  "1.0.0",
		Decision: "approved",
	}

	att := unsignedAttestationForTest(t, pred)

	verifier := NewVerifier() // allowUnsigned defaults to false
	_, err := verifier.Verify(context.Background(), att)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsigned")
}

func TestVerifier_Verify_InvalidSignature(t *testing.T) {
	pred := GovernancePredicate{Version: "1.0.0", Decision: "approved"}
	att, _ := signedAttestationForTest(t, pred)

	// Use a different key pair for verification.
	_, otherPub, err := GenerateTestKey()
	require.NoError(t, err)

	verifier := NewVerifier(WithPublicKey(otherPub))
	_, err = verifier.Verify(context.Background(), att)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signature verification failed")
}

func TestVerifier_Verify_NilAttestation(t *testing.T) {
	verifier := NewVerifier()
	_, err := verifier.Verify(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestVerifier_Verify_InvalidPayload(t *testing.T) {
	att := &SignedAttestation{
		Payload:     []byte("not json"),
		PayloadType: PayloadType,
	}

	verifier := NewVerifier(WithAllowUnsigned(true))
	_, err := verifier.Verify(context.Background(), att)
	require.Error(t, err)
}

func TestVerifier_Verify_WrongStatementType(t *testing.T) {
	stmt := &Statement{
		Type:          "wrong-type",
		Subject:       []Subject{{Name: "test", Digest: map[string]string{"sha256": "abc"}}},
		PredicateType: PredicateTypeGovernance,
		Predicate:     GovernancePredicate{},
	}
	payload, err := json.Marshal(stmt)
	require.NoError(t, err)

	att := &SignedAttestation{Payload: payload, PayloadType: PayloadType}

	verifier := NewVerifier(WithAllowUnsigned(true))
	_, err = verifier.Verify(context.Background(), att)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected statement type")
}

func TestVerifier_CheckGovernance_RiskScore(t *testing.T) {
	verifier := NewVerifier()

	result := &VerificationResult{
		Valid: true,
		Predicate: &GovernancePredicate{
			RiskScore: 0.8,
		},
	}

	err := verifier.CheckGovernance(result, GovernanceConstraints{MaxRiskScore: 0.5})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "risk score 0.80 exceeds maximum 0.50")

	err = verifier.CheckGovernance(result, GovernanceConstraints{MaxRiskScore: 0.9})
	require.NoError(t, err)
}

func TestVerifier_CheckGovernance_MinApprovals(t *testing.T) {
	verifier := NewVerifier()

	result := &VerificationResult{
		Valid: true,
		Predicate: &GovernancePredicate{
			Approvals: []ApprovalRecord{
				{ApproverID: "alice", ApproverKind: "human"},
			},
		},
	}

	err := verifier.CheckGovernance(result, GovernanceConstraints{MinApprovals: 2})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "found 1 approvals, need at least 2")

	err = verifier.CheckGovernance(result, GovernanceConstraints{MinApprovals: 1})
	require.NoError(t, err)
}

func TestVerifier_CheckGovernance_RequireHuman(t *testing.T) {
	verifier := NewVerifier()

	// No human approval.
	result := &VerificationResult{
		Valid: true,
		Predicate: &GovernancePredicate{
			Approvals: []ApprovalRecord{
				{ApproverID: "ci:gh", ApproverKind: "ci", AutoApproved: true},
			},
		},
	}

	err := verifier.CheckGovernance(result, GovernanceConstraints{RequireHuman: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no human approval found")

	// With human approval.
	result.Predicate.Approvals = append(result.Predicate.Approvals, ApprovalRecord{
		ApproverID:   "human:bob",
		ApproverKind: "human",
		AutoApproved: false,
	})

	err = verifier.CheckGovernance(result, GovernanceConstraints{RequireHuman: true})
	require.NoError(t, err)
}

func TestVerifier_CheckGovernance_NilPredicate(t *testing.T) {
	verifier := NewVerifier()

	err := verifier.CheckGovernance(nil, GovernanceConstraints{})
	require.Error(t, err)

	err = verifier.CheckGovernance(&VerificationResult{}, GovernanceConstraints{})
	require.Error(t, err)
}

func TestVerifier_CheckGovernance_NoConstraints(t *testing.T) {
	verifier := NewVerifier()

	result := &VerificationResult{
		Valid:     true,
		Predicate: &GovernancePredicate{RiskScore: 0.9},
	}

	// No constraints means everything passes.
	err := verifier.CheckGovernance(result, GovernanceConstraints{})
	require.NoError(t, err)
}

func TestVerifier_Verify_NoPublicKey_FailsClosed(t *testing.T) {
	pred := GovernancePredicate{Version: "1.0.0", Decision: "approved"}
	att, _ := signedAttestationForTest(t, pred)

	// Verifying a signed attestation without a public key can't establish
	// validity, so it must fail closed (was previously accepted blindly).
	verifier := NewVerifier()
	_, err := verifier.Verify(context.Background(), att)
	require.Error(t, err)
}

// TestVerifier_Verify_SignedNoKey_FailsClosed is the regression test for the
// fail-open vuln: a signed attestation with no configured public key must be
// rejected, not blindly accepted as valid.
func TestVerifier_Verify_SignedNoKey_FailsClosed(t *testing.T) {
	pred := GovernancePredicate{Version: "1.0.0", Decision: "approved"}
	att, _ := signedAttestationForTest(t, pred)

	verifier := NewVerifier() // no public key, no allow-unsigned
	_, err := verifier.Verify(context.Background(), att)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no public key")
}

// TestVerifier_Verify_SignedNoKey_AllowUnsigned lets the operator explicitly
// accept unverifiable attestations.
func TestVerifier_Verify_SignedNoKey_AllowUnsigned(t *testing.T) {
	pred := GovernancePredicate{Version: "1.0.0", Decision: "approved"}
	att, _ := signedAttestationForTest(t, pred)

	verifier := NewVerifier(WithAllowUnsigned(true))
	result, err := verifier.Verify(context.Background(), att)
	require.NoError(t, err)
	assert.True(t, result.Valid)
}
