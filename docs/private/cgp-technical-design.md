# Technical Design Document: Change Governance Protocol (CGP)

## 1. Executive Summary

The Change Governance Protocol (CGP) is the foundational protocol that positions Relicta as the authoritative control plane for agentic software delivery. This document specifies the technical implementation of CGP within Relicta, including protocol messages, policy engine, risk scoring, audit trails, and MCP integration.

**Key Principle**: Relicta does not replace agents — it governs them.

---

## 2. Goals & Non-Goals

### 2.1 Goals

- Define a vendor-neutral, model-agnostic protocol for change governance
- Enable autonomous agents to propose releases through structured requests
- Implement policy-based decision making for release approval
- Provide semantic code analysis beyond commit conventions
- Maintain immutable audit trails for all governance decisions
- Support human-in-the-loop workflows with explicit checkpoints
- Integrate with Model Context Protocol (MCP) for agent communication

### 2.2 Non-Goals

- CGP does not replace existing CI/CD pipelines
- CGP does not execute deployments directly (delegates to executors)
- CGP is not a code review tool (complements, not replaces)
- CGP does not store source code or artifacts

---

## 3. System Architecture

### 3.1 CGP High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              CGP Architecture                                    │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  ┌────────────────────────────────────────────────────────────────────────────┐ │
│  │                           PROPOSERS                                         │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │ │
│  │  │  AI Agent   │  │  CI System  │  │   Human     │  │  MCP Client │       │ │
│  │  │  (Cursor,   │  │  (GitHub    │  │  Developer  │  │             │       │ │
│  │  │   Claude)   │  │   Actions)  │  │             │  │             │       │ │
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘       │ │
│  └─────────┼────────────────┼────────────────┼────────────────┼──────────────┘ │
│            │                │                │                │                 │
│            └────────────────┴────────────────┴────────────────┘                 │
│                                      │                                          │
│                                      ▼                                          │
│  ┌────────────────────────────────────────────────────────────────────────────┐ │
│  │                        CGP GOVERNOR (Relicta)                               │ │
│  │  ┌─────────────────────────────────────────────────────────────────────┐   │ │
│  │  │                      Protocol Handler                                │   │ │
│  │  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                  │   │ │
│  │  │  │   Proposal  │  │  Decision   │  │   Auth      │                  │   │ │
│  │  │  │   Receiver  │  │   Emitter   │  │   Handler   │                  │   │ │
│  │  │  └─────────────┘  └─────────────┘  └─────────────┘                  │   │ │
│  │  └─────────────────────────────────────────────────────────────────────┘   │ │
│  │                                      │                                      │ │
│  │  ┌─────────────────────────────────────────────────────────────────────┐   │ │
│  │  │                      Governance Engine                               │   │ │
│  │  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌────────────┐  │   │ │
│  │  │  │  Semantic   │  │   Policy    │  │    Risk     │  │   Human    │  │   │ │
│  │  │  │  Analyzer   │  │   Engine    │  │   Scorer    │  │   Review   │  │   │ │
│  │  │  └─────────────┘  └─────────────┘  └─────────────┘  └────────────┘  │   │ │
│  │  └─────────────────────────────────────────────────────────────────────┘   │ │
│  │                                      │                                      │ │
│  │  ┌─────────────────────────────────────────────────────────────────────┐   │ │
│  │  │                      Persistence Layer                               │   │ │
│  │  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                  │   │ │
│  │  │  │   Audit     │  │   Release   │  │   Policy    │                  │   │ │
│  │  │  │   Trail     │  │   Memory    │  │   Store     │                  │   │ │
│  │  │  └─────────────┘  └─────────────┘  └─────────────┘                  │   │ │
│  │  └─────────────────────────────────────────────────────────────────────┘   │ │
│  └────────────────────────────────────────────────────────────────────────────┘ │
│                                      │                                          │
│                                      ▼                                          │
│  ┌────────────────────────────────────────────────────────────────────────────┐ │
│  │                           EXECUTORS                                         │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │ │
│  │  │   GitHub    │  │   GitLab    │  │    npm      │  │   Slack     │       │ │
│  │  │   Plugin    │  │   Plugin    │  │   Plugin    │  │   Plugin    │       │ │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘       │ │
│  └────────────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Component Responsibilities

| Component | Responsibility |
|-----------|----------------|
| **Protocol Handler** | Receives CGP messages, validates schema, routes to governance engine |
| **Semantic Analyzer** | Analyzes code changes for API breaks, dependency impact, blast radius |
| **Policy Engine** | Evaluates organizational rules and constraints |
| **Risk Scorer** | Calculates risk score based on multiple factors |
| **Human Review** | Manages interactive approval workflows |
| **Audit Trail** | Immutable record of all governance decisions |
| **Release Memory** | Historical context for learning and pattern detection |

---

## 4. CGP Protocol Specification

### 4.1 Protocol Version

```
CGP Version: 0.1.0
Transport: MCP (Model Context Protocol), HTTP/JSON, gRPC
Encoding: JSON (primary), Protocol Buffers (optional)
```

### 4.2 Message Types

CGP defines four core message types:

| Type | Direction | Purpose |
|------|-----------|---------|
| `change.proposal` | Proposer → Governor | Submit change intent |
| `change.evaluation` | Governor internal | Analysis results |
| `change.decision` | Governor → Proposer | Governance outcome |
| `change.execution_authorized` | Governor → Executor | Permission to act |

### 4.3 Actor Identification

```go
// internal/cgp/actor.go

package cgp

// ActorKind represents the type of actor in CGP
type ActorKind string

const (
    ActorKindAgent  ActorKind = "agent"   // AI coding agents
    ActorKindCI     ActorKind = "ci"      // CI/CD systems
    ActorKindHuman  ActorKind = "human"   // Human developers
    ActorKindSystem ActorKind = "system"  // Automated systems
)

// Actor identifies who is proposing or authorizing a change
type Actor struct {
    Kind        ActorKind         `json:"kind"`
    ID          string            `json:"id"`
    Name        string            `json:"name,omitempty"`
    Attributes  map[string]string `json:"attributes,omitempty"`
    Credentials *Credentials      `json:"credentials,omitempty"`
}

// Credentials for actor authentication
type Credentials struct {
    Type      string `json:"type"`      // "token", "certificate", "oauth"
    TokenHash string `json:"tokenHash"` // SHA256 of credential (never store raw)
}

// TrustLevel determines how much autonomy an actor has
type TrustLevel int

const (
    TrustLevelUntrusted TrustLevel = iota // Requires full human review
    TrustLevelLimited                      // Can propose, limited auto-approval
    TrustLevelTrusted                      // Can auto-approve low-risk changes
    TrustLevelFull                         // Full automation (human equivalent)
)
```

---

## 5. CGP Message Schemas

### 5.1 Change Proposal

