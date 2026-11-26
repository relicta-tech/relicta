# Multi-Package/Monorepo Versioning Design

## Status (2026-11-26)

This document describes the design in full. Part of it ships; the rest does not, and the
difference matters more here than usual — the whole subsystem spent a long time implemented,
tested and reachable from nothing. See ADR-015.

**Implemented and reachable:** `strategy: independent`. `relicta bump` discovers packages from
`monorepo.package_paths`, maps commits to the packages they touch, computes each package's bump
from its own commits, and writes its version file (for example `package.json`, `Cargo.toml`,
`pyproject.toml`, a Go `version.go` constant, a Maven or Gradle file, or a plain `VERSION`).

**Refused at config load, because nothing implements them:** `lockstep`, `hybrid`,
`release_groups`.

**Implemented and reachable:** per-package tags. `relicta publish` creates one per package at
the version its manifest claims — `api-v1.5.0` by default, or `package_overrides.<path>.
tag_prefix` — alongside the repository's own tag, and `relicta bump` reads them back so each
package measures from its own last release.

**Implemented and reachable:** per-package changelogs. `publish` writes each changed package's
own `CHANGELOG.md` from that package's commits, under the repository's `changelog.*` settings,
and the release commit carries them.

**Implemented and reachable:** a governance decision per package release. Each changed package
has its own release run, policy evaluation, approval, and audit entry. `relicta approve --package
<name>` decides one package; publishing creates tags and changelog entries only for approved
packages. The repository release run still coordinates the overall workflow. See
[ADR-015](../adr/015-per-package-versioning-is-independent-only.md) for the independent-versioning
decision and implementation boundaries.

**Not implemented:** dependency-coordinated version propagation and the `MonorepoRelease`
aggregate's own lifecycle.

This status supersedes the historical implementation plan and API sketches below. Package-level
release runs and approvals are implemented; the older plan's steps should not be read as pending.

## Overview

Independent versioning for packages within a monorepo, enabling:
- Per-package semantic versioning
- Automatic version file updates (including package manifests and supported project version files)
- Package-specific changelogs

## Architecture

### Domain Model

```
MonorepoRelease (Aggregate Root)
├── ID: MonorepoReleaseID
├── Packages: []PackageRelease
├── State: MonorepoReleaseState
├── CreatedAt, UpdatedAt
└── Events: []DomainEvent

PackageRelease (Entity)
├── Package: Package (from blast)
├── CurrentVersion: SemanticVersion
├── NextVersion: SemanticVersion
├── BumpType: BumpType
├── State: PackageReleaseState
├── ChangeSet: ChangeSet (commits affecting this package)
└── Notes: ReleaseNotes
```

### State Machine

```
MonorepoReleaseState:
  Draft → Planned → Versioned → NotesReady → Approved → Publishing → Published
                                                        ↓
                                                      Failed

PackageReleaseState:
  Pending → Included → Excluded → Released → Skipped
```

### Current configuration

This example uses configuration currently accepted by Relicta. `lockstep`, `hybrid`,
`release_groups`, custom `version_files`, and `package_overrides.version_file` or
`version_field` are not supported and are rejected during configuration validation.

```yaml
# .relicta.yaml
monorepo:
  enabled: true
  strategy: independent
  package_paths:
    - "packages/*"
    - "plugins/*"
  exclude_paths:
    - "packages/internal-*"
  package_overrides:
    packages/core:
      tag_prefix: "core-"
      changelog_file: RELEASE_NOTES.md
    packages/cli:
      skip_versioning: true
  changelog:
    per_package: true
    root_changelog: true
```

## Historical Implementation Plan

### Phase 1: Configuration Extension

1. Add `MonorepoConfig` to config schema
2. Add version file patterns configuration
3. Add package override configuration

### Phase 2: Domain Model

1. Create `MonorepoRelease` aggregate
2. Create `PackageRelease` entity
3. Implement state machines
4. Add domain events

### Phase 3: Version File Writers

1. Implement `VersionFileWriter` interface
2. Add writers for:
   - NPM (package.json)
   - Cargo (Cargo.toml)
   - Python (pyproject.toml, setup.py)
   - Maven (pom.xml)
   - Gradle (build.gradle)
   - Go (version.go files)

### Phase 4: CLI Integration

1. Add `--package` flag to commands
2. Add `relicta packages list` command
3. Add `relicta packages release` command
4. Modify `relicta plan/bump/notes/publish` for monorepo

### Phase 5: MCP Integration

1. Add `relicta.packages` tool
2. Add `relicta.package_release` tool
3. Add package filtering to existing tools

## Historical API Sketches

### CLI Commands

