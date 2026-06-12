// Package plugin provides the plugin runtime for Relicta.
package plugin

import (
	"fmt"

	"github.com/relicta-tech/relicta/v4/pkg/plugin"
)

// hookRiskFloor maps hooks to the minimum risk class a plugin registering
// them must declare. Publish-phase hooks mutate repository or remote state
// (tags, releases, package registries), so a plugin claiming to be passive
// while registering them carries an inconsistent — and therefore untrusted —
// declaration.
var hookRiskFloor = map[plugin.Hook]plugin.RiskClass{
	plugin.HookPrePublish:  plugin.RiskActive,
	plugin.HookPostPublish: plugin.RiskActive,
}

// riskRank orders risk classes for comparison.
func riskRank(c plugin.RiskClass) int {
	switch c {
	case plugin.RiskPassive:
		return 0
	case plugin.RiskActive:
		return 1
	case plugin.RiskRuntime:
		return 2
	default:
		return -1
	}
}

// SafetyDecision is the outcome of validating a plugin's safety declaration.
type SafetyDecision struct {
	// RiskClass is the effective risk class (declared, or RiskRuntime when
	// the plugin declares nothing).
	RiskClass plugin.RiskClass
	// Declared reports whether the plugin shipped a declaration at all.
	Declared bool
	// Warning carries a non-fatal note for the operator (e.g. undeclared
	// legacy plugin grace period).
	Warning string
}

// ValidateSafety checks a plugin's declared SafetyRequirements against the
// host policy (ADR-008 Phase 2). Enforcement, by case:
//
//   - inconsistent declaration (risk class below the floor implied by the
//     hooks the plugin registers): refused outright — a manifest that
//     understates its power is worse than none;
//   - declared runtime class: requires explicit operator trust
//     (--allow-untrusted-plugins), same gate as unverified binaries;
//   - undeclared (legacy plugins): allowed with a warning for now; treated
//     as runtime-class for reporting. This becomes a refusal once the
//     ecosystem ships declarations.
func ValidateSafety(info plugin.Info, allowUntrusted bool) (*SafetyDecision, error) {
	if info.Safety == nil {
		return &SafetyDecision{
			RiskClass: plugin.RiskRuntime,
			Declared:  false,
			Warning: fmt.Sprintf(
				"plugin %q declares no safety requirements; treating as risk_class=runtime (legacy grace — future versions will refuse undeclared plugins)",
				info.Name),
		}, nil
	}

	decl := info.Safety
	if riskRank(decl.RiskClass) < 0 {
		return nil, fmt.Errorf("plugin %q declares unknown risk_class %q (passive | active | runtime)", info.Name, decl.RiskClass)
	}

	// A declaration must be at least as powerful as the hooks it registers.
	for _, h := range info.Hooks {
		if floor, ok := hookRiskFloor[h]; ok && riskRank(decl.RiskClass) < riskRank(floor) {
			return nil, fmt.Errorf(
				"plugin %q declares risk_class=%s but registers hook %q which requires at least %s: declaration is inconsistent; refusing to load",
				info.Name, decl.RiskClass, h, floor)
		}
	}

	if decl.RiskClass == plugin.RiskRuntime && !allowUntrusted {
		return nil, fmt.Errorf(
			"plugin %q declares risk_class=runtime (arbitrary execution); pass --allow-untrusted-plugins to load it",
			info.Name)
	}

	return &SafetyDecision{RiskClass: decl.RiskClass, Declared: true}, nil
}
