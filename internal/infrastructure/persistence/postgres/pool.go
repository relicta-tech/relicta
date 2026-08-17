package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Target describes where a connection string points, without the credentials in it.
//
// For log lines and error messages that have to say *which* database, in a tool whose
// connection strings routinely come from ${DATABASE_URL} and carry a password. There is no
// safe way to print a DSN, so this prints what an operator needs to recognize the target and
// nothing that could reach a log aggregator as a secret.
func Target(connStr string) string {
	cfg, err := pgxpool.ParseConfig(connStr)
	if err != nil || cfg.ConnConfig == nil {
		// Unparseable, so there is nothing to describe and nothing may be echoed: the
		// string itself is what is malformed, and it is exactly the string that might
		// contain a password.
		return "the configured postgres database"
	}
	conn := cfg.ConnConfig
	if conn.Host == "" {
		return "the configured postgres database"
	}
	return fmt.Sprintf("postgres://%s:%d/%s", conn.Host, conn.Port, conn.Database)
}

// NewPool creates a standalone pgxpool.Pool for use by the migrator or other components.
func NewPool(ctx context.Context, connStr string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}

	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return pool, nil
}
