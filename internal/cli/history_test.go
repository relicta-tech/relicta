package cli

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/memory"
)

func TestGetOutcomeSymbol(t *testing.T) {
	tests := []struct {
		outcome  memory.ReleaseOutcome
		expected string
	}{
		{memory.OutcomeSuccess, "✓"},
		{memory.OutcomeFailed, "✗"},
		{memory.OutcomeRollback, "↩"},
		{memory.OutcomePartial, "◐"},
		{memory.ReleaseOutcome("unknown"), "?"},
	}

	for _, tt := range tests {
		t.Run(string(tt.outcome), func(t *testing.T) {
			result := getOutcomeSymbol(tt.outcome)
			if result != tt.expected {
				t.Errorf("getOutcomeSymbol(%q) = %q, want %q", tt.outcome, result, tt.expected)
			}
		})
	}
}

func TestGetTrendSymbol(t *testing.T) {
	tests := []struct {
		trend    memory.RiskTrend
		expected string
	}{
		{memory.TrendIncreasing, "↑"},
		{memory.TrendDecreasing, "↓"},
		{memory.TrendStable, "→"},
		{memory.RiskTrend("unknown"), "?"},
	}

	for _, tt := range tests {
		t.Run(string(tt.trend), func(t *testing.T) {
			result := getTrendSymbol(tt.trend)
			if result != tt.expected {
				t.Errorf("getTrendSymbol(%q) = %q, want %q", tt.trend, result, tt.expected)
			}
		})
	}
}

func TestGetReliabilityLabel(t *testing.T) {
	tests := []struct {
		score    float64
		expected string
	}{
		{0.95, "Excellent"},
		{0.9, "Excellent"},
		{0.85, "Very Good"},
		{0.8, "Very Good"},
		{0.75, "Good"},
		{0.7, "Good"},
		{0.65, "Fair"},
		{0.6, "Fair"},
		{0.55, "Needs Improvement"},
		{0.5, "Needs Improvement"},
		{0.4, "Poor"},
		{0.0, "Poor"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := getReliabilityLabel(tt.score)
			if result != tt.expected {
				t.Errorf("getReliabilityLabel(%.2f) = %q, want %q", tt.score, result, tt.expected)
			}
		})
	}
}

func TestCalculateReleaseStats(t *testing.T) {
	tests := []struct {
		name     string
		releases []*memory.ReleaseRecord
		expected releaseStats
	}{
		{
			name:     "empty releases",
			releases: []*memory.ReleaseRecord{},
			expected: releaseStats{total: 0, successful: 0, failed: 0, successRate: 0},
		},
		{
			name: "all successful",
			releases: []*memory.ReleaseRecord{
				{Outcome: memory.OutcomeSuccess},
				{Outcome: memory.OutcomeSuccess},
				{Outcome: memory.OutcomeSuccess},
			},
			expected: releaseStats{total: 3, successful: 3, failed: 0, successRate: 1.0},
		},
		{
			name: "mixed outcomes",
			releases: []*memory.ReleaseRecord{
				{Outcome: memory.OutcomeSuccess},
				{Outcome: memory.OutcomeFailed},
				{Outcome: memory.OutcomeSuccess},
				{Outcome: memory.OutcomeRollback},
			},
			expected: releaseStats{total: 4, successful: 2, failed: 2, successRate: 0.5},
		},
		{
			name: "partial outcomes",
			releases: []*memory.ReleaseRecord{
				{Outcome: memory.OutcomeSuccess},
				{Outcome: memory.OutcomePartial},
			},
			expected: releaseStats{total: 2, successful: 1, failed: 0, successRate: 0.5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateReleaseStats(tt.releases)
			if result.total != tt.expected.total {
				t.Errorf("total = %d, want %d", result.total, tt.expected.total)
			}
			if result.successful != tt.expected.successful {
				t.Errorf("successful = %d, want %d", result.successful, tt.expected.successful)
			}
			if result.failed != tt.expected.failed {
				t.Errorf("failed = %d, want %d", result.failed, tt.expected.failed)
			}
			if result.successRate != tt.expected.successRate {
				t.Errorf("successRate = %.2f, want %.2f", result.successRate, tt.expected.successRate)
			}
		})
	}
}

func TestExtractRepoFromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"git@github.com:owner/repo.git", "owner/repo"},
		{"git@github.com:owner/repo", "owner/repo"},
		{"https://github.com/owner/repo.git", "owner/repo"},
		{"https://github.com/owner/repo", "owner/repo"},
		{"http://github.com/owner/repo.git", "owner/repo"},
		{"git@gitlab.com:org/project.git", "org/project"},
		{"https://gitlab.com/org/project.git", "org/project"},
		{"invalid-url", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := extractRepoFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("extractRepoFromURL(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestHistoryCommand_FlagsExist(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"limit flag", "limit"},
		{"repo flag", "repo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := historyCmd.PersistentFlags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("history command missing %s flag", tt.flagName)
			}
		})
	}
}

func TestHistoryReleasesCommand_FlagsExist(t *testing.T) {
	flag := historyReleasesCmd.Flags().Lookup("risk")
	if flag == nil {
		t.Error("history releases command missing risk flag")
	}
}

func TestHistoryActorCommand_FlagsExist(t *testing.T) {
	flag := historyActorCmd.Flags().Lookup("actor")
	if flag == nil {
		t.Error("history actor command missing actor flag")
	}
}

func TestHistoryCommand_Configuration(t *testing.T) {
	if historyCmd == nil {
		t.Fatal("historyCmd is nil")
	}
	if historyCmd.Use != "history" {
		t.Errorf("historyCmd.Use = %v, want history", historyCmd.Use)
	}
	if historyCmd.RunE == nil {
		t.Error("historyCmd.RunE is nil")
	}
}

func TestHistoryReleasesCommand_Configuration(t *testing.T) {
	if historyReleasesCmd == nil {
		t.Fatal("historyReleasesCmd is nil")
	}
	if historyReleasesCmd.Use != "releases" {
		t.Errorf("historyReleasesCmd.Use = %v, want releases", historyReleasesCmd.Use)
	}
	if historyReleasesCmd.RunE == nil {
		t.Error("historyReleasesCmd.RunE is nil")
	}
}

func TestHistoryActorCommand_Configuration(t *testing.T) {
	if historyActorCmd == nil {
		t.Fatal("historyActorCmd is nil")
	}
	if historyActorCmd.Use != "actor [actor-id]" {
		t.Errorf("historyActorCmd.Use = %v, want actor [actor-id]", historyActorCmd.Use)
	}
	if historyActorCmd.RunE == nil {
		t.Error("historyActorCmd.RunE is nil")
	}
}

func TestHistoryRiskCommand_Configuration(t *testing.T) {
	if historyRiskCmd == nil {
		t.Fatal("historyRiskCmd is nil")
	}
	if historyRiskCmd.Use != "risk" {
		t.Errorf("historyRiskCmd.Use = %v, want risk", historyRiskCmd.Use)
	}
	if historyRiskCmd.RunE == nil {
		t.Error("historyRiskCmd.RunE is nil")
	}
}

func TestPrintJSONOutput(t *testing.T) {
	// Test with a simple struct
	data := struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}{Name: "test", Value: 42}

	// The function prints to stdout, but we can verify it doesn't panic
	err := printJSONOutput(data)
	if err != nil {
		t.Errorf("printJSONOutput() error = %v", err)
	}
}

func TestPrintJSONOutput_Map(t *testing.T) {
	data := map[string]any{
		"key1": "value1",
		"key2": 123,
	}

	err := printJSONOutput(data)
	if err != nil {
		t.Errorf("printJSONOutput() error = %v", err)
	}
}

func TestPrintJSONOutput_Slice(t *testing.T) {
	data := []string{"one", "two", "three"}

	err := printJSONOutput(data)
	if err != nil {
		t.Errorf("printJSONOutput() error = %v", err)
	}
}

