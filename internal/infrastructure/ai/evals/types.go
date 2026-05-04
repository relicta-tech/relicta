// Package evals defines the AI evaluation harness for Relicta.
//
// Eval harness is non-negotiable for a governance product that ships LLM
// output. Without it, prompt drift on `gpt-5` / `claude-sonnet-4-6` /
// `gemini-2.5-flash` silently degrades release-notes / risk-narrative /
// communicate-cmd quality between model bumps.
//
// V0 scope:
//   - Golden YAML format (input + context + rubric)
//   - Runner that drives an ai.Service against goldens
//   - LLM-as-judge interface (mock impl for CI without API keys; real impl
//     gated behind build tag)
//   - Aggregate verdict with per-criterion mean + min, gate predicate
//
// Future:
//   - 150+ golden corpus across categories (release-notes, risk-narrative,
//     communicate-cmd, summarize_diff)
//   - GitHub Actions workflow gating model-bump PRs
//   - Cross-judge bias detection (10% sample with second judge)
package evals

import "time"

// Category buckets goldens by use case so callers can run subsets and so
// rubric weights can vary by category.
type Category string

const (
	CategoryReleaseNotes  Category = "release_notes"
	CategoryRiskNarrative Category = "risk_narrative"
	CategoryCommunicate   Category = "communicate"
	CategorySummarizeDiff Category = "summarize_diff"
)

// Valid reports whether c is a recognized category.
func (c Category) Valid() bool {
	switch c {
	case CategoryReleaseNotes, CategoryRiskNarrative, CategoryCommunicate, CategorySummarizeDiff:
		return true
	}
	return false
}

// Golden is a single evaluation case. Loaded from YAML in `goldens/*.yaml`.
type Golden struct {
	// ID is a stable identifier. Used in failure messages and CI artifacts.
	ID string `yaml:"id"`

	// Category buckets the case for selective runs and per-category gating.
	Category Category `yaml:"category"`

	// Description briefly explains what the case checks.
	Description string `yaml:"description,omitempty"`

	// SystemPrompt overrides the service default; empty uses service default.
	SystemPrompt string `yaml:"system_prompt,omitempty"`

	// UserPrompt is the user-side input that drives the model output.
	UserPrompt string `yaml:"user_prompt"`

	// Reference is an optional ideal/reference output the judge may compare against.
	// Leave empty for free-form judging against rubric criteria only.
	Reference string `yaml:"reference,omitempty"`

	// Rubric defines criteria the judge scores. Empty rubric uses DefaultRubric().
	Rubric Rubric `yaml:"rubric,omitempty"`

	// MinPass is the per-criterion minimum (1-5). 0 means "use Rubric.MinPass".
	MinPass int `yaml:"min_pass,omitempty"`

	// Tags are free-form labels for filtering.
	Tags []string `yaml:"tags,omitempty"`
}

// Rubric is the scoring rubric the judge applies to one Golden's output.
type Rubric struct {
	// Criteria lists the dimensions to score (1-5 per criterion).
	Criteria []Criterion `yaml:"criteria"`

	// MinPass is the criterion-level floor. Verdicts with any criterion
	// below this fail the gate. Default 3.
	MinPass int `yaml:"min_pass,omitempty"`

	// MeanPass is the rubric-mean floor. Verdicts with mean below this
	// fail the gate. Default 4.0.
	MeanPass float64 `yaml:"mean_pass,omitempty"`
}

// Criterion is a single dimension the judge scores.
type Criterion struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Weight      int    `yaml:"weight,omitempty"` // default 1
}

// DefaultRubric returns the standard rubric for governance-prose evaluation.
// Five criteria from the Relicta v2 product synthesis: factuality,
// governance-alignment, terseness, no-hallucination, structured-output-valid.
func DefaultRubric() Rubric {
	return Rubric{
		MinPass:  3,
		MeanPass: 4.0,
		Criteria: []Criterion{
			{Name: "factuality", Description: "Output reflects only claims supported by the input and context.", Weight: 2},
			{Name: "governance_alignment", Description: "Output follows organizational policy tone and decision criteria.", Weight: 2},
			{Name: "terseness", Description: "Output is concise without sacrificing meaning.", Weight: 1},
			{Name: "no_hallucination", Description: "No invented file paths, version strings, or commit references.", Weight: 2},
			{Name: "structured_output_valid", Description: "When a schema is required, output validates against it.", Weight: 1},
		},
	}
}

// Score is one criterion's judgment.
type Score struct {
	Criterion string  `json:"criterion"`
	Value     int     `json:"value"`     // 1-5
	Rationale string  `json:"rationale"` // judge's brief justification
	Weight    int     `json:"weight"`
}

// Verdict is the judge's evaluation of one Golden's output.
type Verdict struct {
	GoldenID  string    `json:"goldenId"`
	Category  Category  `json:"category"`
	Output    string    `json:"output,omitempty"`
	Scores    []Score   `json:"scores"`
	Mean      float64   `json:"mean"`
	Min       int       `json:"min"`
	Passed    bool      `json:"passed"`
	JudgedAt  time.Time `json:"judgedAt"`
	JudgeName string    `json:"judgeName"`
}

// Run is the aggregate result of running the harness over a corpus.
type Run struct {
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt"`
	Verdicts  []Verdict `json:"verdicts"`
	Passed    bool      `json:"passed"`
	Summary   Summary   `json:"summary"`
}

// Summary is per-category aggregation suitable for CI status output.
type Summary struct {
	Total     int                       `json:"total"`
	Passed    int                       `json:"passed"`
	Failed    int                       `json:"failed"`
	OverallMean float64                 `json:"overallMean"`
	ByCategory  map[Category]CategoryAgg `json:"byCategory"`
}

// CategoryAgg is per-category aggregation.
type CategoryAgg struct {
	Total  int     `json:"total"`
	Passed int     `json:"passed"`
	Failed int     `json:"failed"`
	Mean   float64 `json:"mean"`
}
