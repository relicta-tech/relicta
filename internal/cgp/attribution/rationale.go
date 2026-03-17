// Package attribution provides agent attribution and rationale capture for CGP.
//
// This package implements the CGP specification requirements for tracking
// which agents proposed changes and why, enabling governance decisions
// to be made with full context about the source and reasoning behind changes.
package attribution

import (
	"time"
)

// RationaleType classifies the type of reasoning provided.
type RationaleType string

// Rationale types.
const (
	RationaleTypeAnalysis      RationaleType = "analysis"      // Based on code analysis
	RationaleTypeConvention    RationaleType = "convention"    // Based on project conventions
	RationaleTypeUserRequest   RationaleType = "user_request"  // Explicitly requested by user
	RationaleTypeAutomation    RationaleType = "automation"    // Automated decision
	RationaleTypeHistory       RationaleType = "history"       // Based on historical patterns
	RationaleTypeDocumentation RationaleType = "documentation" // Based on documentation
)

// Rationale captures why an agent proposed a change.
// This provides transparency into the agent's decision-making process.
type Rationale struct {
	// Type classifies the reasoning.
	Type RationaleType `json:"type"`

	// Summary is a human-readable explanation.
	Summary string `json:"summary"`

	// Details provides additional structured context.
	Details map[string]any `json:"details,omitempty"`

	// Sources lists references that informed the decision.
	Sources []Source `json:"sources,omitempty"`

	// Confidence indicates how confident the agent is (0.0-1.0).
	Confidence float64 `json:"confidence"`

	// Timestamp is when this rationale was generated.
	Timestamp time.Time `json:"timestamp"`
}

// Source represents a reference that informed a decision.
type Source struct {
	// Type is the source type: "file", "commit", "documentation", "conversation", "url".
	Type string `json:"type"`

	// Reference is the source identifier (file path, commit hash, URL, etc.).
	Reference string `json:"reference"`

	// Context provides relevant excerpt or context from the source.
	Context string `json:"context,omitempty"`
}

// RationaleBuilder provides a fluent API for creating rationales.
type RationaleBuilder struct {
	rationale *Rationale
}

// NewRationale creates a new rationale builder.
func NewRationale(rationaleType RationaleType, summary string) *RationaleBuilder {
	return &RationaleBuilder{
		rationale: &Rationale{
			Type:       rationaleType,
			Summary:    summary,
			Timestamp:  time.Now().UTC(),
			Confidence: 0.5, // Default medium confidence
			Details:    make(map[string]any),
		},
	}
}

// WithConfidence sets the confidence level.
func (b *RationaleBuilder) WithConfidence(confidence float64) *RationaleBuilder {
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}
	b.rationale.Confidence = confidence
	return b
}

// WithDetail adds a detail key-value pair.
func (b *RationaleBuilder) WithDetail(key string, value any) *RationaleBuilder {
	b.rationale.Details[key] = value
	return b
}

// WithDetails sets all details at once.
func (b *RationaleBuilder) WithDetails(details map[string]any) *RationaleBuilder {
	b.rationale.Details = details
	return b
}

// AddSource adds a source reference.
func (b *RationaleBuilder) AddSource(sourceType, reference, context string) *RationaleBuilder {
	b.rationale.Sources = append(b.rationale.Sources, Source{
		Type:      sourceType,
		Reference: reference,
		Context:   context,
	})
	return b
}

// AddFileSource adds a file as a source.
func (b *RationaleBuilder) AddFileSource(filePath, context string) *RationaleBuilder {
	return b.AddSource("file", filePath, context)
}

// AddCommitSource adds a commit as a source.
func (b *RationaleBuilder) AddCommitSource(commitHash, context string) *RationaleBuilder {
	return b.AddSource("commit", commitHash, context)
}

// AddDocSource adds documentation as a source.
func (b *RationaleBuilder) AddDocSource(docRef, context string) *RationaleBuilder {
	return b.AddSource("documentation", docRef, context)
}

// AddURLSource adds a URL as a source.
func (b *RationaleBuilder) AddURLSource(url, context string) *RationaleBuilder {
	return b.AddSource("url", url, context)
}

// AddConversationSource adds a conversation reference as a source.
func (b *RationaleBuilder) AddConversationSource(sessionID, context string) *RationaleBuilder {
	return b.AddSource("conversation", sessionID, context)
}

// WithTimestamp overrides the timestamp.
func (b *RationaleBuilder) WithTimestamp(t time.Time) *RationaleBuilder {
	b.rationale.Timestamp = t.UTC()
	return b
}

// Build returns the constructed rationale.
func (b *RationaleBuilder) Build() *Rationale {
	return b.rationale
}

// IsHighConfidence returns true if confidence >= 0.8.
func (r *Rationale) IsHighConfidence() bool {
	return r.Confidence >= 0.8
}

// IsMediumConfidence returns true if confidence is 0.5-0.8.
func (r *Rationale) IsMediumConfidence() bool {
	return r.Confidence >= 0.5 && r.Confidence < 0.8
}

// IsLowConfidence returns true if confidence < 0.5.
func (r *Rationale) IsLowConfidence() bool {
	return r.Confidence < 0.5
}

// HasSources returns true if the rationale has source references.
func (r *Rationale) HasSources() bool {
	return len(r.Sources) > 0
}

// SourcesByType returns sources filtered by type.
func (r *Rationale) SourcesByType(sourceType string) []Source {
	var result []Source
	for _, s := range r.Sources {
		if s.Type == sourceType {
			result = append(result, s)
		}
	}
	return result
}
