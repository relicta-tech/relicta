package container

import (
	"github.com/relicta-tech/relicta/v4/internal/application/blast"
	"github.com/relicta-tech/relicta/v4/internal/config"
)

// Reading blast_radius.
//
// The whole section was unread. `relicta blast` built its analysis from
// blast.DefaultMonorepoConfig() plus two flags, so a repository that configured
// package_paths, exclude_paths, shared_dirs, root_package, max_transitive_depth or
// ignore_dev_dependencies was analyzed with the defaults and told nothing.
//
// It hid from the config-field sweep because the sweep counts references by Go field name, and
// every name here — PackagePaths, ExcludePaths, RootPackage — also exists on the monorepo
// config, which is read. One field, IgnoreDevDependencies, happened to be unique, and pulling
// on it brought out the other eight.
//
// This matters more than an ordinary unread section: blast radius feeds the risk score, and a
// risk score computed over the wrong file set is a confident wrong answer rather than a missing
// one. It is the same defect ADR-015 recorded for monorepo versioning, in the analysis that the
// same entry said was working.

// blastConfigFrom translates the configured analysis settings.
//
// Empty values keep the default rather than clearing it: a repository that names no shared
// directories means "the usual ones", not "none". The two settings where empty is a real choice
// — root_package and ignore_dev_dependencies — are booleans, so the configuration's value is
// taken as given.
// BlastConfigFrom is the exported form, for the CLI, which builds its own service for a
// one-off analysis and must read the same settings the container does.
func BlastConfigFrom(cfg config.BlastRadiusConfig) *blast.MonorepoConfig {
	return blastConfigFrom(cfg)
}

func blastConfigFrom(cfg config.BlastRadiusConfig) *blast.MonorepoConfig {
	blastCfg := blast.DefaultMonorepoConfig()

	if len(cfg.PackagePaths) > 0 {
		blastCfg.PackagePaths = cfg.PackagePaths
	}
	if len(cfg.ExcludePaths) > 0 {
		blastCfg.ExcludePaths = cfg.ExcludePaths
	}
	if len(cfg.SharedDirs) > 0 {
		blastCfg.SharedDirs = cfg.SharedDirs
	}
	if cfg.MaxTransitiveDepth > 0 {
		blastCfg.MaxTransitiveDepth = cfg.MaxTransitiveDepth
	}
	blastCfg.RootPackage = cfg.RootPackage
	blastCfg.IgnoreDevDependencies = cfg.IgnoreDevDependencies

	return blastCfg
}
