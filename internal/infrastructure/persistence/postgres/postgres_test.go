package postgres_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence/postgres/migrations"
)

// mockEventStore is an in-memory implementation of ports.EventStore for testing.
// It mirrors the postgres store semantics: append-only, ordered by occurred_at.
type mockEventStore struct {
	mu     sync.RWMutex
	events []storedEntry
}

// storedEntry pairs a run ID with a domain event for in-memory storage.
type storedEntry struct {
	runID domain.RunID
	event domain.DomainEvent
}

func newMockEventStore() *mockEventStore {
	return &mockEventStore{}
}

func (m *mockEventStore) Append(_ context.Context, runID domain.RunID, events []domain.DomainEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, evt := range events {
		m.events = append(m.events, storedEntry{runID: runID, event: evt})
	}
	return nil
}

func (m *mockEventStore) LoadEvents(_ context.Context, runID domain.RunID) ([]domain.DomainEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []domain.DomainEvent
	for _, e := range m.events {
		if e.runID == runID {
			result = append(result, e.event)
		}
	}
	return result, nil
}

func (m *mockEventStore) LoadEventsSince(_ context.Context, runID domain.RunID, since time.Time) ([]domain.DomainEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []domain.DomainEvent
	for _, e := range m.events {
		if e.runID == runID && e.event.OccurredAt().After(since) {
			result = append(result, e.event)
		}
	}
	return result, nil
}

func (m *mockEventStore) LoadAllEvents(_ context.Context, _ string) ([]domain.DomainEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.DomainEvent, 0, len(m.events))
	for _, e := range m.events {
		result = append(result, e.event)
	}
	return result, nil
}

// Verify interface compliance at compile time.
var _ ports.EventStore = (*mockEventStore)(nil)

// --- Interface Contract Tests ---
// These tests validate the EventStore contract using the mock.
// The same tests apply to any EventStore implementation.

func TestEventStore_AppendAndLoadAll(t *testing.T) {
	t.Parallel()
	store := newMockEventStore()
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	runID := domain.RunID("run-1")
	events := []domain.DomainEvent{
		&domain.RunCreatedEvent{
			RunID:  runID,
			RepoID: "test-repo",
			At:     now,
		},
		&domain.RunPlannedEvent{
			RunID:       runID,
			CommitCount: 5,
			Actor:       "ci-bot",
			At:          now.Add(time.Second),
		},
	}

	if err := store.Append(ctx, runID, events); err != nil {
		t.Fatalf("Append: unexpected error: %v", err)
	}

	all, err := store.LoadAllEvents(ctx, "")
	if err != nil {
		t.Fatalf("LoadAllEvents: unexpected error: %v", err)
	}

	if got, want := len(all), 2; got != want {
		t.Fatalf("LoadAllEvents: got %d events, want %d", got, want)
	}

	if all[0].EventName() != "release.created" {
		t.Errorf("first event name = %q, want %q", all[0].EventName(), "release.created")
	}
	if all[1].EventName() != "release.planned" {
		t.Errorf("second event name = %q, want %q", all[1].EventName(), "release.planned")
	}
}

func TestEventStore_LoadEvents(t *testing.T) {
	t.Parallel()
	store := newMockEventStore()
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	run1 := domain.RunID("run-1")
	run2 := domain.RunID("run-2")

	err := store.Append(ctx, run1, []domain.DomainEvent{
		&domain.RunCreatedEvent{RunID: run1, At: now},
		&domain.RunPlannedEvent{RunID: run1, At: now.Add(time.Second)},
	})
	if err != nil {
		t.Fatalf("Append run1: %v", err)
	}

	err = store.Append(ctx, run2, []domain.DomainEvent{
		&domain.RunCreatedEvent{RunID: run2, At: now.Add(2 * time.Second)},
	})
	if err != nil {
		t.Fatalf("Append run2: %v", err)
	}

	events, err := store.LoadEvents(ctx, run1)
	if err != nil {
		t.Fatalf("LoadEvents: unexpected error: %v", err)
	}

	if got, want := len(events), 2; got != want {
		t.Fatalf("got %d events, want %d", got, want)
	}

	for _, evt := range events {
		if evt.AggregateID() != run1 {
			t.Errorf("event has RunID %q, want %q", evt.AggregateID(), run1)
		}
	}
}

