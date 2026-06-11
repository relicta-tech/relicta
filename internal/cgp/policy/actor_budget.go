package policy

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/pkg/cgp"
)

// BlastRadius categorizes the operational scope an action affects.
// Ordered from least to most disruptive.
type BlastRadius string

const (
	BlastRadiusNone     BlastRadius = "none"
	BlastRadiusLow      BlastRadius = "low"
	BlastRadiusMedium   BlastRadius = "medium"
	BlastRadiusHigh     BlastRadius = "high"
	BlastRadiusCritical BlastRadius = "critical"
)

// blastRadiusOrder maps levels to ordinal values for comparison.
var blastRadiusOrder = map[BlastRadius]int{
	BlastRadiusNone:     0,
	BlastRadiusLow:      1,
	BlastRadiusMedium:   2,
	BlastRadiusHigh:     3,
	BlastRadiusCritical: 4,
}

// LessThan reports whether b is strictly less severe than other.
func (b BlastRadius) LessThan(other BlastRadius) bool {
	return blastRadiusOrder[b] < blastRadiusOrder[other]
}

// LessThanOrEqual reports whether b is at most as severe as other.
func (b BlastRadius) LessThanOrEqual(other BlastRadius) bool {
	return blastRadiusOrder[b] <= blastRadiusOrder[other]
}

// Valid reports whether b is a known blast-radius level.
func (b BlastRadius) Valid() bool {
	_, ok := blastRadiusOrder[b]
	return ok
}

// ActorBudget defines per-actor governance limits enforced before privileged
// operations (publish, approve, rollback). Budgets compose with the policy
// engine: they constrain WHAT an actor may do, while policies decide WHEN.
//
// A budget matching agent or CI actors is the foundation of the autonomy
// slider — humans get permissive defaults; non-human actors get explicit caps.
type ActorBudget struct {
	// ActorKind matches against cgp.Actor.Kind: "agent", "ci", "human", "system".
	// Empty matches any kind.
	ActorKind string `json:"actorKind,omitempty" yaml:"actor_kind,omitempty"`

	// ActorID is a glob pattern matched against cgp.Actor.ID.
	// Empty or "*" matches any ID. Examples: "claude-code-*", "agent:cursor".
	ActorID string `json:"actorId,omitempty" yaml:"actor_id,omitempty"`

	// MaxBlastRadius caps the operational scope. Zero value means unrestricted.
	MaxBlastRadius BlastRadius `json:"maxBlastRadius,omitempty" yaml:"max_blast_radius,omitempty"`

	// MaxRiskScore caps the CGP risk score (0.0–1.0). Zero means unrestricted.
	MaxRiskScore float64 `json:"maxRiskScore,omitempty" yaml:"max_risk_score,omitempty"`

	// MaxDollarCostUSD caps cumulative dollar cost (e.g., LLM spend or
	// infra cost) within the time window. Zero means unrestricted.
	MaxDollarCostUSD float64 `json:"maxDollarCostUsd,omitempty" yaml:"max_dollar_cost_usd,omitempty"`

	// RequiresCosign lists operation names that require a human cosigner
	// regardless of risk. e.g. ["publish", "approve", "rollback"].
	RequiresCosign []string `json:"requiresCosign,omitempty" yaml:"requires_cosign,omitempty"`

	// AllowedTools restricts which MCP tool names this actor may invoke.
	// Empty list means all tools allowed (subject to DeniedTools).
	AllowedTools []string `json:"allowedTools,omitempty" yaml:"allowed_tools,omitempty"`

	// DeniedTools blocks specific MCP tool names. Always overrides AllowedTools.
	DeniedTools []string `json:"deniedTools,omitempty" yaml:"denied_tools,omitempty"`

	// TimeWindow restricts when budgeted operations may run.
	// Zero value means any time. Format follows TimeContext.
	TimeWindow *BudgetTimeWindow `json:"timeWindow,omitempty" yaml:"time_window,omitempty"`
}

// BudgetTimeWindow restricts when an actor may operate.
type BudgetTimeWindow struct {
	// Days lists permitted weekdays as lowercase 3-letter abbreviations
	// (e.g. ["mon", "tue", "wed", "thu", "fri"]). Empty means any day.
	Days []string `json:"days,omitempty" yaml:"days,omitempty"`

	// StartHour is the inclusive starting hour in UTC (0-23).
	// Both StartHour and EndHour must be set to enforce a window.
	StartHour *int `json:"startHour,omitempty" yaml:"start_hour,omitempty"`

	// EndHour is the exclusive ending hour in UTC (0-23).
	EndHour *int `json:"endHour,omitempty" yaml:"end_hour,omitempty"`
}

