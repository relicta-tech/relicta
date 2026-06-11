package evals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/infrastructure/ai/schemas"
)

// LLMService is the surface the LLM judge needs from an AI provider.
//
// Defined locally to avoid pulling the full ai.Service interface (which has
// release-notes / changelog / marketing methods irrelevant to judging).
// Adapters convert ai.Service.Complete into this shape.
type LLMService interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// LLMStructuredService is an opt-in extension implemented by services that
// support provider-native structured output. When the configured judge
// service satisfies this interface, the judge calls CompleteStructured with
// the EvalJudge schema and gets provider-validated JSON back — no parsing
// fallback path needed.
//
// `schemaProvider` mirrors `internal/infrastructure/ai/schemas.Schema` to
// avoid pulling that package as a hard dependency.
type LLMStructuredService interface {
	CompleteStructured(ctx context.Context, systemPrompt, userPrompt string, schema schemaProvider) ([]byte, error)
}

// schemaProvider matches `schemas.Schema` shape without importing schemas
// here; concrete schema types in `schemas/` satisfy it structurally.
type schemaProvider interface {
	json.Marshaler
	Name() string
	Description() string
	Strict() bool
}

// NewLLMJudge constructs an LLM-as-judge backed by the supplied service.
// Defaults: judgeName "llm" if empty.
//
// The judge issues one Complete call per Score() invocation. Provide a
// service whose temperature is low (≤ 0.2) for stable scoring; higher
// temperatures cause inter-run variance that confounds CI gates.
func NewLLMJudge(svc LLMService, judgeName string) *LLMJudge {
	if judgeName == "" {
		judgeName = "llm"
	}
	return &LLMJudge{Service: svc, JudgeName: judgeName}
}

// llmJudgeImpl is the concrete LLM judge. Returned via NewLLMJudge so
// callers don't accidentally instantiate it without a service.
type llmJudgeImpl struct {
	svc       LLMService
	judgeName string
}

// Name returns the judge identifier surfaced in verdicts.
func (j *llmJudgeImpl) Name() string { return j.judgeName }

// Score sends the (golden, output) pair to the LLM with a structured rubric
// prompt and parses the JSON response into per-criterion Scores.
//
// On parse failure, falls back to neutral 3-across-the-board with a rationale
// noting the parse failure — never crashes the eval run, but flags the issue
// so prompt engineers can fix the rubric template.
func (j *llmJudgeImpl) Score(ctx context.Context, g Golden, output string) ([]Score, error) {
	if j.svc == nil {
		return nil, errors.New("evals: LLMJudge has no service configured")
	}

	criteria := g.Rubric.Criteria
	if len(criteria) == 0 {
		criteria = DefaultRubric().Criteria
	}

	systemPrompt := buildLLMJudgeSystemPrompt(criteria)
	userPrompt := buildLLMJudgeUserPrompt(g, output)

	// Prefer provider-native structured output when supported. Eliminates
	// markdown-fence handling, prose-around-JSON edge cases, and the
	// neutral-fallback path most of the time.
	var raw string
	if structured, ok := j.svc.(LLMStructuredService); ok {
		bytes, err := structured.CompleteStructured(ctx, systemPrompt, userPrompt, schemas.EvalJudgeSchema())
		if err != nil {
			return nil, fmt.Errorf("llm judge structured complete: %w", err)
		}
		raw = string(bytes)
	} else {
		txt, err := j.svc.Complete(ctx, systemPrompt, userPrompt)
		if err != nil {
			return nil, fmt.Errorf("llm judge complete: %w", err)
		}
		raw = txt
	}

	scores, parseErr := parseLLMJudgeResponse(raw, criteria)
	if parseErr != nil {
		// Emit neutral fallback so the run continues; surface the parse
		// problem in rationale so it's visible in artifacts.
		fallback := make([]Score, 0, len(criteria))
		for _, c := range criteria {
			fallback = append(fallback, Score{
				Criterion: c.Name,
				Value:     3,
				Rationale: fmt.Sprintf("llm judge parse failed: %v", parseErr),
				Weight:    weightOrOne(c.Weight),
			})
		}
		return fallback, nil
	}

	return scores, nil
}

