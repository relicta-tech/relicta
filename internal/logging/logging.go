// Package logging configures the application's structured logger.
//
// Operational logs go through Bolt (go.klarlabs.de/bolt) — a zero-allocation
// structured logger — exposed as an slog.Handler so existing slog call sites
// are unchanged. All output is written to stderr, never stdout, so machine
// consumers parsing a command's stdout (CI pipelines reading --json) never
// see log noise. The default level is WARN: routine operational INFO lines
// stay silent unless the operator opts into --verbose / --log-level.
package logging

import (
	"io"
	"log/slog"
	"os"

	"go.klarlabs.de/bolt"
)

// LevelFromString maps a config/flag level string to an slog.Level.
// Unknown values fall back to the quiet default (WARN).
func LevelFromString(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "error":
		return slog.LevelError
	case "warn", "":
		return slog.LevelWarn
	default:
		return slog.LevelWarn
	}
}

// New returns an slog.Logger backed by Bolt, writing to w at the given level.
func New(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(bolt.NewSlogHandler(w, &bolt.SlogHandlerOptions{Level: level}))
}

// Configure installs a Bolt-backed slog logger as the process default,
// writing to stderr at the given level. Call once during startup after the
// effective level is known. Passing a non-nil w overrides the stderr target
// (used when logs are redirected to a file).
func Configure(level slog.Level, w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stderr
	}
	l := New(w, level)
	slog.SetDefault(l)
	return l
}
