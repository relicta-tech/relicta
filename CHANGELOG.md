# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.9.0] - 2025-12-23

### Features

- **mcp:** add plugin integration via MCP protocol (27a10a4)
- **mcp:** add multi-repository support for MCP (aa9961e)
- **mcp:** add streaming support for long operations (90a73a9)
- **mcp:** add Client SDK for AI agents (61674a1)


## [2.5.0] - 2025-12-18

### Features

- **security:** implement quality review P1 improvements (ee75aef)
- **build:** add pre-commit hooks to reduce CI costs (2128c5f)

### Bug Fixes

- **security:** correct decompression bomb size check (daf6845)
- **ci:** exclude gosec false positives from security scan (2bcbf48)
- **security:** add #nosec comments for all gosec false positives (4e8ca4e)
- **security:** add #nosec comments for false positive gosec alerts (6b60658)
- **lint:** remove unused nolint directive for gosec G115 (e92ae0c)
- **bump:** update release state when using --force flag (258a1b6)
- **security:** handle all unhandled errors flagged by CodeQL (b814e30)


## [2.4.0] - 2025-12-18

### Features

- **security:** add GitHub artifact attestations for supply chain security (5d7f556)

### Bug Fixes

- **plugin:** verify archive checksums before extraction (71a0baa)


## [2.3.0] - 2025-12-17

### Features

- **plugin:** add development mode with file watching (f96b38d)
- **plugin:** add search and update commands (4407043)
- **plugin:** add checksum verification and SDK compatibility (cee6565)
- **plugin:** add plugin template generator command (6139b20)
- **plugin:** add support for multiple plugin registries (2d58822)

### Bug Fixes

- **plugin:** support both plugin naming conventions (23356b6)
- **plugin:** find platform-specific binary names in archives (e652a12)
- **plugin:** support compressed archives for plugin installation (c027e79)


## [2.2.0] - 2025-12-17

### Features

- **cgp:** implement Change Governance Protocol for release management (#7) (f7cca96)


## [2.1.0] - 2025-12-15

### Features

- add brand assets and update README with logo (700f062)

### Bug Fixes

- **ai:** make API key optional for Ollama and auto-detect repository URL (7866844)


## [2.1.0] - 2025-12-15

### Features

- add brand assets and update README with logo (700f062)

### Bug Fixes

- **ai:** make API key optional for Ollama provider (cc8d2f1)


## [1.2.4] - 2025-12-12

### Bug Fixes

- **build:** disable docker builds in goreleaser (83294a6)


## [1.2.3] - 2025-12-12

### Bug Fixes

- **build:** disable sboms and signing in goreleaser (64f917e)


## [1.2.2] - 2025-12-12

### Bug Fixes

- **jira:** use ADF format for issue comments (7d0e199)


## [1.2.1] - 2025-12-12

### Bug Fixes

- **build:** temporarily disable gitlab and jira plugins (f98c590)


## [1.2.0] - 2025-12-11

### Features

- **plugins:** implement CLI-based plugin management system (e02acc4)
- **plugin:** implement plugin management infrastructure (Phase 1) (225840b)

### Bug Fixes

- remove unused import in plugin.go (e4538f7)
- **security:** address CodeQL warnings (78e1ae0)
- **release:** remove deprecated folder field from homebrew config (b6e8760)


## [1.1.0] - 2025-12-11

### Features

- **plugins:** add language package registry plugins (365c2b4)
- **plugins:** add package registry plugins for MVP Nice-to-Have scope (caf0198)
- **jira:** migrate to jirasdk and expand PRD plugin roadmap (dd8f9cd)

### Bug Fixes

- **ci:** downgrade to Go 1.24.11 for golangci-lint compatibility (9343328)
- **security:** upgrade go-jira to fix jwt vulnerabilities (f7ab5d7)
- **ci:** fix test failures and security scan SARIF upload (ad39fbd)
- resolve lint errors and formatting issues (e6e6d04)


## [1.0.0] - 2025-12-11

### ⚠ BREAKING CHANGES

- **ai:** Removed custom RateLimiter type in favor of Fortify (9b8efc6)

### Features

- **ai:** integrate Fortify resilience library for AI services (9b8efc6)


## [0.2.1] - 2025-12-11

### Bug Fixes

- **git:** add CLI fallback for push operations and auth configuration (b82fe3d)
- prevent duplicate changelog headers when updating file (ec2513a)
- skip tag creation in publish if already exists (2a22802)


## [0.2.0] - 2025-12-11

### Features

- persist changeset with release state for notes generation (28ff8f4)

### Bug Fixes

- update release state after version bump (5ff7ddf)
- add git CLI fallback for working tree status check (4c1cc12)


## [Unreleased]

### Added
- Domain-Driven Design architecture with hexagonal/clean architecture patterns
- Release workflow management with state machine (init, plan, version, notes, approve, publish)
- Conventional commit parsing and semantic version calculation
- AI-powered changelog and release notes generation via OpenAI
- Plugin system using HashiCorp go-plugin (gRPC) for extensibility
- Official plugins: GitHub (releases), npm (publish), Slack (notifications)
- Comprehensive CLI with commands: init, plan, bump, notes, approve, publish, health
- State persistence for release workflow continuity
- Template system for customizable changelog and release notes
- Health check command with JSON output for monitoring
- Multi-platform builds (Linux, macOS, Windows) for amd64 and arm64
- Docker support with multi-stage builds and health checks
- Comprehensive CI/CD with GitHub Actions
- Security scanning with CodeQL, Gosec, Trivy, Gitleaks, and TruffleHog
- SBOM generation and artifact signing with Cosign

### Security
- Command injection prevention with editor whitelist validation
- JSON deserialization size limits to prevent DoS attacks
- Atomic file writes to prevent data corruption
- Path traversal protection in file operations
- SSRF protection in plugin configurations
- Thread-safe ChangeSet with proper mutex synchronization
- Secure file permissions (0600 for sensitive files, 0700 for directories)

### Fixed
- Race conditions in ChangeSet methods (Commits, CommitCount, IsEmpty, ReleaseType, etc.)
- Nil pointer dereference in SetPlan when ChangeSet is nil
- Nil pointer dereference in toDTO when ChangeSet is nil
- Error.Is() implementation to use type assertion instead of errors.As()
- Lock held during I/O operations in state manager (now uses atomic writes)

## [0.1.0] - 2025-12-08

### Added
- Initial release of Relicta
- Core release management functionality
- Basic plugin architecture
- CLI interface with Cobra

[Unreleased]: https://github.com/relicta-tech/relicta/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/relicta-tech/relicta/releases/tag/v0.1.0
