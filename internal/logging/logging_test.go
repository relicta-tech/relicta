package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestLevelFromString(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"error":   slog.LevelError,
		"":        slog.LevelWarn, // quiet default
		"bogus":   slog.LevelWarn, // unknown falls back to quiet default
		"WARNING": slog.LevelWarn, // unrecognized casing falls back
	}
	for in, want := range cases {
		if got := LevelFromString(in); got != want {
			t.Errorf("LevelFromString(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestNewRespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, slog.LevelWarn)

	l.Info("routine plumbing")
	if buf.Len() != 0 {
		t.Fatalf("INFO record leaked at WARN level: %q", buf.String())
	}

	l.Warn("something to surface")
	if !strings.Contains(buf.String(), "something to surface") {
		t.Fatalf("WARN record missing: %q", buf.String())
	}
}

func TestNewEnabledThreshold(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, slog.LevelError)
	if l.Handler().Enabled(context.Background(), slog.LevelWarn) {
		t.Error("handler enabled WARN below ERROR threshold")
	}
	if !l.Handler().Enabled(context.Background(), slog.LevelError) {
		t.Error("handler disabled ERROR at ERROR threshold")
	}
}

func TestConfigureSetsDefault(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	var buf bytes.Buffer
	returned := Configure(slog.LevelWarn, &buf)
	if returned == nil {
		t.Fatal("Configure returned nil logger")
	}
	if slog.Default() != returned {
		t.Error("Configure did not install the logger as slog default")
	}

	slog.Info("noise")
	if buf.Len() != 0 {
		t.Errorf("INFO leaked through default logger at WARN: %q", buf.String())
	}
	slog.Error("real problem")
	if !strings.Contains(buf.String(), "real problem") {
		t.Errorf("ERROR missing from default logger: %q", buf.String())
	}
}
