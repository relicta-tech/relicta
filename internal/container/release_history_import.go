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
	cgpmemory "github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/adapters"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence"
)

// ImportedInto names the stores the records were written to, for the lines that tell an
// operator the migration happened rather than asking them to trust it.
type ImportedInto struct {
	Backend  config.PersistenceBackend
	Location string

	// GovernanceLocation is where the governance memory went. Separate from Location
	// because the two are the same file under sqlite and the same database under postgres,
	// and an operator reading one line for each should be able to see that.
	GovernanceLocation string
}

// ImportResult is everything one `relicta db import` moved.
//
// Two reports rather than one total, because the two halves fail independently and an
// operator has to be able to tell which one they are looking at: a run store that migrated
// and a governance store that did not is a history with no risk scores, decisions or actor
// metrics attached to it.
type ImportResult struct {
	Runs       releasehistory.Report
	Governance releasehistory.GovernanceReport
	Into       ImportedInto
}

// ImportHistory reads the file-based `.relicta/` history under repoRoot into the stores
// persistence.backend selects — the release runs and the governance record both.
//
// It covers both because ADR-013 names both as the system of record, and because covering
// only the runs is a trap. Governance memory became selectable in the same change that added
// this: an operator who switched to sqlite and ran an importer that moved runs alone would
// get a clean bill of health and an empty `relicta history`, empty DORA and SOC 2 reports,
// and a deployment gate authorizing against a record with no releases in it. Nothing would
// fail. That is the shape of defect ADR-013 exists to remove, so the importer moves the
// governance record too, and reports separately on what it could not move.
//
// The source is always the file store, and not the configured one: this command exists to
// move a `.relicta/` tree that was written before the operator changed the setting.
//
// Schema is not this command's business, and that is a decision rather than an omission. The
// resolvers already migrate SQLite (a file relicta created) and already refuse an unmigrated
// PostgreSQL under migration_mode: manual with a message naming `relicta db migrate`. Import
// inherits both. What it must not do is apply migrations of its own: an importer that quietly
// reshapes an operator's provisioned schema is the same overreach as one that quietly moves
// their audit trail.
func ImportHistory(
	ctx context.Context,
	cfg *config.Config,
	repoRoot string,
	opts releasehistory.Options,
) (ImportResult, error) {
	opts.RepoRoot = repoRoot

	persistenceCfg := config.PersistenceConfig{}
	if cfg != nil {
		persistenceCfg = cfg.Persistence
	}

	// The file backend is refused rather than made a no-op.
	//
	// `db migrate` and `db status` refuse everything but postgres because they drive the
	// postgres migrator; import has no such tie — it works against any destination the port
	// has an adapter for. What it cannot do is import a store into itself. Succeeding here
	// with "12 runs imported" would be true and useless, and it would tell an operator who
	// forgot to change the setting that their history is now in a database.
	if persistenceCfg.Backend == "" || persistenceCfg.Backend == config.BackendFile {
		return ImportResult{}, fmt.Errorf(
			"persistence.backend is %q, so there is nothing to import into: the file store "+
				"under .relicta is both the source and the destination. Set "+
				"persistence.backend to \"sqlite\" or \"postgres\" in .relicta.yaml first, "+
				"then run `relicta db import` — the JSON tree is left in place either way",
			backendForMessage(persistenceCfg.Backend))
	}

	result := ImportResult{}

	runStore, err := persistence.OpenReleaseRunStore(ctx, persistenceCfg, repoRoot)
	if err != nil {
		return result, err
	}
	if runStore.Closer != nil {
		// Closed here rather than handed back, because the import is the whole lifetime of
		// this connection. A caller that forgot would leave a PostgreSQL pool open for the
		// rest of the process and a SQLite file with a live WAL.
		defer func() { _ = runStore.Closer.Close() }()
	}
	result.Into = ImportedInto{Backend: runStore.Backend, Location: runStore.Location}

	// Runs first, and the governance record only if they arrived. Governance memory refers
	// to releases by ID, so importing it into a destination whose runs failed would produce
	// an audit trail pointing at runs that are not there — the more confusing of the two
	// partial states, because it reads as complete.
	result.Runs, err = releasehistory.Import(
		ctx, adapters.NewFileReleaseRunRepository(), runStore.Repository, opts)
	if err != nil {
		return result, err
	}

	governanceStore, err := persistence.OpenGovernanceMemoryStore(
		ctx, persistenceCfg, repoRoot, GovernanceMemoryFileDir(cfg, repoRoot))
	if err != nil {
		return result, err
	}
	if governanceStore.Closer != nil {
		defer func() { _ = governanceStore.Closer.Close() }()
	}
	result.Into.GovernanceLocation = governanceStore.Location

	// The source, opened directly rather than through the resolver with the backend forced
	// to `file`. There is one file store and it takes a directory; naming it here says
	// plainly that the export side does not depend on what the operator configured.
	source, err := cgpmemory.NewFileStore(GovernanceMemoryFileDir(cfg, repoRoot))
	if err != nil {
		return result, fmt.Errorf(
			"reading the governance memory under %s: %w", GovernanceMemoryFileDir(cfg, repoRoot), err)
	}

	result.Governance, err = releasehistory.ImportGovernance(ctx, source, governanceStore.Store, opts)
	return result, err
}

// backendForMessage renders an unset backend as the default it means, so the error names the
// value that is in effect rather than an empty string the operator cannot find in their file.
func backendForMessage(backend config.PersistenceBackend) config.PersistenceBackend {
	if backend == "" {
		return config.BackendFile
	}
	return backend
}