```go
// internal/cgp/proposal.go

package cgp

import "time"

// ChangeProposal represents a request to release changes
type ChangeProposal struct {
    // Protocol metadata
    CGPVersion string    `json:"cgpVersion"`
    Type       string    `json:"type"` // "change.proposal"
    ID         string    `json:"id"`   // Unique proposal ID
    Timestamp  time.Time `json:"timestamp"`

    // Who is proposing
    Actor Actor `json:"actor"`

    // What is being proposed
    Scope  ProposalScope  `json:"scope"`
    Intent ProposalIntent `json:"intent"`

    // Optional context
    Context *ProposalContext `json:"context,omitempty"`
}

// ProposalScope defines what changes are included
type ProposalScope struct {
    Repository  string   `json:"repository"`            // "owner/repo"
    Branch      string   `json:"branch,omitempty"`      // Target branch
    CommitRange string   `json:"commitRange"`           // "abc123..def456"
    Commits     []string `json:"commits,omitempty"`     // Individual commit SHAs
    Files       []string `json:"files,omitempty"`       // Changed file paths
}

// ProposalIntent describes the proposer's understanding
type ProposalIntent struct {
    Summary         string   `json:"summary"`                   // Human-readable description
    SuggestedBump   string   `json:"suggestedBump,omitempty"`   // "major", "minor", "patch"
    Confidence      float64  `json:"confidence"`                // 0.0-1.0, proposer's confidence
    Categories      []string `json:"categories,omitempty"`      // ["feature", "bugfix", "security"]
    BreakingChanges []string `json:"breakingChanges,omitempty"` // Known breaking changes
}

// ProposalContext provides additional information
type ProposalContext struct {
    // Related tickets/issues
    Issues []IssueReference `json:"issues,omitempty"`

    // Agent-specific context
    AgentSession   string                 `json:"agentSession,omitempty"`   // Session ID for multi-turn
    PriorProposals []string               `json:"priorProposals,omitempty"` // Previous proposal IDs
    Metadata       map[string]interface{} `json:"metadata,omitempty"`       // Arbitrary metadata
}

// IssueReference links to external issue trackers
type IssueReference struct {
    Provider string `json:"provider"` // "github", "jira", "linear"
    ID       string `json:"id"`       // Issue identifier
    URL      string `json:"url,omitempty"`
}
```

### 5.2 Governance Decision

```go
// internal/cgp/decision.go

package cgp

import "time"

// DecisionType represents the governance outcome
type DecisionType string

const (
    DecisionApproved         DecisionType = "approved"          // Auto-approved
    DecisionApprovalRequired DecisionType = "approval_required" // Needs human review
    DecisionRejected         DecisionType = "rejected"          // Policy violation
    DecisionDeferred         DecisionType = "deferred"          // Needs more info
)

// GovernanceDecision is the response to a ChangeProposal
type GovernanceDecision struct {
    // Protocol metadata
    CGPVersion string    `json:"cgpVersion"`
    Type       string    `json:"type"` // "change.decision"
    ID         string    `json:"id"`   // Decision ID
    ProposalID string    `json:"proposalId"`
    Timestamp  time.Time `json:"timestamp"`

    // Decision outcome
    Decision           DecisionType `json:"decision"`
    RecommendedVersion string       `json:"recommendedVersion,omitempty"`

    // Risk assessment
    RiskScore   float64      `json:"riskScore"`   // 0.0-1.0
    RiskFactors []RiskFactor `json:"riskFactors"` // Contributing factors

    // Explanation
    Rationale []string `json:"rationale"` // Human-readable reasons

    // What happens next
    RequiredActions []RequiredAction `json:"requiredActions,omitempty"`
    Conditions      []Condition      `json:"conditions,omitempty"`

    // Analysis details
    Analysis *ChangeAnalysis `json:"analysis,omitempty"`
}

// RiskFactor describes a specific risk contribution
type RiskFactor struct {
    Category    string  `json:"category"`    // "api_change", "dependency", "security"
    Description string  `json:"description"` // Human-readable
    Score       float64 `json:"score"`       // 0.0-1.0 contribution
    Severity    string  `json:"severity"`    // "low", "medium", "high", "critical"
}

// RequiredAction specifies what must happen before execution
type RequiredAction struct {
    Type        string `json:"type"`        // "human_approval", "release_note_review", "security_review"
    Description string `json:"description"` // What needs to be done
    Assignee    string `json:"assignee,omitempty"`
    Deadline    string `json:"deadline,omitempty"` // ISO8601 duration or timestamp
}

// Condition specifies constraints on execution
type Condition struct {
    Type  string `json:"type"`  // "time_window", "feature_flag", "manual_gate"
    Value string `json:"value"` // Condition-specific value
}

// ChangeAnalysis contains detailed analysis results
type ChangeAnalysis struct {
    // Semantic analysis
    APIChanges       []APIChange       `json:"apiChanges,omitempty"`
    DependencyImpact *DependencyImpact `json:"dependencyImpact,omitempty"`
    BlastRadius      *BlastRadius      `json:"blastRadius,omitempty"`

    // Categorized commits
    Features     int `json:"features"`
    Fixes        int `json:"fixes"`
    Breaking     int `json:"breaking"`
    Security     int `json:"security"`
    Dependencies int `json:"dependencies"`
    Other        int `json:"other"`
}

// APIChange describes a public API modification
type APIChange struct {
    Type        string `json:"type"`        // "added", "removed", "modified", "deprecated"
    Symbol      string `json:"symbol"`      // Function/type/endpoint name
    Location    string `json:"location"`    // File path and line
    Breaking    bool   `json:"breaking"`    // Is this a breaking change
    Description string `json:"description"` // What changed
}

// DependencyImpact describes impact on downstream consumers
type DependencyImpact struct {
    DirectDependents   int      `json:"directDependents"`   // Immediate consumers
    TransitiveDependents int    `json:"transitiveDependents"` // All affected
    AffectedServices   []string `json:"affectedServices,omitempty"`
    AffectedPackages   []string `json:"affectedPackages,omitempty"`
}

// BlastRadius quantifies potential impact
type BlastRadius struct {
    Score       float64  `json:"score"`       // 0.0-1.0
    FilesChanged int     `json:"filesChanged"`
    LinesChanged int     `json:"linesChanged"`
    Components   []string `json:"components"`  // Affected system components
}
```

### 5.3 Execution Authorization

```go
// internal/cgp/authorization.go

package cgp

import "time"

// ExecutionAuthorization grants permission to execute a release
type ExecutionAuthorization struct {
    // Protocol metadata
    CGPVersion string    `json:"cgpVersion"`
    Type       string    `json:"type"` // "change.execution_authorized"
    ID         string    `json:"id"`
    DecisionID string    `json:"decisionId"`
    ProposalID string    `json:"proposalId"`
    Timestamp  time.Time `json:"timestamp"`

    // Who authorized
    ApprovedBy Actor     `json:"approvedBy"`
    ApprovedAt time.Time `json:"approvedAt"`

    // What is authorized
    Version      string   `json:"version"`       // Final version number
    Tag          string   `json:"tag"`           // Git tag name
    ReleaseNotes string   `json:"releaseNotes"`  // Approved release notes
    Changelog    string   `json:"changelog"`     // Approved changelog

    // Constraints on execution
    ValidUntil    time.Time `json:"validUntil"`              // Authorization expiry
    AllowedSteps  []string  `json:"allowedSteps"`            // ["tag", "publish", "notify"]
    Restrictions  []string  `json:"restrictions,omitempty"`  // Any limitations

    // Audit
    ApprovalChain []ApprovalRecord `json:"approvalChain"` // Full approval history
}

// ApprovalRecord tracks each approval in the chain
type ApprovalRecord struct {
    Actor     Actor     `json:"actor"`
    Action    string    `json:"action"`    // "approve", "request_changes", "comment"
    Timestamp time.Time `json:"timestamp"`
    Comment   string    `json:"comment,omitempty"`
    Signature string    `json:"signature,omitempty"` // Cryptographic signature
}
```

---

## 6. Policy Engine

### 6.1 Policy Definition

