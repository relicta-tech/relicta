# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

# Release Notes for Project Version 4.0.1

Enhanced cognitive backends with tests, benchmarks, CLI flags, and CI improvements.

## 1. New Features and Enhancements

- Add integration tests for Mnemos and Chronos adapters
- Add benchmarks for both cognitive backends
- Add `--skip-cognitive` flag to `relicta plan` command
- Add `/health/cognitive` endpoint to HTTP server
- Refactor CI workflow (merge lint+test, remove duplicates)
- Refactor Homebrew formula to `on_intel`/`on_arm` blocks
- Add `relicta demo` CLI command with Docker Compose management
- Add `--log` as alias for `--log-level` with runtime configuration
- Add `relicta version --cognitive` probe for backend health
- Add `make changelog` target for automated changelog generation
- Enforce Nox security scan in CI (removed `|| true`)
- Add `--chronos-threads` flag with semaphore-based concurrency control
- Make Mnemos adapter fail-soft with graceful degradation

## 2. Changes

- Update Chronos adapter to support configurable ingest concurrency
- Wire Chronos thread config through container and CLI flags
- Add missing dependency: `github.com/rs/zerolog`
- Fix test compilation issues (actor types, constructor signatures)
- Update documentation: `docs/cli-new-features.md`

# Release Notes for Project Version 4.0.0

We are excited to announce the release of version 4.0.0 of Relicta — the “go big” release that adds cognitive backends for release memory and pattern detection.

## 1. New Features and Enhancements

This major release introduces two optional cognitive backends that make Relicta self‑learning:

- **Mnemos integration** — optional memory backend that stores governance decisions, approvals, and release events as claims with evidence, enabling historical queries and contradiction detection
- **Chronos integration** — optional pattern detection backend that analyzes time‑series signals (trend, spike, drop, stall, anomaly) in release metrics
- **Post‑v3.5.0 features** — approval cards with visual audit trails, actor budgets and risk caps, architecture tests, autonomy profiles, compliance modules, MCP resources, eval harness, Playwright tests, and web a11y improvements

## 2. Changes

- Add `mnemos` and `chronos` config structs and schema defaults
- Wire Mnemos and Chronos adapters into the dependency injection container
- Update `DefaultConfig()` with sensible defaults for both backends
- Add `MnemosStore()` and `ChronosClient()` accessors in the container
- Fix CGP field mappings in Mnemos adapter (`Decision`, `ApprovedBy`, `AllowedSteps`)
- Add dedicated `docs/cognitive-backends.md` setup guide
- Update `docs/governance.md` and `docs/quick-start.md` with Mnemos/Chronos config snippets

## 3. Documentation

- New `docs/cognitive-backends.md` with run commands for `mnemos serve` and `chronos serve`
- Updated governance and quick‑start guides with optional backend configuration examples