// buildLLMJudgeSystemPrompt constructs the rubric-driving system prompt.
//
// Cached per call but stable across invocations — prime target for Anthropic
// prompt caching (already enabled in our anthropic adapter). Cost discipline
// matters because the judge runs at least once per golden per CI run.
func buildLLMJudgeSystemPrompt(criteria []Criterion) string {
	var b strings.Builder
	b.WriteString("You are a strict evaluator scoring AI output for a release-governance product. ")
	b.WriteString("Score each criterion below on a 1-5 integer scale where 1=poor and 5=excellent. ")
	b.WriteString("Return ONLY a JSON object of the shape: ")
	b.WriteString(`{"scores": [{"criterion": "<name>", "value": <1-5>, "rationale": "<one sentence>"}]}`)
	b.WriteString(" — no markdown, no preamble.\n\nCriteria:\n")
	for _, c := range criteria {
		fmt.Fprintf(&b, "  - %s: %s\n", c.Name, c.Description)
	}
	b.WriteString("\nBe terse, specific, and unforgiving — false high scores let regressions ship.")
	return b.String()
}

// buildLLMJudgeUserPrompt frames the golden and output for the judge.
func buildLLMJudgeUserPrompt(g Golden, output string) string {
	var b strings.Builder
	b.WriteString("Evaluate the following output.\n\n")
	if g.Description != "" {
		fmt.Fprintf(&b, "Test description: %s\n\n", g.Description)
	}
	fmt.Fprintf(&b, "Category: %s\n\n", g.Category)
	if g.UserPrompt != "" {
		fmt.Fprintf(&b, "Original prompt:\n```\n%s\n```\n\n", g.UserPrompt)
	}
	if g.Reference != "" {
		fmt.Fprintf(&b, "Reference (ideal answer for comparison):\n```\n%s\n```\n\n", g.Reference)
	}
	fmt.Fprintf(&b, "Generated output:\n```\n%s\n```\n", output)
	return b.String()
}

// parseLLMJudgeResponse decodes the judge's JSON response into Scores.
//
// Tolerant of common LLM output deviations: leading/trailing whitespace,
// ```json fences, extra prose before or after the JSON object. Strict on
// the score range (1-5) and requires every rubric criterion be covered.
func parseLLMJudgeResponse(raw string, criteria []Criterion) ([]Score, error) {
	body := extractJSONObject(raw)
	if body == "" {
		return nil, errors.New("no JSON object found in response")
	}

	var resp struct {
		Scores []struct {
			Criterion string `json:"criterion"`
			Value     int    `json:"value"`
			Rationale string `json:"rationale"`
		} `json:"scores"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}

	byName := make(map[string]int, len(criteria))
	for i, c := range criteria {
		byName[c.Name] = i
	}

	scores := make([]Score, 0, len(criteria))
	for _, c := range criteria {
		var found bool
		for _, s := range resp.Scores {
			if s.Criterion != c.Name {
				continue
			}
			value := s.Value
			if value < 1 {
				value = 1
			} else if value > 5 {
				value = 5
			}
			scores = append(scores, Score{
				Criterion: c.Name,
				Value:     value,
				Rationale: s.Rationale,
				Weight:    weightOrOne(c.Weight),
			})
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("response missing score for criterion %q", c.Name)
		}
	}
	return scores, nil
}

// extractJSONObject pulls the first balanced { ... } JSON object out of raw,
// ignoring any markdown fences or prose around it.
func extractJSONObject(raw string) string {
	raw = strings.TrimSpace(raw)
	// Strip ```json or ``` fences if present.
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		if idx := strings.LastIndex(raw, "```"); idx >= 0 {
			raw = raw[:idx]
		}
		raw = strings.TrimSpace(raw)
	}

	// Find first { and matching }.
	start := strings.IndexByte(raw, '{')
	if start < 0 {
		return ""
	}
	depth := 0
	for i := start; i < len(raw); i++ {
		switch raw[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[start : i+1]
			}
		}
	}
	return ""
}
