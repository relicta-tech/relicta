// Package persistence provides infrastructure implementations for data persistence.
package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/version"
	"github.com/relicta-tech/relicta/internal/fileutil"
)

// MaxReleaseFileSize is the maximum allowed size for release files (2MB).
// This prevents denial of service from maliciously crafted large files.
const MaxReleaseFileSize = 2 << 20 // 2MB

// checkContext checks if the context is canceled and returns the error if so.
// This helper eliminates repeated select/case patterns throughout the repository.
func checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// FileReleaseRepository implements release.Repository using file-based storage.
type FileReleaseRepository struct {
	basePath string
	mu       sync.RWMutex
}

// NewFileReleaseRepository creates a new file-based release repository.
func NewFileReleaseRepository(basePath string) (*FileReleaseRepository, error) {
	// Ensure the directory exists (0700 for directory since it may contain sensitive release data)
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create repository directory: %w", err)
	}

	return &FileReleaseRepository{basePath: basePath}, nil
}

// releaseDTO is a data transfer object for serializing releases.
type releaseDTO struct {
	ID             string       `json:"id"`
	State          string       `json:"state"`
	Branch         string       `json:"branch"`
	RepositoryPath string       `json:"repository_path"`
	RepositoryName string       `json:"repository_name"`
	TagName        string       `json:"tag_name"`
	Plan           *planDTO     `json:"plan,omitempty"`
	Version        *versionDTO  `json:"version,omitempty"`
	Notes          *notesDTO    `json:"notes,omitempty"`
	Approval       *approvalDTO `json:"approval,omitempty"`
	CreatedAt      string       `json:"created_at"`
	UpdatedAt      string       `json:"updated_at"`
	PublishedAt    *string      `json:"published_at,omitempty"`
	LastError      string       `json:"last_error,omitempty"`
}

type planDTO struct {
	CurrentVersion string        `json:"current_version"`
	NextVersion    string        `json:"next_version"`
	ReleaseType    string        `json:"release_type"`
	CommitCount    int           `json:"commit_count"`
	ChangeSetID    string        `json:"changeset_id,omitempty"`
	ChangeSet      *changeSetDTO `json:"changeset,omitempty"`
	DryRun         bool          `json:"dry_run"`
}

type changeSetDTO struct {
	ID        string       `json:"id"`
	FromRef   string       `json:"from_ref"`
	ToRef     string       `json:"to_ref"`
	Commits   []*commitDTO `json:"commits"`
	CreatedAt string       `json:"created_at"`
}

type commitDTO struct {
	Hash        string `json:"hash"`
	Type        string `json:"type"`
	Scope       string `json:"scope,omitempty"`
	Subject     string `json:"subject"`
	Body        string `json:"body,omitempty"`
	Footer      string `json:"footer,omitempty"`
	Breaking    bool   `json:"breaking,omitempty"`
	BreakingMsg string `json:"breaking_msg,omitempty"`
	Author      string `json:"author,omitempty"`
	AuthorEmail string `json:"author_email,omitempty"`
	Date        string `json:"date,omitempty"`
	RawMessage  string `json:"raw_message,omitempty"`
}

type versionDTO struct {
	Major      uint64 `json:"major"`
	Minor      uint64 `json:"minor"`
	Patch      uint64 `json:"patch"`
	Prerelease string `json:"prerelease,omitempty"`
	Metadata   string `json:"metadata,omitempty"`
}

type notesDTO struct {
	Text           string `json:"text"`
	AudiencePreset string `json:"audience_preset,omitempty"`
	TonePreset     string `json:"tone_preset,omitempty"`
	Provider       string `json:"provider,omitempty"`
	Model          string `json:"model,omitempty"`
	GeneratedAt    string `json:"generated_at"`
}

type approvalDTO struct {
	ApprovedBy   string `json:"approved_by"`
	ApprovedAt   string `json:"approved_at"`
	AutoApproved bool   `json:"auto_approved"`
}