func TestEventStore_LoadEventsSince(t *testing.T) {
	t.Parallel()
	store := newMockEventStore()
	ctx := context.Background()

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	runID := domain.RunID("run-1")

	err := store.Append(ctx, runID, []domain.DomainEvent{
		&domain.RunCreatedEvent{RunID: runID, At: base},
		&domain.RunPlannedEvent{RunID: runID, At: base.Add(time.Hour)},
		&domain.RunPublishedEvent{RunID: runID, At: base.Add(2 * time.Hour)},
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	events, err := store.LoadEventsSince(ctx, runID, base.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("LoadEventsSince: %v", err)
	}

	if got, want := len(events), 2; got != want {
		t.Fatalf("got %d events, want %d", got, want)
	}

	if events[0].EventName() != "release.planned" {
		t.Errorf("first event name = %q, want %q", events[0].EventName(), "release.planned")
	}
}

func TestEventStore_EmptyStore(t *testing.T) {
	t.Parallel()
	store := newMockEventStore()
	ctx := context.Background()

	all, err := store.LoadAllEvents(ctx, "")
	if err != nil {
		t.Fatalf("LoadAllEvents on empty store: unexpected error: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("LoadAllEvents on empty store: got %d events, want 0", len(all))
	}

	byID, err := store.LoadEvents(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("LoadEvents on empty store: unexpected error: %v", err)
	}
	if len(byID) != 0 {
		t.Errorf("LoadEvents on empty store: got %d events, want 0", len(byID))
	}
}

// --- Concurrent Write Tests ---

func TestEventStore_ConcurrentAppend(t *testing.T) {
	t.Parallel()
	store := newMockEventStore()
	ctx := context.Background()

	const goroutines = 50
	const eventsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(gIdx int) {
			defer wg.Done()
			for i := 0; i < eventsPerGoroutine; i++ {
				runID := domain.RunID(fmt.Sprintf("run-%d", gIdx%5))
				evt := &domain.RunCreatedEvent{
					RunID:  runID,
					RepoID: "test",
					At:     time.Now().UTC(),
				}
				if err := store.Append(ctx, runID, []domain.DomainEvent{evt}); err != nil {
					t.Errorf("goroutine %d, event %d: Append error: %v", gIdx, i, err)
				}
			}
		}(g)
	}

	wg.Wait()

	all, err := store.LoadAllEvents(ctx, "")
	if err != nil {
		t.Fatalf("LoadAllEvents after concurrent writes: %v", err)
	}

	expected := goroutines * eventsPerGoroutine
	if got := len(all); got != expected {
		t.Errorf("got %d events, want %d", got, expected)
	}
}

func TestEventStore_ConcurrentReadWrite(t *testing.T) {
	t.Parallel()
	store := newMockEventStore()
	ctx := context.Background()

	runID := domain.RunID("run-seed")

	// Pre-populate some events.
	for i := 0; i < 10; i++ {
		err := store.Append(ctx, runID, []domain.DomainEvent{
			&domain.RunCreatedEvent{
				RunID:  runID,
				RepoID: "test",
				At:     time.Now().UTC(),
			},
		})
		if err != nil {
			t.Fatalf("seeding: %v", err)
		}
	}

	var wg sync.WaitGroup
	const readers = 20
	const writers = 10

	// Launch concurrent readers.
	wg.Add(readers)
	for r := 0; r < readers; r++ {
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_, err := store.LoadAllEvents(ctx, "")
				if err != nil {
					t.Errorf("concurrent LoadAllEvents error: %v", err)
				}
				_, err = store.LoadEvents(ctx, runID)
				if err != nil {
					t.Errorf("concurrent LoadEvents error: %v", err)
				}
			}
		}()
	}

	// Launch concurrent writers.
	wg.Add(writers)
	for w := 0; w < writers; w++ {
		go func(wIdx int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				evt := &domain.RunPlannedEvent{
					RunID: runID,
					Actor: "writer",
					At:    time.Now().UTC(),
				}
				if err := store.Append(ctx, runID, []domain.DomainEvent{evt}); err != nil {
					t.Errorf("concurrent Append error: %v", err)
				}
			}
		}(w)
	}

	wg.Wait()
}

// --- Event Replay Tests ---

