package evals

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// stubLLMService returns a canned response and tracks the prompts it received.
type stubLLMService struct {
	response   string
	err        error
	lastSystem string
	lastUser   string
}

func (s *stubLLMService) Complete(_ context.Context, systemPrompt, userPrompt string) (string, error) {
	s.lastSystem = systemPrompt
	s.lastUser = userPrompt
	return s.response, s.err
}

func TestNewLLMJudge_DefaultsName(t *testing.T) {
	j := NewLLMJudge(&stubLLMService{}, "")
	if j.Name() != "llm" {
		t.Errorf("expected default name 'llm'; got %q", j.Name())
	}
}

func TestNewLLMJudge_PreservesName(t *testing.T) {
	j := NewLLMJudge(&stubLLMService{}, "claude-judge")
	if j.Name() != "claude-judge" {
		t.Errorf("name: got %q", j.Name())
	}
}

func TestLLMJudge_Score_ParsesValidJSON(t *testing.T) {
	rubric := DefaultRubric()
	stub := &stubLLMService{
		response: `{
			"scores": [
				{"criterion": "factuality", "value": 5, "rationale": "all claims grounded"},
				{"criterion": "governance_alignment", "value": 4, "rationale": "uses correct vocabulary"},
				{"criterion": "terseness", "value": 3, "rationale": "slightly verbose"},
				{"criterion": "no_hallucination", "value": 5, "rationale": "no fabrication"},
				{"criterion": "structured_output_valid", "value": 4, "rationale": "valid"}
			]
		}`,
	}

	j := NewLLMJudge(stub, "test")
	scores, err := j.Score(context.Background(), Golden{Category: CategoryReleaseNotes, Rubric: rubric}, "release notes here")
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if len(scores) != len(rubric.Criteria) {
		t.Fatalf("expected %d scores, got %d", len(rubric.Criteria), len(scores))
	}

	if !strings.Contains(stub.lastSystem, "1-5 integer scale") {
		t.Errorf("system prompt missing scoring guidance: %q", stub.lastSystem)
	}
	if !strings.Contains(stub.lastUser, "release notes here") {
		t.Errorf("user prompt missing output: %q", stub.lastUser)
	}

	for _, s := range scores {
		if s.Value < 1 || s.Value > 5 {
			t.Errorf("score out of range: %+v", s)
		}
	}
}

func TestLLMJudge_Score_HandlesMarkdownFences(t *testing.T) {
	rubric := DefaultRubric()
	stub := &stubLLMService{
		response: "Here is the evaluation:\n\n```json\n" +
			`{"scores": [` +
			`{"criterion": "factuality", "value": 4, "rationale": "ok"},` +
			`{"criterion": "governance_alignment", "value": 4, "rationale": "ok"},` +
			`{"criterion": "terseness", "value": 4, "rationale": "ok"},` +
			`{"criterion": "no_hallucination", "value": 5, "rationale": "ok"},` +
			`{"criterion": "structured_output_valid", "value": 4, "rationale": "ok"}` +
			`]}` + "\n```",
	}
	j := NewLLMJudge(stub, "")
	scores, err := j.Score(context.Background(), Golden{Category: CategoryReleaseNotes, Rubric: rubric}, "x")
	if err != nil {
		t.Fatalf("expected fence-stripping to succeed; got %v", err)
	}
	if len(scores) != 5 {
		t.Errorf("expected 5 scores, got %d", len(scores))
	}
}

func TestLLMJudge_Score_ClampsOutOfRangeValues(t *testing.T) {
	rubric := DefaultRubric()
	stub := &stubLLMService{
		response: `{"scores": [
			{"criterion": "factuality", "value": 7, "rationale": "model went over"},
			{"criterion": "governance_alignment", "value": 0, "rationale": "model went under"},
			{"criterion": "terseness", "value": 3, "rationale": "ok"},
			{"criterion": "no_hallucination", "value": 4, "rationale": "ok"},
			{"criterion": "structured_output_valid", "value": 5, "rationale": "ok"}
		]}`,
	}
	j := NewLLMJudge(stub, "")
	scores, err := j.Score(context.Background(), Golden{Category: CategoryReleaseNotes, Rubric: rubric}, "x")
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]int{}
	for _, s := range scores {
		byName[s.Criterion] = s.Value
	}
	if byName["factuality"] != 5 {
		t.Errorf("expected clamp to 5; got %d", byName["factuality"])
	}
	if byName["governance_alignment"] != 1 {
		t.Errorf("expected clamp to 1; got %d", byName["governance_alignment"])
	}
}

