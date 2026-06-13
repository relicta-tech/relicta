---
updated: 2026-06-13
---
## Now

## Next

## Later
- Capability-based identity grants (registry Capabilities action/scope gating, beyond TrustScore→level)
- Decompose internal/mcp/server.go (done on refactor/mcp-server-split → PR #161)
- Darwin plugin sandbox (done via sandbox-exec on feat/darwin-sandbox-exec → PR #162)
- CLI exit-code path test coverage (done on feat/cli-exit-codes → PR #160)

## Done
- v4.1.0 release (risk calibration, actor-reputation guarding, security hardening)
- Wire attribution detection into evaluation (machine authors govern human-initiated releases)
- Configurable reputation probation threshold
- Post-calibration accuracy validation (warn / strict fail-closed)
- E2E EvaluateRelease integration test (calibration + reputation + attribution)
- Earned-trust model: reputation-driven trust escalation (Tier B; tiers 10→trusted, 50→full)
- Identity-registry trust grants (org-assigned trust via actors.json; raise-only, composes with earned trust)
