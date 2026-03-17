# Multi-Package/Monorepo Versioning Design

## Overview

Independent versioning for packages within a monorepo, enabling:
- Per-package semantic versioning
- Coordinated multi-package releases
- Automatic version file updates (package.json, go.mod, Cargo.toml, etc.)
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

### Configuration

```yaml
# .relicta.yaml
monorepo:
  enabled: true
  strategy: independent  # independent | lockstep | hybrid

  # Package discovery (extends blast_radius config)
  packages:
    paths:
      - "packages/*"
      - "plugins/*"
    exclude:
      - "packages/internal-*"

  # Per-package overrides
  package_overrides:
    packages/core:
      tag_prefix: "core-v"
      version_file: package.json
    packages/cli:
      tag_prefix: "cli-v"
      version_file: package.json

  # Version file patterns
  version_files:
    go_module:
      file: "go.mod"
      pattern: "^module"  # Go modules don't store version in go.mod
      update: false       # Use git tags only
    npm:
      file: "package.json"
      field: "version"
      update: true
    cargo:
      file: "Cargo.toml"
      field: "version"
      update: true
    python:
      files: ["setup.py", "pyproject.toml", "__version__.py"]
      update: true

  # Coordinated releases
  release_groups:
    - name: "core"
      packages: ["packages/core", "packages/shared"]
      strategy: lockstep  # All packages in group share same version
    - name: "plugins"
      packages: ["plugins/*"]
      strategy: independent

  # Changelog settings
  changelog:
    per_package: true     # Generate per-package changelogs
    root_changelog: true  # Also generate root CHANGELOG.md
    format: conventional
```

## Implementation Plan

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

## API Design

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