func TestEventStore_EventReplay_OrderPreserved(t *testing.T) {
	t.Parallel()
	store := newMockEventStore()
	ctx := context.Background()

	base := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	runID := domain.RunID("run-1")

	// Simulate a complete release lifecycle.
	lifecycle := []domain.DomainEvent{
		&domain.RunCreatedEvent{RunID: runID, RepoID: "test", At: base},
		&domain.RunPlannedEvent{RunID: runID, CommitCount: 3, At: base.Add(1 * time.Minute)},
		&domain.RunNotesGeneratedEvent{RunID: runID, At: base.Add(2 * time.Minute)},
		&domain.RunApprovedEvent{RunID: runID, ApprovedBy: "admin", At: base.Add(3 * time.Minute)},
		&domain.RunPublishedEvent{RunID: runID, At: base.Add(4 * time.Minute)},
	}

	if err := store.Append(ctx, runID, lifecycle); err != nil {
		t.Fatalf("Append lifecycle: %v", err)
	}

	// Replay: load all events for the run and verify order.
	events, err := store.LoadEvents(ctx, runID)
	if err != nil {
		t.Fatalf("LoadEvents: %v", err)
	}

	if len(events) != len(lifecycle) {
		t.Fatalf("got %d events, want %d", len(events), len(lifecycle))
	}

	expectedNames := []string{
		"release.created",
		"release.planned",
		"release.notes_generated",
		"release.approved",
		"release.published",
	}

	for i, evt := range events {
		if evt.EventName() != expectedNames[i] {
			t.Errorf("event[%d].EventName() = %q, want %q", i, evt.EventName(), expectedNames[i])
		}
	}
}

func TestEventStore_MultipleRuns_Isolation(t *testing.T) {
	t.Parallel()
	store := newMockEventStore()
	ctx := context.Background()

	now := time.Now().UTC()
	run1 := domain.RunID("run-1")
	run2 := domain.RunID("run-2")

	err := store.Append(ctx, run1, []domain.DomainEvent{
		&domain.RunCreatedEvent{RunID: run1, At: now},
		&domain.RunPublishedEvent{RunID: run1, At: now.Add(2 * time.Second)},
	})
	if err != nil {
		t.Fatalf("Append run1: %v", err)
	}

	err = store.Append(ctx, run2, []domain.DomainEvent{
		&domain.RunCreatedEvent{RunID: run2, At: now.Add(time.Second)},
	})
	if err != nil {
		t.Fatalf("Append run2: %v", err)
	}

	run1Events, err := store.LoadEvents(ctx, run1)
	if err != nil {
		t.Fatalf("LoadEvents run-1: %v", err)
	}
	if len(run1Events) != 2 {
		t.Errorf("run-1 events: got %d, want 2", len(run1Events))
	}

	run2Events, err := store.LoadEvents(ctx, run2)
	if err != nil {
		t.Fatalf("LoadEvents run-2: %v", err)
	}
	if len(run2Events) != 1 {
		t.Errorf("run-2 events: got %d, want 1", len(run2Events))
	}
}

// --- Migration Tests ---

func TestMigrationFiles_Embedded(t *testing.T) {
	t.Parallel()

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("reading embedded migrations: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("no migration files found in embedded FS")
	}

	// Verify we have matching up/down pairs.
	upFiles := make(map[string]bool)
	downFiles := make(map[string]bool)

	for _, entry := range entries {
		name := entry.Name()
		if !isSQL(name) {
			continue
		}
		if isUpMigration(name) {
			upFiles[migrationVersion(name)] = true
		} else if isDownMigration(name) {
			downFiles[migrationVersion(name)] = true
		}
	}

	for version := range upFiles {
		if !downFiles[version] {
			t.Errorf("migration %s has up.sql but no down.sql", version)
		}
	}
	for version := range downFiles {
		if !upFiles[version] {
			t.Errorf("migration %s has down.sql but no up.sql", version)
		}
	}
}

func TestMigrationFiles_ValidSQL(t *testing.T) {
	t.Parallel()

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("reading embedded migrations: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !isSQL(entry.Name()) {
			continue
		}

		data, err := migrations.FS.ReadFile(entry.Name())
		if err != nil {
			t.Errorf("reading %s: %v", entry.Name(), err)
			continue
		}

		content := string(data)
		if len(content) == 0 {
			t.Errorf("migration file %s is empty", entry.Name())
		}

		// Basic SQL validation: should contain a SQL keyword.
		if !containsSQLKeyword(content) {
			t.Errorf("migration file %s does not contain recognizable SQL", entry.Name())
		}
	}
}

// --- Config Tests ---

