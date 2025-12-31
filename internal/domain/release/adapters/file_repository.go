// Package adapters provides infrastructure implementations for the release governance domain.
package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

const (
	runsDir           = ".relicta/releases"
	latestFile        = "latest"
	runFileSuffix     = ".json"
	machineFileSuffix = ".machine.json"
	stateFileSuffix   = ".state.json"
)

// FileReleaseRunRepository implements ReleaseRunRepository using file-based storage.
type FileReleaseRunRepository struct {
	mu        sync.RWMutex
	repoRoots map[string]struct{} // Track known repository roots for Load() scanning
}

// NewFileReleaseRunRepository creates a new file-based repository.
func NewFileReleaseRunRepository() *FileReleaseRunRepository {
	return &FileReleaseRunRepository{
		repoRoots: make(map[string]struct{}),
	}
}

// Ensure FileReleaseRunRepository implements the interface.
var _ ports.ReleaseRunRepository = (*FileReleaseRunRepository)(nil)

// runsPath returns the path to the runs directory for a repo.
func runsPath(repoRoot string) string {
	return filepath.Join(repoRoot, runsDir)
}

// runPath returns the path to a specific run file.
// It validates that the runID doesn't contain path traversal characters.
func runPath(repoRoot string, runID domain.RunID) string {
	return filepath.Join(runsPath(repoRoot), filepath.Base(string(runID))+runFileSuffix)
}

// latestPath returns the path to the latest pointer file.
func latestPath(repoRoot string) string {
	return filepath.Join(runsPath(repoRoot), latestFile)
}

// machinePath returns the path to the state machine JSON export.
func machinePath(repoRoot string, runID domain.RunID) string {
	return filepath.Join(runsPath(repoRoot), string(runID)+machineFileSuffix)
}

// statePath returns the path to the state snapshot JSON.
func statePath(repoRoot string, runID domain.RunID) string {
	return filepath.Join(runsPath(repoRoot), string(runID)+stateFileSuffix)
}

// ensureDir creates the runs directory if it doesn't exist.
func ensureDir(repoRoot string) error {
	dir := runsPath(repoRoot)
	return os.MkdirAll(dir, 0755)
}

// ReleaseRunDTO is the data transfer object for serialization.
type ReleaseRunDTO struct {
	ID             string                   `json:"id"`
	PlanHash       string                   `json:"plan_hash"`
	RepoID         string                   `json:"repo_id"`
	RepoRoot       string                   `json:"repo_root"`
	BaseRef        string                   `json:"base_ref"`
	HeadSHA        string                   `json:"head_sha"`
	Commits        []string                 `json:"commits"`
	ConfigHash     string                   `json:"config_hash"`
	PluginPlanHash string                   `json:"plugin_plan_hash"`
	VersionCurrent string                   `json:"version_current"`
	VersionNext    string                   `json:"version_next"`
	BumpKind       string                   `json:"bump_kind"`
	Confidence     float64                  `json:"confidence"`
	RiskScore      float64                  `json:"risk_score"`
	Reasons        []string                 `json:"reasons"`
	ActorType      string                   `json:"actor_type"`
	ActorID        string                   `json:"actor_id"`
	Thresholds     PolicyThresholdsDTO      `json:"thresholds"`
	TagName        string                   `json:"tag_name,omitempty"`
	Notes          *ReleaseNotesDTO         `json:"notes,omitempty"`
	NotesInputHash string                   `json:"notes_inputs_hash,omitempty"`
	Approval       *ApprovalDTO             `json:"approval,omitempty"`
	Steps          []StepPlanDTO            `json:"steps"`
	StepStatus     map[string]StepStatusDTO `json:"step_status"`
	State          string                   `json:"state"`
	History        []TransitionRecordDTO    `json:"history"`
	LastError      string                   `json:"last_error,omitempty"`
	ChangesetID    string                   `json:"changeset_id,omitempty"`
	CreatedAt      time.Time                `json:"created_at"`
	UpdatedAt      time.Time                `json:"updated_at"`
	PublishedAt    *time.Time               `json:"published_at,omitempty"`
}