func TestGetRepositoryName_NoGitDir(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	result := getRepositoryName()
	if result != "" {
		t.Errorf("getRepositoryName() = %q, want empty string for non-git dir", result)
	}
}

func TestGetMemoryStore_Fallback(t *testing.T) {
	// Test that getMemoryStore returns a store (using fallback paths)
	store, err := getMemoryStore()
	if err != nil {
		t.Logf("getMemoryStore() error = %v (expected in test env)", err)
	}
	if store == nil && err == nil {
		t.Error("getMemoryStore() returned nil store with no error")
	}
}

// historyMockStore is a mock implementation of memory.Store for testing.
type historyMockStore struct {
	releases     []*memory.ReleaseRecord
	actorMetrics *memory.ActorMetrics
	riskPatterns *memory.RiskPatterns
	storeError   error
}

func (m *historyMockStore) RecordRelease(ctx context.Context, record *memory.ReleaseRecord) error {
	return m.storeError
}

func (m *historyMockStore) RecordIncident(ctx context.Context, incident *memory.IncidentRecord) error {
	return m.storeError
}

func (m *historyMockStore) RecordDecision(ctx context.Context, decision *cgp.GovernanceDecision) error {
	return m.storeError
}

func (m *historyMockStore) RecordAuthorization(ctx context.Context, auth *cgp.ExecutionAuthorization) error {
	return m.storeError
}

func (m *historyMockStore) GetReleaseHistory(ctx context.Context, repository string, limit int) ([]*memory.ReleaseRecord, error) {
	if m.storeError != nil {
		return nil, m.storeError
	}
	return m.releases, nil
}

func (m *historyMockStore) GetIncidentHistory(ctx context.Context, repository string, limit int) ([]*memory.IncidentRecord, error) {
	return nil, m.storeError
}

func (m *historyMockStore) GetDecision(ctx context.Context, decisionID string) (*cgp.GovernanceDecision, error) {
	return nil, m.storeError
}

func (m *historyMockStore) GetDecisionsByProposal(ctx context.Context, proposalID string) ([]*cgp.GovernanceDecision, error) {
	return nil, m.storeError
}

func (m *historyMockStore) GetAuthorization(ctx context.Context, authID string) (*cgp.ExecutionAuthorization, error) {
	return nil, m.storeError
}

func (m *historyMockStore) GetAuthorizationsByDecision(ctx context.Context, decisionID string) ([]*cgp.ExecutionAuthorization, error) {
	return nil, m.storeError
}

func (m *historyMockStore) GetActorMetrics(ctx context.Context, actorID string) (*memory.ActorMetrics, error) {
	if m.storeError != nil {
		return nil, m.storeError
	}
	if m.actorMetrics == nil {
		return nil, fmt.Errorf("no metrics found for actor: %s", actorID)
	}
	return m.actorMetrics, nil
}

func (m *historyMockStore) GetRiskPatterns(ctx context.Context, repository string) (*memory.RiskPatterns, error) {
	if m.storeError != nil {
		return nil, m.storeError
	}
	if m.riskPatterns == nil {
		return nil, fmt.Errorf("no releases found for repository: %s", repository)
	}
	return m.riskPatterns, nil
}

func (m *historyMockStore) UpdateActorMetrics(ctx context.Context, actorID string, outcome memory.ReleaseOutcome) error {
	return m.storeError
}

func (m *historyMockStore) GetAuditTrail(ctx context.Context, proposalID string) (*memory.AuditTrail, error) {
	return nil, m.storeError
}