For more details, you can refer to the complete changelog [here](https://github.com/relicta-tech/relicta/compare/v3.5.0..v4.0.0).

# Release Notes for Project Version 3.5.0

We are excited to announce the release of version 3.5.0 of our project. In this release, we have worked on implementing significant enhancements, addressing security issues, and providing updated documentation. The highlights of this release include new middleware with mcp-go v1.9.0, phase scaling, Relicta v3 Change Governance Platform, production hardening features, and session authentication with JWT tokens. In addition, we have fixed several issues, updated dependencies, and made improvements to the quality of our codebase.

## 1. New Features and Enhancements

The cornerstone of this release is the new middleware, elicitation, and discovery that comes with mcp-go v1.9.0. We have also added Phase 3, 2, and 1 scaling, each bringing unique features to enable org risk aggregation, actor identity, and risk budgets, respectively. 

The Relicta v3 Change Governance Platform has been introduced in this release. This platform will revolutionize the way you handle changes in your project. 

We have also added some significant production hardening features: rollback, fuzzing, SBOM, and test coverage. These features will help you ensure that your application is robust and reliable.

Additionally, we have introduced OIDC provider config and API version negotiation to the dashboard, session authentication with JWT tokens, and SLSA governance attestation with verify command. 

## 2. Fixed Issues

We have addressed several issues in this release. These include fixing benchmark action permissions and threshold issues in the CI, a Nox installation issue, and CodeQL replacement with nox and benchmark action SHA fix in the CI. 

We have also fixed a hot-reloading configuration issue and stabilized local plugin path validation. Security fixes include addressing OIDC error leakage, JSON injection, adapter data race issues, auditing findings, and patching go-git and MCP SDK security vulnerabilities.

## 3. Changes

In our commitment to excellence, we have made several changes to upgrade our project. We have achieved nox grade A by setting baseline false positives and adding .noxignore. The README has been updated with the nox grade badge SVG. Several brand assets have been restored, and nox and coverctl badges have been added. 

All dependencies have been updated to the latest versions, and Go version has been bumped to 1.25. We have also ignored security scan artifacts and upgraded various dependencies across the project.

## 4. Security

In this release, we have made significant strides in improving the security of our application. We have addressed OIDC error leakage, JSON injection, adapter data race issues, and audit findings across the codebase. Go-git and MCP SDK security vulnerabilities have been patched.

## 5. Documentation

The documentation has been enriched with the addition of the Relicta Hub SaaS PRD, a scaling strategy, and landing page enrichment. The README has been rewritten and augmented with dashboard views and pre-commit hooks. We have also marked all policy engine enhancements as completed in the backlog.

We believe this release brings valuable enhancements that will significantly improve your productivity and efficiency. We look forward to your feedback.

For more details, you can refer to the complete changelog [here](https://github.com/your_project/compare/v3.4.0..v3.5.0).

## Changelog

# Changelog

- ci: consolidate all security scanning on nox (#133)
- ci: bump the github-actions group across 1 directory with 18 updates (#121)
- fix: MCP governance bugs — dry-run state poisoning, ai=false ignored, replan path, no-op fallbacks (#132)
- ci: fix dangling job reference breaking workflow parse (#129)
- chore(deps): sync web lockfile and consolidate npm dependabot bumps (#131)
- deps: bump the go-dependencies group across 1 directory with 14 updates (#125)
- build: migrate Klarlabs library deps to go.klarlabs.de vanity paths (#123)
- docs: update version references to v4.0.1


- fix(cli): approve --ci actually approves instead of dumping status JSON (#138)
- fix: declare /v4 module path and restore version stamping (#137)


## [3.5.0] - 2022-04-21

### Added

- New middleware, elicitation, and discovery with mcp-go v1.9.0 (mcp).
- Phase 3 scaling: org risk aggregation, agent reputation, supply chain governance.
- Phase 2 scaling: actor identity, compliance reports, governance analytics.
- Phase 1 scaling: risk budgets, incident correlation, weight calibration.
- Relicta v3 Change Governance Platform.
- Initial project setup.
- Multi-repo governance and runtime observability.
- Production hardening features: rollback, fuzzing, SBOM, test coverage.
- Phase 2B Governance Intelligence.
- Phase 2A Enterprise Foundation.
- OIDC provider config and API version negotiation to the dashboard.
- Session authentication with JWT tokens (auth).
- Phase 2A Enterprise Foundation spec and plan (roady).
- SLSA governance attestation with verify command (attestation).
- Fixture scaffolding command and workflows (policy).

### Fixed

- Benchmark action permissions and threshold issues in the CI.
- Nox installation issue in the CI.
- CodeQL replacement with nox and benchmark action SHA fix in the CI.
- Lint and coverage gate failures in the CI.
- All remaining review findings across the codebase.
- OIDC error leakage, JSON injection, adapter data race issues (security).
- Audit findings across the codebase (security).
- Security findings and a hanging test.
- Upgraded google.golang.org/grpc to v1.79.3 (deps).
- Resolved a hot-reloading configuration issue (#83).
- Patched go-git and MCP SDK security vulnerabilities (deps).
- Moved gopkg.in/yaml.v3 to third-party import group in policy.go (lint).
- Stabilized local plugin path validation (#53) (plugin).
- Replaced dots with underscores in tool names for Claude Desktop (mcp).

### Changed

- Achieved nox grade A by setting baseline false positives and adding .noxignore.
- Updated README with nox grade badge SVG.
- Restored brand assets and added nox and coverctl badges.
- Updated all dependencies to the latest versions (deps).
- Ignored security scan artifacts.
- Bumped go-dependencies and Go version to 1.25.
- Upgraded various dependencies across the project (deps).

### Deprecated

_No deprecated items in this release._

### Removed

_No removed items in this release._

### Security

- Addressed OIDC error leakage, JSON injection, adapter data race issues (security).
- Addressed audit findings across the codebase (security).
- Resolved security findings and fixed a hanging test.
- Patched go-git and MCP SDK security vulnerabilities (deps).

### Documentation

- Added Relicta Hub SaaS PRD.
- Added scaling strategy and landing page enrichment.
- Rewrote README and added dashboard views and pre-commit hooks (#80).
- Marked all policy engine enhancements as completed (backlog).

[3.5.0]: https://github.com/your_project/compare/v3.4.0..v3.5.0

## [3.4.5] - 2026-02-01

### Chores

- **gitignore:** ignore stale `.relicta.yaml` in `internal/mcp/` (8a6c204)

## [3.4.4] - 2026-02-01

### Tests

- **persistence:** add changeset round-trip tests for file repository (c56fa6c)

## [3.4.3] - 2026-02-01

### Chores

- **release:** remove provider-specific openai-only build variant (ee08e6b)
- **release:** clean up openai-only assets from v3.4.2 and v3.4.1
- **release:** delete draft v3.3.4 release

## [3.4.2] - 2026-02-01

### Bug Fixes

- **persistence:** persist changeset data in FileReleaseRunRepository (bc911a8)
  - The file repository only serialized `changeset_id` but never the actual changeset data
  - All multi-command workflows (`plan → bump → notes`) were broken in both CLI and MCP
  - Only `relicta release` worked because it kept the changeset in memory
  - Closes #47

## [Unreleased]

### Bug Fixes

- **cli:** transition release state to Versioned after bump (b2c4073)
- **release:** cache plan with changeset when loading from persistence (2568b3c)
- **release:** fix List() incorrectly including .state.json and .machine.json files (840a6b4)

## [2.10.0] - 2025-12-25

### Features

- **cgp:** wire event publisher chain and add policy CLI tests (c59c63e)
- **cgp:** add team-based approval policies with time context (46dd87f)
- **cgp:** add release outcome tracking, history CLI, and event webhooks (ace12ec)
- **cgp:** add policy DSL, audit trail persistence, and MCP improvements (6e21f3e)
- **mcp:** improve MCP tools with better workflow guidance (70bac17)


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
