package config

import (
	"strings"
	"testing"
)

// versioning.prerelease_suffix is documented as "the suffix for prerelease versions" and is read
// by nothing. It says so now rather than sitting there looking like it works.
//
// Two wirings were tried and rejected before settling on this, and the reasons are why a warning
// is the fix rather than a shrug:
//
//   - Read literally, every bump becomes a prerelease, and a project could never cut a stable
//     release through `bump` again — measured, 1.3.0-beta.1 bumps to 1.3.0-beta.2, never 1.3.0.
//   - Wired as the default for a bare `--prerelease`, it needs pflag's NoOptDefVal, which
//     silently stops `--prerelease beta` and `-p beta` from binding their value. An unread
//     setting would have become one that overrides an explicit flag, which is worse than the
//     defect it replaced.
//
// The capability exists and is reachable through --prerelease, --channel and promote, so the
// honest fix is to point at it.

func warningsFor(t *testing.T, mutate func(*Config)) []string {
	t.Helper()

	cfg := DefaultConfig()
	mutate(cfg)

	v := NewValidator()
	_ = v.Validate(cfg)
	return v.errors.Warnings
}

func TestAConfiguredPrereleaseSuffixSaysItDoesNothing(t *testing.T) {
	warnings := warningsFor(t, func(c *Config) { c.Versioning.PrereleaseSuffix = "rc" })

	var found string
	for _, w := range warnings {
		if strings.Contains(w, "prerelease_suffix") {
			found = w
		}
	}
	if found == "" {
		t.Fatalf("no warning for a setting nothing reads; warnings were %v.\nA setting that "+
			"silently does nothing is worse than a missing one, because the operator believes "+
			"it is in force", warnings)
	}
	if !strings.Contains(found, "--prerelease") || !strings.Contains(found, "promote") {
		t.Errorf("the warning is %q but does not name what to use instead, which leaves the "+
			"reader exactly where they started", found)
	}
}

// A project that has not set it must hear nothing: a warning everyone sees is one everyone
// learns to scroll past.
func TestAnUnsetPrereleaseSuffixIsSilent(t *testing.T) {
	for _, w := range warningsFor(t, func(*Config) {}) {
		if strings.Contains(w, "prerelease_suffix") {
			t.Errorf("warned about prerelease_suffix when it was not configured: %q", w)
		}
	}
}

// It is a warning, not an error: the project is doing something ineffective, not something
// invalid, and failing their release over it would be the worse trade.
func TestTheSuffixDoesNotFailTheRelease(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Versioning.PrereleaseSuffix = "rc"

	if err := NewValidator().Validate(cfg); err != nil {
		t.Errorf("setting prerelease_suffix fails validation: %v.\nIt has no effect, which is "+
			"worth saying and not worth refusing a release over", err)
	}
}