```bash
# List discovered packages
relicta packages list

# Plan release for all affected packages
relicta plan --monorepo

# Plan release for specific package
relicta plan --package packages/core

# Bump specific packages
relicta bump --package packages/core,packages/cli

# Generate notes for all releasing packages
relicta notes --monorepo

# Publish coordinated release
relicta publish --monorepo

# Release single package
relicta release --package packages/core
```

### MCP Tools

```json
{
  "name": "relicta.packages",
  "description": "List and analyze packages in monorepo",
  "input": {
    "include_affected": true,
    "from_ref": "v1.0.0"
  },
  "output": {
    "packages": [...],
    "affected_packages": [...]
  }
}

{
  "name": "relicta.package_release",
  "description": "Execute release for specific packages",
  "input": {
    "packages": ["packages/core", "packages/cli"],
    "dry_run": false
  }
}
```

## Version File Writers

### Interface

```go
type VersionFileWriter interface {
    // CanHandle returns true if this writer handles the package type
    CanHandle(pkg *blast.Package) bool

    // ReadVersion reads the current version from the package
    ReadVersion(ctx context.Context, pkgPath string) (version.SemanticVersion, error)

    // WriteVersion updates the version in the package files
    WriteVersion(ctx context.Context, pkgPath string, ver version.SemanticVersion) error

    // Files returns the files that will be modified
    Files(pkgPath string) []string
}
```

### NPM Writer

```go
type NPMVersionWriter struct{}

func (w *NPMVersionWriter) CanHandle(pkg *blast.Package) bool {
    return pkg.Type == blast.PackageTypeNPM
}

func (w *NPMVersionWriter) WriteVersion(ctx context.Context, pkgPath string, ver version.SemanticVersion) error {
    // Read package.json
    // Update version field
    // Write package.json
    // Optionally run npm version (for lock file)
    return nil
}
```

## Tag Naming Convention

Independent tags are implemented. Lockstep and group tags below describe possible future
strategies; those strategies are currently refused by configuration validation.

| Strategy | Tag Format | Example |
|----------|------------|---------|
| Independent | `{prefix}{pkg}-v{version}` | `@scope/core-v1.2.3` |
| Lockstep | `v{version}` | `v1.2.3` |
| Group | `{group}-v{version}` | `core-v1.2.3` |

## Dependency Coordination

When releasing packages with internal dependencies:

1. Analyze dependency graph
2. Sort packages by dependency order
3. For each package:
   - Calculate version bump
   - Check if dependents need updates
   - Update internal dependency versions
4. Release in order (dependencies first)

```
Example:
packages/core → v1.0.0 → v1.1.0 (breaking change)
packages/cli (depends on core) → needs major bump due to core breaking
packages/api (depends on core) → needs major bump due to core breaking
```

## Risk Assessment Integration

The blast radius service already calculates risk per package. This integrates with CGP:

```go
func (s *MonorepoReleaseService) EvaluateRisk(ctx context.Context, rel *MonorepoRelease) (*MonorepoRiskAssessment, error) {
    assessment := &MonorepoRiskAssessment{
        OverallRisk: 0,
        PackageRisks: make(map[string]*RiskAssessment),
    }

    for _, pkg := range rel.Packages {
        // Get blast radius impact for package
        impact := s.blastService.GetImpact(pkg.Package)

        // Calculate risk using CGP
        pkgRisk := s.cgpService.CalculatePackageRisk(pkg, impact)
        assessment.PackageRisks[pkg.Package.Path] = pkgRisk

        // Aggregate risk (highest wins)
        if pkgRisk.Score > assessment.OverallRisk {
            assessment.OverallRisk = pkgRisk.Score
        }
    }

    return assessment, nil
}
```

## Files to Create/Modify

### New Files

- `internal/domain/monorepo/aggregate.go` - MonorepoRelease aggregate
- `internal/domain/monorepo/package_release.go` - PackageRelease entity
- `internal/domain/monorepo/events.go` - Domain events
- `internal/domain/monorepo/repository.go` - Repository interface
- `internal/application/monorepo/service.go` - Application service
- `internal/application/monorepo/version_writers.go` - Version file writers
- `internal/cli/packages.go` - CLI commands

### Modified Files

- `internal/config/schema.go` - Add MonorepoConfig
- `internal/cli/plan.go` - Add --package, --monorepo flags
- `internal/cli/bump.go` - Add --package, --monorepo flags
- `internal/cli/notes.go` - Add --package, --monorepo flags
- `internal/cli/publish.go` - Add --package, --monorepo flags
- `internal/mcp/server.go` - Add package tools
- `internal/mcp/adapters.go` - Add monorepo adapter methods
