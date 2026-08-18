package container

// governance_memory.go is how everything outside the composition root reaches the governance
// memory store persistence.backend selected.
//
// It exists because of FF#1 in internal/architecture: internal/cli must not import
// internal/infrastructure, and `relicta history`, `relicta audit`, `relicta report`, `relicta
// deploy`, `relicta hub sync` and `relicta integrations` all need the store. The release-run
// work answered the same question the same way — see release_history_import.go — and the
// answer is not an allowlist entry. The composition root is already the single importer of
// internal/infrastructure/persistence, and opening an adapter from a configuration is exactly
// what a composition root is for.
//
// The one thing this adds over the resolver is the file backend's *path*, which is
// governance.MemoryStorePath: an application-layer answer that infrastructure must not reach
// up to ask for. Putting the two together here means a caller says which repository it is
// working in and gets a store, and neither the path rule nor the backend rule is spelled twice.

import (
	"context"
	"path/filepath"

	"github.com/relicta-tech/relicta/v4/internal/application/governance"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence"
)

// OpenGovernanceMemory resolves persistence.backend into a governance memory store for the
// repository rooted at repoRoot.
//
// The returned store's Closer is nil for the file backend and holds a connection for the
// database ones, so a caller that opens a store must close it — a CLI command with a defer, a
// container through registerCloseable. Leaving a SQLite handle open for the life of a
// long-running process is how `relicta serve` would accumulate one per request.
//
// cfg is the whole configuration rather than only its persistence section, because the file
// backend's location comes from governance.memory_path and the two must be read from the same
// Config. A caller that loaded persistence from the repository and governance from the process
// working directory would open the right backend in the wrong place.
func OpenGovernanceMemory(
	ctx context.Context, cfg *config.Config, repoRoot string,
) (persistence.GovernanceMemoryStore, error) {
	persistenceCfg := config.PersistenceConfig{}
	if cfg != nil {
		persistenceCfg = cfg.Persistence
	}

	return persistence.OpenGovernanceMemoryStore(
		ctx, persistenceCfg, repoRoot, GovernanceMemoryFileDir(cfg, repoRoot))
}

// GovernanceMemoryFileDir is where the file backend keeps memory.json for repoRoot.
//
// The same resolution every reader has used since `relicta history` and the release path were
// found to be reading two different stores: a relative governance.memory_path is anchored to
// the repository root, never to the process working directory, and never to a home directory.
//
// Exported because `relicta db import` needs it for the *source* side, which is always the
// files regardless of what persistence.backend now selects.
func GovernanceMemoryFileDir(cfg *config.Config, repoRoot string) string {
	memoryPath := ""
	if cfg != nil {
		memoryPath = cfg.Governance.MemoryPath
	}
	return filepath.Dir(governance.MemoryStorePath(memoryPath, repoRoot))
}