// PolicyThresholdsDTO is the DTO for policy thresholds.
type PolicyThresholdsDTO struct {
	AutoApproveRiskThreshold float64 `json:"auto_approve_risk_threshold"`
	RequireApprovalAbove     float64 `json:"require_approval_above"`
	BlockReleaseAbove        float64 `json:"block_release_above"`
}

// ReleaseNotesDTO is the DTO for release notes.
type ReleaseNotesDTO struct {
	Text           string    `json:"text"`
	AudiencePreset string    `json:"audience_preset"`
	TonePreset     string    `json:"tone_preset"`
	Provider       string    `json:"provider"`
	Model          string    `json:"model"`
	GeneratedAt    time.Time `json:"generated_at"`
}

// StepPlanDTO is the DTO for step plans.
type StepPlanDTO struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	ConfigHash     string `json:"config_hash"`
	IdempotencyKey string `json:"idempotency_key"`
	PluginName     string `json:"plugin_name,omitempty"`
	Hook           string `json:"hook,omitempty"`
	Unsafe         bool   `json:"unsafe,omitempty"`
}

// StepStatusDTO is the DTO for step status.
type StepStatusDTO struct {
	State       string     `json:"state"`
	Attempts    int        `json:"attempts"`
	LastError   string     `json:"last_error,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Output      string     `json:"output,omitempty"`
}

// TransitionRecordDTO is the DTO for transition records.
type TransitionRecordDTO struct {
	At       time.Time         `json:"at"`
	From     string            `json:"from"`
	To       string            `json:"to"`
	Event    string            `json:"event"`
	Actor    string            `json:"actor"`
	Reason   string            `json:"reason"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ApprovalDTO is the DTO for approval information.
type ApprovalDTO struct {
	ApprovedBy    string    `json:"approved_by"`
	ApprovedAt    time.Time `json:"approved_at"`
	AutoApproved  bool      `json:"auto_approved"`
	PlanHash      string    `json:"plan_hash"`
	RiskScore     float64   `json:"risk_score"`
	ApproverType  string    `json:"approver_type"`
	Justification string    `json:"justification,omitempty"`
}

// trackRepoRoot adds a repo root to the known set (must be called with lock held).
func (r *FileReleaseRunRepository) trackRepoRoot(repoRoot string) {
	if r.repoRoots == nil {
		r.repoRoots = make(map[string]struct{})
	}
	r.repoRoots[repoRoot] = struct{}{}
}

// Save persists a release run atomically.
func (r *FileReleaseRunRepository) Save(ctx context.Context, run *domain.ReleaseRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := ensureDir(run.RepoRoot()); err != nil {
		return fmt.Errorf("failed to create runs directory: %w", err)
	}

	// Track this repo root for future Load() calls
	r.trackRepoRoot(run.RepoRoot())

	dto := toDTO(run)
	data, err := json.MarshalIndent(dto, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal run: %w", err)
	}

	// Atomic write: write to temp file then rename
	path := runPath(run.RepoRoot(), run.ID())
	tmpPath := path + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath) // Best-effort cleanup on failure
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Also export state snapshot
	if stateData, err := domain.ExportStateSnapshot(run); err == nil {
		statePath := statePath(run.RepoRoot(), run.ID())
		_ = os.WriteFile(statePath, stateData, 0644)
	}

	return nil
}

