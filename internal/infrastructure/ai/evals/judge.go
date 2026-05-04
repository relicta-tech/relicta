package evals

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Judge scores generated output against a golden's rubric.
//
// Two implementations expected:
//   - DeterministicJudge: rule-based heuristic scoring, no API calls.
//     Suitable for CI without API keys. Lower fidelity but consistent.
//   - LLMJudge: LLM-as-judge with structured output. Higher fidelity.
//     Real impl is provider-specific; lives in judge_llm.go and is opt-in
//     via a build flag or runner option.
type Judge interface {
	// Name identifies the judge in verdicts and logs.
	Name() string

	// Score evaluates `output` against `golden` and returns a Verdict.
	// The Verdict.GoldenID, Category, JudgedAt, JudgeName are populated
	// by the runner — implementations only need to fill Scores.
	Score(ctx context.Context, golden Golden, output string) ([]Score, error)
}

// DeterministicJudge is a heuristic, deterministic Judge that scores output
// without external API calls. It applies a small set of rules:
//
//   - factuality: penalize any reference text not present in input/reference.
//   - governance_alignment: reward presence of governance keywords matching category.
//   - terseness: score on output length relative to input length.
//   - no_hallucination: penalize fabricated file paths or version strings.
//   - structured_output_valid: 5 if output is a parseable form expected by category, else 3.
//
// This is NOT a substitute for LLM-as-judge — it's a smoke-test floor that
// catches obvious regressions (empty output, length blow-ups, complete topic
// drift) without burning API quota.
type DeterministicJudge struct{}

// Name returns the judge identifier.
func (DeterministicJudge) Name() string { return "deterministic" }

// Score evaluates output via heuristic rules.
func (DeterministicJudge) Score(_ context.Context, g Golden, output string) ([]Score, error) {
	criteria := g.Rubric.Criteria
	if len(criteria) == 0 {
		criteria = DefaultRubric().Criteria
	}

	if strings.TrimSpace(output) == "" {
		// Empty output fails everything.
		out := make([]Score, 0, len(criteria))
		for _, c := range criteria {
			out = append(out, Score{
				Criterion: c.Name,
				Value:     1,
				Rationale: "empty output",
				Weight:    weightOrOne(c.Weight),
			})
		}
		return out, nil
	}

	out := make([]Score, 0, len(criteria))
	for _, c := range criteria {
		score := scoreCriterion(c.Name, g, output)
		out = append(out, Score{
			Criterion: c.Name,
			Value:     score.value,
			Rationale: score.rationale,
			Weight:    weightOrOne(c.Weight),
		})
	}
	return out, nil
}

// scoreCriterion implements per-criterion deterministic scoring.
func scoreCriterion(name string, g Golden, output string) struct {
	value     int
	rationale string
} {
	type result = struct {
		value     int
		rationale string
	}

	switch name {
	case "factuality":
		// 5 if output references input keywords; lower otherwise. Crude.
		if g.Reference != "" && partialMatch(output, g.Reference) {
			return result{5, "output overlaps reference content"}
		}
		if mentionsAnyOf(output, keyTerms(g.UserPrompt)) {
			return result{4, "output references prompt key terms"}
		}
		return result{3, "no overt factual mismatch detected (heuristic)"}

	case "governance_alignment":
		if mentionsAnyOf(output, governanceKeywords(g.Category)) {
			return result{4, "uses governance vocabulary appropriate to category"}
		}
		return result{3, "neutral; no clear governance vocabulary"}

	case "terseness":
		ratio := lengthRatio(g.UserPrompt, output)
		switch {
		case ratio > 4.0:
			return result{2, "output is much longer than input — possibly verbose"}
		case ratio > 2.5:
			return result{3, "output is moderately longer than input"}
		case ratio < 0.2:
			return result{2, "output is suspiciously short — possibly truncated"}
		default:
			return result{4, "output length is reasonable for the input"}
		}

	case "no_hallucination":
		if hasObviousHallucination(output) {
			return result{2, "output contains suspicious fabricated content (heuristic flag)"}
		}
		return result{4, "no obvious hallucination markers"}

	case "structured_output_valid":
		// Heuristic: if reference is JSON, expect output to parse as JSON.
		// Without reference, default to 3 (neutral).
		return result{3, "no schema specified — neutral score"}

	default:
		return result{3, "unknown criterion — neutral score"}
	}
}

// LLMJudge is the LLM-as-judge implementation.
//
// Use NewLLMJudge(svc, name) to construct one with a backing ai.Service.
// A zero-value LLMJudge (no service) returns an error from Score so misuse
// is caught at run time rather than silently returning neutral scores.
type LLMJudge struct {
	JudgeName string
	Service   LLMService
}

// Name returns the configured judge identifier.
func (j *LLMJudge) Name() string {
	if j.JudgeName == "" {
		return "llm"
	}
	return j.JudgeName
}

// Score forwards to the real judge implementation in judge_llm.go.
// Returns an error when no Service is configured — V0-pattern guard.
func (j *LLMJudge) Score(ctx context.Context, g Golden, output string) ([]Score, error) {
	if j.Service == nil {
		return nil, errors.New("evals: LLMJudge.Service is nil; use NewLLMJudge(svc, name)")
	}
	impl := &llmJudgeImpl{svc: j.Service, judgeName: j.Name()}
	return impl.Score(ctx, g, output)
}

