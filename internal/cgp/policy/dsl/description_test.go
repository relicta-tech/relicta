package dsl

import (
	"strings"
	"testing"
)

// A policy's purpose was unrepresentable. Rules could describe themselves, the
// file could not, and the compiler filled the gap with the constant
// "Policy compiled from DSL" — which `relicta policy list` then printed under
// "Description:" for every policy in the repository. The field that should tell
// an auditor what a policy is for instead told them how it was built.

func TestFileLevelDescriptionIsParsed(t *testing.T) {
	const src = `
description = "Governance for regulated environments"

rule "r" {
  when { risk.score > 0.5 }
  then { require_approval(count: 1) }
}
`
	pol, err := NewLoader(LoaderOptions{}).LoadString(src, "test.policy")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pol.Description != "Governance for regulated environments" {
		t.Errorf("description = %q, want the declared string", pol.Description)
	}
}

// Absent must read as absent. The placeholder was worse than an empty string
// because callers cannot tell it apart from a real one.
func TestMissingDescriptionIsEmptyNotAPlaceholder(t *testing.T) {
	const src = `
rule "r" {
  when { risk.score > 0.5 }
  then { require_approval(count: 1) }
}
`
	pol, err := NewLoader(LoaderOptions{}).LoadString(src, "test.policy")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pol.Description != "" {
		t.Errorf("description = %q, want empty for a file that declares none", pol.Description)
	}
}

// The description is optional and additive: every policy written before it
// existed must still parse. This is the compatibility guarantee the change rests
// on, so it is asserted rather than assumed.
func TestPolicyWithoutDescriptionStillParses(t *testing.T) {
	const src = `
rule "r" {
  priority = 10
  description = "a rule may still describe itself"
  when { change.breaking == true }
  then { require_approval(count: 2) }
}

defaults {
  decision = "approve"
  required_approvers = 0
}
`
	pol, err := NewLoader(LoaderOptions{}).LoadString(src, "legacy.policy")
	if err != nil {
		t.Fatalf("a policy without a file-level description must still parse: %v", err)
	}
	if len(pol.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(pol.Rules))
	}
	if pol.Rules[0].Description != "a rule may still describe itself" {
		t.Errorf("rule description was lost: %q", pol.Rules[0].Description)
	}
}

func TestDuplicateDescriptionIsRejected(t *testing.T) {
	const src = `
description = "first"
description = "second"

rule "r" {
  when { risk.score > 0.5 }
  then { require_approval(count: 1) }
}
`
	_, err := NewLoader(LoaderOptions{}).LoadString(src, "test.policy")
	if err == nil {
		t.Fatal("two descriptions must be an error; silently keeping one hides an editing mistake")
	}
	if !strings.Contains(err.Error(), "duplicate description") {
		t.Errorf("error should name the problem; got %v", err)
	}
}

// The parse error names what is accepted at the top level. It previously said
// "expected 'rule' or 'defaults'", which after this change would be a wrong list.
func TestTopLevelErrorNamesDescription(t *testing.T) {
	_, err := NewLoader(LoaderOptions{}).LoadString(`nonsense = 1`, "test.policy")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "description") {
		t.Errorf("the error should list 'description' among what is valid here; got %v", err)
	}
}
