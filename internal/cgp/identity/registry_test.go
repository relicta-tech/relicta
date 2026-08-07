package identity

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

// inMemoryStore is a simple in-memory RegistryStore for testing.
type inMemoryStore struct {
	mu     sync.Mutex
	actors map[string]*ActorIdentity
}

func newInMemoryStore() *inMemoryStore {
	return &inMemoryStore{actors: make(map[string]*ActorIdentity)}
}

func (s *inMemoryStore) LoadAll(ctx context.Context) ([]*ActorIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*ActorIdentity, 0, len(s.actors))
	for _, a := range s.actors {
		result = append(result, a)
	}
	return result, nil
}

func (s *inMemoryStore) Save(ctx context.Context, actor *ActorIdentity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.actors[actor.ID] = actor
	return nil
}

func (s *inMemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.actors, id)
	return nil
}

func testIdentity(id string, kind cgp.ActorKind, team string) *ActorIdentity {
	return &ActorIdentity{
		ID:           id,
		Kind:         kind,
		Organization: "relicta-tech",
		Team:         team,
		TrustScore:   0.5,
		Capabilities: []Capability{
			{Action: "plan", Scope: "all"},
		},
	}
}

func TestNewRegistry_NilStore(t *testing.T) {
	_, err := NewRegistry(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil store")
	}
}