func TestRunHistoryReleases_WithMockStore(t *testing.T) {
	origGetMemoryStoreFunc := getMemoryStoreFunc
	origHistoryRepo := historyRepo
	defer func() {
		getMemoryStoreFunc = origGetMemoryStoreFunc
		historyRepo = origHistoryRepo
	}()

	mockStore := &historyMockStore{
		releases: []*memory.ReleaseRecord{
			{
				ID:         "release-1",
				Repository: "test/repo",
				Version:    "1.0.0",
				Outcome:    memory.OutcomeSuccess,
				ReleasedAt: time.Now(),
				RiskScore:  0.3,
			},
		},
	}

	getMemoryStoreFunc = func() (memory.Store, error) {
		return mockStore, nil
	}
	historyRepo = "test/repo"

	cmd := historyReleasesCmd
	cmd.SetContext(context.Background())
	err := runHistoryReleases(cmd, nil)

	if err != nil {
		t.Errorf("runHistoryReleases() error = %v", err)
	}
}

func TestRunHistoryReleases_EmptyHistory(t *testing.T) {
	origGetMemoryStoreFunc := getMemoryStoreFunc
	origHistoryRepo := historyRepo
	defer func() {
		getMemoryStoreFunc = origGetMemoryStoreFunc
		historyRepo = origHistoryRepo
	}()

	mockStore := &historyMockStore{
		releases: []*memory.ReleaseRecord{},
	}

	getMemoryStoreFunc = func() (memory.Store, error) {
		return mockStore, nil
	}
	historyRepo = "test/repo"

	cmd := historyReleasesCmd
	cmd.SetContext(context.Background())
	err := runHistoryReleases(cmd, nil)

	if err != nil {
		t.Errorf("runHistoryReleases() empty history error = %v", err)
	}
}

func TestRunHistoryReleases_NoRepo(t *testing.T) {
	origGetMemoryStoreFunc := getMemoryStoreFunc
	origHistoryRepo := historyRepo
	defer func() {
		getMemoryStoreFunc = origGetMemoryStoreFunc
		historyRepo = origHistoryRepo
	}()

	mockStore := &historyMockStore{}

	getMemoryStoreFunc = func() (memory.Store, error) {
		return mockStore, nil
	}
	historyRepo = ""

	// Change to a non-git directory
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	cmd := historyReleasesCmd
	cmd.SetContext(context.Background())
	err := runHistoryReleases(cmd, nil)

	if err == nil {
		t.Error("runHistoryReleases() should return error when no repo specified")
	}
}

func TestRunHistoryReleases_StoreError(t *testing.T) {
	origGetMemoryStoreFunc := getMemoryStoreFunc
	origHistoryRepo := historyRepo
	defer func() {
		getMemoryStoreFunc = origGetMemoryStoreFunc
		historyRepo = origHistoryRepo
	}()

	mockStore := &historyMockStore{
		storeError: fmt.Errorf("database error"),
	}

	getMemoryStoreFunc = func() (memory.Store, error) {
		return mockStore, nil
	}
	historyRepo = "test/repo"

	cmd := historyReleasesCmd
	cmd.SetContext(context.Background())
	err := runHistoryReleases(cmd, nil)

	if err == nil {
		t.Error("runHistoryReleases() should return error on store error")
	}
}

func TestRunHistoryReleases_WithJSONOutput(t *testing.T) {
	origGetMemoryStoreFunc := getMemoryStoreFunc
	origHistoryRepo := historyRepo
	origOutputJSON := outputJSON
	defer func() {
		getMemoryStoreFunc = origGetMemoryStoreFunc
		historyRepo = origHistoryRepo
		outputJSON = origOutputJSON
	}()

	mockStore := &historyMockStore{
		releases: []*memory.ReleaseRecord{
			{
				ID:         "release-1",
				Repository: "test/repo",
				Version:    "1.0.0",
				Outcome:    memory.OutcomeSuccess,
				ReleasedAt: time.Now(),
			},
		},
	}

	getMemoryStoreFunc = func() (memory.Store, error) {
		return mockStore, nil
	}
	historyRepo = "test/repo"
	outputJSON = true

	cmd := historyReleasesCmd
	cmd.SetContext(context.Background())
	err := runHistoryReleases(cmd, nil)

	if err != nil {
		t.Errorf("runHistoryReleases() with JSON error = %v", err)
	}
}

