# ADR-007: All Interfaces Must Use Application Services Layer

## Status

Accepted

## Date

2025-01-01

## Context

Relicta exposes multiple interfaces for interacting with release governance:
- **CLI** (`cmd/relicta`, `internal/cli`) - Primary user interface
- **MCP** (`internal/mcp`) - AI agent integration via Model Context Protocol
- **HTTP API** (`internal/httpserver`) - Dashboard and webhook integrations

These interfaces need to perform the same operations: plan releases, bump versions, generate notes, approve, and publish. The operations involve:
- State machine transitions
- Repository persistence
- Audit trail recording
- Plugin execution

### The Problem

The MCP adapter was implemented with direct repository access instead of using the application services layer:

```go
// What was built (WRONG):
type Adapter struct {
    releaseRepository ports.ReleaseRunRepository  // Direct infrastructure access
    analyzer          *analysis.CommitAnalyzer
}

func (a *Adapter) Plan(ctx context.Context, input PlanInput) (*PlanOutput, error) {
    // Analyzed commits but never persisted state!
    commits := a.analyzer.Analyze(...)
    return &PlanOutput{Commits: commits}, nil
}
```

This caused:
- **Issue #30**: State inconsistency between `relicta_status` and `relicta_plan`
- **Issue #31**: State transition errors prevent approve after plan
- **Issue #32**: Notes generation fails with 'cannot set notes in state published'

The CLI worked correctly because it used the application services layer:

```go
// CLI approach (CORRECT):
func (c *PlanCommand) Run() error {
    output, err := c.services.PlanRelease.Execute(ctx, input)
    // State is persisted, transitions are validated
}
```

## Decision

**All external interfaces (CLI, MCP, HTTP) MUST use the `release.Services` application layer, not infrastructure directly.**

### Implementation Requirements

1. **Service Injection**: Interfaces must receive `*release.Services`, not `ports.ReleaseRunRepository`:

```go
// Correct pattern:
type Adapter struct {
    releaseServices *release.Services  // Application services layer
}

func NewAdapter(services *release.Services, opts ...Option) *Adapter {
    return &Adapter{releaseServices: services}
}
```

2. **Use Case Calls**: Operations must call use cases, not implement logic:

```go
// Correct pattern:
func (a *Adapter) Plan(ctx context.Context, input PlanInput) (*PlanOutput, error) {
    result, err := a.releaseServices.PlanRelease.Execute(ctx, releaseapp.PlanReleaseInput{
        RepoRoot: a.repoRoot,
        // ... map input
    })
    return mapToOutput(result), err
}
```

3. **No Infrastructure Bypass**: Interfaces must not:
   - Access repositories directly
   - Implement state transitions
   - Skip audit logging
   - Bypass validation

### Architecture Layers

```
┌─────────────────────────────────────────────────────────────┐
│                    External Interfaces                       │
│  ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐  │
│  │   CLI   │    │   MCP   │    │  HTTP   │    │ Webhook │  │
│  └────┬────┘    └────┬────┘    └────┬────┘    └────┬────┘  │
│       │              │              │              │        │
│       └──────────────┴──────────────┴──────────────┘        │
│                           │                                  │
│                           ▼                                  │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Application Services Layer              │   │
│  │  release.Services (PlanRelease, BumpVersion, etc.)   │   │
│  └─────────────────────────┬───────────────────────────┘   │
│                            │                                 │
│                            ▼                                 │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                   Domain Layer                        │   │
│  │     State Machine, Aggregates, Domain Events         │   │
│  └─────────────────────────┬───────────────────────────┘   │
│                            │                                 │
│                            ▼                                 │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Infrastructure Layer                     │   │
│  │      Repositories, Git Adapter, Plugin Manager       │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Consequences

### Positive

1. **Consistency**: All interfaces behave identically
2. **Single Source of Truth**: Business logic lives in one place
3. **Testability**: Integration tests can verify cross-interface consistency
4. **Maintainability**: Bug fixes apply to all interfaces automatically
5. **Audit Trail**: All operations are logged consistently

### Negative

1. **Indirection**: Interfaces can't optimize for their specific needs
2. **Coupling**: Interfaces depend on `release.Services` structure
3. **Initial Effort**: Existing interfaces need refactoring

### Enforcement

The following enforcement mechanisms are implemented:

1. **Integration Tests** (`test/integration/mcp_workflow_test.go`):
   - `TestMCPWorkflowE2E`: Verifies full workflow uses DDD use cases
   - `TestMCPAndCLIStateConsistency`: Verifies MCP and CLI share state
   - `TestMCPAdapterRequiresServices`: Verifies operations fail without services
   - `TestMCPStateTransitionsAreEnforced`: Verifies state machine rules

2. **API Deprecation Notices** (`internal/mcp/adapters.go`):
   - `WithCalculateVersionUseCase`: Deprecated, use `WithReleaseServices`
   - `WithSetVersionUseCase`: Deprecated, use `WithReleaseServices`
   - `WithAdapterReleaseRepository`: Deprecated, violates ADR-007

3. **Dual-Path Implementation**:
   - Operations check for `releaseServices` first (ADR-007 compliant)
   - Legacy fallback exists for backward compatibility (deprecated)
   - Example: `Bump()` calls `bumpViaDDD()` if services configured

4. **Code Review Checklist**:
   - New interface methods must use `a.releaseServices.<UseCase>.Execute()`
   - No direct repository access (`a.releaseRepo.*`) in new code
   - All state transitions via domain aggregate methods

## Related Decisions

- ADR-001: Hexagonal Architecture with DDD
- ADR-003: XState-compatible State Machine for Release Workflow
