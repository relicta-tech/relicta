# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the Relicta project.

ADRs document significant architectural decisions made during the development of this project.

## Index

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [ADR-001](001-hexagonal-architecture.md) | Hexagonal Architecture with DDD | Accepted | 2024-01 |
| [ADR-002](002-plugin-system.md) | HashiCorp go-plugin for Extensibility | Accepted | 2024-01 |
| [ADR-003](003-state-machine.md) | XState-compatible State Machine for Release Workflow | Accepted | 2024-02 |
| [ADR-004](004-conventional-commits.md) | Conventional Commits for Version Calculation | Accepted | 2024-02 |
| [ADR-005](005-ai-integration.md) | Multi-provider AI Integration | Accepted | 2024-03 |
| [ADR-006](006-mcp-protocol.md) | Model Context Protocol for AI Agent Integration | Accepted | 2024-06 |
| [ADR-007](007-interface-service-layer.md) | All Interfaces Must Use Application Services Layer | Accepted | 2025-01 |

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
