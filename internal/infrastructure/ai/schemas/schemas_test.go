package schemas

import (
	"encoding/json"
	"regexp"
	"testing"
)

// nameRE mirrors OpenAI's strict-mode constraint on JSON schema names.
var nameRE = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func TestSchemaNamesValid(t *testing.T) {
	all := []Schema{
		GovernanceDecisionSchema(),
		ReleaseNotesSchema(),
		DiffSummarySchema(),
		AudienceNarrativeSchema(),
		EvalJudgeSchema(),
	}
	for _, s := range all {
		if !nameRE.MatchString(s.Name()) {
			t.Errorf("schema name %q does not match OpenAI strict-mode constraint", s.Name())
		}
		if s.Description() == "" {
			t.Errorf("schema %q has empty description", s.Name())
		}
	}
}

func TestSchemasMarshalToValidJSON(t *testing.T) {
	for _, s := range []Schema{
		GovernanceDecisionSchema(),
		ReleaseNotesSchema(),
		DiffSummarySchema(),
		AudienceNarrativeSchema(),
		EvalJudgeSchema(),
	} {
		b, err := s.MarshalJSON()
		if err != nil {
			t.Errorf("marshal %s: %v", s.Name(), err)
			continue
		}
		var raw any
		if err := json.Unmarshal(b, &raw); err != nil {
			t.Errorf("unmarshal %s: %v", s.Name(), err)
		}
	}
}

func TestGovernanceDecisionSchemaShape(t *testing.T) {
	s := GovernanceDecisionSchema()
	if !s.Strict() {
		t.Error("governance decision schema should be strict")
	}

	doc := mustDecodeMap(t, s)
	props, ok := doc["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties map")
	}

	for _, want := range []string{"decision", "risk_score", "rationale", "required_actions"} {
		if _, ok := props[want]; !ok {
			t.Errorf("missing required property %q", want)
		}
	}

	risk := props["risk_score"].(map[string]any)
	if risk["minimum"].(float64) != 0.0 {
		t.Errorf("risk_score minimum should be 0.0")
	}
	if risk["maximum"].(float64) != 1.0 {
		t.Errorf("risk_score maximum should be 1.0")
	}
}

func TestReleaseNotesSchema_NotStrict(t *testing.T) {
	s := ReleaseNotesSchema()
	if s.Strict() {
		t.Error("release notes are cosmetic prose; strict=false")
	}
}

func TestDiffSummarySchema_BooleanSignals(t *testing.T) {
	s := DiffSummarySchema()
	doc := mustDecodeMap(t, s)
	signals := doc["properties"].(map[string]any)["signals"].(map[string]any)
	required := signals["required"].([]any)

	wantSignals := map[string]bool{
		"touches_auth":    false,
		"touches_secrets": false,
		"removes_guard":   false,
		"schema_change":   false,
	}
	for _, r := range required {
		wantSignals[r.(string)] = true
	}
	for sig, present := range wantSignals {
		if !present {
			t.Errorf("missing required signal %q", sig)
		}
	}
}

func TestAudienceNarrativeSchema_AudienceEnum(t *testing.T) {
	s := AudienceNarrativeSchema()
	doc := mustDecodeMap(t, s)
	audience := doc["properties"].(map[string]any)["audience"].(map[string]any)
	enum := audience["enum"].([]any)
	if len(enum) != 4 {
		t.Errorf("expected 4 audiences, got %d", len(enum))
	}
}

func TestDiffCategories(t *testing.T) {
	cats := diffCategories()
	for _, must := range []string{"feature", "fix", "security", "breaking"} {
		found := false
		for _, c := range cats {
			if c == must {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing category %q", must)
		}
	}
}

func TestMustSchemaJSON_RoundTrip(t *testing.T) {
	s := GovernanceDecisionSchema()
	out := MustSchemaJSON(s)
	if out == "" {
		t.Error("expected non-empty pretty output")
	}
	var raw any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Errorf("MustSchemaJSON output not valid JSON: %v", err)
	}
}

func mustDecodeMap(t *testing.T, s Schema) map[string]any {
	t.Helper()
	b, err := s.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