func TestRunHistoryReleases_WithVerboseRisk(t *testing.T) {
	origGetMemoryStoreFunc := getMemoryStoreFunc
	origHistoryRepo := historyRepo
	origVerbose := verbose
	origHistoryShowRisk := historyShowRisk
	defer func() {
		getMemoryStoreFunc = origGetMemoryStoreFunc
		historyRepo = origHistoryRepo
		verbose = origVerbose
		historyShowRisk = origHistoryShowRisk
	}()

	mockStore := &historyMockStore{
		releases: []*memory.ReleaseRecord{
			{
				ID:              "release-1",
				Repository:      "test/repo",
				Version:         "1.0.0",
				Outcome:         memory.OutcomeSuccess,
				ReleasedAt:      time.Now(),
				RiskScore:       0.5,
				FilesChanged:    10,
				LinesChanged:    100,
				BreakingChanges: 1,
				Tags:            []string{"release", "major"},
			},
		},
	}

	getMemoryStoreFunc = func() (memory.Store, error) {
		return mockStore, nil
	}
	historyRepo = "test/repo"
	verbose = true
	historyShowRisk = true

	cmd := historyReleasesCmd
	cmd.SetContext(context.Background())
	err := runHistoryReleases(cmd, nil)

	if err != nil {
		t.Errorf("runHistoryReleases() with verbose/risk error = %v", err)
	}
}

func TestRunHistoryActor_WithMockStore(t *testing.T) {
	origGetMemoryStoreFunc := getMemoryStoreFunc
	origHistoryActorID := historyActorID
	defer func() {
		getMemoryStoreFunc = origGetMemoryStoreFunc
		historyActorID = origHistoryActorID
	}()

	now := time.Now()
	mockStore := &historyMockStore{
		actorMetrics: &memory.ActorMetrics{
			ActorID:                 "human:dev",
			TotalReleases:           10,
			SuccessfulReleases:      8,
			FailedReleases:          2,
			RollbackCount:           1,
			IncidentCount:           1,
			AverageRiskScore:        0.3,
			HighRiskReleases:        2,
			BreakingChangeReleases:  1,
			SuccessRate:             0.8,
			ReliabilityScore:        0.85,
			FirstReleaseAt:          &now,
			LastReleaseAt:           &now,
		},
	}

	getMemoryStoreFunc = func() (memory.Store, error) {
		return mockStore, nil
	}
	historyActorID = "human:dev"

	cmd := historyActorCmd
	cmd.SetContext(context.Background())
	err := runHistoryActor(cmd, nil)

	if err != nil {
		t.Errorf("runHistoryActor() error = %v", err)
	}
}

func TestRunHistoryActor_NoActorID(t *testing.T) {
	origGetMemoryStoreFunc := getMemoryStoreFunc
	origHistoryActorID := historyActorID
	defer func() {
		getMemoryStoreFunc = origGetMemoryStoreFunc
		historyActorID = origHistoryActorID
	}()

	mockStore := &historyMockStore{}

	getMemoryStoreFunc = func() (memory.Store, error) {
		return mockStore, nil
	}
	historyActorID = ""

	cmd := historyActorCmd
	cmd.SetContext(context.Background())
	err := runHistoryActor(cmd, []string{})

	if err == nil {
		t.Error("runHistoryActor() should return error when no actor ID")
	}
}

func TestRunHistoryActor_WithArg(t *testing.T) {
	origGetMemoryStoreFunc := getMemoryStoreFunc
	origHistoryActorID := historyActorID
	defer func() {
		getMemoryStoreFunc = origGetMemoryStoreFunc
		historyActorID = origHistoryActorID
	}()

	mockStore := &historyMockStore{
		actorMetrics: &memory.ActorMetrics{
			ActorID:          "agent:copilot",
			TotalReleases:    5,
			SuccessfulReleases: 5,
			SuccessRate:      1.0,
			ReliabilityScore: 0.9,
		},
	}

	getMemoryStoreFunc = func() (memory.Store, error) {
		return mockStore, nil
	}
	historyActorID = ""

	cmd := historyActorCmd
	cmd.SetContext(context.Background())
	err := runHistoryActor(cmd, []string{"agent:copilot"})

	if err != nil {
		t.Errorf("runHistoryActor() with arg error = %v", err)
	}
}

