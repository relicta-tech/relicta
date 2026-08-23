package cli

import (
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp/policy/library"
)

// internal/cgp/policy/library ships SOC 2, separation of duties, multi-team approval, audit
// trail, hotfix fast track and the rest — complete, tested, and reachable from nothing: no
// command listed them, no configuration named them, and the package had no importer.

func TestTheBuiltInTemplatesAreReachable(t *testing.T) {
	templates := library.DefaultRegistry.List()

	if len(templates) == 0 {
		t.Fatal("the registry the command reads is empty. NewRegistry() starts empty and " +
			"RegisterBuiltins fills it; reading the wrong one lists nothing and looks fine")
	}

	var hasSOC2 bool
	for _, tmpl := range templates {
		if strings.Contains(strings.ToLower(tmpl.ID), "soc2") {
			hasSOC2 = true
		}
		if tmpl.ID == "" || tmpl.Name == "" {
			t.Errorf("template %+v has no id or name to show", tmpl)
		}
	}
	if !hasSOC2 {
		t.Error("the SOC 2 template is missing, which is the one a compliance reader looks for")
	}
}

// Every template must build, or listing it offers something that fails when asked for.
func TestEveryListedTemplateBuilds(t *testing.T) {
	for _, tmpl := range library.DefaultRegistry.List() {
		built, err := library.DefaultRegistry.Build(tmpl.ID, library.DefaultTemplateOptions())
		if err != nil {
			t.Errorf("template %s is listed and does not build: %v", tmpl.ID, err)
			continue
		}
		if built == nil {
			t.Errorf("template %s built nothing", tmpl.ID)
		}
	}
}

// An unknown id says what is available, because the ids are not guessable.
func TestAnUnknownTemplateNamesTheKnownOnes(t *testing.T) {
	err := showPolicyTemplate(library.DefaultRegistry, "no-such-template")
	if err == nil {
		t.Fatal("an unknown template id succeeded")
	}
	if !strings.Contains(err.Error(), "enterprise-soc2") {
		t.Errorf("the error does not list what is available: %v", err)
	}
}

// The machine-readable listing describes the templates, not the policies they build: a caller
// choosing one needs the id, the category and the description, and the policy itself is what
// --show is for.
func TestTheJSONListingDescribesEachTemplate(t *testing.T) {
	rows := templatesAsJSON(library.DefaultRegistry.List())

	if len(rows) != len(library.DefaultRegistry.List()) {
		t.Fatalf("listed %d of %d templates", len(rows), len(library.DefaultRegistry.List()))
	}

	for _, row := range rows {
		for _, key := range []string{"id", "name", "description", "category"} {
			if _, ok := row[key]; !ok {
				t.Errorf("a template row has no %q: %+v", key, row)
			}
		}
		if row["id"] == "" {
			t.Errorf("a template row has an empty id: %+v", row)
		}
	}
}

// Categories group the catalogue, and an uncategorized template would print under a blank
// heading.
func TestEveryTemplateHasACategory(t *testing.T) {
	for _, tmpl := range library.DefaultRegistry.List() {
		if strings.TrimSpace(tmpl.Category) == "" {
			t.Errorf("template %s has no category, so it prints under a blank heading", tmpl.ID)
		}
	}
}

// The listing runs end to end, in both output modes, so a formatting mistake is a test failure
// rather than something a reader discovers.
func TestTheTemplateListingRuns(t *testing.T) {
	origJSON, origShow := outputJSON, policyTemplateShow
	t.Cleanup(func() { outputJSON, policyTemplateShow = origJSON, origShow })

	policyTemplateShow = ""
	for _, jsonMode := range []bool{false, true} {
		outputJSON = jsonMode
		if err := runPolicyTemplates(nil, nil); err != nil {
			t.Errorf("listing templates with json=%v: %v", jsonMode, err)
		}
	}

	outputJSON = false
	policyTemplateShow = "enterprise-soc2"
	if err := runPolicyTemplates(nil, nil); err != nil {
		t.Errorf("showing a template: %v", err)
	}
}
