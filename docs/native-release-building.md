# Native Release Building Design

## Overview

Replace GoReleaser dependency with native ReleasePilot capabilities for self-releasing. This ensures strategic independence and demonstrates that ReleasePilot can fully manage its own releases.

## Current State (with GoReleaser)

GoReleaser handles:
- Cross-platform binary compilation (Linux, Darwin, Windows for amd64/arm64)
- Archive creation (tar.gz, zip)
- Checksum generation (SHA256)
- GitHub release asset upload
- Package manager publishing (Homebrew, Debian, RPM, Alpine)
- Docker image building and pushing
- SBOM generation
- Artifact signing (cosign)

## Target State (Native)

ReleasePilot will handle all release artifact creation and distribution through:

### 1. Build Service
**Purpose:** Cross-platform binary compilation

**Implementation:**
```go
package build

type Config struct {
    Binary   string   // Binary name
    Main     string   // Main package path
    GOOS     []string // Target operating systems
    GOARCH   []string // Target architectures
    LDFlags  []string // Linker flags
    Tags     []string // Build tags
    Env      []string // Build environment
}

type Builder struct {
    config Config
}

func (b *Builder) BuildAll(ctx context.Context, version string) ([]Artifact, error)
func (b *Builder) Build(ctx context.Context, goos, goarch, version string) (*Artifact, error)
```

**Key Features:**
- Use `go build` with CGO_ENABLED=0 for static binaries
- Support ldflags for version injection: `-X main.version={{.Version}}`
- Parallel builds for faster compilation
- Build matrix: ignore incompatible combinations (e.g., windows/arm64)

### 2. Archive Service
**Purpose:** Create distribution packages

**Implementation:**
```go
package archive

type Archiver interface {
    Create(ctx context.Context, artifacts []build.Artifact, dest string) error
}

type TarGzArchiver struct{}
type ZipArchiver struct{}

func NewArchiver(format string) Archiver
```

**Key Features:**
- tar.gz for Unix systems (Linux, macOS)
- zip for Windows
- Include README, LICENSE, CHANGELOG in archives
- Name template: `{project}_{OS}_{arch}.{ext}`

### 3. Checksum Service
**Purpose:** Generate artifact checksums

**Implementation:**
```go
package checksum

type Generator struct{}

func (g *Generator) Generate(ctx context.Context, files []string) (map[string]string, error)
func (g *Generator) WriteChecksumFile(checksums map[string]string, dest string) error
```

**Key Features:**
- SHA256 algorithm (industry standard)
- checksums.txt format: `{hash}  {filename}`
- Optional signing of checksum file

### 4. Asset Upload (GitHub Plugin Enhancement)
**Purpose:** Upload release artifacts to GitHub

**Current:** GitHub plugin only creates releases
**Enhancement:** Add asset upload capability

**Implementation:**
```go
// In plugins/github/plugin.go

type Config struct {
    // ...existing fields...
    Assets      []string `json:"assets,omitempty"`
    AssetGlob   string   `json:"asset_glob,omitempty"` // e.g., "dist/*"
}

func (p *Plugin) uploadAssets(ctx context.Context, releaseID int64, assets []string) error
```

**Key Features:**
- Upload binaries, archives, checksums
- Support glob patterns for asset discovery
- Retry logic for network failures
- Progress reporting

### 5. Package Manager Plugins
**Purpose:** Publish to package registries

**Homebrew Plugin:**
```go
// plugins/homebrew/plugin.go

type Config struct {
    TapRepo     string   `json:"tap_repo"`     // e.g., "felixgeelhaar/homebrew-tap"
    Formula     string   `json:"formula"`      // Formula name
    Homepage    string   `json:"homepage"`
    Description string   `json:"description"`
    License     string   `json:"license"`
    Binaries    []string `json:"binaries"`     // Binaries to install
}

func (p *Plugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error)
```

**Debian/RPM Plugin:**
```go
// plugins/linuxpkg/plugin.go (enhance existing)

- Build .deb packages using dpkg-deb
- Build .rpm packages using rpmbuild
- Build .apk packages for Alpine
```

### 6. Docker Plugin
**Purpose:** Build and push Docker images

```go
// plugins/docker/plugin.go

type Config struct {
    Images      []string          `json:"images"`      // Image tags
    Dockerfile  string            `json:"dockerfile"`
    BuildArgs   map[string]string `json:"build_args"`
    Labels      map[string]string `json:"labels"`
    Platforms   []string          `json:"platforms"`   // Multi-arch support
}
```

### 7. Signing Service (Optional)
**Purpose:** Sign release artifacts

```go
package signing

type Signer interface {
    Sign(ctx context.Context, artifact string) (signature string, err error)
}

type CosignSigner struct{} // Uses sigstore/cosign
type GPGSigner struct{}    // Uses GPG
```

## Architecture

