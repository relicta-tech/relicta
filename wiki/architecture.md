---
updated: 2026-06-13
tags: [architecture]
---
# Architecture

Hexagonal core in `internal/domain/` (release aggregate, value objects, ports).
Application layer orchestrates use-cases. Infrastructure adapters for git, ai, persistence, webhook.
Change Governance Protocol (CGP) in `internal/cgp/` — risk, policy, audit, autoapproval, reputation.

See CLAUDE.md for full directory map and stack table.
