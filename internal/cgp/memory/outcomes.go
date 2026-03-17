// Package memory provides the Release Memory store for CGP.
package memory

import (
	"context"
	"fmt"
	"time"
)

// Outcome represents a post-release outcome classification.
type Outcome string

const (
	// OutcomeTypeSuccessfulRelease indicates the release completed without issues.
	OutcomeTypeSuccessfulRelease Outcome = "successful_release"

	// OutcomeTypeRollback indicates the release was rolled back.
	OutcomeTypeRollback Outcome = "rollback"

	// OutcomeTypeIncident indicates the release caused an incident.
	OutcomeTypeIncident Outcome = "incident"

	// OutcomeTypeHotfix indicates the release required a hotfix.
	OutcomeTypeHotfix Outcome = "hotfix"
)

// IsValid returns true if the outcome is a recognized type.
func (o Outcome) IsValid() bool {
	switch o {
	case OutcomeTypeSuccessfulRelease, OutcomeTypeRollback,
		OutcomeTypeIncident, OutcomeTypeHotfix:
		return true
	default:
		return false
	}
}

// IsNegative returns true if this outcome indicates a problem.
func (o Outcome) IsNegative() bool {
	return o == OutcomeTypeRollback || o == OutcomeTypeIncident || o == OutcomeTypeHotfix
}