// MockJudge is a test-only judge that returns canned scores.
// Useful for testing the runner + verdict aggregation deterministically.
type MockJudge struct {
	NameStr  string
	Default  int
	Override map[string]int // criterion → score override
	Err      error
}

// Name returns the configured mock identifier.
func (m *MockJudge) Name() string {
	if m.NameStr == "" {
		return "mock"
	}
	return m.NameStr
}

// Score returns canned scores or an error.
func (m *MockJudge) Score(_ context.Context, g Golden, _ string) ([]Score, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	criteria := g.Rubric.Criteria
	if len(criteria) == 0 {
		criteria = DefaultRubric().Criteria
	}
	out := make([]Score, 0, len(criteria))
	for _, c := range criteria {
		val := m.Default
		if val == 0 {
			val = 4
		}
		if override, ok := m.Override[c.Name]; ok {
			val = override
		}
		out = append(out, Score{
			Criterion: c.Name,
			Value:     val,
			Rationale: fmt.Sprintf("mock score for %s", c.Name),
			Weight:    weightOrOne(c.Weight),
		})
	}
	return out, nil
}

// finalize fills runner-managed fields on a Verdict.
func finalize(v *Verdict, judgeName string) {
	v.JudgedAt = time.Now().UTC()
	v.JudgeName = judgeName
	v.Mean, v.Min = aggregate(v.Scores)
}

// aggregate computes weighted mean + minimum across a score list.
func aggregate(scores []Score) (mean float64, min int) {
	if len(scores) == 0 {
		return 0, 0
	}
	var sum, totalWeight float64
	min = 5
	for _, s := range scores {
		w := float64(s.Weight)
		if w == 0 {
			w = 1
		}
		sum += float64(s.Value) * w
		totalWeight += w
		if s.Value < min {
			min = s.Value
		}
	}
	if totalWeight == 0 {
		return 0, min
	}
	return sum / totalWeight, min
}

func weightOrOne(w int) int {
	if w <= 0 {
		return 1
	}
	return w
}

// partialMatch reports whether two strings share substantial substring overlap.
// Heuristic: lowercased token overlap above 30%.
func partialMatch(a, b string) bool {
	at := tokenSet(a)
	bt := tokenSet(b)
	if len(at) == 0 || len(bt) == 0 {
		return false
	}
	common := 0
	for k := range at {
		if bt[k] {
			common++
		}
	}
	return float64(common)/float64(len(at)) > 0.3
}

// mentionsAnyOf reports whether s contains any of the substrings (case-insensitive).
func mentionsAnyOf(s string, terms []string) bool {
	low := strings.ToLower(s)
	for _, t := range terms {
		if t == "" {
			continue
		}
		if strings.Contains(low, strings.ToLower(t)) {
			return true
		}
	}
	return false
}

// keyTerms extracts short capitalised tokens from text — crude key-term proxy.
func keyTerms(s string) []string {
	tokens := strings.Fields(s)
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.Trim(t, ".,!?;:'\"()[]")
		if len(t) >= 4 && strings.ToUpper(t[:1]) == t[:1] {
			out = append(out, t)
		}
	}
	return out
}

// governanceKeywords returns the per-category vocabulary the judge looks for.
func governanceKeywords(c Category) []string {
	switch c {
	case CategoryRiskNarrative:
		return []string{"risk", "policy", "approval", "blast", "cosign", "actor", "audit"}
	case CategoryReleaseNotes:
		return []string{"changelog", "feature", "fix", "breaking", "version", "release"}
	case CategoryCommunicate:
		return []string{"stakeholder", "audience", "impact", "rollout", "notice"}
	case CategorySummarizeDiff:
		return []string{"diff", "changed", "added", "removed", "refactor"}
	default:
		return nil
	}
}

// hasObviousHallucination flags blatant fabrication patterns.
// Currently returns true if the output mentions a confidence claim like
// "100%", which release-notes models often hallucinate from training data.
// Heuristic floor — real LLM-as-judge does this much better.
func hasObviousHallucination(output string) bool {
	low := strings.ToLower(output)
	bad := []string{
		"100% test coverage",
		"production-ready",
		"battle-tested",
		"world's first",
		"revolutionary",
	}
	for _, b := range bad {
		if strings.Contains(low, b) {
			return true
		}
	}
	return false
}

// lengthRatio returns len(output) / len(input). Returns 1.0 if input is empty.
func lengthRatio(input, output string) float64 {
	if len(input) == 0 {
		return 1.0
	}
	return float64(len(output)) / float64(len(input))
}

// tokenSet returns a lowercased set of word tokens in s.
func tokenSet(s string) map[string]bool {
	out := make(map[string]bool)
	for _, t := range strings.Fields(strings.ToLower(s)) {
		t = strings.Trim(t, ".,!?;:'\"()[]")
		if len(t) >= 3 {
			out[t] = true
		}
	}
	return out
}