```go
// internal/cgp/policy/policy.go

package policy

// Policy defines organizational release governance rules
type Policy struct {
    Version     string       `json:"version" yaml:"version"`
    Name        string       `json:"name" yaml:"name"`
    Description string       `json:"description,omitempty" yaml:"description,omitempty"`
    Rules       []Rule       `json:"rules" yaml:"rules"`
    Defaults    PolicyDefaults `json:"defaults" yaml:"defaults"`
}

// PolicyDefaults specifies default behavior when no rules match
type PolicyDefaults struct {
    Decision          string   `json:"decision" yaml:"decision"`                   // "approve", "require_review", "reject"
    RequiredApprovers int      `json:"requiredApprovers" yaml:"requiredApprovers"` // Minimum approvals
    AllowedActors     []string `json:"allowedActors" yaml:"allowedActors"`         // Actor IDs or patterns
}

// Rule defines a single governance rule
type Rule struct {
    ID          string      `json:"id" yaml:"id"`
    Name        string      `json:"name" yaml:"name"`
    Description string      `json:"description,omitempty" yaml:"description,omitempty"`
    Priority    int         `json:"priority" yaml:"priority"` // Higher = evaluated first
    Enabled     bool        `json:"enabled" yaml:"enabled"`
    Conditions  []Condition `json:"conditions" yaml:"conditions"`
    Actions     []Action    `json:"actions" yaml:"actions"`
}

// Condition defines when a rule applies
type Condition struct {
    Field    string      `json:"field" yaml:"field"`       // "actor.kind", "risk.score", "change.breaking"
    Operator string      `json:"operator" yaml:"operator"` // "eq", "ne", "gt", "lt", "in", "contains", "matches"
    Value    interface{} `json:"value" yaml:"value"`       // Comparison value
}

// Action defines what happens when rule matches
type Action struct {
    Type   string                 `json:"type" yaml:"type"`     // "set_decision", "require_approval", "add_reviewer", "block"
    Params map[string]interface{} `json:"params" yaml:"params"` // Action-specific parameters
}
```

### 6.2 Policy Engine Implementation

```go
// internal/cgp/policy/engine.go

package policy

import (
    "context"
    "fmt"
    "sort"

    "github.com/relicta-tech/relicta/internal/cgp"
)

// Engine evaluates policies against proposals
type Engine struct {
    policies []Policy
    logger   *slog.Logger
}

// NewEngine creates a policy engine with loaded policies
func NewEngine(policies []Policy) *Engine {
    return &Engine{
        policies: policies,
    }
}

// Evaluate runs all policies against a proposal and analysis
func (e *Engine) Evaluate(ctx context.Context, proposal *cgp.ChangeProposal, analysis *cgp.ChangeAnalysis) (*PolicyResult, error) {
    result := &PolicyResult{
        Decision:        cgp.DecisionApproved,
        RequiredActions: []cgp.RequiredAction{},
        MatchedRules:    []string{},
        Rationale:       []string{},
    }

    // Collect all rules from all policies, sorted by priority
    var allRules []ruleWithPolicy
    for _, policy := range e.policies {
        for _, rule := range policy.Rules {
            if rule.Enabled {
                allRules = append(allRules, ruleWithPolicy{
                    rule:   rule,
                    policy: policy,
                })
            }
        }
    }
    sort.Slice(allRules, func(i, j int) bool {
        return allRules[i].rule.Priority > allRules[j].rule.Priority
    })

    // Build evaluation context
    evalCtx := buildEvalContext(proposal, analysis)

    // Evaluate each rule
    for _, rp := range allRules {
        matched, err := e.evaluateRule(ctx, rp.rule, evalCtx)
        if err != nil {
            e.logger.Warn("rule evaluation failed",
                "rule", rp.rule.ID,
                "error", err,
            )
            continue
        }

        if matched {
            result.MatchedRules = append(result.MatchedRules, rp.rule.ID)
            e.applyActions(result, rp.rule.Actions)
            result.Rationale = append(result.Rationale,
                fmt.Sprintf("Rule '%s' matched: %s", rp.rule.Name, rp.rule.Description))
        }
    }

    return result, nil
}

// evaluateRule checks if all conditions match
func (e *Engine) evaluateRule(ctx context.Context, rule Rule, evalCtx map[string]interface{}) (bool, error) {
    for _, cond := range rule.Conditions {
        matched, err := e.evaluateCondition(cond, evalCtx)
        if err != nil {
            return false, err
        }
        if !matched {
            return false, nil
        }
    }
    return true, nil
}

// evaluateCondition checks a single condition
func (e *Engine) evaluateCondition(cond Condition, evalCtx map[string]interface{}) (bool, error) {
    fieldValue, ok := getNestedValue(evalCtx, cond.Field)
    if !ok {
        return false, nil // Field doesn't exist, condition doesn't match
    }

    switch cond.Operator {
    case "eq":
        return fmt.Sprintf("%v", fieldValue) == fmt.Sprintf("%v", cond.Value), nil
    case "ne":
        return fmt.Sprintf("%v", fieldValue) != fmt.Sprintf("%v", cond.Value), nil
    case "gt":
        return compareNumeric(fieldValue, cond.Value) > 0, nil
    case "gte":
        return compareNumeric(fieldValue, cond.Value) >= 0, nil
    case "lt":
        return compareNumeric(fieldValue, cond.Value) < 0, nil
    case "lte":
        return compareNumeric(fieldValue, cond.Value) <= 0, nil
    case "in":
        return containsValue(cond.Value, fieldValue), nil
    case "contains":
        return containsSubstring(fieldValue, cond.Value), nil
    case "matches":
        return matchesPattern(fieldValue, cond.Value)
    default:
        return false, fmt.Errorf("unknown operator: %s", cond.Operator)
    }
}

// PolicyResult contains the evaluation outcome
type PolicyResult struct {
    Decision        cgp.DecisionType
    RequiredActions []cgp.RequiredAction
    MatchedRules    []string
    Rationale       []string
    Overrides       map[string]interface{}
}
```

### 6.3 Example Policy Configuration

```yaml
# release-policy.yaml

version: "1.0"
name: "Standard Release Policy"
description: "Default governance policy for all repositories"

defaults:
  decision: require_review
  requiredApprovers: 1
  allowedActors:
    - "human:*"
    - "ci:github-actions"

rules:
  # Block autonomous major releases
  - id: block-autonomous-major
    name: "Block Autonomous Major Releases"
    description: "AI agents cannot auto-release major versions"
    priority: 100
    enabled: true
    conditions:
      - field: "actor.kind"
        operator: "eq"
        value: "agent"
      - field: "analysis.breaking"
        operator: "gt"
        value: 0
    actions:
      - type: set_decision
        params:
          decision: approval_required
      - type: require_approval
        params:
          approvers: ["team:release-managers"]
          minimum: 2

  # High-risk changes require security review
  - id: security-review-high-risk
    name: "Security Review for High Risk"
    description: "Changes with risk score > 0.7 require security team review"
    priority: 90
    enabled: true
    conditions:
      - field: "risk.score"
        operator: "gt"
        value: 0.7
    actions:
      - type: require_approval
        params:
          approvers: ["team:security"]
          minimum: 1
      - type: add_required_action
        params:
          type: security_review
          description: "Security team must review high-risk changes"

  # Auto-approve low-risk patches from trusted CI
  - id: auto-approve-trusted-patches
    name: "Auto-approve Trusted Patches"
    description: "Low-risk patches from CI can be auto-approved"
    priority: 50
    enabled: true
    conditions:
      - field: "actor.kind"
        operator: "in"
        value: ["ci", "human"]
      - field: "risk.score"
        operator: "lt"
        value: 0.3
      - field: "intent.suggestedBump"
        operator: "eq"
        value: "patch"
    actions:
      - type: set_decision
        params:
          decision: approved

  # Time-based restrictions
  - id: no-friday-majors
    name: "No Major Releases on Friday"
    description: "Prevent major releases before weekends"
    priority: 80
    enabled: true
    conditions:
      - field: "intent.suggestedBump"
        operator: "eq"
        value: "major"
      - field: "time.dayOfWeek"
        operator: "eq"
        value: 5  # Friday
    actions:
      - type: set_decision
        params:
          decision: deferred
      - type: add_condition
        params:
          type: time_window
          value: "next Monday 09:00"
```

---

## 7. Risk Scoring System

### 7.1 Risk Calculator

