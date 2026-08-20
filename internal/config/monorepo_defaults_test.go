package config

import (
	"os"
	"path/filepath"
	"testing"
)

// A configuration that names only what it cares about must still get the defaults for what it
// does not. Viper merges defaults key by key, but only for keys that were registered, and the
// monorepo section had none.
//
// The cost was immediate once the section started being read: this file
//
//	monorepo:
//	  enabled: true
//	  package_paths: ["packages/*"]
//
// loaded with an empty strategy, and validation refused it with
//
//	monorepo.strategy: must be "independent", got ""
//
// A reasonable configuration rejected for leaving out a key that has a default.

func TestAMonorepoSectionInheritsTheDefaultsItOmits(t *testing.T) {
	dir := t.TempDir()
	body := "monorepo:\n  enabled: true\n  package_paths:\n    - \"packages/*\"\n"
	if err := os.WriteFile(filepath.Join(dir, ".relicta.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("LoadFromDirectory: %v", err)
	}

	if cfg.Monorepo.Strategy != MonorepoStrategyIndependent {
		t.Errorf("strategy = %q, want the default %q", cfg.Monorepo.Strategy, MonorepoStrategyIndependent)
	}
	if !cfg.Monorepo.Changelog.PerPackage {
		t.Error("changelog.per_package = false, want the default true: the packages this " +
			"release tags would get no changelog, silently")
	}
	if !cfg.Monorepo.Changelog.RootChangelog {
		t.Error("changelog.root_changelog = false, want the default true")
	}

	if err := NewValidator().Validate(cfg); err != nil {
		t.Errorf("a config naming only enabled and package_paths was refused: %v", err)
	}
}

// What the file does say still wins over the default.
func TestAnExplicitMonorepoSettingBeatsItsDefault(t *testing.T) {
	dir := t.TempDir()
	body := "monorepo:\n  enabled: true\n  package_paths: [\"packages/*\"]\n  changelog:\n    per_package: false\n"
	if err := os.WriteFile(filepath.Join(dir, ".relicta.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("LoadFromDirectory: %v", err)
	}
	if cfg.Monorepo.Changelog.PerPackage {
		t.Error("per_package: false was overridden by the default")
	}
	if !cfg.Monorepo.Changelog.RootChangelog {
		t.Error("setting one changelog key discarded the defaults for its siblings")
	}
}
