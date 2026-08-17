// Package migrations embeds the SQL migration files for the SQLite release run store.
//
// Embedded rather than read from disk because the SQLite store's whole reason for
// existing is a single self-contained binary (ADR-013: CGO_ENABLED=0 rules out any
// driver needing a C toolchain). A schema the binary had to find on the filesystem
// would give that back.
package migrations

import "embed"

// FS contains all embedded SQL migration files.
//
//go:embed *.sql
var FS embed.FS
