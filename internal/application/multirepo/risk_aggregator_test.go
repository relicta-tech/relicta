package multirepo

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/multirepo"
)

func newTestGroups() []multirepo.RepositoryGroup {
	return []multirepo.RepositoryGroup{
		{
			Name:     "platform",
			Strategy: multirepo.StrategyCoordinated,
			Repositories: []multirepo.RepoConfig{
				{Name: "core-lib"},
				{Name: "auth-service", Dependencies: []string{"core-lib"}},
				{Name: "api-gateway", Dependencies: []string{"auth-service"}},
			},
		},
	}
}

func seedRelease(t *testing.T, store memory.Store, repo, version string, risk float64, releasedAt time.Time) {
	t.Helper()
	err := store.RecordRelease(context.Background(), &memory.ReleaseRecord{
		ID:         "rel-" + repo + "-" + version,
		Repository: repo,
		Version:    version,
		Actor:      cgp.Actor{ID: "test-actor", Kind: cgp.ActorKindHuman},
		RiskScore:  risk,
		Decision:   cgp.DecisionApproved,
		Outcome:    memory.OutcomeSuccess,
		ReleasedAt: releasedAt,
	})
	if err != nil {
		t.Fatalf("failed to seed release for %s: %v", repo, err)
	}
}

func TestAggregateRisk_SingleRepo(t *testing.T) {
	groups := newTestGroups()
	store := memory.NewInMemoryStore()
	agg := NewOrgRiskAggregator(groups, store)

	entries := []RepoRiskEntry{
		{Repository: "core-lib", RiskScore: 0.45, State: "releasing"},
	}

	got := agg.AggregateRisk(entries)
	want := 0.45

	if got != want {
		t.Errorf("AggregateRisk() = %v, want %v", got, want)
	}
}

func TestAggregateRisk_NoRepos(t *testing.T) {
	groups := newTestGroups()
	store := memory.NewInMemoryStore()
	agg := NewOrgRiskAggregator(groups, store)

	got := agg.AggregateRisk(nil)
	if got != 0.0 {
		t.Errorf("AggregateRisk(nil) = %v, want 0.0", got)
	}
}

func TestAggregateRisk_ConcurrentPenalty(t *testing.T) {
	groups := newTestGroups()
	store := memory.NewInMemoryStore()
	agg := NewOrgRiskAggregator(groups, store)

	// Three concurrent high-risk releases: penalty = (3-1) * 0.05 = 0.10
	entries := []RepoRiskEntry{
		{Repository: "core-lib", RiskScore: 0.5, State: "releasing"},
		{Repository: "auth-service", RiskScore: 0.4, State: "releasing"},
		{Repository: "api-gateway", RiskScore: 0.6, State: "releasing"},
	}

	got := agg.AggregateRisk(entries)
	// max=0.6, concurrentPenalty=0.10, no dep penalty (deps not set here)
	want := 0.7

	if got != want {
		t.Errorf("AggregateRisk() = %v, want %v", got, want)
	}
}

func TestAggregateRisk_DependencyPenalty(t *testing.T) {
	groups := newTestGroups()
	store := memory.NewInMemoryStore()
	agg := NewOrgRiskAggregator(groups, store)

	// core-lib and auth-service (which depends on core-lib) are both releasing.
	entries := []RepoRiskEntry{
		{Repository: "core-lib", RiskScore: 0.5, State: "releasing"},
		{Repository: "auth-service", RiskScore: 0.4, State: "releasing", Dependencies: []string{"core-lib"}},
	}

	got := agg.AggregateRisk(entries)
	// max=0.5, concurrentPenalty=(2-1)*0.05=0.05, depPenalty=0.1
	want := 0.65

	if got != want {
		t.Errorf("AggregateRisk() = %v, want %v", got, want)
	}
}

