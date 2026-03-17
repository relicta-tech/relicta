// Package correlation links production incidents to releases using
// time-based proximity, service name matching, and deployment labels.
package correlation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp/memory"
	"github.com/relicta-tech/relicta/internal/observability/receiver"
)

// ReleaseCorrelation represents a link between an incident and a release.
type ReleaseCorrelation struct {
	// ReleaseID is the correlated release.
	ReleaseID string `json:"release_id"`
	// Repository is the release repository.
	Repository string `json:"repository"`
	// Version is the release version.
	Version string `json:"version"`
	// Confidence is how certain the correlation is (0.0 to 1.0).
	Confidence float64 `json:"confidence"`
	// Reasons explains why this correlation was made.
	Reasons []string `json:"reasons"`
	// Incident is the source incident.
	Incident receiver.Incident `json:"incident"`
	// CorrelatedAt is when this correlation was computed.
	CorrelatedAt time.Time `json:"correlated_at"`
}

// EngineConfig configures the correlation engine.
type EngineConfig struct {
	// TimeWindow is the maximum time between a release and an incident for correlation.
	TimeWindow time.Duration `json:"time_window"`
	// MinConfidence is the minimum confidence score to accept a correlation.
	MinConfidence float64 `json:"min_confidence"`
}

// DefaultEngineConfig returns sensible defaults.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		TimeWindow:    2 * time.Hour,
		MinConfidence: 0.3,
	}
}

// Engine correlates production incidents with releases.
type Engine struct {
	store  memory.Store
	config EngineConfig
	logger *slog.Logger
}

// NewEngine creates a new correlation engine.
func NewEngine(store memory.Store, cfg EngineConfig) *Engine {
	return &Engine{
		store:  store,
		config: cfg,
		logger: slog.Default().With("component", "correlation_engine"),
	}
}

// Correlate attempts to find releases that may be linked to the given incident.
func (e *Engine) Correlate(ctx context.Context, incident receiver.Incident) ([]ReleaseCorrelation, error) {
	if incident.Name == "" {
		return nil, fmt.Errorf("incident name is required")
	}

	// Fetch recent releases across all repositories.
	// The store interface requires a repository, so we try common patterns.
	repository := e.inferRepository(incident)

	releases, err := e.store.GetReleaseHistory(ctx, repository, 20)
	if err != nil {
		// If no releases found for the inferred repo, return empty.
		return nil, nil
	}

	var correlations []ReleaseCorrelation
	for _, rel := range releases {
		confidence, reasons := e.scoreCorrelation(incident, rel)

		if confidence >= e.config.MinConfidence {
			correlations = append(correlations, ReleaseCorrelation{
				ReleaseID:    rel.ID,
				Repository:   rel.Repository,
				Version:      rel.Version,
				Confidence:   confidence,
				Reasons:      reasons,
				Incident:     incident,
				CorrelatedAt: time.Now(),
			})
		}
	}

	return correlations, nil
}

// CorrelateForRelease returns all correlations for a specific release ID.
func (e *Engine) CorrelateForRelease(ctx context.Context, releaseID string, incidents []receiver.Incident) ([]ReleaseCorrelation, error) {
	var correlations []ReleaseCorrelation

	for _, inc := range incidents {
		// Build a minimal correlation based on the incident and release ID.
		corr := ReleaseCorrelation{
			ReleaseID:    releaseID,
			Confidence:   0.5, // Base confidence for manual association.
			Reasons:      []string{"manually associated with release"},
			Incident:     inc,
			CorrelatedAt: time.Now(),
		}
		correlations = append(correlations, corr)
	}

	return correlations, nil
}

// scoreCorrelation computes a confidence score for a release-incident pair.
func (e *Engine) scoreCorrelation(incident receiver.Incident, rel *memory.ReleaseRecord) (float64, []string) {
	var score float64
	var reasons []string

	// Time proximity scoring (max 0.5).
	timeDelta := incident.StartedAt.Sub(rel.ReleasedAt)
	if timeDelta < 0 {
		// Incident started before the release — unlikely correlation.
		return 0, nil
	}
	if timeDelta > e.config.TimeWindow {
		return 0, nil
	}

	// Closer in time = higher score. Linear decay over the window.
	timeScore := 0.5 * (1.0 - float64(timeDelta)/float64(e.config.TimeWindow))
	score += timeScore
	reasons = append(reasons, fmt.Sprintf("incident occurred %.0f minutes after release", timeDelta.Minutes()))

	// Service name matching (max 0.3).
	if incident.ServiceName != "" {
		if containsIgnoreCase(rel.Repository, incident.ServiceName) ||
			containsIgnoreCase(incident.ServiceName, rel.Repository) {
			score += 0.3
			reasons = append(reasons, fmt.Sprintf("service name %q matches repository %q", incident.ServiceName, rel.Repository))
		}
	}

	// Label matching — check for deployment/release labels (max 0.2).
	if incident.Labels != nil {
		if ver, ok := incident.Labels["version"]; ok && ver == rel.Version {
			score += 0.2
			reasons = append(reasons, fmt.Sprintf("version label %q matches release version", ver))
		}
		if repo, ok := incident.Labels["repository"]; ok && repo == rel.Repository {
			score += 0.15
			reasons = append(reasons, fmt.Sprintf("repository label %q matches release", repo))
		}
		if releaseID, ok := incident.Labels["release_id"]; ok && releaseID == rel.ID {
			score += 0.3
			reasons = append(reasons, "release_id label directly matches")
		}
	}

	// High severity incidents get a slight boost for alerting.
	if incident.Severity == "critical" && score > 0 {
		score += 0.05
		reasons = append(reasons, "critical severity incident")
	}

	// Cap at 1.0.
	if score > 1.0 {
		score = 1.0
	}

	return score, reasons
}

// inferRepository attempts to determine the repository from incident labels.
func (e *Engine) inferRepository(incident receiver.Incident) string {
	if incident.Labels != nil {
		if repo, ok := incident.Labels["repository"]; ok {
			return repo
		}
		if repo, ok := incident.Labels["repo"]; ok {
			return repo
		}
	}
	if incident.ServiceName != "" {
		return incident.ServiceName
	}
	return ""
}

// containsIgnoreCase checks if s contains substr (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
