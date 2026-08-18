// Package migrations embeds SQL migration files for the PostgreSQL backend — the event
// log in 001, the release runs that are the system of record under ADR-013 in 002, and
// the governance memory that records what was decided about them in 003.
package migrations

import "embed"

// FS contains all embedded SQL migration files.
//
//go:embed *.sql
var FS embed.FS