func TestAggregateRisk_ClampsToOne(t *testing.T) {
	groups := newTestGroups()
	store := memory.NewInMemoryStore()
	agg := NewOrgRiskAggregator(groups, store)

	// High base risk + many concurrent + dependency = should clamp to 1.0
	entries := []RepoRiskEntry{
		{Repository: "a", RiskScore: 0.9, State: "releasing"},
		{Repository: "b", RiskScore: 0.8, State: "releasing", Dependencies: []string{"a"}},
		{Repository: "c", RiskScore: 0.7, State: "releasing", Dependencies: []string{"a"}},
		{Repository: "d", RiskScore: 0.6, State: "releasing", Dependencies: []string{"b"}},
	}

	got := agg.AggregateRisk(entries)
	if got != 1.0 {
		t.Errorf("AggregateRisk() = %v, want 1.0 (clamped)", got)
	}
}

func TestAggregateRisk_NoPenaltyForLowRisk(t *testing.T) {
	groups := newTestGroups()
	store := memory.NewInMemoryStore()
	agg := NewOrgRiskAggregator(groups, store)

	// Two concurrent releases both below 0.3 threshold: no concurrent penalty.
	entries := []RepoRiskEntry{
		{Repository: "core-lib", RiskScore: 0.2, State: "releasing"},
		{Repository: "auth-service", RiskScore: 0.1, State: "releasing"},
	}

	got := agg.AggregateRisk(entries)
	want := 0.2 // just max, no penalties

	if got != want {
		t.Errorf("AggregateRisk() = %v, want %v", got, want)
	}
}

func TestAggregateRisk_ReleasedReposNotActive(t *testing.T) {
	groups := newTestGroups()
	store := memory.NewInMemoryStore()
	agg := NewOrgRiskAggregator(groups, store)

	// Already-released repos should not count for concurrent penalty.
	entries := []RepoRiskEntry{
		{Repository: "core-lib", RiskScore: 0.5, State: "released"},
		{Repository: "auth-service", RiskScore: 0.4, State: "released"},
	}

	got := agg.AggregateRisk(entries)
	want := 0.5 // max only, no penalties since state is "released"

	if got != want {
		t.Errorf("AggregateRisk() = %v, want %v", got, want)
	}
}

func TestCheckOrgBudget_Allowed(t *testing.T) {
	groups := newTestGroups()
	store := memory.NewInMemoryStore()
	agg := NewOrgRiskAggregator(groups, store)

	budget := &config.RiskBudgetConfig{
		WeeklyLimit:     2.0,
		ConcurrentLimit: 5,
	}

	newRelease := RepoRiskEntry{
		Repository: "core-lib",
		RiskScore:  0.3,
		State:      "planned",
	}

	allowed, reason := agg.CheckOrgBudget(context.Background(), newRelease, budget)
	if !allowed {
		t.Errorf("CheckOrgBudget() should be allowed, got reason: %s", reason)
	}
}

func TestCheckOrgBudget_ExceedsConcurrentLimit(t *testing.T) {
	groups := newTestGroups()
	store := memory.NewInMemoryStore()

	now := time.Now().UTC()
	// Seed two very recent releases so they appear active.
	seedRelease(t, store, "core-lib", "v1.0.0", 0.3, now.Add(-30*time.Minute))
	seedRelease(t, store, "auth-service", "v2.0.0", 0.4, now.Add(-15*time.Minute))

	agg := NewOrgRiskAggregator(groups, store)

	budget := &config.RiskBudgetConfig{
		ConcurrentLimit: 2, // Two already active + one new = 3, which exceeds 2.
	}

	newRelease := RepoRiskEntry{
		Repository: "api-gateway",
		RiskScore:  0.5,
		State:      "planned",
	}

	allowed, reason := agg.CheckOrgBudget(context.Background(), newRelease, budget)
	if allowed {
		t.Errorf("CheckOrgBudget() should be rejected for exceeding concurrent limit")
	}
	if reason == "" {
		t.Error("reason should not be empty when rejected")
	}
}

