package persistence

import (
	"context"
	"fmt"
	"io"

	cgpprotocol "github.com/relicta-tech/relicta/v5/internal/cgp/protocol"
	"github.com/relicta-tech/relicta/v5/internal/config"
	"github.com/relicta-tech/relicta/v5/internal/infrastructure/persistence/postgres"
	"github.com/relicta-tech/relicta/v5/internal/infrastructure/persistence/sqlite"
)

// CGPProtocolStore is the resolved proposal store and its database lifetime.
type CGPProtocolStore struct {
	Store    cgpprotocol.RecordStore
	Closer   io.Closer
	Backend  config.PersistenceBackend
	Location string
}

// OpenCGPProtocolStore resolves the configured persistence backend for the CGP
// proposal/decision/authorization handshake.
func OpenCGPProtocolStore(ctx context.Context, cfg config.PersistenceConfig, repoRoot string) (CGPProtocolStore, error) {
	if cfg.Backend == "" {
		cfg.Backend = config.BackendFile
	}
	if err := cfg.Validate(); err != nil {
		return CGPProtocolStore{}, fmt.Errorf("persistence: %w", err)
	}

	switch cfg.Backend {
	case config.BackendFile:
		if repoRoot == "" {
			return CGPProtocolStore{}, fmt.Errorf("persistence: the file backend needs a repository root for CGP protocol records")
		}
		return CGPProtocolStore{
			Store:    cgpprotocol.NewFileProposalStore(repoRoot),
			Backend:  config.BackendFile,
			Location: repoRoot,
		}, nil

	case config.BackendSQLite:
		if repoRoot == "" {
			return CGPProtocolStore{}, fmt.Errorf("persistence: the sqlite backend needs a repository root to place its database")
		}
		path := sqlite.DefaultPath(repoRoot)
		openCtx, cancel := context.WithTimeout(ctx, connectTimeout)
		defer cancel()
		store, err := sqlite.OpenCGPProtocolStore(openCtx, path)
		if err != nil {
			return CGPProtocolStore{}, fmt.Errorf("persistence: opening the sqlite CGP protocol store: %w", err)
		}
		return CGPProtocolStore{Store: store, Closer: store, Backend: config.BackendSQLite, Location: path}, nil

	case config.BackendPostgres:
		connStr := config.ExpandEnvVars(cfg.ConnectionString)
		target := postgres.Target(connStr)
		connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
		defer cancel()
		pool, err := postgres.NewPool(connectCtx, connStr, cfg.PoolSize)
		if err != nil {
			return CGPProtocolStore{}, fmt.Errorf("persistence: connecting to the postgres CGP protocol store at %s: %w", target, err)
		}
		closer := closerFunc(func() error { pool.Close(); return nil })
		if cfg.MigrationMode == config.MigrationModeAuto {
			if _, err := postgres.NewMigrator(pool).Up(ctx); err != nil {
				pool.Close()
				return CGPProtocolStore{}, fmt.Errorf("persistence: migrating the postgres CGP protocol store at %s: %w", target, err)
			}
		} else if err := postgres.VerifyCGPProtocolSchema(connectCtx, pool); err != nil {
			pool.Close()
			return CGPProtocolStore{}, fmt.Errorf("persistence: postgres CGP protocol store at %s is not ready: %w", target, err)
		}
		return CGPProtocolStore{
			Store: postgres.NewCGPProtocolStore(pool), Closer: closer,
			Backend: config.BackendPostgres, Location: target,
		}, nil

	default:
		return CGPProtocolStore{}, fmt.Errorf("persistence: unsupported backend %q for the CGP protocol store", cfg.Backend)
	}
}
