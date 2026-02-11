// Package attestation provides SLSA-compatible governance attestation
// with Sigstore signing for Relicta releases.
//
// Attestations follow the in-toto v1 Statement format and include a
// custom GovernancePredicate that records the governance decision,
// approval chain, and audit trail hash for each release.
package attestation

import "time"

// SLSA / in-toto v1 constants.
const (
	// StatementType is the in-toto v1 statement type URI.
	StatementType = "https://in-toto.io/Statement/v1"

	// PredicateTypeGovernance is the Relicta governance predicate type URI.
	PredicateTypeGovernance = "https://relicta.dev/attestation/governance/v1"

	// PayloadType is the DSSE payload content type for in-toto statements.
	PayloadType = "application/vnd.in-toto+json"
)

// Statement is an in-toto v1 attestation statement.
// See https://github.com/in-toto/attestation/blob/main/spec/v1/statement.md
type Statement struct {
	// Type is the in-toto statement type (always StatementType).
	Type string `json:"_type"`

	// Subject identifies what the attestation is about.
	Subject []Subject `json:"subject"`

	// PredicateType identifies the predicate schema.
	PredicateType string `json:"predicateType"`

	// Predicate contains the attestation-specific data.
	Predicate any `json:"predicate"`
}

// Subject identifies a software artifact.
type Subject struct {
	// Name is a human-readable identifier (e.g., tag name).
	Name string `json:"name"`

	// Digest maps algorithm names to hex-encoded digest values.
	Digest map[string]string `json:"digest"`
}

// GovernancePredicate records the governance decision for a release.
type GovernancePredicate struct {
	// Release identity
	Version    string    `json:"version"`
	Tag        string    `json:"tag"`
	Repository string    `json:"repository"`
	CommitSHA  string    `json:"commitSha"`
	ReleasedAt time.Time `json:"releasedAt"`

	// Governance decision
	RiskScore    float64  `json:"riskScore"`
	Decision     string   `json:"decision"`
	AutoApproved bool     `json:"autoApproved"`
	Rationale    []string `json:"rationale,omitempty"`

	// Approval chain
	Approvals []ApprovalRecord `json:"approvals"`

	// Actor identity
	Initiator ActorIdentity `json:"initiator"`

	// Audit trail link
	AuditChainHash  string `json:"auditChainHash"`
	AuditEntryCount int    `json:"auditEntryCount"`
}

// ApprovalRecord records a single approval in the governance chain.
type ApprovalRecord struct {
	ApproverID   string    `json:"approverId"`
	ApproverKind string    `json:"approverKind"`
	ApprovedAt   time.Time `json:"approvedAt"`
	AutoApproved bool      `json:"autoApproved"`
	Level        string    `json:"level,omitempty"`
}

// ActorIdentity identifies who initiated the release.
type ActorIdentity struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name,omitempty"`
}

// SignedAttestation wraps a Statement with its cryptographic signatures
// in DSSE (Dead Simple Signing Envelope) format.
type SignedAttestation struct {
	// Payload is the JSON-encoded Statement.
	Payload []byte `json:"payload"`

	// PayloadType is the media type of the payload.
	PayloadType string `json:"payloadType"`

	// Signatures contains the cryptographic signatures.
	Signatures []Signature `json:"signatures,omitempty"`
}

// Signature holds a single cryptographic signature.
type Signature struct {
	// KeyID identifies the signing key.
	KeyID string `json:"keyid,omitempty"`

	// Sig is the base64-encoded signature.
	Sig string `json:"sig"`

	// Cert is the PEM-encoded signing certificate (keyless mode).
	Cert string `json:"cert,omitempty"`
}

// VerificationResult holds the result of attestation verification.
type VerificationResult struct {
	// Valid indicates the attestation signature is valid (or unsigned is accepted).
	Valid bool `json:"valid"`

	// Unsigned indicates no signature was present.
	Unsigned bool `json:"unsigned"`

	// SignedBy is the OIDC identity or key ID of the signer.
	SignedBy string `json:"signedBy,omitempty"`

	// Predicate is the extracted governance predicate.
	Predicate *GovernancePredicate `json:"predicate,omitempty"`
}

// GovernanceConstraints defines policy constraints for verification.
type GovernanceConstraints struct {
	// MaxRiskScore is the maximum allowed risk score (0.0-1.0).
	MaxRiskScore float64

	// MinApprovals is the minimum number of approvals required.
	MinApprovals int

	// RequireHuman requires at least one human approval.
	RequireHuman bool
}
