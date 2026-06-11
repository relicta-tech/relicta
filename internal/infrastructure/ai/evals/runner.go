package evals

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Service is the surface the runner needs from an AI provider.
//
// Defined locally to avoid an import cycle with `internal/infrastructure/ai`.
// Adapters convert ai.Service.Complete (or whatever method matches the
// golden's category) into this shape.
type Service interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// Config configures a runner pass.
type Config struct {
	// Service is the AI service under evaluation. Required.
	Service Service

	// Judge scores outputs. Required. Use DeterministicJudge for CI without
	// API keys, or wire a real LLMJudge in follow-up.
	Judge Judge

	// Categories optionally restricts the run to specific categories.
	// Empty = run all loaded goldens.
	Categories []Category

	// PerGoldenTimeout caps how long any single golden may take.
	// Zero defaults to 30s.
	PerGoldenTimeout time.Duration

	// FailFast stops the run on the first failed verdict. Useful in pre-commit.
	FailFast bool
}

// Runner drives an AI service against a corpus of goldens and aggregates
// verdicts into a Run record.
type Runner struct {
	cfg Config
}

// NewRunner constructs a Runner. Returns an error if required fields are missing.
func NewRunner(cfg Config) (*Runner, error) {
	if cfg.Service == nil {
		return nil, errors.New("evals: Service is required")
	}
	if cfg.Judge == nil {
		return nil, errors.New("evals: Judge is required")
	}
	if cfg.PerGoldenTimeout == 0 {
		cfg.PerGoldenTimeout = 30 * time.Second
	}
	return &Runner{cfg: cfg}, nil
}

// Run evaluates each Golden in the corpus and returns the aggregate Run.
func (r *Runner) Run(ctx context.Context, corpus []Golden) (*Run, error) {
	run := &Run{StartedAt: time.Now().UTC()}
	defer func() { run.EndedAt = time.Now().UTC() }()

	for _, g := range corpus {
		if !r.includes(g.Category) {
			continue
		}

		verdict, err := r.runOne(ctx, g)
		if err != nil {
			return run, fmt.Errorf("golden %q: %w", g.ID, err)
		}
		run.Verdicts = append(run.Verdicts, verdict)

		if r.cfg.FailFast && !verdict.Passed {
			break
		}
	}

	run.Summary = summarize(run.Verdicts)
	run.Passed = run.Summary.Failed == 0 && len(run.Verdicts) > 0
	return run, nil
}

// runOne drives a single golden through service + judge + gate evaluation.
func (r *Runner) runOne(ctx context.Context, g Golden) (Verdict, error) {
	subCtx, cancel := context.WithTimeout(ctx, r.cfg.PerGoldenTimeout)
	defer cancel()

	output, err := r.cfg.Service.Complete(subCtx, g.SystemPrompt, g.UserPrompt)
	if err != nil {
		return Verdict{
			GoldenID: g.ID,
			Category: g.Category,
		}, fmt.Errorf("service.Complete: %w", err)
	}

	scores, err := r.cfg.Judge.Score(subCtx, g, output)
	if err != nil {
		return Verdict{
			GoldenID: g.ID,
			Category: g.Category,
			Output:   output,
		}, fmt.Errorf("judge.Score: %w", err)
	}

	v := Verdict{
		GoldenID: g.ID,
		Category: g.Category,
		Output:   output,
		Scores:   scores,
	}
	finalize(&v, r.cfg.Judge.Name())

	v.Passed = passes(v, g, r.cfg.Judge)
	return v, nil
}

// includes reports whether the runner's category filter accepts c.
func (r *Runner) includes(c Category) bool {
	if len(r.cfg.Categories) == 0 {
		return true
	}
	for _, allowed := range r.cfg.Categories {
		if allowed == c {
			return true
		}
	}
	return false
}

// passes applies the rubric's MinPass / MeanPass gates plus any per-golden
// override. Gates the golden/rubric leave unset fall back to thresholds the
// judge declares for its own scoring ceiling (see PassThresholder), then to
// the LLM-grade defaults (min 3, mean 4.0).
func passes(v Verdict, g Golden, judge Judge) bool {
	minPass := g.Rubric.MinPass
	if g.MinPass > 0 {
		minPass = g.MinPass
	}
	meanPass := g.Rubric.MeanPass

	if jt, ok := judge.(PassThresholder); ok {
		jMin, jMean := jt.PassThresholds()
		if minPass == 0 {
			minPass = jMin
		}
		if meanPass == 0 {
			meanPass = jMean
		}
	}
	if minPass == 0 {
		minPass = 3
	}
	if meanPass == 0 {
		meanPass = 4.0
	}

	if v.Min < minPass {
		return false
	}
	if v.Mean < meanPass {
		return false
	}
	return true
}

// summarize builds the Run.Summary aggregate from per-golden verdicts.
func summarize(verdicts []Verdict) Summary {
	s := Summary{
		ByCategory: make(map[Category]CategoryAgg),
	}

	if len(verdicts) == 0 {
		return s
	}

	totalMean := 0.0
	for _, v := range verdicts {
		s.Total++
		if v.Passed {
			s.Passed++
		} else {
			s.Failed++
		}
		totalMean += v.Mean

		agg := s.ByCategory[v.Category]
		agg.Total++
		if v.Passed {
			agg.Passed++
		} else {
			agg.Failed++
		}
		agg.Mean += v.Mean
		s.ByCategory[v.Category] = agg
	}

	for cat, agg := range s.ByCategory {
		if agg.Total > 0 {
			agg.Mean = agg.Mean / float64(agg.Total)
		}
		s.ByCategory[cat] = agg
	}

	s.OverallMean = totalMean / float64(len(verdicts))
	return s
}
