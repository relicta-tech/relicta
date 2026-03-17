# Bug Report: Tag-Push Mode State Transition Error

**Status**: ✅ RESOLVED (GitHub Issue #34)

## Issue Summary

**Error**: `failed to set release notes: invalid state transition: cannot set notes in state planned`

**Reproduction Steps**:
1. Run `relicta bump` first (creates tag v0.3.0, state → `versioned`)
2. Run `relicta plan` (sees HEAD is already tagged, enters "tag-push mode")
3. Run `relicta notes --ai` → **ERROR**: cannot set notes in state `planned`

## Solution Implemented

The fix adds `TagPushMode` and `TagName` fields to `PlanReleaseInput`, allowing the CLI to signal
when HEAD is already tagged. In tag-push mode, the use case transitions directly to `versioned`
state, bypassing the need for the `bump` command.

### Key Changes:
1. **`internal/domain/release/app/plan.go`**: Added `TagPushMode` and `TagName` input fields
2. **`internal/cli/release.go`**: `persistReleasePlan` now passes workflow context to use case
3. **Input validation**: `TagPushMode=true` requires `NextVersion` to be set
4. **Table-driven tests**: Comprehensive test coverage for all tag-push scenarios

## Root Cause Analysis

### 1. State Machine Definition (`machine.go:74-77`)

The state machine requires this transition path:
```
draft → (PLAN) → planned → (BUMP) → versioned → (GENERATE_NOTES) → notes_ready
```

The `GenerateNotes` method in `run.go:756-759` enforces this:
```go
func (r *ReleaseRun) GenerateNotes(notes *ReleaseNotes, inputsHash, actor string) error {
    if r.state != StateVersioned {
        return NewStateTransitionError(r.state, "generate notes")
    }
    // ...
}
```

### 2. The Bug: Tag-Push Mode Creates Wrong State

When HEAD is already tagged, `relicta plan` enters "tag-push mode" and:
- Recognizes the tag exists
- Creates a **new run in `planned` state** instead of `versioned` state
- Tells user: "Since HEAD is already tagged, bump is not needed"

**But the state machine doesn't have a transition:**
```
planned → (GENERATE_NOTES) → notes_ready  // INVALID!
```

The only valid path from `planned` to `notes_ready` goes through `versioned`:
```
planned → (BUMP) → versioned → (GENERATE_NOTES) → notes_ready
```

### 3. Why This Happens

The plan command in tag-push mode doesn't account for the fact that:
- The version bump already happened (tag exists)
- The run should start in `versioned` state, not `planned` state

## DDD Review

### Aggregate Invariant Violation

The `ReleaseRun` aggregate has a clear invariant: notes can only be generated in `versioned` state. This invariant is correctly enforced. However, the **application layer** (plan use case) creates aggregates in an inconsistent state for tag-push mode.

**DDD Principle Violated**: Application services should not create aggregates that violate domain invariants.

### Recommended Fix (DDD Approach)

Option A: **Add factory method for tag-push mode**
```go
// domain/run.go
func NewReleaseRunForTagPush(planID PlanID, version SemVer, headSHA CommitSHA) (*ReleaseRun, error) {
    run := &ReleaseRun{
        id:          NewRunID(),
        planID:      planID,
        headSHA:     headSHA,
        versionNext: version,
        state:       StateVersioned,  // Start in versioned state
    }
    // Record synthetic transitions for audit trail
    run.recordTransition(StateDraft, StatePlanned, "PLAN", "system", "Tag-push mode: tag already exists")
    run.recordTransition(StatePlanned, StateVersioned, "BUMP", "system", "Tag-push mode: using existing tag")
    return run, nil
}
```

Option B: **State machine transition for tag-push mode**
Add transition from `planned` directly to `versioned` with guard `tagAlreadyExists`:
```go
State(StateIDPlanned).
    On(EventBump).Target(StateIDVersioned).Guard(GuardHeadMatches).
    On(EventTagExists).Target(StateIDVersioned).  // NEW: for tag-push mode
    On(EventCancel).Target(StateIDCanceled).
    Done().
```

## Idiomatic Go Review

### 1. Error Wrapping (`run.go:758`)
```go
// Current:
return NewStateTransitionError(r.state, "generate notes")

// More idiomatic with context:
return fmt.Errorf("generate notes: %w", NewStateTransitionError(r.state, "generate notes"))
```

### 2. Guard Pattern (`machine.go:130-138`)
Guards take context by value (correct), but could benefit from named returns for clarity:
```go
func guardHeadMatches(ctx RunContext, _ statekit.Event) (matches bool) {
    if ctx.ForceMode {
        return true
    }
    if ctx.Run == nil {
        return false
    }
    return ctx.Run.HeadSHA() == ctx.CurrentHead
}
```

### 3. State Check Pattern (`run.go:756-759`)
Consider using a specification pattern consistently:
```go
// Current:
if r.state != StateVersioned {
    return NewStateTransitionError(r.state, "generate notes")
}

// More consistent with existing specs:
if !CanGenerateNotes().IsSatisfiedBy(r) {
    return NewStateTransitionError(r.state, "generate notes")
}
```

## Code Review Findings

### 1. State Persistence Issue

When running commands out of order (`bump` before `plan`), the state from `bump` may not be persisted or recognized by subsequent commands. The plan command should:
- Check for existing runs with matching HEAD SHA
- Resume existing runs instead of creating new ones

### 2. Missing State Recovery

Tag-push mode should recover the correct state:
```go
// plan.go (pseudocode)
if headAlreadyTagged {
    run, err := repo.FindByHeadSHA(ctx, headSHA)
    if err == nil && run.State() == StateVersioned {
        // Resume existing run
        return run, nil
    }
    // Create new run in versioned state for tag-push mode
    return NewReleaseRunForTagPush(...)
}
```

### 3. ADR-007 Compliance Gap

ADR-007 states all interfaces must use application services. The MCP adapter now uses `GenerateNotesUseCase`, but the plan use case itself creates runs in incorrect states for tag-push mode.

## Recommended Actions

1. **Immediate Fix**: Add `StateVersioned` as valid initial state for tag-push mode
2. **Short-term**: Implement run recovery when HEAD matches existing versioned run
3. **Long-term**: Add `EventTagExists` transition to state machine
4. **Testing**: Add integration test for tag-push workflow

## Test Case

```go
func TestTagPushModeWorkflow(t *testing.T) {
    // Setup: create git repo with existing tag
    repo := setupRepoWithTag(t, "v1.0.0")

    // Step 1: Plan should recognize tag-push mode
    planOutput, err := services.Plan.Execute(ctx, PlanInput{RepoRoot: repo})
    require.NoError(t, err)
    assert.Equal(t, "tag-push", planOutput.Mode)

    // Step 2: Run should be in versioned state (not planned!)
    run, err := services.Repository.LoadLatest(ctx, repo)
    require.NoError(t, err)
    assert.Equal(t, StateVersioned, run.State())  // FAILS currently

    // Step 3: Notes should work without bump
    notesOutput, err := services.GenerateNotes.Execute(ctx, GenerateNotesInput{...})
    require.NoError(t, err)  // FAILS currently with state transition error
}
```

## References

- ADR-007: All Interfaces Must Use Application Services Layer
- Issue #32: Notes generation fails with 'cannot set notes in state published'
- State machine definition: `internal/domain/release/domain/machine.go`
- GenerateNotes method: `internal/domain/release/domain/run.go:756`