func TestNewRegistry_LoadsExisting(t *testing.T) {
	store := newInMemoryStore()
	ctx := context.Background()
	_ = store.Save(ctx, testIdentity("alice@team-a", cgp.ActorKindHuman, "team-a"))

	reg, err := NewRegistry(context.Background(), store)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	actor, err := reg.Get(ctx, "alice@team-a")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if actor.ID != "alice@team-a" {
		t.Errorf("ID = %q, want %q", actor.ID, "alice@team-a")
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	store := newInMemoryStore()
	reg, err := NewRegistry(context.Background(), store)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	ctx := context.Background()
	identity := testIdentity("claude@team-platform", cgp.ActorKindAgent, "team-platform")

	if err := reg.Register(ctx, identity); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got, err := reg.Get(ctx, "claude@team-platform")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.ID != identity.ID {
		t.Errorf("ID = %q, want %q", got.ID, identity.ID)
	}
	if got.Kind != identity.Kind {
		t.Errorf("Kind = %q, want %q", got.Kind, identity.Kind)
	}
	if got.Team != identity.Team {
		t.Errorf("Team = %q, want %q", got.Team, identity.Team)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestRegistry_RegisterUpdate_PreservesCreatedAt(t *testing.T) {
	store := newInMemoryStore()
	reg, err := NewRegistry(context.Background(), store)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	ctx := context.Background()
	identity := testIdentity("bot@ci", cgp.ActorKindCI, "ci")

	if err := reg.Register(ctx, identity); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	first, _ := reg.Get(ctx, "bot@ci")
	createdAt := first.CreatedAt

	// Update the identity.
	identity.TrustScore = 0.9
	if err := reg.Register(ctx, identity); err != nil {
		t.Fatalf("Register() update error = %v", err)
	}

	updated, _ := reg.Get(ctx, "bot@ci")
	if !updated.CreatedAt.Equal(createdAt) {
		t.Errorf("CreatedAt changed: got %v, want %v", updated.CreatedAt, createdAt)
	}
	if updated.TrustScore != 0.9 {
		t.Errorf("TrustScore = %v, want 0.9", updated.TrustScore)
	}
}

func TestRegistry_RegisterValidation(t *testing.T) {
	store := newInMemoryStore()
	reg, err := NewRegistry(context.Background(), store)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name     string
		identity *ActorIdentity
		wantErr  bool
	}{
		{
			name:     "nil identity",
			identity: nil,
			wantErr:  true,
		},
		{
			name: "empty ID",
			identity: &ActorIdentity{
				Kind:         cgp.ActorKindHuman,
				Organization: "org",
			},
			wantErr: true,
		},
		{
			name: "invalid kind",
			identity: &ActorIdentity{
				ID:           "test@team",
				Kind:         "invalid",
				Organization: "org",
			},
			wantErr: true,
		},
		{
			name: "empty organization",
			identity: &ActorIdentity{
				ID:   "test@team",
				Kind: cgp.ActorKindHuman,
			},
			wantErr: true,
		},
		{
			name: "trust score out of range",
			identity: &ActorIdentity{
				ID:           "test@team",
				Kind:         cgp.ActorKindHuman,
				Organization: "org",
				TrustScore:   1.5,
			},
			wantErr: true,
		},
		{
			name: "valid identity",
			identity: &ActorIdentity{
				ID:           "test@team",
				Kind:         cgp.ActorKindHuman,
				Organization: "org",
				TrustScore:   0.8,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reg.Register(ctx, tt.identity)
			if (err != nil) != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	store := newInMemoryStore()
	reg, err := NewRegistry(context.Background(), store)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	_, err = reg.Get(context.Background(), "nonexistent@team")
	if err == nil {
		t.Fatal("expected error for nonexistent actor")
	}
}

func TestRegistry_GetByTeam(t *testing.T) {
	store := newInMemoryStore()
	reg, err := NewRegistry(context.Background(), store)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	ctx := context.Background()

	_ = reg.Register(ctx, testIdentity("alice@platform", cgp.ActorKindHuman, "platform"))
	_ = reg.Register(ctx, testIdentity("bob@platform", cgp.ActorKindHuman, "platform"))
	_ = reg.Register(ctx, testIdentity("charlie@infra", cgp.ActorKindHuman, "infra"))

	members, err := reg.GetByTeam(ctx, "platform")
	if err != nil {
		t.Fatalf("GetByTeam() error = %v", err)
	}

	if len(members) != 2 {
		t.Errorf("len(members) = %d, want 2", len(members))
	}

	// Empty team returns empty slice.
	empty, err := reg.GetByTeam(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetByTeam() error = %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("len(empty) = %d, want 0", len(empty))
	}
}

func TestRegistry_List(t *testing.T) {
	store := newInMemoryStore()
	reg, err := NewRegistry(context.Background(), store)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	ctx := context.Background()

	// Empty list.
	actors, err := reg.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(actors) != 0 {
		t.Errorf("len(actors) = %d, want 0", len(actors))
	}

	// Add actors.
	_ = reg.Register(ctx, testIdentity("a@team", cgp.ActorKindHuman, "team"))
	_ = reg.Register(ctx, testIdentity("b@team", cgp.ActorKindAgent, "team"))

	actors, err = reg.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(actors) != 2 {
		t.Errorf("len(actors) = %d, want 2", len(actors))
	}
}

func TestRegistry_UpdateTrust(t *testing.T) {
	store := newInMemoryStore()
	reg, err := NewRegistry(context.Background(), store)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	ctx := context.Background()
	identity := testIdentity("agent@team", cgp.ActorKindAgent, "team")
	identity.TrustScore = 0.3
	_ = reg.Register(ctx, identity)

	now := time.Now()
	metrics := memory.ActorMetrics{
		ActorID:            "agent@team",
		ActorKind:          cgp.ActorKindAgent,
		TotalReleases:      20,
		SuccessfulReleases: 18,
		FailedReleases:     1,
		RollbackCount:      1,
		IncidentCount:      0,
		AverageRiskScore:   0.2,
		SuccessRate:        0.9,
		LastReleaseAt:      &now,
	}

	if err := reg.UpdateTrust(ctx, "agent@team", metrics); err != nil {
		t.Fatalf("UpdateTrust() error = %v", err)
	}

	actor, _ := reg.Get(ctx, "agent@team")
	if actor.TrustScore <= 0.3 {
		t.Errorf("TrustScore should have increased, got %.4f", actor.TrustScore)
	}
	if actor.TrustScore > 1.0 || actor.TrustScore < 0.0 {
		t.Errorf("TrustScore out of range: %.4f", actor.TrustScore)
	}
}

func TestRegistry_UpdateTrust_NotFound(t *testing.T) {
	store := newInMemoryStore()
	reg, err := NewRegistry(context.Background(), store)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	err = reg.UpdateTrust(context.Background(), "nonexistent@team", memory.ActorMetrics{})
	if err == nil {
		t.Fatal("expected error for nonexistent actor")
	}
}

func TestRegistry_UpdateTrust_RecencyDecay(t *testing.T) {
	store := newInMemoryStore()
	reg, err := NewRegistry(context.Background(), store)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	ctx := context.Background()
	_ = reg.Register(ctx, testIdentity("decay@team", cgp.ActorKindAgent, "team"))

	now := time.Now()
	recentMetrics := memory.ActorMetrics{
		TotalReleases:      10,
		SuccessfulReleases: 9,
		FailedReleases:     0,
		RollbackCount:      1,
		IncidentCount:      0,
		AverageRiskScore:   0.1,
		SuccessRate:        0.9,
		LastReleaseAt:      &now,
	}

	_ = reg.UpdateTrust(ctx, "decay@team", recentMetrics)
	recentActor, _ := reg.Get(ctx, "decay@team")

	// Same metrics but last release was 365 days ago.
	staleTime := now.Add(-365 * 24 * time.Hour)
	staleMetrics := recentMetrics
	staleMetrics.LastReleaseAt = &staleTime

	_ = reg.Register(ctx, testIdentity("stale@team", cgp.ActorKindAgent, "team"))
	_ = reg.UpdateTrust(ctx, "stale@team", staleMetrics)
	staleActor, _ := reg.Get(ctx, "stale@team")

	// Recent actor should have a higher trust score due to recency weighting.
	if recentActor.TrustScore <= staleActor.TrustScore {
		t.Errorf("recent trust (%.4f) should be > stale trust (%.4f)",
			recentActor.TrustScore, staleActor.TrustScore)
	}
}

func TestRegistry_CheckCapability(t *testing.T) {
	tests := []struct {
		name         string
		capabilities []Capability
		action       string
		scope        string
		riskScore    float64
		wantAllowed  bool
	}{
		{
			name: "exact match allowed",
			capabilities: []Capability{
				{Action: "bump", Scope: "patch"},
			},
			action:      "bump",
			scope:       "patch",
			riskScore:   0.1,
			wantAllowed: true,
		},
		{
			name: "scope all matches any",
			capabilities: []Capability{
				{Action: "plan", Scope: "all"},
			},
			action:      "plan",
			scope:       "major",
			riskScore:   0.0,
			wantAllowed: true,
		},
		{
			name: "higher scope covers lower",
			capabilities: []Capability{
				{Action: "bump", Scope: "major"},
			},
			action:      "bump",
			scope:       "patch",
			riskScore:   0.0,
			wantAllowed: true,
		},
		{
			name: "lower scope denies higher",
			capabilities: []Capability{
				{Action: "bump", Scope: "patch"},
			},
			action:      "bump",
			scope:       "major",
			riskScore:   0.0,
			wantAllowed: false,
		},
		{
			name: "condition met",
			capabilities: []Capability{
				{Action: "approve", Scope: "all", Condition: "risk_score < 0.3"},
			},
			action:      "approve",
			scope:       "minor",
			riskScore:   0.1,
			wantAllowed: true,
		},
		{
			name: "condition not met",
			capabilities: []Capability{
				{Action: "approve", Scope: "all", Condition: "risk_score < 0.3"},
			},
			action:      "approve",
			scope:       "minor",
			riskScore:   0.5,
			wantAllowed: false,
		},
		{
			name: "condition less than or equal met",
			capabilities: []Capability{
				{Action: "publish", Scope: "all", Condition: "risk_score <= 0.5"},
			},
			action:      "publish",
			scope:       "minor",
			riskScore:   0.5,
			wantAllowed: true,
		},
		{
			name: "condition greater than",
			capabilities: []Capability{
				{Action: "publish", Scope: "all", Condition: "risk_score > 0.7"},
			},
			action:      "publish",
			scope:       "minor",
			riskScore:   0.8,
			wantAllowed: true,
		},
		{
			name: "wrong action",
			capabilities: []Capability{
				{Action: "plan", Scope: "all"},
			},
			action:      "approve",
			scope:       "all",
			riskScore:   0.0,
			wantAllowed: false,
		},
		{
			name:         "no capabilities",
			capabilities: []Capability{},
			action:       "plan",
			scope:        "all",
			riskScore:    0.0,
			wantAllowed:  false,
		},
		{
			name: "multiple capabilities first matches",
			capabilities: []Capability{
				{Action: "plan", Scope: "all"},
				{Action: "bump", Scope: "patch", Condition: "risk_score < 0.5"},
			},
			action:      "bump",
			scope:       "patch",
			riskScore:   0.2,
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newInMemoryStore()
			reg, _ := NewRegistry(context.Background(), store)
			ctx := context.Background()

			identity := &ActorIdentity{
				ID:           "test@team",
				Kind:         cgp.ActorKindAgent,
				Organization: "org",
				Team:         "team",
				TrustScore:   0.5,
				Capabilities: tt.capabilities,
			}
			_ = reg.Register(ctx, identity)

			allowed, reason := reg.CheckCapability(ctx, "test@team", tt.action, tt.scope, tt.riskScore)
			if allowed != tt.wantAllowed {
				t.Errorf("CheckCapability() = %v, want %v (reason: %s)", allowed, tt.wantAllowed, reason)
			}
			if reason == "" {
				t.Error("reason should not be empty")
			}
		})
	}
}

func TestRegistry_CheckCapability_NotFound(t *testing.T) {
	store := newInMemoryStore()
	reg, _ := NewRegistry(context.Background(), store)

	allowed, reason := reg.CheckCapability(context.Background(), "nobody@team", "plan", "all", 0.0)
	if allowed {
		t.Error("expected denied for nonexistent actor")
	}
	if reason == "" {
		t.Error("reason should not be empty")
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	store := newInMemoryStore()
	reg, err := NewRegistry(context.Background(), store)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	ctx := context.Background()
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	// Concurrent writes.
	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			identity := &ActorIdentity{
				ID:           "concurrent@team",
				Kind:         cgp.ActorKindAgent,
				Organization: "org",
				Team:         "team",
				TrustScore:   float64(n) / float64(goroutines),
				Capabilities: []Capability{{Action: "plan", Scope: "all"}},
			}
			_ = reg.Register(ctx, identity)
		}(i)
	}

	// Concurrent reads.
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, _ = reg.Get(ctx, "concurrent@team")
		}()
	}

	// Concurrent capability checks.
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			reg.CheckCapability(ctx, "concurrent@team", "plan", "all", 0.1)
		}()
	}

	wg.Wait()

	// Verify the actor exists and is consistent.
	actor, err := reg.Get(ctx, "concurrent@team")
	if err != nil {
		t.Fatalf("Get() after concurrent access: %v", err)
	}
	if actor.ID != "concurrent@team" {
		t.Errorf("ID = %q, want %q", actor.ID, "concurrent@team")
	}
}