```
releasePilot/
├── internal/
│   ├── build/           # Cross-platform builder
│   │   ├── builder.go
│   │   ├── matrix.go    # Build matrix management
│   │   └── artifact.go
│   ├── archive/         # Archive creator
│   │   ├── archiver.go
│   │   ├── targz.go
│   │   └── zip.go
│   ├── checksum/        # Checksum generator
│   │   └── generator.go
│   └── signing/         # Artifact signing
│       ├── signer.go
│       ├── cosign.go
│       └── gpg.go
└── plugins/
    ├── github/          # Enhanced with asset upload
    ├── homebrew/        # New: Homebrew tap publishing
    ├── docker/          # New: Docker image building
    └── linuxpkg/        # Enhanced with deb/rpm/apk building
```

## Configuration

**Release Config:**
```yaml
versioning:
  strategy: conventional
  tagprefix: v
  gittag: true
  gitpush: true

build:
  binary: release-pilot
  main: ./cmd/release-pilot
  platforms:
    - os: linux
      arch: [amd64, arm64]
    - os: darwin
      arch: [amd64, arm64]
    - os: windows
      arch: [amd64]
  ldflags:
    - "-s -w"
    - "-X main.version={{.Version}}"
    - "-X main.commit={{.Commit}}"
    - "-X main.date={{.Date}}"
  additional_binaries:
    - binary: release-pilot-github
      main: ./plugins/github
    - binary: release-pilot-npm
      main: ./plugins/npm

archives:
  format: tar.gz
  format_overrides:
    windows: zip
  files:
    - README.md
    - LICENSE
    - CHANGELOG.md

checksums:
  algorithm: sha256
  filename: checksums.txt

plugins:
  - name: github
    enabled: true
    config:
      owner: felixgeelhaar
      repo: release-pilot
      assets:
        - "dist/*.tar.gz"
        - "dist/*.zip"
        - "dist/checksums.txt"

  - name: homebrew
    enabled: true
    config:
      tap_repo: felixgeelhaar/homebrew-tap
      formula: release-pilot
      binaries:
        - release-pilot
        - release-pilot-github
        - release-pilot-npm

  - name: docker
    enabled: true
    config:
      images:
        - "ghcr.io/felixgeelhaar/release-pilot:{{.Version}}"
        - "ghcr.io/felixgeelhaar/release-pilot:latest"
      platforms:
        - linux/amd64
        - linux/arm64
```

## Implementation Plan

### Phase 1: Core Building (Week 1-2)
- [ ] Implement Build Service with cross-compilation
- [ ] Implement Archive Service (tar.gz, zip)
- [ ] Implement Checksum Service
- [ ] Add `release-pilot build` command

### Phase 2: Asset Distribution (Week 3-4)
- [ ] Enhance GitHub plugin with asset upload
- [ ] Test end-to-end release with asset upload
- [ ] Update release workflow to use native building

### Phase 3: Package Managers (Week 5-6)
- [ ] Implement Homebrew plugin
- [ ] Enhance linuxpkg plugin for deb/rpm/apk
- [ ] Implement Docker plugin

### Phase 4: Advanced Features (Week 7-8)
- [ ] Implement signing service (cosign, GPG)
- [ ] SBOM generation
- [ ] Performance optimization
- [ ] Documentation

### Phase 5: Migration (Week 9)
- [ ] Remove GoReleaser from CI/CD
- [ ] Update documentation
- [ ] Release v2.0.0 using ReleasePilot to build itself

## Benefits

1. **Strategic Independence:** No dependency on competitor tools
2. **Self-Sufficient:** ReleasePilot releases itself
3. **Customization:** Full control over build and release process
4. **Integration:** Native integration with ReleasePilot workflow
5. **Dogfooding:** Demonstrates confidence in our own tool
6. **No License Concerns:** Complete control over licensing

## Testing Strategy

1. **Unit Tests:** Each service independently tested
2. **Integration Tests:** End-to-end release workflow
3. **Compatibility Tests:** All platform combinations
4. **Regression Tests:** Compare outputs with GoReleaser
5. **Dogfooding:** Use for ReleasePilot's own releases

## Success Metrics

- [ ] ReleasePilot v2.0.0 released using itself
- [ ] No GoReleaser dependency
- [ ] All artifacts match GoReleaser quality
- [ ] Release time < 10 minutes
- [ ] Zero manual steps required

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Go toolchain changes | Pin Go version, test across versions |
| Platform-specific bugs | Comprehensive test matrix |
| Archive format issues | Follow standards, test extraction |
| Package manager rejections | Follow each platform's guidelines |
| Performance regression | Benchmark and optimize |

## Future Enhancements

- Multi-language support (Python, Node.js, Rust)
- Cloud storage integration (S3, GCS)
- CDN distribution
- Binary signing verification tools
- Release analytics and metrics
