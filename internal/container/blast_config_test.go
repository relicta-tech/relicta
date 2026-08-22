package container

import (
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// The whole blast_radius section was unread — BlastRadiusConfig was not even a field on Config,
// so `blast_radius:` in a config file went nowhere and `relicta blast` ran on its defaults.
//
// It hid from the config-field sweep because that counts references by Go field name, and every
// name here — PackagePaths, ExcludePaths, RootPackage — also exists on the monorepo config,
// which is read. One field was unique, and pulling on it brought out the rest.
//
// Blast radius feeds the risk score, so this was a confident answer over the wrong file set
// rather than a missing feature.

func TestAnEmptyConfigurationKeepsTheAnalyzerDefaults(t *testing.T) {
	defaults := blastConfigFrom(config.BlastRadiusConfig{})

	if len(defaults.SharedDirs) == 0 {
		t.Error("a repository that names no shared directories got none. Empty means \"the " +
			"usual ones\", not \"exclude nothing\"")
	}
	if len(defaults.PackagePaths) == 0 {
		t.Error("a repository that names no package paths got none, so nothing would be found")
	}
}

func TestConfiguredPathsReplaceTheDefaults(t *testing.T) {
	cfg := blastConfigFrom(config.BlastRadiusConfig{
		PackagePaths: []string{"services/*"},
		ExcludePaths: []string{"services/legacy"},
		SharedDirs:   []string{"platform"},
	})

	if len(cfg.PackagePaths) != 1 || cfg.PackagePaths[0] != "services/*" {
		t.Errorf("package paths = %v, want the configured ones", cfg.PackagePaths)
	}
	if len(cfg.SharedDirs) != 1 || cfg.SharedDirs[0] != "platform" {
		t.Errorf("shared dirs = %v; a repository that names its own means those, not those "+
			"plus the defaults", cfg.SharedDirs)
	}
	if len(cfg.ExcludePaths) != 1 {
		t.Errorf("exclude paths = %v", cfg.ExcludePaths)
	}
}

// The two settings where "off" is a real choice are booleans, so the configuration's value is
// taken as given rather than treated as unset.
func TestTheBooleanSettingsAreTakenAsGiven(t *testing.T) {
	off := blastConfigFrom(config.BlastRadiusConfig{IgnoreDevDependencies: false, RootPackage: false})
	if off.IgnoreDevDependencies {
		t.Error("ignore_dev_dependencies: false was overridden by the default true, so " +
			"clearing it does nothing — the shape this whole sweep keeps finding")
	}
	if off.RootPackage {
		t.Error("root_package: false was overridden")
	}

	on := blastConfigFrom(config.BlastRadiusConfig{IgnoreDevDependencies: true, RootPackage: true})
	if !on.IgnoreDevDependencies || !on.RootPackage {
		t.Error("the booleans did not carry when set")
	}
}

// A depth of zero means unlimited in the analyzer, so it is not a value to copy over a default.
func TestAnUnsetDepthKeepsTheDefault(t *testing.T) {
	if got := blastConfigFrom(config.BlastRadiusConfig{MaxTransitiveDepth: 0}).MaxTransitiveDepth; got != 0 {
		t.Errorf("max transitive depth = %d, want the default", got)
	}
	if got := blastConfigFrom(config.BlastRadiusConfig{MaxTransitiveDepth: 3}).MaxTransitiveDepth; got != 3 {
		t.Errorf("max transitive depth = %d, want the configured 3", got)
	}
}
