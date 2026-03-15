// Package cgp provides the public SDK for the Change Governance Protocol (CGP).
//
// CGP is a vendor-neutral, model-agnostic wire format for governing production changes.
// It defines how autonomous systems, CI pipelines, and human operators propose,
// evaluate, approve, and execute changes in a controlled, auditable manner.
//
// This package contains the serializable message types and codec suitable for
// external consumers building CGP-compatible tools and integrations.
package cgp

// ProtocolVersion is the current CGP protocol version.
const ProtocolVersion = "0.1"