// Load retrieves a release run by its ID by scanning known repository roots.
// For better performance when the repo root is known, use LoadFromRepo instead.
func (r *FileReleaseRunRepository) Load(ctx context.Context, runID domain.RunID) (*domain.ReleaseRun, error) {
	r.mu.RLock()
	knownRoots := make([]string, 0, len(r.repoRoots))
	for root := range r.repoRoots {
		knownRoots = append(knownRoots, root)
	}
	r.mu.RUnlock()

	// Scan known repo roots to find the run
	for _, repoRoot := range knownRoots {
		run, err := r.loadFromRepoInternal(ctx, repoRoot, runID)
		if err == nil {
			return run, nil
		}
		// Continue searching if not found in this repo
		if !errors.Is(err, domain.ErrRunNotFound) {
			return nil, err
		}
	}

	return nil, domain.ErrRunNotFound
}

// LoadFromRepo retrieves a release run from a specific repository.
func (r *FileReleaseRunRepository) LoadFromRepo(ctx context.Context, repoRoot string, runID domain.RunID) (*domain.ReleaseRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	path := runPath(repoRoot, runID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrRunNotFound
		}
		return nil, fmt.Errorf("failed to read run file: %w", err)
	}

	var dto ReleaseRunDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return nil, fmt.Errorf("failed to unmarshal run: %w", err)
	}

	return fromDTO(&dto)
}

// LoadLatest retrieves the latest release run for a repository.
func (r *FileReleaseRunRepository) LoadLatest(ctx context.Context, repoRoot string) (*domain.ReleaseRun, error) {
	// Get the latest run ID under read lock
	runID, err := r.getLatestRunID(repoRoot)
	if err != nil {
		return nil, err
	}

	// Load the run (LoadFromRepo handles its own locking)
	return r.loadFromRepoInternal(ctx, repoRoot, runID)
}

// getLatestRunID reads the latest pointer file under read lock.
func (r *FileReleaseRunRepository) getLatestRunID(repoRoot string) (domain.RunID, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	path := latestPath(repoRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", domain.ErrRunNotFound
		}
		return "", fmt.Errorf("failed to read latest pointer: %w", err)
	}

	return domain.RunID(strings.TrimSpace(string(data))), nil
}

// loadFromRepoInternal is the internal implementation that handles locking.
func (r *FileReleaseRunRepository) loadFromRepoInternal(ctx context.Context, repoRoot string, runID domain.RunID) (*domain.ReleaseRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	path := runPath(repoRoot, runID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrRunNotFound
		}
		return nil, fmt.Errorf("failed to read run file: %w", err)
	}

	var dto ReleaseRunDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return nil, fmt.Errorf("failed to unmarshal run: %w", err)
	}

	return fromDTO(&dto)
}

// SetLatest sets the latest run ID pointer for a repository.
func (r *FileReleaseRunRepository) SetLatest(ctx context.Context, repoRoot string, runID domain.RunID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := ensureDir(repoRoot); err != nil {
		return fmt.Errorf("failed to create runs directory: %w", err)
	}

	// Track this repo root for future Load() calls
	r.trackRepoRoot(repoRoot)

	path := latestPath(repoRoot)
	tmpPath := path + ".tmp"

	if err := os.WriteFile(tmpPath, []byte(string(runID)), 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath) // Best-effort cleanup on failure
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// List returns all run IDs for a repository, ordered by creation time (newest first).
func (r *FileReleaseRunRepository) List(ctx context.Context, repoRoot string) ([]domain.RunID, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	dir := runsPath(repoRoot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read runs directory: %w", err)
	}

	type runWithTime struct {
		id      domain.RunID
		modTime time.Time
	}

	var runs []runWithTime
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), runFileSuffix) {
			continue
		}
		if entry.Name() == latestFile {
			continue
		}
		// Skip state and machine files (which also end with .json)
		if strings.HasSuffix(entry.Name(), stateFileSuffix) || strings.HasSuffix(entry.Name(), machineFileSuffix) {
			continue
		}

		runID := domain.RunID(strings.TrimSuffix(entry.Name(), runFileSuffix))
		info, err := entry.Info()
		if err != nil {
			continue
		}
		runs = append(runs, runWithTime{id: runID, modTime: info.ModTime()})
	}

	// Sort by modification time, newest first
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].modTime.After(runs[j].modTime)
	})

	result := make([]domain.RunID, len(runs))
	for i, run := range runs {
		result[i] = run.id
	}

	return result, nil
}

