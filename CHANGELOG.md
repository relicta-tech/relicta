# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
- Initial release of ReleasePilot
- Core release management functionality
- Basic plugin architecture
- CLI interface with Cobra

[Unreleased]: https://github.com/felixgeelhaar/release-pilot/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/felixgeelhaar/release-pilot/releases/tag/v0.1.0
