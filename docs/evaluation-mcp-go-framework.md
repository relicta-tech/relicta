# Evaluation: Replacing Custom MCP Implementation with mcp-go Framework

## Executive Summary

This document evaluates whether to replace Relicta's custom MCP (Model Context Protocol) implementation with an existing mcp-go framework. After thorough analysis, **the recommendation is to MIGRATE to `felixgeelhaar/mcp-go`**. This framework provides production-ready features that closely match Relicta's current implementation while significantly reducing code maintenance burden.

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

### Current Pain Points

1. **Protocol Maintenance**: Custom JSON-RPC 2.0 implementation requires manual spec updates
2. **Type Safety**: Manual `map[string]any` argument parsing with type assertions
3. **Missing Transports**: Only stdio supported; no HTTP/SSE or WebSocket
4. **No Built-in Middleware**: Custom logging, recovery, and auth implementations
5. **MCP Version**: Using `2024-11-05` spec, behind current `2025-06-18`

---

## Framework Analysis

### Recommended: `felixgeelhaar/mcp-go`

| Attribute | Details |
|-----------|---------|
| **Repository** | [github.com/felixgeelhaar/mcp-go](https://pkg.go.dev/github.com/felixgeelhaar/mcp-go) |
| **Philosophy** | "Typed > dynamic", "Safe defaults > flexibility" |
| **Go Version** | 1.23+ |
| **License** | MIT |
| **Production Use** | Obvia (incident automation platform) |

#### Key Features

**1. Strongly-Typed Handlers**
```go
type SearchInput struct {
    Query string `json:"query" jsonschema:"required,description=Search query"`
    Limit int    `json:"limit" jsonschema:"default=10,minimum=1"`
}

srv.Tool("search").
    Description("Search for items").
    Handler(func(ctx context.Context, input SearchInput) ([]string, error) {
        return results, nil  // Automatic JSON serialization
    })
```

**2. Multiple Transports**
- **Stdio**: `mcp.ServeStdio(ctx, srv)` - CLI/agent integration
- **HTTP + SSE**: `mcp.ServeHTTP(ctx, srv, ":8080")` - Service deployments
- **WebSocket**: `mcp.ServeWebSocket(ctx, srv, ":8080")` - Bidirectional

**3. Built-in Middleware Stack**
```go
middleware := mcp.Chain(
    mcp.Recover(),           // Panic recovery
    mcp.RequestID(),         // Request tracing
    mcp.Timeout(30*time.Second),
    mcp.Logging(logger),     // Structured logging
    mcp.RateLimit(config),   // Rate limiting
    mcp.Auth(authenticator), // API key/Bearer auth
    mcp.OTel(options),       // OpenTelemetry
)
```

**4. Built-in Progress Reporting**
```go
progress := mcp.ProgressFromContext(ctx)
total := 100.0
for i := 0; i < 100; i++ {
    progress.Report(float64(i), &total)
}
```

**5. Session Features (MCP v1.1+)**
- Logging to client
- LLM sampling requests
- Workspace roots awareness
- Resource subscriptions
- Request cancellation

**6. URI Template Resources**
```go
srv.Resource("file://{path}").
    Name("File").
    MimeType("text/plain").
    Handler(func(ctx context.Context, uri string, params map[string]string) (*mcp.ResourceContent, error) {
        content, _ := os.ReadFile(params["path"])
        return &mcp.ResourceContent{URI: uri, Text: string(content)}, nil
    })
```

**7. Automatic JSON Schema Generation**
Struct tags define validation without manual schema maintenance:
- `jsonschema:"required"` - Required field
- `jsonschema:"description=..."` - Field description
- `jsonschema:"default=10"` - Default value
- `jsonschema:"minimum=0,maximum=100"` - Numeric bounds
- `jsonschema:"enum=a|b|c"` - Enum values

---

### Alternative Frameworks Considered

#### Official Go SDK (`modelcontextprotocol/go-sdk`)

| Pros | Cons |
|------|------|
| Official MCP organization | Stabilizing until July 2025 |
| Google collaboration | Less mature API |
| OAuth support | Smaller community |

#### mark3labs/mcp-go

| Pros | Cons |
|------|------|
| 7.9k stars, high adoption | "Under active development" |
| Builder pattern API | No built-in middleware |
| Per-session tools | Manual progress handling |

#### metoro-io/mcp-golang

| Pros | Cons |
|------|------|
| Type-safe structs | Smaller community |
| Low boilerplate | Less feature-complete |

---

## Migration Plan

### Phase 1: Core Server Migration (3-4 days)

**Files to replace:**
- `protocol.go` (362 lines) → Use framework types
- `transport.go` (178 lines) → Use `mcp.ServeStdio()`
- `server.go` (1,639 lines) → Rewrite with typed handlers

**Before (current):**
```go
type ToolHandler func(ctx context.Context, args map[string]any) (*CallToolResult, error)

func (s *Server) toolPlan(ctx context.Context, args map[string]any) (*CallToolResult, error) {
    analyze, _ := args["analyze"].(bool)
    fromRef, _ := args["from"].(string)
    // Manual type assertions...
}
```

**After (felixgeelhaar/mcp-go):**
```go
type PlanInput struct {
    Analyze bool   `json:"analyze" jsonschema:"description=Include detailed commit analysis"`
    From    string `json:"from" jsonschema:"description=Starting reference (tag or commit)"`
}

srv.Tool("relicta.plan").
    Description("Analyze commits since last release").
    Handler(func(ctx context.Context, input PlanInput) (*PlanOutput, error) {
        return adapter.Plan(ctx, input)  // Type-safe!
    })
```

### Phase 2: Adapter Simplification (2-3 days)

The `adapters.go` layer remains but simplifies:
- Remove manual type conversion from `map[string]any`
- Input/output types already match handler signatures
- Direct pass-through to use cases

### Phase 3: Advanced Features (3-4 days)

| Feature | Migration Approach |
|---------|-------------------|
| **Progress Streaming** | Use `mcp.ProgressFromContext(ctx)` - native support |
| **Resource Caching** | Implement as middleware or keep `cache.go` |
| **Multi-repo** | Keep as application-layer feature |
| **Plugin Integration** | Keep as application-layer feature |

### Phase 4: New Capabilities (2-3 days)

Features gained from migration:
- HTTP+SSE transport for web integrations
- WebSocket transport for real-time updates
- OpenTelemetry instrumentation
- Built-in rate limiting and auth
- Session-based logging to clients

### Phase 5: Test Migration (2-3 days)

Update integration tests to use framework APIs. Most test logic remains; only API calls change.

### Estimated Total Effort

| Phase | Effort |
|-------|--------|
| Core Server Migration | 3-4 days |
| Adapter Simplification | 2-3 days |
| Advanced Features | 3-4 days |
| New Capabilities | 2-3 days |
| Test Migration | 2-3 days |
| **Total** | **12-17 days** |

---

## Code Reduction Analysis

| Component | Current Lines | After Migration | Reduction |
|-----------|---------------|-----------------|-----------|
| `protocol.go` | 362 | 0 (framework) | -362 |
| `transport.go` | 178 | 0 (framework) | -178 |
| `server.go` | 1,639 | ~400 (handlers only) | -1,239 |
| `streaming.go` | 100+ | 0 (framework) | -100 |
| `adapters.go` | 670 | ~400 (simplified) | -270 |
| **Total Reduction** | | | **~2,100 lines** |

**Net result**: ~13,000 lines → ~10,900 lines (~16% reduction) while gaining significant new capabilities.

---

## Feature Comparison Matrix

| Feature | Current | felixgeelhaar/mcp-go | Benefit |
|---------|---------|---------------------|---------|
| **Type Safety** | Manual assertions | Struct-based handlers | Compile-time safety |
| **JSON Schema** | Hand-crafted | Auto-generated | Always in sync |
| **Progress** | Custom implementation | `ProgressFromContext` | Less code |
| **Transports** | Stdio only | Stdio, HTTP, WebSocket | More deployment options |
| **Middleware** | None | Full stack | Production-ready |
| **Auth** | None | API key, Bearer | Security built-in |
| **Observability** | Manual logging | OpenTelemetry | Industry standard |
| **MCP Version** | 2024-11-05 | Latest | Spec compliance |

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Breaking changes | Pin to specific version, review releases |
| Feature gaps | Keep custom `cache.go`, `multirepo.go`, `plugins.go` |
| Performance | Benchmark before/after migration |
| Learning curve | Framework has Gin-like familiar API |

---

## Recommendation

### Decision: **Migrate to `felixgeelhaar/mcp-go`**

#### Rationale

1. **Strong Feature Parity**: Built-in progress reporting, typed handlers, and middleware match current needs

2. **Code Reduction**: ~2,100 fewer lines to maintain while gaining capabilities

3. **Type Safety**: Eliminates error-prone `map[string]any` argument parsing

4. **Transport Flexibility**: HTTP+SSE and WebSocket enable new integration patterns

5. **Production Middleware**: Recovery, auth, rate limiting, and observability out of the box

6. **Spec Compliance**: Automatic alignment with latest MCP specification

7. **Reasonable Effort**: 12-17 days is acceptable for the benefits gained

### Migration Priority

1. Start with core server migration to validate framework fit
2. Keep adapter pattern - it cleanly separates MCP from business logic
3. Retain custom caching and multi-repo as application-layer features
4. Add HTTP transport after stdio works to enable web integrations

---

## Sources

- [felixgeelhaar/mcp-go](https://pkg.go.dev/github.com/felixgeelhaar/mcp-go) - Recommended framework
- [Official Go SDK](https://github.com/modelcontextprotocol/go-sdk) - MCP organization repository
- [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) - Community implementation
- [metoro-io/mcp-golang](https://github.com/metoro-io/mcp-golang) - Type-safe implementation

---

*Evaluation completed: 2025-12-27*
*Revised: 2025-12-27 - Updated recommendation to felixgeelhaar/mcp-go*
*Author: Claude Code*