// Delete removes a release run by scanning known repository roots.
// For better performance when the repo root is known, use DeleteFromRepo instead.
func (r *FileReleaseRunRepository) Delete(ctx context.Context, runID domain.RunID) error {
	r.mu.RLock()
	knownRoots := make([]string, 0, len(r.repoRoots))
	for root := range r.repoRoots {
		knownRoots = append(knownRoots, root)
	}
	r.mu.RUnlock()

	// Find and delete from the first repo that contains this run
	for _, repoRoot := range knownRoots {
		path := runPath(repoRoot, runID)
		if _, err := os.Stat(path); err == nil {
			return r.DeleteFromRepo(ctx, repoRoot, runID)
		}
	}

	return domain.ErrRunNotFound
}

// DeleteFromRepo removes a release run from a specific repository.
func (r *FileReleaseRunRepository) DeleteFromRepo(ctx context.Context, repoRoot string, runID domain.RunID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	path := runPath(repoRoot, runID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete run file: %w", err)
	}

	// Also remove state and machine files (best-effort cleanup)
	_ = os.Remove(statePath(repoRoot, runID))
	_ = os.Remove(machinePath(repoRoot, runID))

	return nil
}

// FindByState finds runs in a specific state.
func (r *FileReleaseRunRepository) FindByState(ctx context.Context, repoRoot string, state domain.RunState) ([]*domain.ReleaseRun, error) {
	runIDs, err := r.List(ctx, repoRoot)
	if err != nil {
		return nil, err
	}

	var runs []*domain.ReleaseRun
	for _, runID := range runIDs {
		run, err := r.LoadFromRepo(ctx, repoRoot, runID)
		if err != nil {
			continue
		}
		if run.State() == state {
			runs = append(runs, run)
		}
	}

	return runs, nil
}

// FindActive finds all non-terminal runs for a repository.
func (r *FileReleaseRunRepository) FindActive(ctx context.Context, repoRoot string) ([]*domain.ReleaseRun, error) {
	runIDs, err := r.List(ctx, repoRoot)
	if err != nil {
		return nil, err
	}

	var runs []*domain.ReleaseRun
	for _, runID := range runIDs {
		run, err := r.LoadFromRepo(ctx, repoRoot, runID)
		if err != nil {
			continue
		}
		if run.State().IsActive() {
			runs = append(runs, run)
		}
	}

	return runs, nil
}

// FindByPlanHash finds a run by its plan hash for duplicate detection.
// Returns nil, nil if no run exists with that plan hash.
func (r *FileReleaseRunRepository) FindByPlanHash(ctx context.Context, repoRoot string, planHash string) (*domain.ReleaseRun, error) {
	runIDs, err := r.List(ctx, repoRoot)
	if err != nil {
		return nil, err
	}

	for _, runID := range runIDs {
		run, err := r.LoadFromRepo(ctx, repoRoot, runID)
		if err != nil {
			continue
		}
		if run.PlanHash() == planHash {
			return run, nil
		}
	}

	return nil, nil // No matching run found
}

// SaveMachineJSON saves the state machine definition JSON for a run.
func (r *FileReleaseRunRepository) SaveMachineJSON(repoRoot string, runID domain.RunID, machineJSON []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := ensureDir(repoRoot); err != nil {
		return err
	}

	path := machinePath(repoRoot, runID)
	return os.WriteFile(path, machineJSON, 0644)
}

// Conversion functions

