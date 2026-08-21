package config

import (
	"strings"
	"testing"
)

// The monorepo section used to be the largest unread block in the configuration: every field had
// zero production readers, while internal/domain/monorepo and internal/application/monorepo held
// roughly 3,000 lines of implemented, tested code that nothing in the release path called.
//
// Verified against the built binary at the time. A repository with `enabled: true`,
// `strategy: independent`, `package_paths: ["packages/*"]` and two packages at 1.0.0 and 2.0.0
// was given one repository-wide version:
//
//	Current version:  0.0.0
//	Next version:     0.1.0
//
// and neither package.json was touched.
//
// `independent` is now wired. What these tests hold in place is the boundary of that: the
// strategies still unimplemented must be refused, not quietly served as something else.

func TestAnIndependentMonorepoNeedsPackagePaths(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Monorepo.Enabled = true
	cfg.Monorepo.Strategy = MonorepoStrategyIndependent
	cfg.Monorepo.PackagePaths = nil

	err := NewValidator().Validate(cfg)
	if err == nil {
		t.Fatal("a monorepo with no package_paths validated.\nNothing would be discovered, so " +
			"every release would silently fall back to versioning the repository as a whole")
	}
	if !strings.Contains(err.Error(), "package_paths") {
		t.Errorf("error = %q, want it to name monorepo.package_paths", err)
	}
}

func TestAConfiguredIndependentMonorepoValidates(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Monorepo.Enabled = true
	cfg.Monorepo.Strategy = MonorepoStrategyIndependent
	cfg.Monorepo.PackagePaths = []string{"packages/*"}

	if err := NewValidator().Validate(cfg); err != nil {
		t.Errorf("a fully configured independent monorepo fails validation: %v", err)
	}
}

// lockstep and hybrid are refused rather than warned about. A warning was right while the whole
// section did nothing — the release still behaved as the repository-wide config said. Now the
// same file gets per-package versioning for independent, so accepting lockstep would hand
// somebody the exact opposite of what they configured.
func TestTheUnimplementedStrategiesAreRefused(t *testing.T) {
	for _, strategy := range []MonorepoStrategy{MonorepoStrategyLockstep, MonorepoStrategyHybrid} {
		cfg := DefaultConfig()
		cfg.Monorepo.Enabled = true
		cfg.Monorepo.Strategy = strategy
		cfg.Monorepo.PackagePaths = []string{"packages/*"}

		err := NewValidator().Validate(cfg)
		if err == nil {
			t.Errorf("strategy %q validated; packages would be versioned independently, "+
				"which is the opposite of what it asks for", strategy)
			continue
		}
		if !strings.Contains(err.Error(), string(strategy)) {
			t.Errorf("the error for %q does not name it: %v", strategy, err)
		}
		if !strings.Contains(err.Error(), string(MonorepoStrategyIndependent)) {
			t.Errorf("the error for %q does not say which strategy does work: %v", strategy, err)
		}
	}
}

func TestReleaseGroupsAreRefusedWhileNothingReleasesThem(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Monorepo.Enabled = true
	cfg.Monorepo.Strategy = MonorepoStrategyIndependent
	cfg.Monorepo.PackagePaths = []string{"packages/*"}
	cfg.Monorepo.ReleaseGroups = []ReleaseGroupConfig{{Name: "platform"}}

	err := NewValidator().Validate(cfg)
	if err == nil {
		t.Fatal("release_groups validated. Every package releases on its own commits, so the " +
			"group is ignored — and a coordinated release that is not coordinated is worse " +
			"than one that is refused")
	}
	if !strings.Contains(err.Error(), "release_groups") {
		t.Errorf("error = %q, want it to name monorepo.release_groups", err)
	}
}

// A repository that is not a monorepo must hear nothing.
func TestADisabledMonorepoSectionIsSilent(t *testing.T) {
	for _, w := range warningsFor(t, func(*Config) {}) {
		if strings.Contains(w, "monorepo") {
			t.Errorf("warned about monorepo in a repository that did not enable it: %q", w)
		}
	}

	cfg := DefaultConfig()
	cfg.Monorepo.Strategy = MonorepoStrategyLockstep // unread while disabled
	if err := NewValidator().Validate(cfg); err != nil {
		t.Errorf("a disabled monorepo section failed validation: %v", err)
	}
}

// The settings that remain unimplemented are refused, and — just as importantly — none of them
// defaults to on. A default that asks for a feature nothing performs is the same lie as a
// setting nothing reads, and it would make every monorepo configuration fail the validation
// that says so.
func TestTheUnimplementedMonorepoSettingsAreRefusedAndDefaultOff(t *testing.T) {
	base := func() *Config {
		cfg := DefaultConfig()
		cfg.Monorepo.Enabled = true
		cfg.Monorepo.Strategy = MonorepoStrategyIndependent
		cfg.Monorepo.PackagePaths = []string{"packages/*"}
		return cfg
	}

	if err := NewValidator().Validate(base()); err != nil {
		t.Fatalf("the defaults do not validate: %v", err)
	}

	cases := map[string]func(*Config){
		"dependency_coordination": func(c *Config) { c.Monorepo.DependencyCoordination = true },
		"include_package_links":   func(c *Config) { c.Monorepo.Changelog.IncludePackageLinks = true },
		"version_files": func(c *Config) {
			c.Monorepo.VersionFiles = map[string]VersionFileConfig{"npm": {File: "somewhere.json"}}
		},
		"version_file": func(c *Config) {
			c.Monorepo.PackageOverrides = map[string]PackageOverrideConfig{
				"packages/api": {VersionFile: "VERSION"},
			}
		},
	}

	for name, apply := range cases {
		cfg := base()
		apply(cfg)
		err := NewValidator().Validate(cfg)
		if err == nil {
			t.Errorf("%s was accepted; it does nothing, so a project setting it would believe "+
				"something happened", name)
			continue
		}
		if !strings.Contains(err.Error(), name) {
			t.Errorf("the error for %s does not name it: %v", name, err)
		}
	}
}

// tag_prefix, changelog_file and skip_versioning are honored, so they must not be refused
// alongside the overrides that are not.
func TestTheHonoredOverridesAreAccepted(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Monorepo.Enabled = true
	cfg.Monorepo.Strategy = MonorepoStrategyIndependent
	cfg.Monorepo.PackagePaths = []string{"packages/*"}
	cfg.Monorepo.PackageOverrides = map[string]PackageOverrideConfig{
		"packages/api": {TagPrefix: "api-v", ChangelogFile: "NEWS.md", SkipVersioning: true},
	}

	if err := NewValidator().Validate(cfg); err != nil {
		t.Errorf("the overrides relicta honors were refused: %v", err)
	}
}