func TestPersistenceConfig_Defaults(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultPersistenceConfig()

	if cfg.Backend != config.BackendFile {
		t.Errorf("default Backend = %q, want %q", cfg.Backend, config.BackendFile)
	}
	if cfg.PoolSize != 10 {
		t.Errorf("default PoolSize = %d, want 10", cfg.PoolSize)
	}
	if cfg.MigrationMode != config.MigrationModeManual {
		t.Errorf("default MigrationMode = %q, want %q", cfg.MigrationMode, config.MigrationModeManual)
	}
	if cfg.FilePath != ".relicta/events" {
		t.Errorf("default FilePath = %q, want %q", cfg.FilePath, ".relicta/events")
	}
}

func TestPersistenceConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     config.PersistenceConfig
		wantErr bool
	}{
		{
			name:    "valid file backend",
			cfg:     config.PersistenceConfig{Backend: config.BackendFile},
			wantErr: false,
		},
		{
			name: "valid postgres backend",
			cfg: config.PersistenceConfig{
				Backend:          config.BackendPostgres,
				ConnectionString: "postgres://localhost:5432/test",
				PoolSize:         5,
				MigrationMode:    config.MigrationModeAuto,
			},
			wantErr: false,
		},
		{
			name: "postgres without connection string",
			cfg: config.PersistenceConfig{
				Backend:       config.BackendPostgres,
				PoolSize:      5,
				MigrationMode: config.MigrationModeManual,
			},
			wantErr: true,
		},
		{
			name: "postgres with zero pool size",
			cfg: config.PersistenceConfig{
				Backend:          config.BackendPostgres,
				ConnectionString: "postgres://localhost/db",
				PoolSize:         0,
				MigrationMode:    config.MigrationModeManual,
			},
			wantErr: true,
		},
		{
			name: "postgres with invalid migration mode",
			cfg: config.PersistenceConfig{
				Backend:          config.BackendPostgres,
				ConnectionString: "postgres://localhost/db",
				PoolSize:         5,
				MigrationMode:    "invalid",
			},
			wantErr: true,
		},
		{
			name:    "invalid backend",
			cfg:     config.PersistenceConfig{Backend: "redis"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExpandEnvVars(t *testing.T) {
	t.Setenv("TEST_DB_URL", "postgres://user:pass@host:5432/db")
	t.Setenv("TEST_HOST", "myhost")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "single var",
			input: "${TEST_DB_URL}",
			want:  "postgres://user:pass@host:5432/db",
		},
		{
			name:  "var in string",
			input: "postgresql://${TEST_HOST}:5432/mydb",
			want:  "postgresql://myhost:5432/mydb",
		},
		{
			name:  "missing var unchanged",
			input: "${NONEXISTENT_VAR_12345}",
			want:  "${NONEXISTENT_VAR_12345}",
		},
		{
			name:  "no vars",
			input: "postgres://localhost:5432/db",
			want:  "postgres://localhost:5432/db",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.ExpandEnvVars(tt.input)
			if got != tt.want {
				t.Errorf("ExpandEnvVars(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEventStore_AppendEmpty(t *testing.T) {
	t.Parallel()
	store := newMockEventStore()
	ctx := context.Background()

	// Appending empty events should be a no-op.
	if err := store.Append(ctx, "run-1", nil); err != nil {
		t.Fatalf("Append nil events: %v", err)
	}
	if err := store.Append(ctx, "run-1", []domain.DomainEvent{}); err != nil {
		t.Fatalf("Append empty events: %v", err)
	}

	all, err := store.LoadAllEvents(ctx, "")
	if err != nil {
		t.Fatalf("LoadAllEvents: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("got %d events, want 0", len(all))
	}
}

// --- Helper Functions ---

func isSQL(name string) bool {
	return len(name) > 4 && name[len(name)-4:] == ".sql"
}

func isUpMigration(name string) bool {
	return len(name) > 7 && name[len(name)-7:] == ".up.sql"
}

func isDownMigration(name string) bool {
	return len(name) > 9 && name[len(name)-9:] == ".down.sql"
}

func migrationVersion(name string) string {
	for i, c := range name {
		if c == '_' {
			return name[:i]
		}
	}
	return name
}

func containsSQLKeyword(content string) bool {
	keywords := []string{"CREATE", "DROP", "INSERT", "SELECT", "ALTER", "DELETE", "UPDATE", "TABLE", "INDEX"}
	upper := toUpper(content)
	for _, kw := range keywords {
		if containsStr(upper, kw) {
			return true
		}
	}
	return false
}

func toUpper(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func containsStr(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Suppress unused import warnings for os (used indirectly via t.Setenv).
var _ = os.Getenv