func TestEvaluateCondition(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		riskScore float64
		want      bool
		wantErr   bool
	}{
		{"less than true", "risk_score < 0.3", 0.1, true, false},
		{"less than false", "risk_score < 0.3", 0.5, false, false},
		{"less than equal true", "risk_score <= 0.5", 0.5, true, false},
		{"less than equal false", "risk_score <= 0.5", 0.6, false, false},
		{"greater than true", "risk_score > 0.7", 0.8, true, false},
		{"greater than false", "risk_score > 0.7", 0.3, false, false},
		{"greater than equal true", "risk_score >= 0.5", 0.5, true, false},
		{"greater than equal false", "risk_score >= 0.5", 0.4, false, false},
		{"equal true", "risk_score == 0.5", 0.5, true, false},
		{"not equal true", "risk_score != 0.5", 0.3, true, false},
		{"unsupported variable", "trust < 0.5", 0.3, false, true},
		{"bad format", "risk_score", 0.3, false, true},
		{"bad value", "risk_score < abc", 0.3, false, true},
		{"bad operator", "risk_score ~ 0.5", 0.3, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evaluateCondition(tt.condition, tt.riskScore)
			if (err != nil) != tt.wantErr {
				t.Errorf("evaluateCondition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("evaluateCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchScope(t *testing.T) {
	tests := []struct {
		name           string
		capScope       string
		requestedScope string
		want           bool
	}{
		{"exact patch", "patch", "patch", true},
		{"exact minor", "minor", "minor", true},
		{"exact major", "major", "major", true},
		{"all covers patch", "all", "patch", true},
		{"all covers major", "all", "major", true},
		{"major covers minor", "major", "minor", true},
		{"major covers patch", "major", "patch", true},
		{"minor covers patch", "minor", "patch", true},
		{"patch denies minor", "patch", "minor", false},
		{"patch denies major", "patch", "major", false},
		{"minor denies major", "minor", "major", false},
		{"case insensitive", "Patch", "patch", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchScope(tt.capScope, tt.requestedScope); got != tt.want {
				t.Errorf("matchScope(%q, %q) = %v, want %v",
					tt.capScope, tt.requestedScope, got, tt.want)
			}
		})
	}
}

func TestCalculateTrustFromMetrics(t *testing.T) {
	t.Run("zero releases returns neutral", func(t *testing.T) {
		m := &memory.ActorMetrics{TotalReleases: 0}
		score := calculateTrustFromMetrics(m)
		if score != 0.5 {
			t.Errorf("score = %v, want 0.5", score)
		}
	})

	t.Run("perfect record returns high trust", func(t *testing.T) {
		now := time.Now()
		m := &memory.ActorMetrics{
			TotalReleases:      100,
			SuccessfulReleases: 100,
			FailedReleases:     0,
			RollbackCount:      0,
			IncidentCount:      0,
			AverageRiskScore:   0.1,
			SuccessRate:        1.0,
			LastReleaseAt:      &now,
		}
		score := calculateTrustFromMetrics(m)
		if score < 0.85 {
			t.Errorf("score = %.4f, want >= 0.85 for perfect record", score)
		}
	})

	t.Run("poor record returns low trust", func(t *testing.T) {
		now := time.Now()
		m := &memory.ActorMetrics{
			TotalReleases:      10,
			SuccessfulReleases: 3,
			FailedReleases:     4,
			RollbackCount:      3,
			IncidentCount:      5,
			AverageRiskScore:   0.8,
			SuccessRate:        0.3,
			LastReleaseAt:      &now,
		}
		score := calculateTrustFromMetrics(m)
		if score > 0.5 {
			t.Errorf("score = %.4f, want <= 0.50 for poor record", score)
		}
	})

	t.Run("score is clamped to valid range", func(t *testing.T) {
		now := time.Now()
		m := &memory.ActorMetrics{
			TotalReleases:      1,
			SuccessfulReleases: 1,
			SuccessRate:        1.0,
			LastReleaseAt:      &now,
		}
		score := calculateTrustFromMetrics(m)
		if score < 0.0 || score > 1.0 {
			t.Errorf("score = %.4f, want in [0.0, 1.0]", score)
		}
	})
}
