package container

// release_history_import.go wires `relicta db import`: the file store on one side, the store
// persistence.backend selects on the other, and releasehistory.Import between them.
//
// It lives in the composition root because that is what it is — the only thing here that the
// application service cannot do is decide *which* stores it is given, and deciding that is
// reading configuration and opening adapters. Putting it here also keeps two rules intact that
// the alternatives break: internal/cli does not reach into internal/infrastructure (FF#1 in
// internal/architecture), and persistence.backend is resolved in exactly one place, which is
// the defect ADR-013 was written about.
//
// It does not build an App. An import needs a source, a destination and a repository root; a
// container would additionally start plugins, AI providers and a webhook queue, and a
// migration that fails because an AI provider is misconfigured would be an odd thing to
// explain.

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta/v4/internal/application/releasehistory"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/adapters"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence"
)

// ImportedInto names the store the runs were written to, for the line that tells an operator
// the migration happened rather than asking them to trust it.
type ImportedInto struct {
	Backend  config.PersistenceBackend
	Location string
}

// ImportReleaseHistory reads the file-based release history under repoRoot into the store
// persistence.backend selects.
//
// The source is always the file store, and not the configured one: this command exists to move
// a `.relicta/releases` tree that was written before the operator changed the setting. Reading
// the file adapter directly — rather than opening a second store with backend forced to file —
// is deliberate, because there is only one file store and it takes its path per call.
//
// Schema is not this command's business, and that is a decision rather than an omission.
// OpenReleaseRunStore already migrates SQLite (a file relicta created) and already refuses an
// unmigrated PostgreSQL under migration_mode: manual with a message naming `relicta db
// migrate`. Import inherits both. What it must not do is apply migrations of its own: an
// importer that quietly reshapes an operator's provisioned schema is the same overreach as one
// that quietly moves their audit trail.
func ImportReleaseHistory(
	ctx context.Context,
	cfg config.PersistenceConfig,
	repoRoot string,
	opts releasehistory.Options,
) (releasehistory.Report, ImportedInto, error) {
	opts.RepoRoot = repoRoot

	// The file backend is refused rather than made a no-op.
	//
	// `db migrate` and `db status` refuse everything but postgres because they drive the
	// postgres migrator; import has no such tie — it works against any destination the port
	// has an adapter for. What it cannot do is import a store into itself. Succeeding here
	// with "12 runs imported" would be true and useless, and it would tell an operator who
	// forgot to change the setting that their history is now in a database.
	if cfg.Backend == "" || cfg.Backend == config.BackendFile {
		return releasehistory.Report{}, ImportedInto{}, fmt.Errorf(
			"persistence.backend is %q, so there is nothing to import into: the file store "+
				"under .relicta/releases is both the source and the destination. Set "+
				"persistence.backend to \"sqlite\" or \"postgres\" in .relicta.yaml first, "+
				"then run `relicta db import` — the JSON tree is left in place either way",
			backendForMessage(cfg.Backend))
	}

	store, err := persistence.OpenReleaseRunStore(ctx, cfg, repoRoot)
	if err != nil {
		return releasehistory.Report{}, ImportedInto{}, err
	}
	if store.Closer != nil {
		// Closed here rather than handed back, because the import is the whole lifetime of
		// this connection. A caller that forgot would leave a PostgreSQL pool open for the
		// rest of the process and a SQLite file with a live WAL.
		defer func() { _ = store.Closer.Close() }()
	}

	into := ImportedInto{Backend: store.Backend, Location: store.Location}

	report, err := releasehistory.Import(ctx, adapters.NewFileReleaseRunRepository(), store.Repository, opts)
	return report, into, err
}

// backendForMessage renders an unset backend as the default it means, so the error names the
// value that is in effect rather than an empty string the operator cannot find in their file.
func backendForMessage(backend config.PersistenceBackend) config.PersistenceBackend {
	if backend == "" {
		return config.BackendFile
	}
	return backend
}
