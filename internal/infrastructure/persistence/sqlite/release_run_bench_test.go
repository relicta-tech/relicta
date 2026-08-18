package sqlite_test

import (
	"context"
	"fmt"
	"testing"

	domainrelease "github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/adapters"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence/sqlite"
)

// BenchmarkReleaseRunQueries measures the claim ADR-013 was argued from: that a database helps a
// single local user because the file adapter answers every query by walking the tree and parsing
// every file.
//
// Half of it holds. `List` is an order of magnitude faster, because it answers from an index
// without materializing anything. `FindByState` is not faster and at small histories is slower —
// the index finds the rows immediately and the adapter then selects `document` and deserializes
// every match, so both backends do identical JSON parsing and SQLite adds query overhead. The
// bottleneck was never the walk; it is the deserialization.
//
// A benchmark rather than a test, deliberately: this measures rather than asserts, and a
// threshold that fails on a busy machine teaches people to ignore the thing that failed. It
// exists so the numbers in ADR-013's amendment can be reproduced instead of believed.
func BenchmarkReleaseRunQueries(b *testing.B) {
	for _, n := range []int{100, 500, 2000} {
		for _, backend := range []struct {
			name string
			open func(b *testing.B, root string) ports.ReleaseRunRepository
		}{
			{"file", func(b *testing.B, _ string) ports.ReleaseRunRepository {
				return adapters.NewFileReleaseRunRepository()
			}},
			{"sqlite", func(b *testing.B, root string) ports.ReleaseRunRepository {
				store, err := sqlite.Open(context.Background(), sqlite.DefaultPath(root))
				if err != nil {
					b.Fatalf("Open: %v", err)
				}
				b.Cleanup(func() { _ = store.Close() })
				return store
			}},
		} {
			root := b.TempDir()
			repo := backend.open(b, root)
			seedRuns(b, repo, root, n)

			b.Run(fmt.Sprintf("List/%s/n=%d", backend.name, n), func(b *testing.B) {
				for b.Loop() {
					if _, err := repo.List(context.Background(), root); err != nil {
						b.Fatalf("List: %v", err)
					}
				}
			})

			b.Run(fmt.Sprintf("FindByState/%s/n=%d", backend.name, n), func(b *testing.B) {
				for b.Loop() {
					if _, err := repo.FindByState(context.Background(), root, domain.StatePlanned); err != nil {
						b.Fatalf("FindByState: %v", err)
					}
				}
			})
		}
	}
}

func seedRuns(b *testing.B, repo ports.ReleaseRunRepository, root string, n int) {
	b.Helper()

	ctx := context.Background()
	for i := range n {
		run := domainrelease.NewReleaseRunForTest(
			domainrelease.RunID(fmt.Sprintf("run-%05d", i)), "main", root)
		if err := run.Plan("bench"); err != nil {
			b.Fatalf("Plan: %v", err)
		}
		if err := repo.Save(ctx, run); err != nil {
			b.Fatalf("Save: %v", err)
		}
	}
}