```go
// internal/cgp/risk/calculator.go

package risk

import (
    "context"

    "github.com/relicta-tech/relicta/internal/cgp"
)

// Calculator computes risk scores for changes
type Calculator struct {
    weights  WeightConfig
    history  HistoryProvider
    analyzer SemanticAnalyzer
}

// WeightConfig defines the contribution of each factor
type WeightConfig struct {
    APIChanges       float64 `json:"apiChanges" yaml:"apiChanges"`
    DependencyImpact float64 `json:"dependencyImpact" yaml:"dependencyImpact"`
    BlastRadius      float64 `json:"blastRadius" yaml:"blastRadius"`
    CodeComplexity   float64 `json:"codeComplexity" yaml:"codeComplexity"`
    TestCoverage     float64 `json:"testCoverage" yaml:"testCoverage"`
    ActorTrust       float64 `json:"actorTrust" yaml:"actorTrust"`
    HistoricalRisk   float64 `json:"historicalRisk" yaml:"historicalRisk"`
    SecurityImpact   float64 `json:"securityImpact" yaml:"securityImpact"`
}

// DefaultWeights returns sensible default risk weights
func DefaultWeights() WeightConfig {
    return WeightConfig{
        APIChanges:       0.25,
        DependencyImpact: 0.20,
        BlastRadius:      0.15,
        CodeComplexity:   0.10,
        TestCoverage:     0.10,
        ActorTrust:       0.05,
        HistoricalRisk:   0.10,
        SecurityImpact:   0.05,
    }
}

// Calculate computes overall risk score
func (c *Calculator) Calculate(ctx context.Context, proposal *cgp.ChangeProposal, analysis *cgp.ChangeAnalysis) (*RiskAssessment, error) {
    factors := []cgp.RiskFactor{}
    totalScore := 0.0

    // API Changes
    if apiScore, factor := c.assessAPIChanges(analysis); factor != nil {
        factors = append(factors, *factor)
        totalScore += apiScore * c.weights.APIChanges
    }

    // Dependency Impact
    if depScore, factor := c.assessDependencyImpact(analysis); factor != nil {
        factors = append(factors, *factor)
        totalScore += depScore * c.weights.DependencyImpact
    }

    // Blast Radius
    if blastScore, factor := c.assessBlastRadius(analysis); factor != nil {
        factors = append(factors, *factor)
        totalScore += blastScore * c.weights.BlastRadius
    }

    // Historical Risk (from release memory)
    if histScore, factor := c.assessHistoricalRisk(ctx, proposal); factor != nil {
        factors = append(factors, *factor)
        totalScore += histScore * c.weights.HistoricalRisk
    }

    // Actor Trust
    if trustScore, factor := c.assessActorTrust(proposal.Actor); factor != nil {
        factors = append(factors, *factor)
        totalScore += trustScore * c.weights.ActorTrust
    }

    // Security Impact
    if secScore, factor := c.assessSecurityImpact(analysis); factor != nil {
        factors = append(factors, *factor)
        totalScore += secScore * c.weights.SecurityImpact
    }

    // Normalize to 0-1 range
    normalizedScore := min(1.0, max(0.0, totalScore))

    return &RiskAssessment{
        Score:    normalizedScore,
        Factors:  factors,
        Severity: scoreSeverity(normalizedScore),
    }, nil
}

// assessAPIChanges evaluates public API modifications
func (c *Calculator) assessAPIChanges(analysis *cgp.ChangeAnalysis) (float64, *cgp.RiskFactor) {
    if analysis == nil || len(analysis.APIChanges) == 0 {
        return 0, nil
    }

    score := 0.0
    breakingCount := 0

    for _, change := range analysis.APIChanges {
        switch change.Type {
        case "removed":
            score += 1.0
            breakingCount++
        case "modified":
            if change.Breaking {
                score += 0.8
                breakingCount++
            } else {
                score += 0.3
            }
        case "deprecated":
            score += 0.2
        case "added":
            score += 0.1
        }
    }

    // Normalize based on number of changes
    normalizedScore := min(1.0, score/float64(len(analysis.APIChanges)))

    severity := "low"
    if breakingCount > 0 {
        severity = "high"
    } else if normalizedScore > 0.5 {
        severity = "medium"
    }

    return normalizedScore, &cgp.RiskFactor{
        Category:    "api_change",
        Description: fmt.Sprintf("%d API changes, %d breaking", len(analysis.APIChanges), breakingCount),
        Score:       normalizedScore,
        Severity:    severity,
    }
}

// assessHistoricalRisk uses release memory to evaluate patterns
func (c *Calculator) assessHistoricalRisk(ctx context.Context, proposal *cgp.ChangeProposal) (float64, *cgp.RiskFactor) {
    if c.history == nil {
        return 0, nil
    }

    // Query historical data
    history, err := c.history.GetReleaseHistory(ctx, proposal.Scope.Repository, 10)
    if err != nil || len(history) == 0 {
        return 0, nil
    }

    // Analyze patterns
    recentRollbacks := 0
    recentIncidents := 0
    actorFailureRate := 0.0

    for _, release := range history {
        if release.RolledBack {
            recentRollbacks++
        }
        if release.CausedIncident {
            recentIncidents++
        }
        if release.Actor.ID == proposal.Actor.ID && release.Status == "failed" {
            actorFailureRate++
        }
    }

    score := 0.0
    score += float64(recentRollbacks) * 0.3
    score += float64(recentIncidents) * 0.5
    score += (actorFailureRate / float64(len(history))) * 0.2

    normalizedScore := min(1.0, score)

    if normalizedScore == 0 {
        return 0, nil
    }

    return normalizedScore, &cgp.RiskFactor{
        Category:    "historical",
        Description: fmt.Sprintf("%d recent rollbacks, %d incidents", recentRollbacks, recentIncidents),
        Score:       normalizedScore,
        Severity:    scoreSeverity(normalizedScore),
    }
}

// RiskAssessment contains the complete risk evaluation
type RiskAssessment struct {
    Score    float64           `json:"score"`
    Factors  []cgp.RiskFactor  `json:"factors"`
    Severity string            `json:"severity"`
}

func scoreSeverity(score float64) string {
    switch {
    case score >= 0.8:
        return "critical"
    case score >= 0.6:
        return "high"
    case score >= 0.4:
        return "medium"
    default:
        return "low"
    }
}
```

---

## 8. Semantic Analyzer

### 8.1 Code Analysis Service

