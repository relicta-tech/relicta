package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/relicta-tech/relicta/internal/fileutil"
)

// MaxMemoryFileSize is the maximum allowed size for memory files (5MB).
const MaxMemoryFileSize = 5 << 20 // 5MB

// FileStore provides a file-based implementation of the Store interface.
// Data is stored in JSON files in the specified directory.
type FileStore struct {
	basePath string
	mu       sync.RWMutex

	// In-memory cache for fast reads
	releases  map[string][]*ReleaseRecord  // keyed by repository
	incidents map[string][]*IncidentRecord // keyed by repository
	actors    map[string]*ActorMetrics     // keyed by actor ID

	// Track if data has been loaded
	loaded bool
}

// NewFileStore creates a new file-based memory store.
func NewFileStore(basePath string) (*FileStore, error) {
	// Ensure the directory exists
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create memory directory: %w", err)
	}

	store := &FileStore{
		basePath:  basePath,
		releases:  make(map[string][]*ReleaseRecord),
		incidents: make(map[string][]*IncidentRecord),
		actors:    make(map[string]*ActorMetrics),
	}

	// Load existing data
	if err := store.load(); err != nil {
		return nil, fmt.Errorf("failed to load memory data: %w", err)
	}

	return store, nil
}

// fileData represents the JSON structure for persistence.
type fileData struct {
	Releases  map[string][]*ReleaseRecord  `json:"releases"`
	Incidents map[string][]*IncidentRecord `json:"incidents"`
	Actors    map[string]*ActorMetrics     `json:"actors"`
	UpdatedAt time.Time                    `json:"updatedAt"`
}

// dataFilePath returns the path to the main data file.
func (s *FileStore) dataFilePath() string {
	return filepath.Join(s.basePath, "memory.json")
}

// load reads existing data from disk.
func (s *FileStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.dataFilePath()
	data, err := fileutil.ReadFileLimited(path, MaxMemoryFileSize)
	if err != nil {
		if os.IsNotExist(err) {
			// No existing data, start fresh
			s.loaded = true
			return nil
		}
		return fmt.Errorf("failed to read memory file: %w", err)
	}

	var fd fileData
	if err := json.Unmarshal(data, &fd); err != nil {
		return fmt.Errorf("failed to unmarshal memory data: %w", err)
	}

	// Load into cache
	if fd.Releases != nil {
		s.releases = fd.Releases
	}
	if fd.Incidents != nil {
		s.incidents = fd.Incidents
	}
	if fd.Actors != nil {
		s.actors = fd.Actors
	}

	s.loaded = true
	return nil
}

// save persists data to disk.
func (s *FileStore) save() error {
	fd := fileData{
		Releases:  s.releases,
		Incidents: s.incidents,
		Actors:    s.actors,
		UpdatedAt: time.Now(),
	}

	data, err := json.MarshalIndent(fd, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal memory data: %w", err)
	}

	if err := fileutil.AtomicWriteFile(s.dataFilePath(), data, 0600); err != nil {
		return fmt.Errorf("failed to write memory file: %w", err)
	}

	return nil
}

// RecordRelease stores a release record.
func (s *FileStore) RecordRelease(ctx context.Context, record *ReleaseRecord) error {
	if record == nil {
		return fmt.Errorf("record is required")
	}
	if record.ID == "" {
		return fmt.Errorf("record ID is required")
	}
	if record.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.releases[record.Repository] = append(s.releases[record.Repository], record)

	// Update actor metrics
	s.updateActorMetricsLocked(record)

	return s.save()
}

// updateActorMetricsLocked updates actor metrics based on a release record.
// Must be called with the lock held.
func (s *FileStore) updateActorMetricsLocked(record *ReleaseRecord) {
	actorID := record.Actor.ID
	metrics, exists := s.actors[actorID]
	if !exists {
		metrics = &ActorMetrics{
			ActorID:   actorID,
			ActorKind: record.Actor.Kind,
		}
		s.actors[actorID] = metrics
	}

	now := time.Now()

	// Update counts
	metrics.TotalReleases++
	switch record.Outcome {
	case OutcomeSuccess:
		metrics.SuccessfulReleases++
	case OutcomeFailed, OutcomePartial:
		metrics.FailedReleases++
	case OutcomeRollback:
		metrics.RollbackCount++
		metrics.FailedReleases++
	}

	if record.RiskScore > 0.7 {
		metrics.HighRiskReleases++
	}
	if record.BreakingChanges > 0 {
		metrics.BreakingChangeReleases++
	}

	// Update average risk score (running average)
	n := float64(metrics.TotalReleases)
	metrics.AverageRiskScore = ((n-1)*metrics.AverageRiskScore + record.RiskScore) / n

	// Update success rate
	metrics.SuccessRate = float64(metrics.SuccessfulReleases) / float64(metrics.TotalReleases)

	// Update timestamps
	if metrics.FirstReleaseAt == nil {
		metrics.FirstReleaseAt = &record.ReleasedAt
	}
	metrics.LastReleaseAt = &record.ReleasedAt
	metrics.UpdatedAt = now

	// Recalculate reliability score
	metrics.ReliabilityScore = metrics.CalculateReliabilityScore()
}

// RecordIncident stores an incident record.
func (s *FileStore) RecordIncident(ctx context.Context, incident *IncidentRecord) error {
	if incident == nil {
		return fmt.Errorf("incident is required")
	}
	if incident.ID == "" {
		return fmt.Errorf("incident ID is required")
	}
	if incident.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.incidents[incident.Repository] = append(s.incidents[incident.Repository], incident)

	// Update actor incident count
	if incident.ActorID != "" {
		if metrics, exists := s.actors[incident.ActorID]; exists {
			metrics.IncidentCount++
			metrics.ReliabilityScore = metrics.CalculateReliabilityScore()
			metrics.UpdatedAt = time.Now()
		}
	}

	return s.save()
}