func toDTO(run *domain.ReleaseRun) *ReleaseRunDTO {
	commits := make([]string, len(run.Commits()))
	for i, c := range run.Commits() {
		commits[i] = string(c)
	}

	steps := make([]StepPlanDTO, len(run.Steps()))
	for i, s := range run.Steps() {
		steps[i] = StepPlanDTO{
			Name:           s.Name,
			Type:           string(s.Type),
			ConfigHash:     s.ConfigHash,
			IdempotencyKey: s.IdempotencyKey,
			PluginName:     s.PluginName,
			Hook:           s.Hook,
			Unsafe:         s.Unsafe,
		}
	}

	stepStatus := make(map[string]StepStatusDTO)
	for name, status := range run.AllStepStatuses() {
		stepStatus[name] = StepStatusDTO{
			State:       string(status.State),
			Attempts:    status.Attempts,
			LastError:   status.LastError,
			StartedAt:   status.StartedAt,
			CompletedAt: status.CompletedAt,
			Output:      status.Output,
		}
	}

	history := make([]TransitionRecordDTO, len(run.History()))
	for i, h := range run.History() {
		history[i] = TransitionRecordDTO{
			At:       h.At,
			From:     string(h.From),
			To:       string(h.To),
			Event:    h.Event,
			Actor:    h.Actor,
			Reason:   h.Reason,
			Metadata: h.Metadata,
		}
	}

	dto := &ReleaseRunDTO{
		ID:             string(run.ID()),
		PlanHash:       run.PlanHash(),
		RepoID:         run.RepoID(),
		RepoRoot:       run.RepoRoot(),
		BaseRef:        run.BaseRef(),
		HeadSHA:        string(run.HeadSHA()),
		Commits:        commits,
		VersionCurrent: run.VersionCurrent().String(),
		VersionNext:    run.VersionNext().String(),
		BumpKind:       string(run.BumpKind()),
		RiskScore:      run.RiskScore(),
		Reasons:        run.Reasons(),
		ActorType:      string(run.ActorType()),
		ActorID:        run.ActorID(),
		TagName:        run.TagName(),
		Steps:          steps,
		StepStatus:     stepStatus,
		State:          string(run.State()),
		History:        history,
		LastError:      run.LastError(),
		ChangesetID:    run.ChangesetID(),
		CreatedAt:      run.CreatedAt(),
		UpdatedAt:      run.UpdatedAt(),
		PublishedAt:    run.PublishedAt(),
	}

	if run.Notes() != nil {
		dto.Notes = &ReleaseNotesDTO{
			Text:           run.Notes().Text,
			AudiencePreset: run.Notes().AudiencePreset,
			TonePreset:     run.Notes().TonePreset,
			Provider:       run.Notes().Provider,
			Model:          run.Notes().Model,
			GeneratedAt:    run.Notes().GeneratedAt,
		}
	}

	if run.Approval() != nil {
		approval := run.Approval()
		dto.Approval = &ApprovalDTO{
			ApprovedBy:    approval.ApprovedBy,
			ApprovedAt:    approval.ApprovedAt,
			AutoApproved:  approval.AutoApproved,
			PlanHash:      approval.PlanHash,
			RiskScore:     approval.RiskScore,
			ApproverType:  string(approval.ApproverType),
			Justification: approval.Justification,
		}
	}

	return dto
}

