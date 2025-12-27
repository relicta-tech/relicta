# Evaluation: Replacing Custom MCP Implementation with mcp-go Framework

## Executive Summary

This document evaluates whether to replace Relicta's custom MCP (Model Context Protocol) implementation with an existing mcp-go framework. After thorough analysis, **the recommendation is to NOT migrate** at this time. The custom implementation provides significant value with minimal ongoing maintenance burden, while migration would require substantial effort with limited benefits.

---

## Current Implementation Overview

### Size and Scope
- **Location**: `/internal/mcp/` (17 files, ~13,000 lines)
- **Core components**: `server.go`, `protocol.go`, `adapters.go`, `transport.go`, `cache.go`
- **Advanced features**: `multirepo.go`, `plugins.go`, `streaming.go`, `client.go`

### Features Implemented

| Category | Features |
|----------|----------|
| **Tools (7)** | status, plan, bump, notes, evaluate, approve, publish |
| **Resources (5)** | state, config, commits, changelog, risk-report |
| **Prompts (7)** | release-summary, risk-analysis, commit-review, breaking-changes, migration-guide, release-announcement, approval-decision |
| **Transport** | Stdio (NDJSON) |
| **Advanced** | Progress streaming, resource caching with TTLs, multi-repo management, plugin integration, client SDK |

### Architecture Strengths

1. **Clean Adapter Pattern**: `Adapter` bridges MCP protocol to application use cases, providing excellent separation of concerns
2. **Type-Safe Integration**: Direct integration with domain types (`release.Repository`, `version.Version`, etc.)
3. **CGP Integration**: Tight coupling with Change Governance Protocol for risk assessment
4. **Zero External Dependencies**: Custom JSON-RPC 2.0 implementation with no framework lock-in
5. **Comprehensive Testing**: Integration and unit tests cover full workflow

---

## Available mcp-go Frameworks

### 1. Official Go SDK (`modelcontextprotocol/go-sdk`)

| Attribute | Details |
|-----------|---------|
| **Maintainer** | MCP Organization + Google |
| **Stars/Forks** | 3.5k stars, 70 contributors |
| **Stability** | Stable target: July 2025 |
| **MCP Version** | 2025-06-18 |
| **License** | MIT |

**Key Features**:
- Typed tool handlers with struct-based inputs
- Command-based and stdio transports
- OAuth support for authentication
- Official specification alignment

**Limitations**:
- Newer project, still stabilizing API
- Less community adoption compared to alternatives

### 2. mark3labs/mcp-go

| Attribute | Details |
|-----------|---------|
| **Maintainer** | Ed Zynda (Community) |
| **Stars/Forks** | 7.9k stars, 744 forks |
| **Imports** | 1,307 packages |
| **Status** | Active development |
| **License** | MIT |

**Key Features**:
- High-level builder API (`mcp.NewTool()`)
- Per-session tool registration
- Request hooks and middleware
- Panic recovery via `WithRecovery()`
- Full context support

**Limitations**:
- "Under active development" - API may change
- Some advanced features still in progress
- Custom extensions may diverge from spec

### 3. metoro-io/mcp-golang

| Attribute | Details |
|-----------|---------|
| **Maintainer** | Metoro.io |
| **Focus** | Type safety, low boilerplate |
| **License** | MIT |

**Key Features**:
- Define tool arguments as native Go structs
- Automatic JSON schema generation
- Custom transports (stdio, HTTP)

**Limitations**:
- Smaller community
- Less feature-complete

---

## Migration Effort Assessment

### What Would Need to Change

#### 1. Protocol Layer (Low Effort)
The `protocol.go` file (~362 lines) would be replaced by framework types. This is straightforward.

#### 2. Server Core (Medium-High Effort)
The `server.go` file (~1,639 lines) contains:
- Custom tool/resource/prompt registration
- Message dispatch logic
- Progress notification handling
- Cache integration

Framework approach would require:
- Rewriting all tool handlers to match framework signatures
- Adapting progress streaming (frameworks may not support this natively)
- Integrating cache invalidation with framework lifecycle

#### 3. Transport Layer (Low Effort)
The `transport.go` (~178 lines) can be replaced by framework stdio transport.

#### 4. Adapter Layer (Medium Effort)
The `adapters.go` (~670 lines) would remain largely unchanged but would need to:
- Convert input/output types to framework formats
- Handle framework-specific error patterns

#### 5. Advanced Features (High Effort)
These custom features have no direct framework equivalents:

| Feature | Custom Lines | Framework Support |
|---------|--------------|-------------------|
| **Resource Caching** | 206 | None - must implement |
| **Multi-repo Management** | 100+ | None - must implement |
| **Plugin Integration** | 100+ | None - must implement |
| **Progress Streaming** | 100+ | Partial in some frameworks |
| **Client SDK** | 350+ | Available but different API |

