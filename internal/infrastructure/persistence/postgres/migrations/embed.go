// Package migrations embeds SQL migration files for the PostgreSQL event store.
package migrations

import "embed"

// FS contains all embedded SQL migration files.
//
//go:embed *.sql
var FS embed.FS