func fromDTO(dto *ReleaseRunDTO) (*domain.ReleaseRun, error) {
	// Convert commits
	commits := make([]domain.CommitSHA, len(dto.Commits))
	for i, c := range dto.Commits {
		commits[i] = domain.CommitSHA(c)
	}

	// Parse versions
	currentVer, _ := version.Parse(dto.VersionCurrent)
	nextVer, _ := version.Parse(dto.VersionNext)
	bumpKind, _ := domain.ParseBumpKind(dto.BumpKind)

	// Convert thresholds
	thresholds := domain.PolicyThresholds{
		AutoApproveRiskThreshold: dto.Thresholds.AutoApproveRiskThreshold,
		RequireApprovalAbove:     dto.Thresholds.RequireApprovalAbove,
		BlockReleaseAbove:        dto.Thresholds.BlockReleaseAbove,
	}

	// Convert notes
	var notes *domain.ReleaseNotes
	if dto.Notes != nil {
		notes = &domain.ReleaseNotes{
			Text:           dto.Notes.Text,
			AudiencePreset: dto.Notes.AudiencePreset,
			TonePreset:     dto.Notes.TonePreset,
			Provider:       dto.Notes.Provider,
			Model:          dto.Notes.Model,
			GeneratedAt:    dto.Notes.GeneratedAt,
		}
	}

	// Convert approval
	var approval *domain.Approval
	if dto.Approval != nil {
		approval = &domain.Approval{
			ApprovedBy:    dto.Approval.ApprovedBy,
			ApprovedAt:    dto.Approval.ApprovedAt,
			AutoApproved:  dto.Approval.AutoApproved,
			PlanHash:      dto.Approval.PlanHash,
			RiskScore:     dto.Approval.RiskScore,
			ApproverType:  domain.ActorType(dto.Approval.ApproverType),
			Justification: dto.Approval.Justification,
		}
	}

	// Convert steps
	steps := make([]domain.StepPlan, len(dto.Steps))
	for i, s := range dto.Steps {
		steps[i] = domain.StepPlan{
			Name:           s.Name,
			Type:           domain.StepType(s.Type),
			ConfigHash:     s.ConfigHash,
			IdempotencyKey: s.IdempotencyKey,
			PluginName:     s.PluginName,
			Hook:           s.Hook,
			Unsafe:         s.Unsafe,
		}
	}

	// Convert step status
	stepStatus := make(map[string]*domain.StepStatus, len(dto.StepStatus))
	for name, s := range dto.StepStatus {
		stepStatus[name] = &domain.StepStatus{
			State:       domain.StepState(s.State),
			Attempts:    s.Attempts,
			LastError:   s.LastError,
			StartedAt:   s.StartedAt,
			CompletedAt: s.CompletedAt,
			Output:      s.Output,
		}
	}

	// Convert history
	history := make([]domain.TransitionRecord, len(dto.History))
	for i, h := range dto.History {
		history[i] = domain.TransitionRecord{
			At:       h.At,
			From:     domain.RunState(h.From),
			To:       domain.RunState(h.To),
			Event:    h.Event,
			Actor:    h.Actor,
			Reason:   h.Reason,
			Metadata: h.Metadata,
		}
	}

	// Create a new run and use ReconstructState to hydrate it
	run := &domain.ReleaseRun{}
	run.ReconstructState(domain.RunSnapshot{
		ID:              domain.RunID(dto.ID),
		PlanHash:        dto.PlanHash,
		RepoID:          dto.RepoID,
		RepoRoot:        dto.RepoRoot,
		BaseRef:         dto.BaseRef,
		HeadSHA:         domain.CommitSHA(dto.HeadSHA),
		Commits:         commits,
		ConfigHash:      dto.ConfigHash,
		PluginPlanHash:  dto.PluginPlanHash,
		VersionCurrent:  currentVer,
		VersionNext:     nextVer,
		BumpKind:        bumpKind,
		Confidence:      dto.Confidence,
		RiskScore:       dto.RiskScore,
		Reasons:         dto.Reasons,
		ActorType:       domain.ActorType(dto.ActorType),
		ActorID:         dto.ActorID,
		Thresholds:      thresholds,
		TagName:         dto.TagName,
		Notes:           notes,
		NotesInputsHash: dto.NotesInputHash,
		Approval:        approval,
		Steps:           steps,
		StepStatus:      stepStatus,
		State:           domain.RunState(dto.State),
		History:         history,
		LastError:       dto.LastError,
		ChangesetID:     dto.ChangesetID,
		CreatedAt:       dto.CreatedAt,
		UpdatedAt:       dto.UpdatedAt,
		PublishedAt:     dto.PublishedAt,
	})

	return run, nil
}