#### 6. Test Suite (High Effort)
All integration tests would need rewriting to use framework APIs.

### Estimated Total Effort

| Component | Lines Affected | Effort |
|-----------|----------------|--------|
| Protocol replacement | 362 | 1-2 days |
| Server rewrite | 1,639 | 3-5 days |
| Adapter updates | 670 | 2-3 days |
| Advanced features | 500+ | 5-7 days |
| Test migration | 2,000+ | 3-5 days |
| Integration testing | - | 2-3 days |
| **Total** | **~5,000 lines** | **16-25 days** |

---

## Benefits Analysis

### Benefits of Migration

1. **Reduced Maintenance**: Protocol updates handled by framework maintainers
2. **Community Support**: Bug fixes and improvements from community
3. **Specification Compliance**: Automatic alignment with MCP spec changes
4. **New Features**: Get new MCP features (e.g., OAuth) for free

### Benefits of Current Implementation

1. **Zero Dependencies**: No external framework to update or worry about breaking changes
2. **Tailored Features**: Custom caching, multi-repo, and plugin support exactly match Relicta's needs
3. **CGP Integration**: Tight coupling with governance features would be harder to maintain through abstraction layers
4. **Performance Control**: Direct control over serialization, caching, and message handling
5. **Stability**: No risk of upstream breaking changes affecting production

---

## Risk Analysis

### Risks of Migration

| Risk | Severity | Likelihood | Mitigation |
|------|----------|------------|------------|
| Breaking changes in framework | High | Medium | Pin versions, monitor releases |
| Feature regression | Medium | High | Extensive testing required |
| Performance degradation | Low | Medium | Benchmark before/after |
| Loss of custom features | High | High | Re-implement as extensions |
| Increased complexity | Medium | Medium | Maintain abstraction layer |

### Risks of Staying

| Risk | Severity | Likelihood | Mitigation |
|------|----------|------------|------------|
| Spec drift | Low | Low | Monitor MCP spec, update as needed |
| Maintenance burden | Low | Low | Protocol is stable, minimal changes expected |
| Missing new features | Low | Medium | Implement as needed |

---

## Framework Comparison Matrix

| Criteria | Custom | Official SDK | mark3labs | metoro-io |
|----------|--------|--------------|-----------|-----------|
| **Type Safety** | Manual | Struct-based | Mixed | Struct-based |
| **Tool Definition** | Handler maps | Typed handlers | Builder pattern | Structs |
| **Progress Streaming** | Full support | Unknown | Partial | Unknown |
| **Resource Caching** | Built-in | None | None | None |
| **Multi-repo** | Built-in | None | None | None |
| **Plugin Integration** | Built-in | None | None | None |
| **Client SDK** | Built-in | Yes | No | No |
| **Stability** | Stable | Stabilizing | Active dev | Active dev |
| **Dependencies** | Zero | Minimal | Minimal | Minimal |
| **Migration Effort** | N/A | High | High | High |

---

## Recommendation

### Decision: **Do Not Migrate**

#### Rationale

1. **High effort, low reward**: Migration would require 16-25 days of work with no significant functional improvement

2. **Feature loss risk**: Custom features (caching, multi-repo, plugins) would need re-implementation

3. **Framework instability**: Both major frameworks are "under active development" with potential breaking changes

4. **Custom implementation is mature**: The current code is well-tested, production-ready, and matches Relicta's specific needs

5. **Protocol stability**: MCP specification updates are infrequent; maintaining protocol compliance is minimal effort

6. **Zero-dependency advantage**: Current implementation has no external dependencies to manage or worry about

### When to Reconsider

Migration should be reconsidered if:

1. **MCP spec undergoes major revision** requiring substantial protocol changes
2. **Framework reaches stable 1.0** with stable API guarantees
3. **New MCP features** (e.g., streaming responses, bidirectional tools) become essential
4. **Maintenance burden increases** significantly due to spec changes
5. **Team bandwidth allows** for the migration effort without impacting features

### Incremental Improvements (Alternative)

Instead of full migration, consider these targeted improvements:

1. **Update MCP version**: Current implementation uses `2024-11-05`; update to `2025-06-18` spec
2. **Improve type safety**: Add typed argument binding similar to framework approaches
3. **Extract protocol types**: Create standalone package for potential reuse
4. **Add OAuth support**: Implement authentication if needed for future features

---

## Sources

- [Official Go SDK](https://github.com/modelcontextprotocol/go-sdk) - MCP organization repository
- [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) - Community implementation with 7.9k stars
- [mcp-go API Documentation](https://pkg.go.dev/github.com/mark3labs/mcp-go/mcp) - Package documentation
- [metoro-io/mcp-golang](https://github.com/metoro-io/mcp-golang) - Type-safe implementation

---

*Evaluation completed: 2025-12-27*
*Author: Claude Code*