func TestRunHistoryActor_WithJSONOutput(t *testing.T) {
	origGetMemoryStoreFunc := getMemoryStoreFunc
	origHistoryActorID := historyActorID
	origOutputJSON := outputJSON
	defer func() {
		getMemoryStoreFunc = origGetMemoryStoreFunc
		historyActorID = origHistoryActorID
		outputJSON = origOutputJSON
	}()

	mockStore := &historyMockStore{
		actorMetrics: &memory.ActorMetrics{
			ActorID:       "human:test",
			TotalReleases: 1,
		},
	}

	getMemoryStoreFunc = func() (memory.Store, error) {
		return mockStore, nil
	}
	historyActorID = "human:test"
	outputJSON = true

	cmd := historyActorCmd
	cmd.SetContext(context.Background())
	err := runHistoryActor(cmd, nil)

	if err != nil {
		t.Errorf("runHistoryActor() with JSON error = %v", err)
	}
}

func TestRunHistoryRisk_WithMockStore(t *testing.T) {
	origGetMemoryStoreFunc := getMemoryStoreFunc
	origHistoryRepo := historyRepo
	defer func() {
		getMemoryStoreFunc = origGetMemoryStoreFunc
		historyRepo = origHistoryRepo
	}()

	mockStore := &historyMockStore{
		riskPatterns: &memory.RiskPatterns{
			Repository:       "test/repo",
			AverageRiskScore: 0.4,
			RiskTrend:        memory.TrendDecreasing,
			TotalReleases:    10,
			CommonRiskFactors: []memory.RiskFactorPattern{
				{Category: "breaking", Frequency: 0.2, CorrelatedIncidents: 1},
			},
			IncidentCorrelations: []memory.IncidentCorrelation{
				{Pattern: "large change", IncidentProbability: 0.15, SampleSize: 5},
			},
		},
	}

	getMemoryStoreFunc = func() (memory.Store, error) {
		return mockStore, nil
	}
	historyRepo = "test/repo"

	cmd := historyRiskCmd
	cmd.SetContext(context.Background())
	err := runHistoryRisk(cmd, nil)

	if err != nil {
		t.Errorf("runHistoryRisk() error = %v", err)
	}
}

func TestRunHistoryRisk_NoRepo(t *testing.T) {
	origGetMemoryStoreFunc := getMemoryStoreFunc
	origHistoryRepo := historyRepo
	defer func() {
		getMemoryStoreFunc = origGetMemoryStoreFunc
		historyRepo = origHistoryRepo
	}()

	mockStore := &historyMockStore{}

	getMemoryStoreFunc = func() (memory.Store, error) {
		return mockStore, nil
	}
	historyRepo = ""

	// Change to a non-git directory
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	cmd := historyRiskCmd
	cmd.SetContext(context.Background())
	err := runHistoryRisk(cmd, nil)

	if err == nil {
		t.Error("runHistoryRisk() should return error when no repo specified")
	}
}

func TestRunHistoryRisk_WithJSONOutput(t *testing.T) {
	origGetMemoryStoreFunc := getMemoryStoreFunc
	origHistoryRepo := historyRepo
	origOutputJSON := outputJSON
	defer func() {
		getMemoryStoreFunc = origGetMemoryStoreFunc
		historyRepo = origHistoryRepo
		outputJSON = origOutputJSON
	}()

	mockStore := &historyMockStore{
		riskPatterns: &memory.RiskPatterns{
			Repository:       "test/repo",
			AverageRiskScore: 0.3,
			RiskTrend:        memory.TrendStable,
			TotalReleases:    5,
		},
	}

	getMemoryStoreFunc = func() (memory.Store, error) {
		return mockStore, nil
	}
	historyRepo = "test/repo"
	outputJSON = true

	cmd := historyRiskCmd
	cmd.SetContext(context.Background())
	err := runHistoryRisk(cmd, nil)

	if err != nil {
		t.Errorf("runHistoryRisk() with JSON error = %v", err)
	}
}

