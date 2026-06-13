---
updated: 2026-06-13
---
## Current State
Two branches open. `chore/agent-os-memory` → PR #158 (Tier A: attribution wiring,
configurable reputation/calibration). `feat/earned-trust` (branched off #158) →
Tier B earned-trust model, ready to push/PR.

## Last Session Summary
After Tier A (PR #158), built Tier B earned-trust: reputation-driven trust
escalation. Strong verifiable record raises an actor's effective trust (10 samples
+ rep≥0.8 → trusted; 50 + rep≥0.95 + non-declining → full), unlocking low-risk
auto-approval. Opt-in, escalation-only, necessary-not-sufficient. Tests + E2E
(earned trust unlocks auto-approve, baseline stays gated) + docs. All green.

## Next Session Should
Push feat/earned-trust + open PR (stacked on #158). Then remaining Tier B:
MCP decomposition, darwin sandbox, CLI exit-code coverage. Or merge #158 first.

## Blocked / Waiting
feat/earned-trust is stacked on #158 — merge #158 before earned-trust, or rebase
earned-trust onto main after #158 lands. Pre-existing macOS git test flake unrelated.