// ActorBudgetSet is an ordered collection of budgets. Order matters:
// the first budget whose ActorKind+ActorID glob matches is applied.
// Place specific entries before wildcards.
type ActorBudgetSet struct {
	Budgets []ActorBudget `json:"budgets" yaml:"budgets"`
}

// Validate ensures the budget set is internally consistent.
// Returns the first validation error encountered or nil.
func (s *ActorBudgetSet) Validate() error {
	for i, b := range s.Budgets {
		if b.MaxBlastRadius != "" && !b.MaxBlastRadius.Valid() {
			return fmt.Errorf("budget %d: invalid max_blast_radius %q", i, b.MaxBlastRadius)
		}
		if b.MaxRiskScore < 0 || b.MaxRiskScore > 1 {
			return fmt.Errorf("budget %d: max_risk_score must be in [0,1], got %v", i, b.MaxRiskScore)
		}
		if b.MaxDollarCostUSD < 0 {
			return fmt.Errorf("budget %d: max_dollar_cost_usd must be non-negative", i)
		}
		if b.TimeWindow != nil {
			if (b.TimeWindow.StartHour == nil) != (b.TimeWindow.EndHour == nil) {
				return fmt.Errorf("budget %d: start_hour and end_hour must both be set or both empty", i)
			}
			if b.TimeWindow.StartHour != nil {
				if *b.TimeWindow.StartHour < 0 || *b.TimeWindow.StartHour > 23 {
					return fmt.Errorf("budget %d: start_hour out of range", i)
				}
				if *b.TimeWindow.EndHour < 0 || *b.TimeWindow.EndHour > 23 {
					return fmt.Errorf("budget %d: end_hour out of range", i)
				}
			}
			for _, day := range b.TimeWindow.Days {
				if !validDay(day) {
					return fmt.Errorf("budget %d: invalid day %q", i, day)
				}
			}
		}
	}
	return nil
}

// Match returns the first budget whose ActorKind+ActorID match the given actor,
// or nil if none match. ActorID supports glob patterns via path.Match semantics.
func (s *ActorBudgetSet) Match(actor cgp.Actor) *ActorBudget {
	for i := range s.Budgets {
		b := &s.Budgets[i]
		if b.matches(actor) {
			return b
		}
	}
	return nil
}

func (b *ActorBudget) matches(actor cgp.Actor) bool {
	if b.ActorKind != "" && b.ActorKind != actor.Kind {
		return false
	}
	if b.ActorID == "" || b.ActorID == "*" {
		return true
	}
	matched, err := path.Match(b.ActorID, actor.ID)
	if err != nil {
		return false
	}
	return matched
}

// Operation is a single privileged action subject to budget enforcement.
type Operation struct {
	// Tool is the MCP tool or CLI command name (e.g. "relicta_publish").
	Tool string

	// BlastRadius is the operation's effective blast radius.
	BlastRadius BlastRadius

	// RiskScore is the CGP risk score for the change being acted upon.
	RiskScore float64

	// DollarCostUSD is the estimated dollar cost incurred by this operation.
	DollarCostUSD float64

	// HasCosigner indicates whether a human cosigner has signed off.
	HasCosigner bool

	// At is the wall-clock time at which the operation is being attempted.
	// Zero value means use time.Now().
	At time.Time
}

// Violation captures a single budget rule infraction.
type Violation struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Limit   any    `json:"limit"`
	Actual  any    `json:"actual,omitempty"`
}

// Decision is the outcome of evaluating a budget against an operation.
type Decision struct {
	Allowed    bool         `json:"allowed"`
	Budget     *ActorBudget `json:"budget,omitempty"`
	Violations []Violation  `json:"violations,omitempty"`
}

// ErrNoBudget indicates Evaluate was called with a nil budget — the caller
// must decide policy (e.g. fail-closed for agents, permissive for humans).
var ErrNoBudget = errors.New("no budget defined for actor")