func TestRunHistory_CallsReleases(t *testing.T) {
	origGetMemoryStoreFunc := getMemoryStoreFunc
	origHistoryRepo := historyRepo
	defer func() {
		getMemoryStoreFunc = origGetMemoryStoreFunc
		historyRepo = origHistoryRepo
	}()

	mockStore := &historyMockStore{
		releases: []*memory.ReleaseRecord{},
	}

	getMemoryStoreFunc = func() (memory.Store, error) {
		return mockStore, nil
	}
	historyRepo = "test/repo"

	cmd := historyCmd
	cmd.SetContext(context.Background())
	err := runHistory(cmd, nil)

	if err != nil {
		t.Errorf("runHistory() error = %v", err)
	}
}

func TestGetMemoryStore_InitError(t *testing.T) {
	origGetMemoryStoreFunc := getMemoryStoreFunc
	defer func() {
		getMemoryStoreFunc = origGetMemoryStoreFunc
	}()

	getMemoryStoreFunc = func() (memory.Store, error) {
		return nil, fmt.Errorf("failed to initialize store")
	}

	cmd := historyReleasesCmd
	cmd.SetContext(context.Background())
	historyRepo = "test/repo"
	err := runHistoryReleases(cmd, nil)

	if err == nil {
		t.Error("runHistoryReleases() should return error when store init fails")
	}
}

func TestGetRepositoryName_InvalidConfigFormat(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	// Create a .git directory with an empty config
	gitDir := tmpDir + "/.git"
	_ = os.MkdirAll(gitDir, 0o755)

	// Write a config without remote origin section
	configContent := `[core]
repositoryformatversion = 0
`
	_ = os.WriteFile(gitDir+"/config", []byte(configContent), 0o644)

	// Since there's no remote origin, should return empty string
	result := getRepositoryName()
	if result != "" {
		t.Errorf("getRepositoryName() = %q, want empty string", result)
	}
}

func TestGetRepositoryName_UnreadableConfig(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	// Create a .git directory but no config file
	gitDir := tmpDir + "/.git"
	_ = os.MkdirAll(gitDir, 0o755)

	// No config file means getRepositoryName should return empty string
	result := getRepositoryName()
	if result != "" {
		t.Errorf("getRepositoryName() = %q, want empty string", result)
	}
}

func TestRunHistoryActor_StoreError(t *testing.T) {
	origGetMemoryStoreFunc := getMemoryStoreFunc
	origHistoryActorID := historyActorID
	defer func() {
		getMemoryStoreFunc = origGetMemoryStoreFunc
		historyActorID = origHistoryActorID
	}()

	mockStore := &historyMockStore{
		storeError: fmt.Errorf("database error"),
	}

	getMemoryStoreFunc = func() (memory.Store, error) {
		return mockStore, nil
	}
	historyActorID = "human:test"

	cmd := historyActorCmd
	cmd.SetContext(context.Background())
	err := runHistoryActor(cmd, nil)

	if err == nil {
		t.Error("runHistoryActor() should return error on store error")
	}
}

func TestRunHistoryRisk_StoreError(t *testing.T) {
	origGetMemoryStoreFunc := getMemoryStoreFunc
	origHistoryRepo := historyRepo
	defer func() {
		getMemoryStoreFunc = origGetMemoryStoreFunc
		historyRepo = origHistoryRepo
	}()

	mockStore := &historyMockStore{
		storeError: fmt.Errorf("database error"),
	}

	getMemoryStoreFunc = func() (memory.Store, error) {
		return mockStore, nil
	}
	historyRepo = "test/repo"

	cmd := historyRiskCmd
	cmd.SetContext(context.Background())
	err := runHistoryRisk(cmd, nil)

	if err == nil {
		t.Error("runHistoryRisk() should return error on store error")
	}
}