func TestCheckOrgBudget_NilBudgetAllows(t *testing.T) {
	groups := newTestGroups()
	store := memory.NewInMemoryStore()
	agg := NewOrgRiskAggregator(groups, store)

	newRelease := RepoRiskEntry{
		Repository: "core-lib",
		RiskScore:  0.9,
		State:      "planned",
	}

	allowed, _ := agg.CheckOrgBudget(context.Background(), newRelease, nil)
	if !allowed {
		t.Error("CheckOrgBudget() with nil budget should always allow")
	}
}

func TestSnapshot_NoActiveReleases(t *testing.T) {
	groups := newTestGroups()
	store := memory.NewInMemoryStore()
	agg := NewOrgRiskAggregator(groups, store)

	snapshot, err := agg.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	if snapshot.TotalRepos != 3 {
		t.Errorf("TotalRepos = %d, want 3", snapshot.TotalRepos)
	}
	if snapshot.ActiveReleases != 0 {
		t.Errorf("ActiveReleases = %d, want 0", snapshot.ActiveReleases)
	}
	if snapshot.AggregateRisk != 0.0 {
		t.Errorf("AggregateRisk = %v, want 0.0", snapshot.AggregateRisk)
	}
}

func TestSnapshot_MixedStates(t *testing.T) {
	groups := newTestGroups()
	store := memory.NewInMemoryStore()

	now := time.Now().UTC()

	// One recent (active) release.
	seedRelease(t, store, "core-lib", "v1.0.0", 0.5, now.Add(-10*time.Minute))
	// One old (released) release.
	seedRelease(t, store, "auth-service", "v2.0.0", 0.3, now.Add(-24*time.Hour))
	// api-gateway has no releases.

	agg := NewOrgRiskAggregator(groups, store)

	snapshot, err := agg.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	if snapshot.TotalRepos != 3 {
		t.Errorf("TotalRepos = %d, want 3", snapshot.TotalRepos)
	}

	if snapshot.ActiveReleases != 1 {
		t.Errorf("ActiveReleases = %d, want 1", snapshot.ActiveReleases)
	}

	if len(snapshot.RepoRisks) != 2 {
		t.Errorf("len(RepoRisks) = %d, want 2", len(snapshot.RepoRisks))
	}

	// Aggregate should be max of 0.5 and 0.3 = 0.5 (no concurrent penalty with only 1 active).
	if snapshot.AggregateRisk != 0.5 {
		t.Errorf("AggregateRisk = %v, want 0.5", snapshot.AggregateRisk)
	}
}

func TestCheckOrgBudget_ExceedsWeeklyLimit(t *testing.T) {
	groups := newTestGroups()
	store := memory.NewInMemoryStore()
	agg := NewOrgRiskAggregator(groups, store)

	budget := &config.RiskBudgetConfig{
		WeeklyLimit: 0.5,
	}

	// A high-risk release that by itself exceeds the weekly limit.
	newRelease := RepoRiskEntry{
		Repository: "core-lib",
		RiskScore:  0.8,
		State:      "planned",
	}

	allowed, reason := agg.CheckOrgBudget(context.Background(), newRelease, budget)
	if allowed {
		t.Errorf("CheckOrgBudget() should reject when projected risk exceeds weekly limit")
	}
	if reason == "" {
		t.Error("reason should not be empty when rejected")
	}
}

func TestIsActiveState(t *testing.T) {
	tests := []struct {
		state string
		want  bool
	}{
		{"planned", true},
		{"releasing", true},
		{"planning", true},
		{"released", false},
		{"failed", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isActiveState(tt.state); got != tt.want {
			t.Errorf("isActiveState(%q) = %v, want %v", tt.state, got, tt.want)
		}
	}
}

func TestDepPairKey_Deterministic(t *testing.T) {
	if depPairKey("a", "b") != depPairKey("b", "a") {
		t.Error("depPairKey should be order-independent")
	}
}
