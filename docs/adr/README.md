# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the Relicta project.

ADRs document significant architectural decisions made during the development of this project.

## Index

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| ADR-001 | Hexagonal Architecture with DDD | Accepted, not written up | 2024-01 |
| ADR-002 | HashiCorp go-plugin for Extensibility | Accepted, not written up | 2024-01 |
| ADR-003 | XState-compatible State Machine for Release Workflow | Accepted, not written up | 2024-02 |
| ADR-004 | Conventional Commits for Version Calculation | Accepted, not written up | 2024-02 |
| ADR-005 | Multi-provider AI Integration | Accepted, not written up | 2024-03 |
| ADR-006 | Model Context Protocol for AI Agent Integration | Accepted, not written up | 2024-06 |
| [ADR-007](007-interface-service-layer.md) | All Interfaces Must Use Application Services Layer | Accepted | 2025-01 |
| [ADR-008](008-nox-style-plugin-distribution.md) | Nox-style Plugin Distribution, Trust, and Safety Model | Accepted | 2026-06 |
| [ADR-009](009-deterministic-recommendation-artifact.md) | Relicta Emits a Deterministic Recommendation, Not Prose | Accepted | 2026-08 |
| [ADR-010](010-ai-providers-leave-the-cli.md) | AI Providers Leave the CLI | Accepted, execution phased | 2026-08 |
| [ADR-011](011-governance-on-by-default.md) | Governance On By Default | Accepted (Option C) | 2026-08 |
| [ADR-012](012-deployment-evidence-over-a-protocol.md) | Deployment Evidence Crosses a Protocol, Not a Dependency | Accepted | 2026-08 |
| [ADR-013](013-one-store-behind-a-backend.md) | One Store Behind a Backend, and SQLite Is the Shape of It | Accepted | 2026-08 |
| [ADR-014](014-the-audit-chain-is-appended-not-derived.md) | The Audit Chain Is Appended, Not Derived | Accepted | 2026-08 |
| [ADR-015](015-per-package-versioning-is-independent-only.md) | Per-Package Versioning Is Independent Only | Accepted | 2026-08 |
| [ADR-016](016-no-data-beats-wrong-data.md) | No Data Beats Wrong Data | Accepted | 2026-08 |

> **ADR-001 to ADR-006 have no document.** The decisions were made and are
> implemented, but the records were never committed — the index linked to six
> files that do not exist. They are described in `CLAUDE.md` (architecture, plugin
> system, state machine, conventional commits, AI providers, MCP) and are listed
> here so the gap is visible rather than implied by a broken link. Writing them up
> is tracked in the backlog.

## ADR Template

When creating a new ADR, use the following template:

```markdown
# ADR-XXX: Title

## Status
Proposed | Accepted | Deprecated | Superseded

## Context
What is the issue that we're seeing that is motivating this decision or change?

## Decision
What is the change that we're proposing and/or doing?

## Consequences
What becomes easier or more difficult to do because of this change?
```