// OutcomeRecord persists a post-release outcome linked to a release.
type OutcomeRecord struct {
	// ID is a unique identifier for this outcome record.
	ID string `json:"id"`

	// ReleaseID links to the associated release.
	ReleaseID string `json:"releaseId"`

	// Repository is the repository path.
	Repository string `json:"repository"`

	// OutcomeType classifies the outcome.
	OutcomeType Outcome `json:"outcomeType"`

	// Description provides human-readable context.
	Description string `json:"description,omitempty"`

	// FilesAffected lists file paths involved in the outcome.
	// For incidents, these are the files that caused problems.
	// For hotfixes, these are the files that were changed.
	FilesAffected []string `json:"filesAffected,omitempty"`

	// PackagesAffected lists packages involved in the outcome.
	PackagesAffected []string `json:"packagesAffected,omitempty"`

	// ChangeSize is the total lines changed in the original release.
	ChangeSize int `json:"changeSize"`

	// DayOfWeek is the day of week when the release occurred (0=Sunday).
	DayOfWeek int `json:"dayOfWeek"`

	// HourOfDay is the hour (0-23) when the release occurred.
	HourOfDay int `json:"hourOfDay"`

	// RecordedAt is when this outcome was recorded.
	RecordedAt time.Time `json:"recordedAt"`

	// Metadata contains additional outcome information.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Incident represents a post-release incident linked to a specific release.
type Incident struct {
	// ID is a unique identifier for this incident.
	ID string `json:"id"`

	// ReleaseID links to the associated release.
	ReleaseID string `json:"releaseId"`

	// Repository is the repository path.
	Repository string `json:"repository"`

	// Severity indicates incident severity (low, medium, high, critical).
	Severity string `json:"severity"`

	// Description provides details about the incident.
	Description string `json:"description"`

	// RootCause is the identified root cause (if known).
	RootCause string `json:"rootCause,omitempty"`

	// FilesInvolved lists file paths that contributed to the incident.
	FilesInvolved []string `json:"filesInvolved,omitempty"`

	// PackagesInvolved lists packages that contributed to the incident.
	PackagesInvolved []string `json:"packagesInvolved,omitempty"`

	// DetectedAt is when the incident was detected.
	DetectedAt time.Time `json:"detectedAt"`

	// ResolvedAt is when the incident was resolved.
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`

	// TimeToDetect is how long until the incident was detected.
	TimeToDetect time.Duration `json:"timeToDetect"`

	// TimeToResolve is how long until the incident was resolved.
	TimeToResolve time.Duration `json:"timeToResolve,omitempty"`

	// Metadata contains additional incident information.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// OutcomeStore provides persistence for outcome tracking.
type OutcomeStore interface {
	// RecordOutcome persists a post-release outcome.
	RecordOutcome(ctx context.Context, outcome *OutcomeRecord) error

	// RecordOutcomeIncident links an incident to a release.
	RecordOutcomeIncident(ctx context.Context, incident *Incident) error

	// GetOutcomes returns outcomes for a repository within a time window.
	GetOutcomes(ctx context.Context, repository string, since time.Time) ([]*OutcomeRecord, error)

	// GetOutcomesByRelease returns all outcomes for a specific release.
	GetOutcomesByRelease(ctx context.Context, releaseID string) ([]*OutcomeRecord, error)

	// GetIncidentsByRelease returns all incidents for a specific release.
	GetIncidentsByRelease(ctx context.Context, releaseID string) ([]*Incident, error)

	// GetAllOutcomes returns all outcomes for a repository.
	GetAllOutcomes(ctx context.Context, repository string) ([]*OutcomeRecord, error)
}

// InMemoryOutcomeStore provides an in-memory implementation of OutcomeStore.
type InMemoryOutcomeStore struct {
	outcomes  []*OutcomeRecord            // all outcomes in insertion order
	incidents []*Incident                 // all incidents in insertion order
	byRelease map[string][]*OutcomeRecord // keyed by releaseID
	incByRel  map[string][]*Incident      // keyed by releaseID
	byRepo    map[string][]*OutcomeRecord // keyed by repository
}

// NewInMemoryOutcomeStore creates a new in-memory outcome store.
func NewInMemoryOutcomeStore() *InMemoryOutcomeStore {
	return &InMemoryOutcomeStore{
		byRelease: make(map[string][]*OutcomeRecord),
		incByRel:  make(map[string][]*Incident),
		byRepo:    make(map[string][]*OutcomeRecord),
	}
}

// RecordOutcome persists a post-release outcome.
func (s *InMemoryOutcomeStore) RecordOutcome(_ context.Context, outcome *OutcomeRecord) error {
	if outcome == nil {
		return fmt.Errorf("outcome is required")
	}
	if outcome.ID == "" {
		return fmt.Errorf("outcome ID is required")
	}
	if outcome.ReleaseID == "" {
		return fmt.Errorf("release ID is required")
	}
	if outcome.Repository == "" {
		return fmt.Errorf("repository is required")
	}
	if !outcome.OutcomeType.IsValid() {
		return fmt.Errorf("invalid outcome type: %s", outcome.OutcomeType)
	}

	s.outcomes = append(s.outcomes, outcome)
	s.byRelease[outcome.ReleaseID] = append(s.byRelease[outcome.ReleaseID], outcome)
	s.byRepo[outcome.Repository] = append(s.byRepo[outcome.Repository], outcome)
	return nil
}

// RecordOutcomeIncident links an incident to a release.
func (s *InMemoryOutcomeStore) RecordOutcomeIncident(_ context.Context, incident *Incident) error {
	if incident == nil {
		return fmt.Errorf("incident is required")
	}
	if incident.ID == "" {
		return fmt.Errorf("incident ID is required")
	}
	if incident.ReleaseID == "" {
		return fmt.Errorf("release ID is required")
	}
	if incident.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	s.incidents = append(s.incidents, incident)
	s.incByRel[incident.ReleaseID] = append(s.incByRel[incident.ReleaseID], incident)
	return nil
}

// GetOutcomes returns outcomes for a repository within a time window.
func (s *InMemoryOutcomeStore) GetOutcomes(_ context.Context, repository string, since time.Time) ([]*OutcomeRecord, error) {
	var result []*OutcomeRecord
	for _, o := range s.byRepo[repository] {
		if !o.RecordedAt.Before(since) {
			result = append(result, o)
		}
	}
	return result, nil
}

// GetOutcomesByRelease returns all outcomes for a specific release.
func (s *InMemoryOutcomeStore) GetOutcomesByRelease(_ context.Context, releaseID string) ([]*OutcomeRecord, error) {
	return s.byRelease[releaseID], nil
}

// GetIncidentsByRelease returns all incidents for a specific release.
func (s *InMemoryOutcomeStore) GetIncidentsByRelease(_ context.Context, releaseID string) ([]*Incident, error) {
	return s.incByRel[releaseID], nil
}

// GetAllOutcomes returns all outcomes for a repository.
func (s *InMemoryOutcomeStore) GetAllOutcomes(_ context.Context, repository string) ([]*OutcomeRecord, error) {
	return s.byRepo[repository], nil
}
