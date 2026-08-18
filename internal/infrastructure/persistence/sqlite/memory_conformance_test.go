package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory/conformance"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence/sqlite"
)

// The bar for this adapter. ADR-013 keeps `file` the default until parity is proven, and
// this is the proof for the governance record: the same suite the file store passes, run
// against SQL, unmodified.
//
// A temp *file* rather than :memory:, for the reason the run store's suite gives — an
// in-memory database skips WAL, the busy timeout, the parent directory and the schema
// surviving a reopen, which is to say it skips the parts an operator meets.
func TestTheSQLiteGovernanceMemoryStoreSatisfiesTheContract(t *testing.T) {
	conformance.Run(t, func(t *testing.T) memory.Store {
		return newMemoryStore(t, filepath.Join(t.TempDir(), "relicta.db"))
	})
}

// newMemoryStore opens a governance memory store that closes itself when the test ends.
func newMemoryStore(t *testing.T, path string) *sqlite.MemoryStore {
	t.Helper()

	store, err := sqlite.OpenMemoryStore(context.Background(), path)
	if err != nil {
		t.Fatalf("OpenMemoryStore(%s): %v", path, err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return store
}
