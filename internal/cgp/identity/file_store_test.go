package identity

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
)

func TestNewFileStore(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}
	if store == nil {
		t.Fatal("NewFileStore() returned nil")
	}
	if store.basePath != tmpDir {
		t.Errorf("basePath = %q, want %q", store.basePath, tmpDir)
	}
}

func TestNewFileStore_EmptyPath(t *testing.T) {
	_, err := NewFileStore("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestNewFileStore_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "identity", "data")

	store, err := NewFileStore(subDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}
	if store == nil {
		t.Fatal("NewFileStore() returned nil")
	}

	info, err := os.Stat(subDir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("path should be a directory")
	}
}

func TestFileStore_LoadAll_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	ctx := context.Background()
	actors, err := store.LoadAll(ctx)
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}
	if len(actors) != 0 {
		t.Errorf("len(actors) = %d, want 0", len(actors))
	}
}

func TestFileStore_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	actor := &ActorIdentity{
		ID:           "alice@team-platform",
		Kind:         cgp.ActorKindHuman,
		Organization: "relicta-tech",
		Team:         "team-platform",
		TrustScore:   0.85,
		Capabilities: []Capability{
			{Action: "plan", Scope: "all"},
			{Action: "approve", Scope: "minor", Condition: "risk_score < 0.5"},
		},
		Metadata:  map[string]string{"email": "alice@example.com"},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := store.Save(ctx, actor); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists.
	filePath := filepath.Join(tmpDir, defaultActorsFile)
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("file not created: %v", err)
	}

	// Load and verify.
	actors, err := store.LoadAll(ctx)
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}
	if len(actors) != 1 {
		t.Fatalf("len(actors) = %d, want 1", len(actors))
	}

	got := actors[0]
	if got.ID != actor.ID {
		t.Errorf("ID = %q, want %q", got.ID, actor.ID)
	}
	if got.Kind != actor.Kind {
		t.Errorf("Kind = %q, want %q", got.Kind, actor.Kind)
	}
	if got.Organization != actor.Organization {
		t.Errorf("Organization = %q, want %q", got.Organization, actor.Organization)
	}
	if got.Team != actor.Team {
		t.Errorf("Team = %q, want %q", got.Team, actor.Team)
	}
	if got.TrustScore != actor.TrustScore {
		t.Errorf("TrustScore = %v, want %v", got.TrustScore, actor.TrustScore)
	}
	if len(got.Capabilities) != 2 {
		t.Errorf("len(Capabilities) = %d, want 2", len(got.Capabilities))
	}
	if got.Metadata["email"] != "alice@example.com" {
		t.Errorf("Metadata[email] = %q, want %q", got.Metadata["email"], "alice@example.com")
	}
}

func TestFileStore_SaveUpdatesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	ctx := context.Background()

	actor := &ActorIdentity{
		ID:           "bot@ci",
		Kind:         cgp.ActorKindCI,
		Organization: "org",
		TrustScore:   0.5,
	}

	if err := store.Save(ctx, actor); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Update trust score.
	actor.TrustScore = 0.9
	if err := store.Save(ctx, actor); err != nil {
		t.Fatalf("Save() update error = %v", err)
	}

	actors, err := store.LoadAll(ctx)
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}
	if len(actors) != 1 {
		t.Fatalf("len(actors) = %d, want 1 (should update, not duplicate)", len(actors))
	}
	if actors[0].TrustScore != 0.9 {
		t.Errorf("TrustScore = %v, want 0.9", actors[0].TrustScore)
	}
}

func TestFileStore_SaveMultiple(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	ctx := context.Background()

	actors := []*ActorIdentity{
		{ID: "a@team", Kind: cgp.ActorKindHuman, Organization: "org", TrustScore: 0.5},
		{ID: "b@team", Kind: cgp.ActorKindAgent, Organization: "org", TrustScore: 0.7},
		{ID: "c@team", Kind: cgp.ActorKindCI, Organization: "org", TrustScore: 0.3},
	}

	for _, a := range actors {
		if err := store.Save(ctx, a); err != nil {
			t.Fatalf("Save(%s) error = %v", a.ID, err)
		}
	}

	loaded, err := store.LoadAll(ctx)
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}
	if len(loaded) != 3 {
		t.Errorf("len(loaded) = %d, want 3", len(loaded))
	}
}

func TestFileStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	ctx := context.Background()

	_ = store.Save(ctx, &ActorIdentity{ID: "a@team", Kind: cgp.ActorKindHuman, Organization: "org"})
	_ = store.Save(ctx, &ActorIdentity{ID: "b@team", Kind: cgp.ActorKindAgent, Organization: "org"})

	if err := store.Delete(ctx, "a@team"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	actors, err := store.LoadAll(ctx)
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}
	if len(actors) != 1 {
		t.Fatalf("len(actors) = %d, want 1", len(actors))
	}
	if actors[0].ID != "b@team" {
		t.Errorf("remaining actor ID = %q, want %q", actors[0].ID, "b@team")
	}
}

func TestFileStore_DeleteNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	err = store.Delete(context.Background(), "nonexistent@team")
	if err == nil {
		t.Fatal("expected error for deleting nonexistent actor")
	}
}

func TestFileStore_DeleteEmptyID(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	err = store.Delete(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestFileStore_SaveNilActor(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	err = store.Save(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil actor")
	}
}

func TestFileStore_PersistenceRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store and save actors.
	store1, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	ctx := context.Background()
	_ = store1.Save(ctx, &ActorIdentity{
		ID:           "persist@team",
		Kind:         cgp.ActorKindHuman,
		Organization: "org",
		Team:         "team",
		TrustScore:   0.75,
		Capabilities: []Capability{
			{Action: "approve", Scope: "minor", Condition: "risk_score <= 0.5"},
		},
		Metadata: map[string]string{"role": "lead"},
	})

	// Create a new store pointing at the same directory.
	store2, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	actors, err := store2.LoadAll(ctx)
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}
	if len(actors) != 1 {
		t.Fatalf("len(actors) = %d, want 1", len(actors))
	}

	got := actors[0]
	if got.ID != "persist@team" {
		t.Errorf("ID = %q, want %q", got.ID, "persist@team")
	}
	if got.TrustScore != 0.75 {
		t.Errorf("TrustScore = %v, want 0.75", got.TrustScore)
	}
	if len(got.Capabilities) != 1 {
		t.Fatalf("len(Capabilities) = %d, want 1", len(got.Capabilities))
	}
	if got.Capabilities[0].Condition != "risk_score <= 0.5" {
		t.Errorf("Condition = %q, want %q", got.Capabilities[0].Condition, "risk_score <= 0.5")
	}
	if got.Metadata["role"] != "lead" {
		t.Errorf("Metadata[role] = %q, want %q", got.Metadata["role"], "lead")
	}
}

func TestFileStore_ConcurrentSaves(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	ctx := context.Background()
	const goroutines = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			actor := &ActorIdentity{
				ID:           "concurrent@team",
				Kind:         cgp.ActorKindAgent,
				Organization: "org",
				TrustScore:   float64(n) / float64(goroutines),
			}
			_ = store.Save(ctx, actor)
		}(i)
	}

	wg.Wait()

	actors, err := store.LoadAll(ctx)
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}
	// All saves go to the same ID, so there should be exactly 1 actor.
	if len(actors) != 1 {
		t.Errorf("len(actors) = %d, want 1", len(actors))
	}
}

func TestFileStore_RegistryIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	// Use FileStore as RegistryStore with the Registry.
	reg, err := NewRegistry(store)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	ctx := context.Background()
	identity := &ActorIdentity{
		ID:           "integration@team",
		Kind:         cgp.ActorKindAgent,
		Organization: "relicta-tech",
		Team:         "platform",
		TrustScore:   0.6,
		Capabilities: []Capability{
			{Action: "plan", Scope: "all"},
			{Action: "bump", Scope: "patch"},
		},
	}

	if err := reg.Register(ctx, identity); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Verify through the registry.
	got, err := reg.Get(ctx, "integration@team")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.TrustScore != 0.6 {
		t.Errorf("TrustScore = %v, want 0.6", got.TrustScore)
	}

	// Verify persistence by creating a new registry from the same store.
	store2, _ := NewFileStore(tmpDir)
	reg2, err := NewRegistry(store2)
	if err != nil {
		t.Fatalf("NewRegistry() from persisted store error = %v", err)
	}

	got2, err := reg2.Get(ctx, "integration@team")
	if err != nil {
		t.Fatalf("Get() from new registry error = %v", err)
	}
	if got2.TrustScore != 0.6 {
		t.Errorf("persisted TrustScore = %v, want 0.6", got2.TrustScore)
	}
	if len(got2.Capabilities) != 2 {
		t.Errorf("persisted Capabilities count = %d, want 2", len(got2.Capabilities))
	}
}
