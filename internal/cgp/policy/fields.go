package policy

import (
	"sort"
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
)

// A condition can name a field the evaluator does not provide, and nothing says
// so. getNestedValue returns (nil, false), evaluateRule records
// MissingField: true, and the rule reports itself as not matched — which is
// indistinguishable from a rule that was correctly evaluated and did not apply.
// The trace field has been set since it was written and read by nothing.
//
// The visible consequence was in the policies this project ships. time-based.policy
// declares a rule named "freeze-period-block", and with it installed a release
// scoring 0.95 risk on a major bump came back `approved`, rationale "Applied
// default policy" — because the condition is `time.is_freeze` and the evaluator
// provides `time.freeze.active`. Five enabled rules, none able to fire, and no
// output anywhere saying so. A governance tool that silently ignores the rules it
// was given is worse than one with no rules, because the operator believes the
// rules are in force.
//
// The fix has to be static: by the time evaluation runs, the answer arrives as a
// decision someone acts on. KnownFieldPaths enumerates what the evaluator can
// resolve, so `relicta policy validate` can say "this condition will never match"
// before the policy is trusted.

// KnownFieldPaths returns every field path the policy evaluator can resolve, in
// sorted order.
//
// It is derived from buildEvalContext rather than hand-listed, so a field added
// to the evaluator is accepted by validation without anyone remembering to
// update a second list. That property is the point: a hand-maintained list of
// valid fields would drift, and the drift would reject correct policies.
func KnownFieldPaths() []string {
	paths := make(map[string]struct{})
	collectPaths("", representativeEvalContext(), paths)

	out := make([]string, 0, len(paths))
	for p := range paths {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// representativeEvalContext builds an evaluation context with every optional
// branch populated.
//
// Optional sub-contexts are omitted when their source is nil — blastRadius only
// exists when analysis.BlastRadius does. A context built from empty inputs would
// therefore report those fields as unknown and validation would reject policies
// that are correct whenever the data is present. The values here are arbitrary;
// only the shape is read.
func representativeEvalContext() map[string]any {
	proposal := &cgp.ChangeProposal{
		Actor: cgp.Actor{
			Kind:       cgp.ActorKindHuman,
			ID:         "human:example",
			Name:       "Example",
			TrustLevel: cgp.TrustLevelTrusted,
		},
		Scope: cgp.ProposalScope{
			Repository:  "owner/repo",
			Branch:      "main",
			CommitRange: "HEAD~1..HEAD",
			Commits:     []string{"0000000"},
			Files:       []string{"main.go"},
		},
		Intent: cgp.ProposalIntent{Summary: "example", SuggestedBump: cgp.BumpTypeMinor},
		// actor.reputation.* exists only when the governance service computed a
		// reputation for this actor, and a context built without one would report
		// those paths as unknown — `policy validate` would then reject a correct
		// policy, and `policy fields` would not list fields the evaluator can in
		// fact resolve. Same reason blastRadius is populated below.
		Context: &cgp.ProposalContext{
			ActorReputation: &cgp.ActorReputation{
				Overall:    0.9,
				Level:      "trusted",
				SampleSize: 12,
				Trend:      "stable",
			},
		},
	}

	analysis := &cgp.ChangeAnalysis{
		Features:    1,
		BlastRadius: &cgp.BlastRadius{Score: 0.1, FilesChanged: 1, LinesChanged: 1},
	}

	return buildEvalContext(proposal, analysis, 0.1, DefaultTimeContext(), DefaultTeamContext(), "human:example")
}

// collectPaths records every path in the context, including intermediate maps —
// a policy may compare a whole branch, and more importantly a caller needs the
// branch to recognize dynamic children under it.
func collectPaths(prefix string, value any, out map[string]struct{}) {
	if prefix != "" {
		out[prefix] = struct{}{}
	}

	m, ok := value.(map[string]any)
	if !ok {
		return
	}
	for key, child := range m {
		next := key
		if prefix != "" {
			next = prefix + "." + key
		}
		collectPaths(next, child, out)
	}
}

// IsKnownFieldPath reports whether the evaluator can resolve a condition field.
//
// A path is accepted when the evaluator provides it, and also when it descends
// into a map the evaluator provides: team.teams and team.roles are keyed by names
// that only exist in a given repository's configuration, so
// `team.teams.platform.leads` is legitimate and cannot appear in a static
// enumeration. Being permissive there is deliberate — the purpose is to catch
// fields that do not exist at all, and a check that rejected configuration-
// specific names would make itself unusable.
func IsKnownFieldPath(field string) bool {
	if field == "" {
		return false
	}

	known := make(map[string]struct{})
	collectPaths("", representativeEvalContext(), known)

	if _, ok := known[field]; ok {
		return true
	}

	// Walk from the longest prefix down: if any ancestor is a known map, the
	// remainder is a dynamic key under it.
	for i := strings.LastIndex(field, "."); i > 0; i = strings.LastIndex(field[:i], ".") {
		if _, ok := known[field[:i]]; ok {
			return dynamicSubtrees[field[:i]] || isDynamicDescendant(field[:i])
		}
	}
	return false
}

// dynamicSubtrees are the contexts whose keys come from repository configuration
// rather than from the evaluator. Anything below them is unverifiable statically,
// so it is accepted.
var dynamicSubtrees = map[string]bool{
	"team.teams": true,
	"team.roles": true,
}

// isDynamicDescendant reports whether a known path sits underneath a dynamic
// subtree, which makes its own children dynamic too — team.teams.platform is
// itself dynamic, so team.teams.platform.leads must be accepted.
func isDynamicDescendant(path string) bool {
	for subtree := range dynamicSubtrees {
		if strings.HasPrefix(path, subtree+".") {
			return true
		}
	}
	return false
}

// Composite conditions carry their operands in Value rather than in Field.
// The DSL compiles `a OR b` to a single condition with Field "_or" and
// Value {"left": [...], "right": [...]}, and `NOT a` to Field "_not" with the
// operand list as its Value. A checker that read only Field would report "_or" as
// an unknown field on every policy that uses OR — false alarms on correct
// policies, while the actual unknown fields nested inside went unreported. Both
// failures come from the same omission.
const (
	compositeFieldOr  = "_or"
	compositeFieldNot = "_not"
)

// UnknownFields returns the condition fields in a rule that the evaluator cannot
// resolve, descending into composite conditions. Order follows the conditions as
// written, and duplicates are collapsed so a field used twice is reported once.
func UnknownFields(conditions []Condition) []string {
	var found []string
	seen := make(map[string]struct{})

	var walk func([]Condition)
	walk = func(conds []Condition) {
		for _, cond := range conds {
			switch cond.Field {
			case compositeFieldOr, compositeFieldNot:
				walk(operandConditions(cond.Value))
				continue
			}
			if IsKnownFieldPath(cond.Field) {
				continue
			}
			if _, dup := seen[cond.Field]; dup {
				continue
			}
			seen[cond.Field] = struct{}{}
			found = append(found, cond.Field)
		}
	}
	walk(conditions)

	return found
}

// operandConditions recovers the nested conditions a composite carries.
//
// They survive as []map[string]any (or []any after a JSON round trip, since a
// policy may arrive over the wire as well as from the DSL compiler), so both
// shapes are handled. Anything else is ignored rather than guessed at: a
// malformed composite is a compiler bug, and inventing conditions from it would
// report fields nobody wrote.
func operandConditions(value any) []Condition {
	var out []Condition

	appendFrom := func(m map[string]any) {
		field, ok := m["field"].(string)
		if !ok {
			return
		}
		operator, _ := m["operator"].(string)
		out = append(out, Condition{Field: field, Operator: operator, Value: m["value"]})
	}

	switch v := value.(type) {
	case []map[string]any:
		for _, m := range v {
			appendFrom(m)
		}
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				appendFrom(m)
			}
		}
	case map[string]any:
		// An _or condition: {"left": [...], "right": [...]}.
		for _, key := range []string{"left", "right"} {
			out = append(out, operandConditions(v[key])...)
		}
	}

	return out
}