```go
// internal/cgp/analysis/semantic.go

package analysis

import (
    "context"
    "go/ast"
    "go/parser"
    "go/token"
    "strings"

    "github.com/relicta-tech/relicta/internal/cgp"
)

// SemanticAnalyzer performs deep code analysis
type SemanticAnalyzer struct {
    gitService GitService
    aiService  AIService // Optional AI-powered analysis
}

// Analyze performs comprehensive semantic analysis of changes
func (a *SemanticAnalyzer) Analyze(ctx context.Context, scope *cgp.ProposalScope) (*cgp.ChangeAnalysis, error) {
    analysis := &cgp.ChangeAnalysis{
        APIChanges: []cgp.APIChange{},
    }

    // Get diff between commits
    diff, err := a.gitService.GetDiff(ctx, scope.CommitRange)
    if err != nil {
        return nil, fmt.Errorf("getting diff: %w", err)
    }

    // Analyze each changed file
    for _, file := range diff.Files {
        if strings.HasSuffix(file.Path, ".go") {
            changes, err := a.analyzeGoFile(ctx, file)
            if err != nil {
                continue // Log and continue
            }
            analysis.APIChanges = append(analysis.APIChanges, changes...)
        }
        // Add analyzers for other languages
    }

    // Calculate blast radius
    analysis.BlastRadius = a.calculateBlastRadius(diff)

    // Detect dependency impact (if dependency files changed)
    analysis.DependencyImpact = a.analyzeDependencyImpact(diff)

    // Categorize commits
    commits, _ := a.gitService.GetCommits(ctx, scope.CommitRange)
    a.categorizeCommits(analysis, commits)

    return analysis, nil
}

// analyzeGoFile detects API changes in Go source files
func (a *SemanticAnalyzer) analyzeGoFile(ctx context.Context, file DiffFile) ([]cgp.APIChange, error) {
    changes := []cgp.APIChange{}

    // Parse old and new versions
    fset := token.NewFileSet()

    oldAST, _ := parser.ParseFile(fset, "", file.OldContent, parser.ParseComments)
    newAST, _ := parser.ParseFile(fset, "", file.NewContent, parser.ParseComments)

    // Extract exported symbols from both versions
    oldSymbols := extractExportedSymbols(oldAST)
    newSymbols := extractExportedSymbols(newAST)

    // Detect removed symbols (breaking)
    for name, oldSym := range oldSymbols {
        if _, exists := newSymbols[name]; !exists {
            changes = append(changes, cgp.APIChange{
                Type:        "removed",
                Symbol:      name,
                Location:    file.Path,
                Breaking:    true,
                Description: fmt.Sprintf("Exported %s '%s' was removed", oldSym.Kind, name),
            })
        }
    }

    // Detect added symbols
    for name, newSym := range newSymbols {
        if _, exists := oldSymbols[name]; !exists {
            changes = append(changes, cgp.APIChange{
                Type:        "added",
                Symbol:      name,
                Location:    file.Path,
                Breaking:    false,
                Description: fmt.Sprintf("New exported %s '%s'", newSym.Kind, name),
            })
        }
    }

    // Detect modified symbols (signature changes)
    for name, oldSym := range oldSymbols {
        if newSym, exists := newSymbols[name]; exists {
            if oldSym.Signature != newSym.Signature {
                breaking := isBreakingChange(oldSym, newSym)
                changes = append(changes, cgp.APIChange{
                    Type:        "modified",
                    Symbol:      name,
                    Location:    file.Path,
                    Breaking:    breaking,
                    Description: fmt.Sprintf("Signature changed: %s -> %s", oldSym.Signature, newSym.Signature),
                })
            }
        }
    }

    return changes, nil
}

// Symbol represents an exported Go symbol
type Symbol struct {
    Name      string
    Kind      string // "func", "type", "var", "const"
    Signature string
}

// extractExportedSymbols finds all exported symbols in a Go AST
func extractExportedSymbols(f *ast.File) map[string]Symbol {
    symbols := make(map[string]Symbol)

    if f == nil {
        return symbols
    }

    for _, decl := range f.Decls {
        switch d := decl.(type) {
        case *ast.FuncDecl:
            if d.Name.IsExported() {
                symbols[d.Name.Name] = Symbol{
                    Name:      d.Name.Name,
                    Kind:      "func",
                    Signature: formatFuncSignature(d),
                }
            }
        case *ast.GenDecl:
            for _, spec := range d.Specs {
                switch s := spec.(type) {
                case *ast.TypeSpec:
                    if s.Name.IsExported() {
                        symbols[s.Name.Name] = Symbol{
                            Name:      s.Name.Name,
                            Kind:      "type",
                            Signature: formatTypeSignature(s),
                        }
                    }
                }
            }
        }
    }

    return symbols
}
```

---

## 9. Audit Trail System

### 9.1 Audit Logger

```go
// internal/cgp/audit/logger.go

package audit

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "time"

    "github.com/relicta-tech/relicta/internal/cgp"
)

// Entry represents an immutable audit record
type Entry struct {
    ID            string                 `json:"id"`
    Timestamp     time.Time              `json:"timestamp"`
    EventType     EventType              `json:"eventType"`
    Actor         cgp.Actor              `json:"actor"`
    Resource      Resource               `json:"resource"`
    Action        string                 `json:"action"`
    Outcome       string                 `json:"outcome"`
    Details       map[string]interface{} `json:"details,omitempty"`
    PreviousHash  string                 `json:"previousHash"`
    Hash          string                 `json:"hash"`
}

// EventType categorizes audit events
type EventType string

const (
    EventProposalReceived    EventType = "proposal.received"
    EventEvaluationStarted   EventType = "evaluation.started"
    EventEvaluationCompleted EventType = "evaluation.completed"
    EventDecisionMade        EventType = "decision.made"
    EventApprovalRequested   EventType = "approval.requested"
    EventApprovalGranted     EventType = "approval.granted"
    EventApprovalDenied      EventType = "approval.denied"
    EventExecutionAuthorized EventType = "execution.authorized"
    EventExecutionStarted    EventType = "execution.started"
    EventExecutionCompleted  EventType = "execution.completed"
    EventExecutionFailed     EventType = "execution.failed"
    EventPolicyViolation     EventType = "policy.violation"
)

// Resource identifies what the audit entry relates to
type Resource struct {
    Type string `json:"type"` // "proposal", "decision", "release"
    ID   string `json:"id"`
}

// Logger provides immutable audit logging
type Logger struct {
    store      Store
    lastHash   string
    hashLock   sync.Mutex
}

// Store persists audit entries
type Store interface {
    Append(ctx context.Context, entry Entry) error
    GetLastHash(ctx context.Context) (string, error)
    Query(ctx context.Context, filter QueryFilter) ([]Entry, error)
}

// Log creates an immutable audit entry
func (l *Logger) Log(ctx context.Context, eventType EventType, actor cgp.Actor, resource Resource, action, outcome string, details map[string]interface{}) error {
    l.hashLock.Lock()
    defer l.hashLock.Unlock()

    entry := Entry{
        ID:           generateID(),
        Timestamp:    time.Now().UTC(),
        EventType:    eventType,
        Actor:        actor,
        Resource:     resource,
        Action:       action,
        Outcome:      outcome,
        Details:      details,
        PreviousHash: l.lastHash,
    }

    // Calculate entry hash (creates tamper-evident chain)
    entry.Hash = l.calculateHash(entry)

    if err := l.store.Append(ctx, entry); err != nil {
        return fmt.Errorf("appending audit entry: %w", err)
    }

    l.lastHash = entry.Hash
    return nil
}

// calculateHash creates a SHA256 hash of the entry
func (l *Logger) calculateHash(entry Entry) string {
    // Exclude Hash field from calculation
    data := struct {
        ID           string
        Timestamp    time.Time
        EventType    EventType
        Actor        cgp.Actor
        Resource     Resource
        Action       string
        Outcome      string
        Details      map[string]interface{}
        PreviousHash string
    }{
        ID:           entry.ID,
        Timestamp:    entry.Timestamp,
        EventType:    entry.EventType,
        Actor:        entry.Actor,
        Resource:     resource,
        Action:       entry.Action,
        Outcome:      entry.Outcome,
        Details:      entry.Details,
        PreviousHash: entry.PreviousHash,
    }

    bytes, _ := json.Marshal(data)
    hash := sha256.Sum256(bytes)
    return hex.EncodeToString(hash[:])
}

// Verify checks the integrity of the audit chain
func (l *Logger) Verify(ctx context.Context) (*VerificationResult, error) {
    entries, err := l.store.Query(ctx, QueryFilter{
        OrderBy: "timestamp",
        Order:   "asc",
    })
    if err != nil {
        return nil, err
    }

    result := &VerificationResult{
        TotalEntries: len(entries),
        Valid:        true,
    }

    var previousHash string
    for i, entry := range entries {
        // Verify hash chain
        if entry.PreviousHash != previousHash {
            result.Valid = false
            result.Errors = append(result.Errors, fmt.Sprintf(
                "Entry %d: previous hash mismatch (expected %s, got %s)",
                i, previousHash, entry.PreviousHash,
            ))
        }

        // Verify entry hash
        calculatedHash := l.calculateHash(entry)
        if entry.Hash != calculatedHash {
            result.Valid = false
            result.Errors = append(result.Errors, fmt.Sprintf(
                "Entry %d: hash mismatch (expected %s, got %s)",
                i, calculatedHash, entry.Hash,
            ))
        }

        previousHash = entry.Hash
    }

    return result, nil
}

// VerificationResult contains audit chain verification outcome
type VerificationResult struct {
    TotalEntries int
    Valid        bool
    Errors       []string
}
```

---

