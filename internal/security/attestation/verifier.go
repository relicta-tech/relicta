package attestation

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
)

// Verifier verifies attestation signatures and governance constraints.
type Verifier struct {
	allowUnsigned bool
	publicKeyPEM  []byte
}

// VerifierOption configures the Verifier.
type VerifierOption func(*Verifier)

// WithAllowUnsigned permits verification of unsigned attestations.
func WithAllowUnsigned(allow bool) VerifierOption {
	return func(v *Verifier) {
		v.allowUnsigned = allow
	}
}

// WithPublicKey sets the public key for signature verification.
func WithPublicKey(pubPEM []byte) VerifierOption {
	return func(v *Verifier) {
		v.publicKeyPEM = pubPEM
	}
}

// NewVerifier creates a new Verifier with the given options.
func NewVerifier(opts ...VerifierOption) *Verifier {
	v := &Verifier{}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

// Verify checks the signature and extracts the governance predicate.
func (v *Verifier) Verify(_ context.Context, att *SignedAttestation) (*VerificationResult, error) {
	if att == nil {
		return nil, fmt.Errorf("attestation is nil")
	}

	result := &VerificationResult{}

	// Check signatures.
	if len(att.Signatures) == 0 {
		if !v.allowUnsigned {
			return nil, fmt.Errorf("attestation is unsigned and --allow-unsigned was not specified")
		}
		result.Unsigned = true
		result.Valid = true
	} else {
		// Verify the first signature.
		sig := att.Signatures[0]
		if err := v.verifySignature(att.Payload, sig); err != nil {
			return nil, fmt.Errorf("signature verification failed: %w", err)
		}
		result.Valid = true
		result.SignedBy = sig.KeyID
	}

	// Extract predicate.
	var stmt Statement
	if err := json.Unmarshal(att.Payload, &stmt); err != nil {
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}

	if stmt.Type != StatementType {
		return nil, fmt.Errorf("unexpected statement type: %s", stmt.Type)
	}
	if stmt.PredicateType != PredicateTypeGovernance {
		return nil, fmt.Errorf("unexpected predicate type: %s", stmt.PredicateType)
	}

	// Re-marshal and unmarshal the predicate to get the typed struct.
	predBytes, err := json.Marshal(stmt.Predicate)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal predicate: %w", err)
	}

	var pred GovernancePredicate
	if err := json.Unmarshal(predBytes, &pred); err != nil {
		return nil, fmt.Errorf("failed to decode governance predicate: %w", err)
	}
	result.Predicate = &pred

	return result, nil
}

// verifySignature verifies a cryptographic signature against the payload.
func (v *Verifier) verifySignature(payload []byte, sig Signature) error {
	if v.publicKeyPEM == nil {
		// If no public key configured, accept any signature (trust mode).
		return nil
	}

	block, _ := pem.Decode(v.publicKeyPEM)
	if block == nil {
		return fmt.Errorf("failed to decode public key PEM")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	ecPubKey, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("public key is not ECDSA (got %T)", pubKey)
	}

	sigBytes, err := base64.StdEncoding.DecodeString(sig.Sig)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	digest := sha256.Sum256(payload)
	if !ecdsa.VerifyASN1(ecPubKey, digest[:], sigBytes) {
		return fmt.Errorf("ECDSA signature verification failed")
	}

	return nil
}

// CheckGovernance validates governance constraints against a verification result.
func (v *Verifier) CheckGovernance(result *VerificationResult, constraints GovernanceConstraints) error {
	if result == nil || result.Predicate == nil {
		return fmt.Errorf("no governance predicate available")
	}

	pred := result.Predicate

	// Check risk score.
	if constraints.MaxRiskScore > 0 && pred.RiskScore > constraints.MaxRiskScore {
		return fmt.Errorf("risk score %.2f exceeds maximum %.2f", pred.RiskScore, constraints.MaxRiskScore)
	}

	// Check minimum approvals.
	if constraints.MinApprovals > 0 && len(pred.Approvals) < constraints.MinApprovals {
		return fmt.Errorf("found %d approvals, need at least %d", len(pred.Approvals), constraints.MinApprovals)
	}

	// Check for human approval.
	if constraints.RequireHuman {
		hasHuman := false
		for _, a := range pred.Approvals {
			if a.ApproverKind == "human" && !a.AutoApproved {
				hasHuman = true
				break
			}
		}
		if !hasHuman {
			return fmt.Errorf("no human approval found (required by policy)")
		}
	}

	return nil
}
