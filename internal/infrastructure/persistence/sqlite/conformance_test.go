package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports/conformance"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence/sqlite"
)

// The bar for this adapter. ADR-013 keeps `file` the default until parity is proven,
// and this is the proof: the same suite the file adapter passes, run against SQL.
//
// A temp *file* rather than :memory: on purpose. An in-memory database would skip
// everything the file forces — WAL, the busy timeout, the parent directory, the
// on-disk schema surviving a reopen — which is to say it would skip the parts an
// operator meets.
func TestTheSQLiteStoreSatisfiesTheContract(t *testing.T) {
	conformance.Run(t, func(t *testing.T) (ports.ReleaseRunRepository, string) {
		return newStore(t, filepath.Join(t.TempDir(), "relicta.db")), t.TempDir()
	})
}

// newStore opens a store that closes itself when the test ends.
func newStore(t *testing.T, path string) *sqlite.Store {
	t.Helper()

	store, err := sqlite.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open(%s): %v", path, err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return store
}