## 10. MCP Integration

### 10.1 MCP Server for CGP

```go
// internal/cgp/mcp/server.go

package mcp

import (
    "context"
    "encoding/json"

    "github.com/relicta-tech/relicta/internal/cgp"
)

// Server implements MCP protocol for CGP
type Server struct {
    governor  *cgp.Governor
    tools     []Tool
    resources []Resource
}

// Tool represents an MCP tool exposed by CGP
type Tool struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    InputSchema json.RawMessage `json:"inputSchema"`
}

// NewServer creates an MCP server for CGP
func NewServer(governor *cgp.Governor) *Server {
    s := &Server{
        governor: governor,
    }
    s.registerTools()
    s.registerResources()
    return s
}

// registerTools defines MCP tools for CGP operations
func (s *Server) registerTools() {
    s.tools = []Tool{
        {
            Name:        "cgp_propose_change",
            Description: "Submit a change proposal for governance evaluation",
            InputSchema: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "repository": {"type": "string", "description": "Repository in owner/name format"},
                    "commitRange": {"type": "string", "description": "Git commit range (e.g., abc123..def456)"},
                    "summary": {"type": "string", "description": "Human-readable summary of changes"},
                    "suggestedBump": {"type": "string", "enum": ["major", "minor", "patch"]},
                    "confidence": {"type": "number", "minimum": 0, "maximum": 1}
                },
                "required": ["repository", "commitRange", "summary"]
            }`),
        },
        {
            Name:        "cgp_get_decision",
            Description: "Get the governance decision for a proposal",
            InputSchema: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "proposalId": {"type": "string", "description": "The proposal ID to query"}
                },
                "required": ["proposalId"]
            }`),
        },
        {
            Name:        "cgp_approve",
            Description: "Approve a pending release (human override)",
            InputSchema: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "proposalId": {"type": "string"},
                    "comment": {"type": "string"},
                    "overrideVersion": {"type": "string"}
                },
                "required": ["proposalId"]
            }`),
        },
        {
            Name:        "cgp_reject",
            Description: "Reject a pending release",
            InputSchema: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "proposalId": {"type": "string"},
                    "reason": {"type": "string"}
                },
                "required": ["proposalId", "reason"]
            }`),
        },
        {
            Name:        "cgp_get_policy",
            Description: "Get current governance policies",
            InputSchema: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "repository": {"type": "string"}
                }
            }`),
        },
    }
}

// HandleToolCall processes an MCP tool invocation
func (s *Server) HandleToolCall(ctx context.Context, toolName string, arguments json.RawMessage, actor cgp.Actor) (interface{}, error) {
    switch toolName {
    case "cgp_propose_change":
        return s.handleProposeChange(ctx, arguments, actor)
    case "cgp_get_decision":
        return s.handleGetDecision(ctx, arguments)
    case "cgp_approve":
        return s.handleApprove(ctx, arguments, actor)
    case "cgp_reject":
        return s.handleReject(ctx, arguments, actor)
    case "cgp_get_policy":
        return s.handleGetPolicy(ctx, arguments)
    default:
        return nil, fmt.Errorf("unknown tool: %s", toolName)
    }
}

// handleProposeChange processes a change proposal from an MCP client
func (s *Server) handleProposeChange(ctx context.Context, args json.RawMessage, actor cgp.Actor) (*cgp.GovernanceDecision, error) {
    var input struct {
        Repository    string  `json:"repository"`
        CommitRange   string  `json:"commitRange"`
        Summary       string  `json:"summary"`
        SuggestedBump string  `json:"suggestedBump"`
        Confidence    float64 `json:"confidence"`
    }

    if err := json.Unmarshal(args, &input); err != nil {
        return nil, fmt.Errorf("parsing arguments: %w", err)
    }

    proposal := &cgp.ChangeProposal{
        CGPVersion: "0.1",
        Type:       "change.proposal",
        ID:         generateProposalID(),
        Timestamp:  time.Now(),
        Actor:      actor,
        Scope: cgp.ProposalScope{
            Repository:  input.Repository,
            CommitRange: input.CommitRange,
        },
        Intent: cgp.ProposalIntent{
            Summary:       input.Summary,
            SuggestedBump: input.SuggestedBump,
            Confidence:    input.Confidence,
        },
    }

    return s.governor.Evaluate(ctx, proposal)
}

// registerResources defines MCP resources exposed by CGP
func (s *Server) registerResources() {
    s.resources = []Resource{
        {
            URI:         "cgp://policies",
            Name:        "Governance Policies",
            Description: "Current release governance policies",
            MimeType:    "application/json",
        },
        {
            URI:         "cgp://pending",
            Name:        "Pending Approvals",
            Description: "Proposals awaiting human approval",
            MimeType:    "application/json",
        },
        {
            URI:         "cgp://audit",
            Name:        "Audit Trail",
            Description: "Recent governance decisions",
            MimeType:    "application/json",
        },
    }
}
```

### 10.2 MCP Configuration

```json
{
  "mcpServers": {
    "relicta-cgp": {
      "command": "relicta",
      "args": ["mcp", "serve"],
      "env": {
        "RELICTA_CGP_ENABLED": "true"
      }
    }
  }
}
```

---

## 11. Governor Implementation

### 11.1 Core Governor

```go
// internal/cgp/governor.go

package cgp

import (
    "context"
    "fmt"
    "time"

    "github.com/relicta-tech/relicta/internal/cgp/analysis"
    "github.com/relicta-tech/relicta/internal/cgp/audit"
    "github.com/relicta-tech/relicta/internal/cgp/policy"
    "github.com/relicta-tech/relicta/internal/cgp/risk"
)

// Governor is the central CGP coordinator
type Governor struct {
    analyzer     *analysis.SemanticAnalyzer
    policyEngine *policy.Engine
    riskCalc     *risk.Calculator
    auditLogger  *audit.Logger
    memory       *ReleaseMemory
    pending      *PendingStore
    config       *GovernorConfig
}

// GovernorConfig configures the governor behavior
type GovernorConfig struct {
    AutoApproveThreshold float64       // Risk score below which auto-approve
    DecisionTimeout      time.Duration // Max time to make decision
    RequireHumanForMajor bool          // Always require human for major releases
}

// NewGovernor creates a configured CGP governor
func NewGovernor(cfg *GovernorConfig, deps GovernorDependencies) *Governor {
    return &Governor{
        analyzer:     deps.Analyzer,
        policyEngine: deps.PolicyEngine,
        riskCalc:     deps.RiskCalculator,
        auditLogger:  deps.AuditLogger,
        memory:       deps.Memory,
        pending:      deps.PendingStore,
        config:       cfg,
    }
}