// Evaluate checks whether an operation respects the actor's budget.
// Returns a Decision with all violations found (does not short-circuit
// so callers can surface every breach to the user).
func (b *ActorBudget) Evaluate(op Operation) Decision {
	d := Decision{Budget: b}

	if b == nil {
		d.Allowed = false
		d.Violations = append(d.Violations, Violation{
			Code:    "no_budget",
			Message: "no budget defined; refusing to evaluate",
		})
		return d
	}

	if b.MaxBlastRadius != "" && op.BlastRadius != "" && !op.BlastRadius.LessThanOrEqual(b.MaxBlastRadius) {
		d.Violations = append(d.Violations, Violation{
			Code:    "blast_radius_exceeded",
			Message: fmt.Sprintf("blast radius %q exceeds budget cap %q", op.BlastRadius, b.MaxBlastRadius),
			Limit:   b.MaxBlastRadius,
			Actual:  op.BlastRadius,
		})
	}

	if b.MaxRiskScore > 0 && op.RiskScore > b.MaxRiskScore {
		d.Violations = append(d.Violations, Violation{
			Code:    "risk_score_exceeded",
			Message: fmt.Sprintf("risk score %.2f exceeds budget cap %.2f", op.RiskScore, b.MaxRiskScore),
			Limit:   b.MaxRiskScore,
			Actual:  op.RiskScore,
		})
	}

	if b.MaxDollarCostUSD > 0 && op.DollarCostUSD > b.MaxDollarCostUSD {
		d.Violations = append(d.Violations, Violation{
			Code:    "cost_exceeded",
			Message: fmt.Sprintf("dollar cost $%.2f exceeds budget cap $%.2f", op.DollarCostUSD, b.MaxDollarCostUSD),
			Limit:   b.MaxDollarCostUSD,
			Actual:  op.DollarCostUSD,
		})
	}

	if requiresCosign(b.RequiresCosign, op.Tool) && !op.HasCosigner {
		d.Violations = append(d.Violations, Violation{
			Code:    "cosigner_required",
			Message: fmt.Sprintf("operation %q requires a human cosigner", op.Tool),
			Limit:   "cosigner",
		})
	}

	if !toolAllowed(b, op.Tool) {
		d.Violations = append(d.Violations, Violation{
			Code:    "tool_denied",
			Message: fmt.Sprintf("tool %q is not permitted by this budget", op.Tool),
			Limit:   "allowed_tools/denied_tools",
			Actual:  op.Tool,
		})
	}

	if b.TimeWindow != nil && !b.TimeWindow.Permits(op.At) {
		d.Violations = append(d.Violations, Violation{
			Code:    "outside_time_window",
			Message: "operation attempted outside permitted time window",
			Limit:   b.TimeWindow,
			Actual:  effectiveTime(op.At).UTC(),
		})
	}

	d.Allowed = len(d.Violations) == 0
	return d
}

// Permits reports whether the given time falls within the window.
// A zero StartHour/EndHour means hour check is skipped; an empty Days
// list means day check is skipped.
func (w *BudgetTimeWindow) Permits(at time.Time) bool {
	t := effectiveTime(at).UTC()

	if len(w.Days) > 0 {
		day := strings.ToLower(t.Weekday().String()[:3])
		if !contains(w.Days, day) {
			return false
		}
	}

	if w.StartHour != nil && w.EndHour != nil {
		hr := t.Hour()
		start := *w.StartHour
		end := *w.EndHour
		if start <= end {
			if hr < start || hr >= end {
				return false
			}
		} else {
			// Wrap-around window (e.g. start=22, end=6).
			if hr < start && hr >= end {
				return false
			}
		}
	}

	return true
}

// DefaultPermissiveHumanBudget returns a budget with no caps — the canonical
// human default. Use as a fallback when no explicit budget matches a human actor.
func DefaultPermissiveHumanBudget() *ActorBudget {
	return &ActorBudget{
		ActorKind: "human",
		ActorID:   "*",
	}
}

// DefaultRestrictiveAgentBudget returns a conservative agent budget for the
// "no policy file authored" case. Refuses major releases and risk > 0.4.
// Used by callers that want to fail-closed when no budget matches an agent actor.
func DefaultRestrictiveAgentBudget() *ActorBudget {
	return &ActorBudget{
		ActorKind:      "agent",
		ActorID:        "*",
		MaxBlastRadius: BlastRadiusMedium,
		MaxRiskScore:   0.4,
		RequiresCosign: []string{"publish", "approve", "rollback"},
	}
}

// requiresCosign returns whether the operation name appears in the cosign list.
func requiresCosign(list []string, op string) bool {
	for _, n := range list {
		if n == op {
			return true
		}
	}
	return false
}

// toolAllowed enforces AllowedTools/DeniedTools precedence: deny wins.
func toolAllowed(b *ActorBudget, tool string) bool {
	if tool == "" {
		return true
	}
	for _, denied := range b.DeniedTools {
		if denied == tool {
			return false
		}
	}
	if len(b.AllowedTools) == 0 {
		return true
	}
	for _, allowed := range b.AllowedTools {
		if allowed == tool {
			return true
		}
	}
	return false
}

// effectiveTime returns t if non-zero, otherwise time.Now().
func effectiveTime(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now()
	}
	return t
}

// contains is a small slice helper for case-insensitive day lookup.
func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if strings.EqualFold(h, needle) {
			return true
		}
	}
	return false
}

// validDay reports whether s is a recognized 3-letter weekday abbreviation.
func validDay(s string) bool {
	switch strings.ToLower(s) {
	case "mon", "tue", "wed", "thu", "fri", "sat", "sun":
		return true
	}
	return false
}