// GetReleaseHistory returns release records for a repository.
func (s *FileStore) GetReleaseHistory(ctx context.Context, repository string, limit int) ([]*ReleaseRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	releases := s.releases[repository]
	if len(releases) == 0 {
		return []*ReleaseRecord{}, nil
	}

	// Return most recent first
	result := make([]*ReleaseRecord, 0, min(limit, len(releases)))
	start := len(releases) - limit
	if start < 0 {
		start = 0
	}
	for i := len(releases) - 1; i >= start; i-- {
		result = append(result, releases[i])
	}

	return result, nil
}

// GetIncidentHistory returns incident records for a repository.
func (s *FileStore) GetIncidentHistory(ctx context.Context, repository string, limit int) ([]*IncidentRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	incidents := s.incidents[repository]
	if len(incidents) == 0 {
		return []*IncidentRecord{}, nil
	}

	// Return most recent first
	result := make([]*IncidentRecord, 0, min(limit, len(incidents)))
	start := len(incidents) - limit
	if start < 0 {
		start = 0
	}
	for i := len(incidents) - 1; i >= start; i-- {
		result = append(result, incidents[i])
	}

	return result, nil
}

// GetActorMetrics returns behavior metrics for an actor.
func (s *FileStore) GetActorMetrics(ctx context.Context, actorID string) (*ActorMetrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics, exists := s.actors[actorID]
	if !exists {
		return nil, fmt.Errorf("no metrics found for actor: %s", actorID)
	}

	// Return a copy
	copy := *metrics
	return &copy, nil
}

// GetRiskPatterns returns historical risk patterns for a repository.
func (s *FileStore) GetRiskPatterns(ctx context.Context, repository string) (*RiskPatterns, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	releases := s.releases[repository]
	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found for repository: %s", repository)
	}

	// Calculate patterns from historical data
	patterns := &RiskPatterns{
		Repository:    repository,
		TotalReleases: len(releases),
		UpdatedAt:     time.Now(),
	}

	// Calculate average risk score
	var totalRisk float64
	riskFactorCounts := make(map[string]int)

	var minTime, maxTime time.Time
	for i, r := range releases {
		totalRisk += r.RiskScore

		if i == 0 || r.ReleasedAt.Before(minTime) {
			minTime = r.ReleasedAt
		}
		if i == 0 || r.ReleasedAt.After(maxTime) {
			maxTime = r.ReleasedAt
		}

		// Count risk factors from tags
		for _, tag := range r.Tags {
			riskFactorCounts[tag]++
		}
	}

	patterns.AverageRiskScore = totalRisk / float64(len(releases))
	patterns.AnalysisPeriod = TimePeriod{Start: minTime, End: maxTime}

	// Determine trend (comparing first half to second half)
	if len(releases) >= 4 {
		mid := len(releases) / 2
		var firstHalfRisk, secondHalfRisk float64
		for i := 0; i < mid; i++ {
			firstHalfRisk += releases[i].RiskScore
		}
		for i := mid; i < len(releases); i++ {
			secondHalfRisk += releases[i].RiskScore
		}
		firstHalfAvg := firstHalfRisk / float64(mid)
		secondHalfAvg := secondHalfRisk / float64(len(releases)-mid)

		diff := secondHalfAvg - firstHalfAvg
		if diff > 0.1 {
			patterns.RiskTrend = TrendIncreasing
		} else if diff < -0.1 {
			patterns.RiskTrend = TrendDecreasing
		} else {
			patterns.RiskTrend = TrendStable
		}
	} else {
		patterns.RiskTrend = TrendStable
	}

	// Build common risk factor patterns
	for factor, count := range riskFactorCounts {
		patterns.CommonRiskFactors = append(patterns.CommonRiskFactors, RiskFactorPattern{
			Category:  factor,
			Frequency: float64(count) / float64(len(releases)),
		})
	}

	return patterns, nil
}

// UpdateActorMetrics updates metrics for an actor based on a release outcome.
func (s *FileStore) UpdateActorMetrics(ctx context.Context, actorID string, outcome ReleaseOutcome) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metrics, exists := s.actors[actorID]
	if !exists {
		return fmt.Errorf("no metrics found for actor: %s", actorID)
	}

	// Update based on outcome (used for updating after initial record)
	switch outcome {
	case OutcomeRollback:
		metrics.RollbackCount++
		metrics.FailedReleases++
		metrics.SuccessfulReleases-- // Undo the initial success count
	}

	metrics.SuccessRate = float64(metrics.SuccessfulReleases) / float64(metrics.TotalReleases)
	metrics.ReliabilityScore = metrics.CalculateReliabilityScore()
	metrics.UpdatedAt = time.Now()

	return s.save()
}

// Flush ensures all data is written to disk.
func (s *FileStore) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save()
}

// Stats returns store statistics.
func (s *FileStore) Stats() StoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalReleases, totalIncidents int
	for _, releases := range s.releases {
		totalReleases += len(releases)
	}
	for _, incidents := range s.incidents {
		totalIncidents += len(incidents)
	}

	return StoreStats{
		Repositories:   len(s.releases),
		TotalReleases:  totalReleases,
		TotalIncidents: totalIncidents,
		TrackedActors:  len(s.actors),
	}
}

// StoreStats contains store statistics.
type StoreStats struct {
	Repositories   int `json:"repositories"`
	TotalReleases  int `json:"totalReleases"`
	TotalIncidents int `json:"totalIncidents"`
	TrackedActors  int `json:"trackedActors"`
}
