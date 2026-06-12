package manager

import (
	"strings"
	"testing"
)

func TestParseRequirement(t *testing.T) {
	cases := []struct {
		spec       string
		wantName   string
		constraint bool
		wantErr    bool
	}{
		{"github", "github", false, false},
		{"github@^2.0", "github", true, false},
		{" slack@>=1.2 ", "slack", true, false},
		{"@^1.0", "", false, true},
		{"", "", false, true},
		{"jira@not-a-constraint!", "", false, true},
	}
	for _, c := range cases {
		r, err := ParseRequirement(c.spec)
		if c.wantErr != (err != nil) {
			t.Errorf("ParseRequirement(%q) err = %v", c.spec, err)
			continue
		}
		if err != nil {
			continue
		}
		if r.Name != c.wantName || (r.Constraint != nil) != c.constraint {
			t.Errorf("ParseRequirement(%q) = %+v", c.spec, r)
		}
	}
}

func TestParseRequirementsCollectsAllErrors(t *testing.T) {
	_, err := ParseRequirements([]string{"good", "@bad", "also@bad!"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "@bad") || !strings.Contains(err.Error(), "also@bad!") {
		t.Errorf("error must list every bad spec: %v", err)
	}
}

func TestRequirementSatisfies(t *testing.T) {
	anyVersion, _ := ParseRequirement("github")
	if !anyVersion.Satisfies("v2.0.3") {
		t.Error("unconstrained requirement satisfied by any installed version")
	}
	if anyVersion.Satisfies("") {
		t.Error("empty version never satisfies")
	}

	caret, _ := ParseRequirement("github@^2.0")
	if !caret.Satisfies("v2.5.1") {
		t.Error("2.5.1 satisfies ^2.0")
	}
	if caret.Satisfies("v3.0.0") {
		t.Error("3.0.0 does not satisfy ^2.0")
	}
	if caret.Satisfies("garbage") {
		t.Error("non-semver versions never satisfy constraints")
	}
}

func TestResolveRequired(t *testing.T) {
	reqs, err := ParseRequirements([]string{"github@^2.0", "slack", "jira@>=3.0"})
	if err != nil {
		t.Fatal(err)
	}
	installed := []InstalledPlugin{
		{Name: "github", Version: "v2.0.3"},
		{Name: "jira", Version: "v2.9.0"},
	}

	actions := ResolveRequired(reqs, installed)
	got := map[string]bool{}
	for _, a := range actions {
		got[a.Requirement.Name] = a.Install
	}

	if got["github"] {
		t.Error("github v2.0.3 satisfies ^2.0 — no install")
	}
	if !got["slack"] {
		t.Error("slack not installed — install required")
	}
	if !got["jira"] {
		t.Error("jira v2.9.0 violates >=3.0 — install required")
	}
}