func TestLLMJudge_Score_FallbackOnGarbageResponse(t *testing.T) {
	rubric := DefaultRubric()
	stub := &stubLLMService{response: "the model rambled and never produced JSON"}
	j := NewLLMJudge(stub, "")

	scores, err := j.Score(context.Background(), Golden{Category: CategoryReleaseNotes, Rubric: rubric}, "x")
	if err != nil {
		t.Fatalf("Score should fall back to neutral, not error: %v", err)
	}
	for _, s := range scores {
		if s.Value != 3 {
			t.Errorf("expected neutral 3 fallback; got %d for %s", s.Value, s.Criterion)
		}
		if !strings.Contains(s.Rationale, "parse failed") {
			t.Errorf("rationale should disclose parse failure; got %q", s.Rationale)
		}
	}
}

func TestLLMJudge_Score_MissingCriterionInResponse(t *testing.T) {
	rubric := DefaultRubric()
	// Response only covers 2 of 5 criteria.
	stub := &stubLLMService{
		response: `{"scores": [
			{"criterion": "factuality", "value": 5, "rationale": "ok"},
			{"criterion": "terseness", "value": 4, "rationale": "ok"}
		]}`,
	}
	j := NewLLMJudge(stub, "")
	scores, err := j.Score(context.Background(), Golden{Category: CategoryReleaseNotes, Rubric: rubric}, "x")
	if err != nil {
		t.Fatalf("partial coverage should fall back to neutral, not error: %v", err)
	}
	// Fallback path → all 3s.
	for _, s := range scores {
		if s.Value != 3 {
			t.Errorf("expected neutral 3 fallback for missing-criterion case; got %d for %s", s.Value, s.Criterion)
		}
	}
}

func TestLLMJudge_Score_ServiceErrorPropagates(t *testing.T) {
	stub := &stubLLMService{err: errors.New("rate limit")}
	j := NewLLMJudge(stub, "")
	_, err := j.Score(context.Background(), Golden{Category: CategoryReleaseNotes, Rubric: DefaultRubric()}, "x")
	if err == nil {
		t.Fatal("expected service error to propagate")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("expected wrapped service error; got %v", err)
	}
}

// stubStructuredLLMService implements both LLMService and LLMStructuredService.
type stubStructuredLLMService struct {
	stubLLMService
	structuredResponse []byte
	structuredErr      error
	structuredSchema   string
}

func (s *stubStructuredLLMService) CompleteStructured(_ context.Context, sys, user string, schema schemaProvider) ([]byte, error) {
	s.lastSystem = sys
	s.lastUser = user
	s.structuredSchema = schema.Name()
	return s.structuredResponse, s.structuredErr
}

func TestLLMJudge_PrefersStructuredOutputWhenSupported(t *testing.T) {
	rubric := DefaultRubric()
	stub := &stubStructuredLLMService{
		structuredResponse: []byte(`{"scores":[
			{"criterion":"factuality","value":5,"rationale":"ok"},
			{"criterion":"governance_alignment","value":4,"rationale":"ok"},
			{"criterion":"terseness","value":4,"rationale":"ok"},
			{"criterion":"no_hallucination","value":5,"rationale":"ok"},
			{"criterion":"structured_output_valid","value":5,"rationale":"ok"}
		]}`),
	}

	j := NewLLMJudge(stub, "structured-test")
	scores, err := j.Score(context.Background(), Golden{Category: CategoryReleaseNotes, Rubric: rubric}, "release notes")
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if len(scores) != 5 {
		t.Errorf("expected 5 scores, got %d", len(scores))
	}
	if stub.structuredSchema != "EvalJudge" {
		t.Errorf("expected EvalJudge schema, got %q", stub.structuredSchema)
	}
	// Free-form Complete must NOT have been called.
	if stub.response != "" {
		t.Errorf("free-form Complete should not have been invoked")
	}
}

func TestLLMJudge_StructuredErrorPropagates(t *testing.T) {
	stub := &stubStructuredLLMService{
		structuredErr: errors.New("rate limit"),
	}
	j := NewLLMJudge(stub, "")
	_, err := j.Score(context.Background(), Golden{Category: CategoryReleaseNotes, Rubric: DefaultRubric()}, "x")
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("expected wrapped service error; got %v", err)
	}
}

func TestExtractJSONObject(t *testing.T) {
	cases := map[string]string{
		`{"a":1}`:                   `{"a":1}`,
		"```json\n{\"a\":1}\n```":   `{"a":1}`,
		"prose first {\"a\":1} end": `{"a":1}`,
		"no braces":                 ``,
		"{nested {object} here}":    `{nested {object} here}`,
	}
	for in, want := range cases {
		if got := extractJSONObject(in); got != want {
			t.Errorf("extractJSONObject(%q) = %q, want %q", in, got, want)
		}
	}
}
