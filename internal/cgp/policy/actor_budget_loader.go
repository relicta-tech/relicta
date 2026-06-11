package policy

import (
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/relicta-tech/relicta/v4/pkg/cgp"
)

// LoadActorBudgets parses an ActorBudgetSet from a YAML file.
// Returns an empty set with no error if the file does not exist —
// callers decide policy (e.g. fall back to DefaultRestrictiveAgentBudget).
func LoadActorBudgets(path string) (*ActorBudgetSet, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &ActorBudgetSet{}, nil
		}
		return nil, fmt.Errorf("open actor budgets: %w", err)
	}
	defer f.Close()

	return ParseActorBudgets(f)
}

// ParseActorBudgets parses an ActorBudgetSet from a YAML reader.
// The expected document shape:
//
//	budgets:
//	  - actor_kind: agent
//	    actor_id: "claude-code-*"
//	    max_blast_radius: medium
//	    max_risk_score: 0.4
//	    requires_cosign: [publish, approve]
func ParseActorBudgets(r io.Reader) (*ActorBudgetSet, error) {
	var set ActorBudgetSet
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&set); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decode actor budgets: %w", err)
	}
	if err := set.Validate(); err != nil {
		return nil, fmt.Errorf("validate actor budgets: %w", err)
	}
	return &set, nil
}

// ResolveBudget returns the matching budget for an actor, or — when no
// explicit budget matches — a sensible default based on actor kind.
// Use this from privileged-operation entry points (MCP tool dispatch,
// CLI publish/approve/rollback) to enforce the autonomy slider.
//
// Defaults:
//   - human → DefaultPermissiveHumanBudget (no caps)
//   - agent → DefaultRestrictiveAgentBudget (refuses high-risk releases)
//   - ci, system, other → DefaultRestrictiveAgentBudget (treat like agents)
func ResolveBudget(set *ActorBudgetSet, actorKind, actorID string) *ActorBudget {
	if set != nil {
		match := set.Match(cgp.Actor{Kind: actorKind, ID: actorID})
		if match != nil {
			return match
		}
	}
	if actorKind == "human" {
		return DefaultPermissiveHumanBudget()
	}
	return DefaultRestrictiveAgentBudget()
}