// Save persists a release.
func (r *FileReleaseRepository) Save(ctx context.Context, rel *release.ReleaseRun) error {
	if err := checkContext(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	dto := r.toDTO(rel)

	data, err := json.MarshalIndent(dto, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal release: %w", err)
	}

	filePath := r.releaseFilePath(rel.ID())
	// Use atomic write for crash safety (0600 for release files as they may contain sensitive release metadata)
	if err := fileutil.AtomicWriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write release file: %w", err)
	}

	return nil
}

// FindByID retrieves a release by its ID.
func (r *FileReleaseRepository) FindByID(ctx context.Context, id release.RunID) (*release.ReleaseRun, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	filePath := r.releaseFilePath(id)
	data, err := fileutil.ReadFileLimited(filePath, MaxReleaseFileSize)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, release.ErrRunNotFound
		}
		return nil, fmt.Errorf("failed to read release file: %w", err)
	}

	var dto releaseDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return nil, fmt.Errorf("failed to unmarshal release: %w", err)
	}

	return r.fromDTO(&dto)
}

// maxScanWorkers is the maximum number of concurrent file read workers.
// This limits parallelism to avoid overwhelming the filesystem.
const maxScanWorkers = 4

// scanResult holds the result of scanning a single file.
type scanResult struct {
	release *release.Release
	err     error
}

// scanReleases scans all release files and returns those matching the filter.
// The filter receives the parsed DTO and returns true if the release should be included.
// This method must be called with the read lock held.
// It uses concurrent file reading for better performance with many files.
func (r *FileReleaseRepository) scanReleases(ctx context.Context, filter func(*releaseDTO) bool) ([]*release.ReleaseRun, error) {
	entries, err := os.ReadDir(r.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read repository directory: %w", err)
	}

	// Filter to only JSON files first
	jsonFiles := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			jsonFiles = append(jsonFiles, filepath.Join(r.basePath, entry.Name()))
		}
	}

	// For small numbers of files, use sequential scanning
	if len(jsonFiles) < maxScanWorkers*2 {
		return r.scanReleasesSequential(ctx, jsonFiles, filter)
	}

	// Use concurrent scanning for larger numbers of files
	return r.scanReleasesConcurrent(ctx, jsonFiles, filter)
}

// scanReleasesSequential scans files sequentially (for small file counts).
func (r *FileReleaseRepository) scanReleasesSequential(ctx context.Context, files []string, filter func(*releaseDTO) bool) ([]*release.ReleaseRun, error) {
	releases := make([]*release.ReleaseRun, 0, len(files)/2)

	for _, filePath := range files {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}

		rel, err := r.scanSingleFile(filePath, filter)
		if err != nil || rel == nil {
			continue
		}
		releases = append(releases, rel)
	}

	return releases, nil
}