// Evaluate processes a change proposal and returns a governance decision
func (g *Governor) Evaluate(ctx context.Context, proposal *ChangeProposal) (*GovernanceDecision, error) {
    // Log proposal received
    g.auditLogger.Log(ctx, audit.EventProposalReceived, proposal.Actor,
        audit.Resource{Type: "proposal", ID: proposal.ID},
        "receive", "success",
        map[string]interface{}{
            "repository":  proposal.Scope.Repository,
            "commitRange": proposal.Scope.CommitRange,
        },
    )

    // Perform semantic analysis
    analysis, err := g.analyzer.Analyze(ctx, &proposal.Scope)
    if err != nil {
        return nil, fmt.Errorf("semantic analysis failed: %w", err)
    }

    // Calculate risk score
    riskAssessment, err := g.riskCalc.Calculate(ctx, proposal, analysis)
    if err != nil {
        return nil, fmt.Errorf("risk calculation failed: %w", err)
    }

    // Evaluate policies
    policyResult, err := g.policyEngine.Evaluate(ctx, proposal, analysis)
    if err != nil {
        return nil, fmt.Errorf("policy evaluation failed: %w", err)
    }

    // Determine recommended version
    recommendedVersion := g.determineVersion(proposal, analysis)

    // Build decision
    decision := &GovernanceDecision{
        CGPVersion:         "0.1",
        Type:               "change.decision",
        ID:                 generateDecisionID(),
        ProposalID:         proposal.ID,
        Timestamp:          time.Now(),
        Decision:           policyResult.Decision,
        RecommendedVersion: recommendedVersion,
        RiskScore:          riskAssessment.Score,
        RiskFactors:        riskAssessment.Factors,
        Rationale:          policyResult.Rationale,
        RequiredActions:    policyResult.RequiredActions,
        Analysis:           analysis,
    }

    // Apply auto-approval logic
    if decision.Decision == DecisionApproved && g.config.RequireHumanForMajor {
        if isMajorRelease(recommendedVersion) {
            decision.Decision = DecisionApprovalRequired
            decision.Rationale = append(decision.Rationale, "Major releases require human approval")
            decision.RequiredActions = append(decision.RequiredActions, RequiredAction{
                Type:        "human_approval",
                Description: "Major version bump requires explicit approval",
            })
        }
    }

    // Store pending if approval required
    if decision.Decision == DecisionApprovalRequired {
        if err := g.pending.Store(ctx, proposal, decision); err != nil {
            return nil, fmt.Errorf("storing pending decision: %w", err)
        }
    }

    // Log decision
    g.auditLogger.Log(ctx, audit.EventDecisionMade, proposal.Actor,
        audit.Resource{Type: "decision", ID: decision.ID},
        "decide", string(decision.Decision),
        map[string]interface{}{
            "riskScore":          decision.RiskScore,
            "recommendedVersion": decision.RecommendedVersion,
            "matchedRules":       policyResult.MatchedRules,
        },
    )

    return decision, nil
}

// Approve processes human approval for a pending proposal
func (g *Governor) Approve(ctx context.Context, proposalID string, actor Actor, opts ApprovalOptions) (*ExecutionAuthorization, error) {
    // Retrieve pending proposal and decision
    pending, err := g.pending.Get(ctx, proposalID)
    if err != nil {
        return nil, fmt.Errorf("proposal not found: %w", err)
    }

    // Verify actor has approval authority
    if err := g.verifyApprovalAuthority(actor, pending.Decision); err != nil {
        g.auditLogger.Log(ctx, audit.EventPolicyViolation, actor,
            audit.Resource{Type: "proposal", ID: proposalID},
            "approve", "denied",
            map[string]interface{}{"error": err.Error()},
        )
        return nil, err
    }

    // Build authorization
    version := pending.Decision.RecommendedVersion
    if opts.OverrideVersion != "" {
        version = opts.OverrideVersion
    }

    auth := &ExecutionAuthorization{
        CGPVersion: "0.1",
        Type:       "change.execution_authorized",
        ID:         generateAuthorizationID(),
        DecisionID: pending.Decision.ID,
        ProposalID: proposalID,
        Timestamp:  time.Now(),
        ApprovedBy: actor,
        ApprovedAt: time.Now(),
        Version:    version,
        Tag:        fmt.Sprintf("v%s", version),
        ValidUntil: time.Now().Add(24 * time.Hour), // 24 hour validity
        AllowedSteps: []string{
            "tag", "changelog", "release_notes", "publish", "notify",
        },
        ApprovalChain: []ApprovalRecord{
            {
                Actor:     actor,
                Action:    "approve",
                Timestamp: time.Now(),
                Comment:   opts.Comment,
            },
        },
    }

    // Log approval
    g.auditLogger.Log(ctx, audit.EventApprovalGranted, actor,
        audit.Resource{Type: "proposal", ID: proposalID},
        "approve", "granted",
        map[string]interface{}{
            "version":    version,
            "validUntil": auth.ValidUntil,
        },
    )

    // Remove from pending
    g.pending.Remove(ctx, proposalID)

    // Record in release memory
    g.memory.RecordApproval(ctx, auth)

    return auth, nil
}

// ApprovalOptions for human approval
type ApprovalOptions struct {
    Comment         string
    OverrideVersion string
    ReleaseNotes    string
}

// determineVersion calculates the recommended version
func (g *Governor) determineVersion(proposal *ChangeProposal, analysis *ChangeAnalysis) string {
    // Use suggested if confident enough
    if proposal.Intent.SuggestedBump != "" && proposal.Intent.Confidence >= 0.8 {
        return proposal.Intent.SuggestedBump
    }

    // Determine from analysis
    if analysis.Breaking > 0 {
        return "major"
    }
    if analysis.Features > 0 {
        return "minor"
    }
    return "patch"
}
```

---

## 12. Release Memory

### 12.1 Historical Learning

```go
// internal/cgp/memory/memory.go

package memory

import (
    "context"
    "time"

    "github.com/relicta-tech/relicta/internal/cgp"
)

// ReleaseMemory maintains historical context for learning
type ReleaseMemory struct {
    store Store
}

// Store persists release history
type Store interface {
    SaveRelease(ctx context.Context, record *ReleaseRecord) error
    GetReleaseHistory(ctx context.Context, repository string, limit int) ([]ReleaseRecord, error)
    GetActorHistory(ctx context.Context, actorID string, limit int) ([]ReleaseRecord, error)
    GetPatterns(ctx context.Context, repository string) (*PatternAnalysis, error)
}

// ReleaseRecord captures a complete release lifecycle
type ReleaseRecord struct {
    ID             string            `json:"id"`
    Repository     string            `json:"repository"`
    Version        string            `json:"version"`
    Actor          cgp.Actor         `json:"actor"`
    ProposalID     string            `json:"proposalId"`
    DecisionID     string            `json:"decisionId"`
    RiskScore      float64           `json:"riskScore"`
    Analysis       *cgp.ChangeAnalysis `json:"analysis"`

    // Timing
    ProposedAt     time.Time         `json:"proposedAt"`
    ApprovedAt     *time.Time        `json:"approvedAt,omitempty"`
    ExecutedAt     *time.Time        `json:"executedAt,omitempty"`

    // Outcome
    Status         ReleaseStatus     `json:"status"`
    RolledBack     bool              `json:"rolledBack"`
    RollbackReason string            `json:"rollbackReason,omitempty"`
    CausedIncident bool              `json:"causedIncident"`
    IncidentID     string            `json:"incidentId,omitempty"`

    // Feedback
    Feedback       *ReleaseFeedback  `json:"feedback,omitempty"`
}

// ReleaseStatus represents the outcome of a release
type ReleaseStatus string

const (
    ReleaseStatusPending   ReleaseStatus = "pending"
    ReleaseStatusApproved  ReleaseStatus = "approved"
    ReleaseStatusExecuted  ReleaseStatus = "executed"
    ReleaseStatusFailed    ReleaseStatus = "failed"
    ReleaseStatusRolledBack ReleaseStatus = "rolled_back"
)

// ReleaseFeedback captures post-release information
type ReleaseFeedback struct {
    UserReported   []string  `json:"userReported,omitempty"`   // User-reported issues
    MetricsImpact  []string  `json:"metricsImpact,omitempty"`  // Metric changes observed
    QualityScore   float64   `json:"qualityScore"`             // 0-1 quality assessment
    LessonsLearned []string  `json:"lessonsLearned,omitempty"` // Retrospective notes
}

// PatternAnalysis identifies trends in release history
type PatternAnalysis struct {
    Repository        string             `json:"repository"`
    TotalReleases     int                `json:"totalReleases"`
    SuccessRate       float64            `json:"successRate"`
    AverageRiskScore  float64            `json:"averageRiskScore"`
    RollbackRate      float64            `json:"rollbackRate"`

    // Risk patterns
    HighRiskFactors   []string           `json:"highRiskFactors"`   // Frequently high-risk areas
    SafePatterns      []string           `json:"safePatterns"`      // Changes that rarely cause issues

    // Actor patterns
    ActorPerformance  map[string]float64 `json:"actorPerformance"`  // Success rate by actor

    // Time patterns
    BestReleaseDay    string             `json:"bestReleaseDay"`    // Day with highest success
    AvoidTimes        []string           `json:"avoidTimes"`        // Times with higher failure
}

