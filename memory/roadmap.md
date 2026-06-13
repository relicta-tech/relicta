---
updated: 2026-06-13
---
## Now

## Next

## Later
- Earned-trust model: replace manual Actor.TrustLevel with reputation/identity-driven escalation (depends on attribution wiring)
- Decompose internal/mcp/server.go (2417 LOC monolith, 30+ handlers)
- Darwin plugin sandbox: container/cgroup runner (RLIMIT_AS unenforced on Apple Silicon)
- CLI exit-code path test coverage (os.Exit skips)

## Done
- v4.1.0 release (risk calibration, actor-reputation guarding, security hardening)
- Wire attribution detection into evaluation (machine authors govern human-initiated releases)
- Configurable reputation probation threshold
- Post-calibration accuracy validation (warn / strict fail-closed)
- E2E EvaluateRelease integration test (calibration + reputation + attribution)