// scanReleasesConcurrent scans files concurrently using a worker pool.
func (r *FileReleaseRepository) scanReleasesConcurrent(ctx context.Context, files []string, filter func(*releaseDTO) bool) ([]*release.ReleaseRun, error) {
	// Create channels for work distribution and results
	jobs := make(chan string, len(files))
	results := make(chan scanResult, len(files))

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start workers
	var wg sync.WaitGroup
	numWorkers := maxScanWorkers
	if len(files) < numWorkers {
		numWorkers = len(files)
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case filePath, ok := <-jobs:
					if !ok {
						return
					}
					rel, err := r.scanSingleFile(filePath, filter)
					select {
					case results <- scanResult{release: rel, err: err}:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	// Send jobs to workers
	go func() {
	jobLoop:
		for _, file := range files {
			select {
			case jobs <- file:
			case <-ctx.Done():
				break jobLoop
			}
		}
		close(jobs)
	}()

	// Wait for workers to finish and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	releases := make([]*release.ReleaseRun, 0, len(files)/2)
	for result := range results {
		if result.release != nil {
			releases = append(releases, result.release)
		}
	}

	if err := checkContext(ctx); err != nil {
		return nil, err
	}

	return releases, nil
}

// scanSingleFile reads and parses a single release file.
func (r *FileReleaseRepository) scanSingleFile(filePath string, filter func(*releaseDTO) bool) (*release.ReleaseRun, error) {
	data, err := fileutil.ReadFileLimited(filePath, MaxReleaseFileSize)
	if err != nil {
		// Skip files that can't be read (may be corrupted or locked)
		return nil, err
	}

	var dto releaseDTO
	if unmarshalErr := json.Unmarshal(data, &dto); unmarshalErr != nil {
		// Skip files that can't be parsed (may be malformed)
		return nil, unmarshalErr
	}

	if !filter(&dto) {
		return nil, nil
	}

	rel, err := r.fromDTO(&dto)
	if err != nil {
		// Skip releases that can't be reconstructed
		return nil, err
	}

	return rel, nil
}

// FindLatest retrieves the latest release for a repository.
func (r *FileReleaseRepository) FindLatest(ctx context.Context, repoPath string) (*release.ReleaseRun, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	releases, err := r.scanReleases(ctx, func(dto *releaseDTO) bool {
		return dto.RepositoryPath == repoPath
	})
	if err != nil {
		return nil, err
	}

	if len(releases) == 0 {
		return nil, release.ErrRunNotFound
	}

	// Find the latest by UpdatedAt
	latest := releases[0]
	for _, rel := range releases[1:] {
		if rel.UpdatedAt().After(latest.UpdatedAt()) {
			latest = rel
		}
	}

	return latest, nil
}

// FindByState retrieves releases in a specific state.
func (r *FileReleaseRepository) FindByState(ctx context.Context, state release.RunState) ([]*release.ReleaseRun, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.scanReleases(ctx, func(dto *releaseDTO) bool {
		return dto.State == string(state)
	})
}

// FindActive retrieves all active (non-final) releases.
func (r *FileReleaseRepository) FindActive(ctx context.Context) ([]*release.ReleaseRun, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.scanReleases(ctx, func(dto *releaseDTO) bool {
		state := release.RunState(dto.State)
		return !state.IsFinal()
	})
}

// FindBySpecification retrieves releases matching the given specification.
// This method scans all releases and applies the specification filter.
func (r *FileReleaseRepository) FindBySpecification(ctx context.Context, spec release.Specification) ([]*release.ReleaseRun, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Get all releases first, then filter by specification
	// This is necessary because specifications operate on domain objects, not DTOs
	allReleases, err := r.scanReleases(ctx, func(dto *releaseDTO) bool {
		return true // Accept all at DTO level
	})
	if err != nil {
		return nil, err
	}

	// Apply specification filter
	result := make([]*release.ReleaseRun, 0, len(allReleases))
	for _, rel := range allReleases {
		if spec.IsSatisfiedBy(rel) {
			result = append(result, rel)
		}
	}

	return result, nil
}

// Delete removes a release.
func (r *FileReleaseRepository) Delete(ctx context.Context, id release.RunID) error {
	if err := checkContext(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	filePath := r.releaseFilePath(id)
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return release.ErrRunNotFound
		}
		return fmt.Errorf("failed to delete release file: %w", err)
	}

	return nil
}

// Helper methods

func (r *FileReleaseRepository) releaseFilePath(id release.RunID) string {
	return filepath.Join(r.basePath, string(id)+".json")
}

func (r *FileReleaseRepository) toDTO(rel *release.ReleaseRun) *releaseDTO {
	dto := &releaseDTO{
		ID:             string(rel.ID()),
		State:          string(rel.State()),
		Branch:         rel.Branch(),
		RepositoryPath: rel.RepoRoot(),
		RepositoryName: rel.RepoID(),
		TagName:        rel.TagName(),
		CreatedAt:      rel.CreatedAt().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      rel.UpdatedAt().Format("2006-01-02T15:04:05Z07:00"),
		LastError:      rel.LastError(),
	}

	if plan := release.GetPlan(rel); plan != nil {
		dto.Plan = &planDTO{
			CurrentVersion: plan.CurrentVersion.String(),
			NextVersion:    plan.NextVersion.String(),
			ReleaseType:    plan.ReleaseType.String(),
			CommitCount:    plan.CommitCount(),
			ChangeSetID:    string(plan.ChangeSetID),
			DryRun:         plan.DryRun,
		}

		// Serialize the changeset if available
		if cs := plan.GetChangeSet(); cs != nil {
			commits := cs.Commits()
			commitDTOs := make([]*commitDTO, 0, len(commits))
			for _, c := range commits {
				commitDTOs = append(commitDTOs, &commitDTO{
					Hash:        c.Hash(),
					Type:        string(c.Type()),
					Scope:       c.Scope(),
					Subject:     c.Subject(),
					Body:        c.Body(),
					Footer:      c.Footer(),
					Breaking:    c.IsBreaking(),
					BreakingMsg: c.BreakingMessage(),
					Author:      c.Author(),
					AuthorEmail: c.AuthorEmail(),
					Date:        c.Date().Format(time.RFC3339),
					RawMessage:  c.RawMessage(),
				})
			}
			dto.Plan.ChangeSet = &changeSetDTO{
				ID:        string(cs.ID()),
				FromRef:   cs.FromRef(),
				ToRef:     cs.ToRef(),
				Commits:   commitDTOs,
				CreatedAt: cs.CreatedAt().Format(time.RFC3339),
			}
		}
	}

	if !rel.VersionNext().IsZero() {
		ver := rel.VersionNext()
		dto.Version = &versionDTO{
			Major:      ver.Major(),
			Minor:      ver.Minor(),
			Patch:      ver.Patch(),
			Prerelease: string(ver.Prerelease()),
			Metadata:   string(ver.Metadata()),
		}
	}

	if rel.Notes() != nil {
		notes := rel.Notes()
		dto.Notes = &notesDTO{
			Text:           notes.Text,
			AudiencePreset: notes.AudiencePreset,
			TonePreset:     notes.TonePreset,
			Provider:       notes.Provider,
			Model:          notes.Model,
			GeneratedAt:    notes.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	if rel.Approval() != nil {
		approval := rel.Approval()
		dto.Approval = &approvalDTO{
			ApprovedBy:   approval.ApprovedBy,
			ApprovedAt:   approval.ApprovedAt.Format("2006-01-02T15:04:05Z07:00"),
			AutoApproved: approval.AutoApproved,
		}
	}

	if rel.PublishedAt() != nil {
		publishedAt := rel.PublishedAt().Format("2006-01-02T15:04:05Z07:00")
		dto.PublishedAt = &publishedAt
	}

	return dto
}

func (r *FileReleaseRepository) fromDTO(dto *releaseDTO) (*release.ReleaseRun, error) {
	// Create base release (this sets state to Initialized)
	rel := release.NewRelease(release.RunID(dto.ID), dto.Branch, dto.RepositoryPath)
	// Note: RepositoryName is now derived from repoID in the new model

	// Parse timestamps
	createdAt, err := time.Parse(time.RFC3339, dto.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created_at: %w", err)
	}
	updatedAt, err := time.Parse(time.RFC3339, dto.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	var publishedAt *time.Time
	if dto.PublishedAt != nil {
		t, err := time.Parse(time.RFC3339, *dto.PublishedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse published_at: %w", err)
		}
		publishedAt = &t
	}

	// Reconstruct plan
	var plan *release.ReleaseRunPlan
	if dto.Plan != nil {
		currentVer, err := version.Parse(dto.Plan.CurrentVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse current version: %w", err)
		}
		nextVer, err := version.Parse(dto.Plan.NextVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse next version: %w", err)
		}
		releaseType, err := changes.ParseReleaseType(dto.Plan.ReleaseType)
		if err != nil {
			return nil, fmt.Errorf("failed to parse release type: %w", err)
		}

		// Reconstruct changeset if available
		var changeSet *changes.ChangeSet
		if dto.Plan.ChangeSet != nil {
			csDTO := dto.Plan.ChangeSet
			changeSet = changes.NewChangeSet(
				changes.ChangeSetID(csDTO.ID),
				csDTO.FromRef,
				csDTO.ToRef,
			)

			// Reconstruct commits
			for _, cDTO := range csDTO.Commits {
				commitType, _ := changes.ParseCommitType(cDTO.Type)

				// Build options for optional fields
				opts := []changes.ConventionalCommitOption{
					changes.WithScope(cDTO.Scope),
					changes.WithBody(cDTO.Body),
					changes.WithFooter(cDTO.Footer),
					changes.WithAuthor(cDTO.Author, cDTO.AuthorEmail),
				}

				if cDTO.Breaking {
					opts = append(opts, changes.WithBreaking(cDTO.BreakingMsg))
				}

				if cDTO.Date != "" {
					if t, err := time.Parse(time.RFC3339, cDTO.Date); err == nil {
						opts = append(opts, changes.WithDate(t))
					}
				}

				if cDTO.RawMessage != "" {
					opts = append(opts, changes.WithRawMessage(cDTO.RawMessage))
				}

				commit := changes.NewConventionalCommit(cDTO.Hash, commitType, cDTO.Subject, opts...)
				changeSet.AddCommit(commit)
			}
		}

		plan = release.NewReleasePlan(currentVer, nextVer, releaseType, changeSet, dto.Plan.DryRun)
		// Restore the original ChangeSetID from persisted data (if different or nil changeset)
		if dto.Plan.ChangeSetID != "" {
			plan.ChangeSetID = changes.ChangeSetID(dto.Plan.ChangeSetID)
		}
	}

	// Reconstruct version
	var ver *version.SemanticVersion
	if dto.Version != nil {
		v := version.NewSemanticVersion(dto.Version.Major, dto.Version.Minor, dto.Version.Patch)
		if dto.Version.Prerelease != "" {
			v = v.WithPrerelease(version.Prerelease(dto.Version.Prerelease))
		}
		if dto.Version.Metadata != "" {
			v = v.WithMetadata(version.BuildMetadata(dto.Version.Metadata))
		}
		ver = &v
	}

	// Reconstruct notes
	var notes *release.ReleaseRunNotes
	if dto.Notes != nil {
		generatedAt, err := time.Parse(time.RFC3339, dto.Notes.GeneratedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse notes generated_at: %w", err)
		}
		notes = &release.ReleaseRunNotes{
			Text:           dto.Notes.Text,
			AudiencePreset: dto.Notes.AudiencePreset,
			TonePreset:     dto.Notes.TonePreset,
			Provider:       dto.Notes.Provider,
			Model:          dto.Notes.Model,
			GeneratedAt:    generatedAt,
		}
	}

	// Reconstruct approval
	var approval *release.Approval
	if dto.Approval != nil {
		approvedAt, err := time.Parse(time.RFC3339, dto.Approval.ApprovedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse approval approved_at: %w", err)
		}
		approval = &release.Approval{
			ApprovedBy:   dto.Approval.ApprovedBy,
			ApprovedAt:   approvedAt,
			AutoApproved: dto.Approval.AutoApproved,
		}
	}

	// Use ReconstructFromLegacy to restore the aggregate without triggering events
	release.ReconstructFromLegacy(
		rel,
		release.RunState(dto.State),
		plan,
		ver,
		dto.TagName,
		notes,
		approval,
		createdAt,
		updatedAt,
		publishedAt,
		dto.LastError,
	)

	return rel, nil
}