// RecordRelease saves a release outcome to memory
func (m *ReleaseMemory) RecordRelease(ctx context.Context, record *ReleaseRecord) error {
    return m.store.SaveRelease(ctx, record)
}

// RecordFeedback updates a release with post-release feedback
func (m *ReleaseMemory) RecordFeedback(ctx context.Context, releaseID string, feedback *ReleaseFeedback) error {
    // Retrieve existing record
    records, err := m.store.GetReleaseHistory(ctx, "", 1000)
    if err != nil {
        return err
    }

    for _, record := range records {
        if record.ID == releaseID {
            record.Feedback = feedback
            return m.store.SaveRelease(ctx, &record)
        }
    }

    return fmt.Errorf("release not found: %s", releaseID)
}

// GetRiskPrediction uses historical data to predict risk
func (m *ReleaseMemory) GetRiskPrediction(ctx context.Context, proposal *cgp.ChangeProposal, analysis *cgp.ChangeAnalysis) (*RiskPrediction, error) {
    patterns, err := m.store.GetPatterns(ctx, proposal.Scope.Repository)
    if err != nil {
        return nil, err
    }

    // Analyze current proposal against patterns
    prediction := &RiskPrediction{
        HistoricalSuccess: patterns.SuccessRate,
        PredictedRisk:     patterns.AverageRiskScore,
        Factors:           []string{},
    }

    // Check if changes match high-risk patterns
    for _, factor := range patterns.HighRiskFactors {
        if matchesPattern(analysis, factor) {
            prediction.PredictedRisk += 0.1
            prediction.Factors = append(prediction.Factors,
                fmt.Sprintf("Historical high-risk pattern: %s", factor))
        }
    }

    // Check actor history
    if actorSuccess, ok := patterns.ActorPerformance[proposal.Actor.ID]; ok {
        if actorSuccess < 0.8 {
            prediction.PredictedRisk += 0.1
            prediction.Factors = append(prediction.Factors,
                fmt.Sprintf("Actor historical success rate: %.0f%%", actorSuccess*100))
        }
    }

    return prediction, nil
}

// RiskPrediction based on historical patterns
type RiskPrediction struct {
    HistoricalSuccess float64  `json:"historicalSuccess"`
    PredictedRisk     float64  `json:"predictedRisk"`
    Factors           []string `json:"factors"`
}
```

---

## 13. Project Structure

```
internal/
├── cgp/                          # CGP implementation
│   ├── actor.go                  # Actor definitions
│   ├── proposal.go               # ChangeProposal types
│   ├── decision.go               # GovernanceDecision types
│   ├── authorization.go          # ExecutionAuthorization types
│   ├── governor.go               # Central governor
│   │
│   ├── analysis/                 # Semantic analysis
│   │   ├── semantic.go           # Code analyzer
│   │   ├── go_analyzer.go        # Go-specific analysis
│   │   └── diff.go               # Diff utilities
│   │
│   ├── policy/                   # Policy engine
│   │   ├── policy.go             # Policy definitions
│   │   ├── engine.go             # Evaluation engine
│   │   ├── conditions.go         # Condition evaluators
│   │   └── actions.go            # Action handlers
│   │
│   ├── risk/                     # Risk scoring
│   │   ├── calculator.go         # Risk calculator
│   │   ├── factors.go            # Individual factors
│   │   └── weights.go            # Weight configuration
│   │
│   ├── audit/                    # Audit trail
│   │   ├── logger.go             # Audit logger
│   │   ├── store.go              # Storage interface
│   │   └── verify.go             # Chain verification
│   │
│   ├── memory/                   # Release memory
│   │   ├── memory.go             # Memory service
│   │   ├── patterns.go           # Pattern analysis
│   │   └── prediction.go         # Risk prediction
│   │
│   ├── mcp/                      # MCP integration
│   │   ├── server.go             # MCP server
│   │   ├── tools.go              # Tool definitions
│   │   └── resources.go          # Resource definitions
│   │
│   └── pending/                  # Pending approvals
│       └── store.go              # Pending storage
```

---

## 14. Configuration

### 14.1 CGP Configuration Schema

```yaml
# release.config.yaml - CGP section

cgp:
  enabled: true
  version: "0.1"

  governor:
    autoApproveThreshold: 0.3      # Auto-approve if risk < 0.3
    decisionTimeout: 5m            # Max time for decision
    requireHumanForMajor: true     # Always require human for major

  policy:
    file: ".relicta/release-policy.yaml"  # Policy file path

  risk:
    weights:
      apiChanges: 0.25
      dependencyImpact: 0.20
      blastRadius: 0.15
      codeComplexity: 0.10
      testCoverage: 0.10
      actorTrust: 0.05
      historicalRisk: 0.10
      securityImpact: 0.05

  audit:
    enabled: true
    retention: 365d               # Keep audit logs for 1 year
    store: file                   # "file", "database", "s3"
    path: ".relicta/audit/"

  memory:
    enabled: true
    learningEnabled: true          # Enable pattern learning
    store: file
    path: ".relicta/memory/"

  mcp:
    enabled: true
    port: 0                        # 0 = stdio mode for MCP
```

---

## 15. Implementation Phases

### Phase 1: Foundation (MVP)
- [ ] CGP message types and schemas
- [ ] Basic governor with policy evaluation
- [ ] Simple risk scoring (no ML)
- [ ] File-based audit trail
- [ ] CLI integration (`relicta cgp evaluate`)

### Phase 2: Intelligence
- [ ] Semantic analysis for Go
- [ ] Historical risk learning
- [ ] Pattern detection
- [ ] Enhanced risk factors

### Phase 3: Integration
- [ ] MCP server implementation
- [ ] Multi-language semantic analysis
- [ ] Database-backed storage
- [ ] Web UI for approvals

### Phase 4: Enterprise
- [ ] Multi-tenant policies
- [ ] SSO/SAML integration
- [ ] Compliance reporting
- [ ] API access controls

---

## 16. Security Considerations

### 16.1 Authentication & Authorization
- All actors must be authenticated
- Credentials are never stored in plain text
- Actor trust levels determine capabilities
- Policy enforcement is not bypassable

### 16.2 Audit Trail Integrity
- Cryptographic hash chain prevents tampering
- All actions are attributed to actors
- Verification available on-demand
- Retention policies enforced

### 16.3 Data Protection
- Sensitive data (API keys, tokens) excluded from logs
- Policy files can reference secrets via environment variables
- MCP communication encrypted in transit

---

## 17. Metrics & Observability

### 17.1 Key Metrics
- `cgp_proposals_total` - Total proposals received
- `cgp_decisions_total{decision}` - Decisions by type
- `cgp_risk_score_histogram` - Distribution of risk scores
- `cgp_approval_latency_seconds` - Time to human approval
- `cgp_policy_violations_total` - Policy violations detected
- `cgp_releases_total{status}` - Releases by outcome

### 17.2 Alerts
- High risk score (> 0.8) requires immediate attention
- Unusual actor behavior patterns
- Audit chain integrity failures
- Policy evaluation errors

---

## 18. Glossary

| Term | Definition |
|------|------------|
| **CGP** | Change Governance Protocol - the standardized protocol for release governance |
| **Proposer** | Entity submitting a change proposal (agent, CI, human) |
| **Governor** | System implementing CGP that evaluates proposals (Relicta) |
| **Executor** | System that carries out approved actions (plugins) |
| **Blast Radius** | Potential impact scope of a change |
| **Risk Score** | 0-1 measure of change risk |
| **Release Memory** | Historical context for learning and prediction |
| **MCP** | Model Context Protocol - AI agent communication standard |

---

*This document is the authoritative technical specification for CGP implementation in Relicta.*