// SuggestFieldPath returns the known field most likely intended for an unknown
// one, so a typo or a renamed field produces a correction rather than a list of
// eighty paths to read.
//
// Every case seen in this repository's own example policies was a naming
// mismatch of exactly this kind: is_freeze for freeze.active, is_weekend for
// isWeekend, day_of_week for weekday.
func SuggestFieldPath(field string) (string, bool) {
	normalize := func(s string) string {
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, "_", "")
		s = strings.ReplaceAll(s, ".", "")
		s = strings.TrimPrefix(s, "is")
		return s
	}

	target := normalize(field)
	if target == "" {
		return "", false
	}

	// Prefer a match within the same top-level context, since `time.is_weekend`
	// means a time field and a coincidental match elsewhere would mislead.
	root := field
	if i := strings.Index(field, "."); i > 0 {
		root = field[:i]
	}

	known := KnownFieldPaths()

	var best string
	for _, candidate := range known {
		cn := normalize(candidate)
		if cn != target {
			continue
		}
		if strings.HasPrefix(candidate, root+".") || candidate == root {
			return candidate, true
		}
		if best == "" {
			best = candidate
		}
	}
	if best != "" {
		return best, true
	}

	// Fall back to matching the last segment alone, which catches a field looked
	// for under the wrong parent — `change.files` for `scope.files`. Only used when
	// exactly one known field has that leaf, since two candidates would make the
	// suggestion a guess.
	leaf := field
	if i := strings.LastIndex(field, "."); i >= 0 {
		leaf = field[i+1:]
	}
	leafTarget := normalize(leaf)
	if leafTarget == "" {
		return "", false
	}

	var matches []string
	for _, candidate := range known {
		candidateLeaf := candidate
		if i := strings.LastIndex(candidate, "."); i >= 0 {
			candidateLeaf = candidate[i+1:]
		}
		if normalize(candidateLeaf) == leafTarget {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	return "", false
}
