package evals

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeService returns a canned output regardless of input.
type fakeService struct {
	out string
	err error
}

func (f *fakeService) Complete(_ context.Context, _, _ string) (string, error) {
	return f.out, f.err
}

func TestNewRunner_RequiresService(t *testing.T) {
	if _, err := NewRunner(Config{Judge: &MockJudge{}}); err == nil {
		t.Error("expected error when Service missing")
	}
}

func TestNewRunner_RequiresJudge(t *testing.T) {
	if _, err := NewRunner(Config{Service: &fakeService{}}); err == nil {
		t.Error("expected error when Judge missing")
	}
}

func TestNewRunner_DefaultsTimeout(t *testing.T) {
	r, err := NewRunner(Config{Service: &fakeService{out: "ok"}, Judge: &MockJudge{}})
	if err != nil {
		t.Fatal(err)
	}
	if r.cfg.PerGoldenTimeout == 0 {
		t.Error("expected default per-golden timeout")
	}
}

func TestRunner_HappyPath_AllPass(t *testing.T) {
	r, _ := NewRunner(Config{
		Service: &fakeService{out: "Release v1.4.1 fixes a nil pointer in token refresh and a race in webhook delivery."},
		Judge:   &MockJudge{Default: 5},
	})

	corpus := []Golden{
		{ID: "g1", Category: CategoryReleaseNotes, UserPrompt: "input"},
		{ID: "g2", Category: CategoryReleaseNotes, UserPrompt: "input"},
	}

	run, err := r.Run(context.Background(), corpus)
	if err != nil {
		t.Fatal(err)
	}
	if !run.Passed {
		t.Errorf("expected run to pass; summary=%+v", run.Summary)
	}
	if run.Summary.Total != 2 || run.Summary.Passed != 2 || run.Summary.Failed != 0 {
		t.Errorf("summary mismatch: %+v", run.Summary)
	}
}

func TestRunner_OneFails_OverallFails(t *testing.T) {
	r, _ := NewRunner(Config{
		Service: &fakeService{out: "x"},
		Judge: &MockJudge{
			Default:  4,
			Override: map[string]int{"factuality": 1}, // forces min<3 → fail
		},
	})

	corpus := []Golden{{ID: "g1", Category: CategoryReleaseNotes, UserPrompt: "p"}}
	run, _ := r.Run(context.Background(), corpus)
	if run.Passed {
		t.Errorf("expected run to fail because min < MinPass")
	}
	if run.Summary.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", run.Summary.Failed)
	}
}

func TestRunner_FailFast(t *testing.T) {
	calls := 0
	svc := &fakeService{out: "x"}
	judge := &MockJudge{Default: 1} // every score = 1, all fail

	// Wrap service to count calls.
	wrapped := serviceFn(func(ctx context.Context, sp, up string) (string, error) {
		calls++
		return svc.Complete(ctx, sp, up)
	})

	r, _ := NewRunner(Config{
		Service:  wrapped,
		Judge:    judge,
		FailFast: true,
	})

	corpus := []Golden{
		{ID: "g1", Category: CategoryReleaseNotes, UserPrompt: "p"},
		{ID: "g2", Category: CategoryReleaseNotes, UserPrompt: "p"},
		{ID: "g3", Category: CategoryReleaseNotes, UserPrompt: "p"},
	}

	_, _ = r.Run(context.Background(), corpus)
	if calls != 1 {
		t.Errorf("expected fail-fast after 1 call, got %d", calls)
	}
}

func TestRunner_CategoryFilter(t *testing.T) {
	calls := 0
	wrapped := serviceFn(func(ctx context.Context, sp, up string) (string, error) {
		calls++
		return "ok", nil
	})

	r, _ := NewRunner(Config{
		Service:    wrapped,
		Judge:      &MockJudge{Default: 5},
		Categories: []Category{CategoryRiskNarrative},
	})

	corpus := []Golden{
		{ID: "rn", Category: CategoryReleaseNotes, UserPrompt: "p"},
		{ID: "rk", Category: CategoryRiskNarrative, UserPrompt: "p"},
	}

	run, _ := r.Run(context.Background(), corpus)
	if calls != 1 {
		t.Errorf("expected exactly 1 service call, got %d", calls)
	}
	if len(run.Verdicts) != 1 {
		t.Errorf("expected 1 verdict, got %d", len(run.Verdicts))
	}
}

func TestRunner_ServiceError_StopsRun(t *testing.T) {
	r, _ := NewRunner(Config{
		Service: &fakeService{err: errors.New("rate limit")},
		Judge:   &MockJudge{Default: 5},
	})

	corpus := []Golden{{ID: "g1", Category: CategoryReleaseNotes, UserPrompt: "p"}}
	_, err := r.Run(context.Background(), corpus)
	if err == nil {
		t.Fatal("expected service error to propagate")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("expected wrapped error: %v", err)
	}
}

func TestDeterministicJudge_EmptyOutput_FailsEverything(t *testing.T) {
	j := DeterministicJudge{}
	scores, err := j.Score(context.Background(), Golden{
		Category:   CategoryReleaseNotes,
		UserPrompt: "any",
		Rubric:     DefaultRubric(),
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range scores {
		if s.Value != 1 {
			t.Errorf("empty output should yield 1 across the board; got %s=%d", s.Criterion, s.Value)
		}
	}
}

func TestDeterministicJudge_HallucinationPenalty(t *testing.T) {
	j := DeterministicJudge{}
	scores, _ := j.Score(context.Background(), Golden{
		Category:   CategoryReleaseNotes,
		UserPrompt: "input",
		Rubric:     DefaultRubric(),
	}, "Production-ready 100% test coverage release")

	var halluc Score
	for _, s := range scores {
		if s.Criterion == "no_hallucination" {
			halluc = s
			break
		}
	}
	if halluc.Value > 2 {
		t.Errorf("expected hallucination penalty (≤2); got %d", halluc.Value)
	}
}

func TestDeterministicJudge_GovernanceVocabulary(t *testing.T) {
	j := DeterministicJudge{}
	scores, _ := j.Score(context.Background(), Golden{
		Category:   CategoryRiskNarrative,
		UserPrompt: "explain risk",
		Rubric:     DefaultRubric(),
	}, "This release has elevated risk because it touches auth and the actor budget requires cosign before publish.")

	for _, s := range scores {
		if s.Criterion == "governance_alignment" && s.Value < 4 {
			t.Errorf("expected governance vocabulary to score ≥4, got %d", s.Value)
		}
	}
}

func TestAggregate_WeightedMean(t *testing.T) {
	scores := []Score{
		{Value: 5, Weight: 2},
		{Value: 3, Weight: 1},
	}
	mean, min := aggregate(scores)
	want := (10.0 + 3.0) / 3.0
	if mean != want {
		t.Errorf("mean: got %v, want %v", mean, want)
	}
	if min != 3 {
		t.Errorf("min: got %d, want 3", min)
	}
}

func TestAggregate_ZeroWeightDefaults(t *testing.T) {
	scores := []Score{{Value: 4, Weight: 0}, {Value: 4, Weight: 0}}
	mean, _ := aggregate(scores)
	if mean != 4.0 {
		t.Errorf("zero weights should default to 1; got mean %v", mean)
	}
}

func TestPasses_RubricGates(t *testing.T) {
	g := Golden{
		ID:       "g",
		Category: CategoryReleaseNotes,
		Rubric:   DefaultRubric(),
	}

	// Mean above + min above → passes.
	pass := passes(Verdict{Mean: 4.5, Min: 4}, g)
	if !pass {
		t.Error("expected pass for high scores")
	}

	// Mean below floor → fails.
	if passes(Verdict{Mean: 3.0, Min: 4}, g) {
		t.Error("expected fail when mean < MeanPass")
	}

	// Min below floor → fails.
	if passes(Verdict{Mean: 4.5, Min: 2}, g) {
		t.Error("expected fail when min < MinPass")
	}
}

func TestSummarize_ByCategory(t *testing.T) {
	verdicts := []Verdict{
		{GoldenID: "a", Category: CategoryReleaseNotes, Mean: 4.5, Min: 4, Passed: true},
		{GoldenID: "b", Category: CategoryReleaseNotes, Mean: 3.5, Min: 2, Passed: false},
		{GoldenID: "c", Category: CategoryRiskNarrative, Mean: 5.0, Min: 5, Passed: true},
	}
	s := summarize(verdicts)
	if s.Total != 3 || s.Passed != 2 || s.Failed != 1 {
		t.Errorf("summary mismatch: %+v", s)
	}
	rn := s.ByCategory[CategoryReleaseNotes]
	if rn.Total != 2 || rn.Passed != 1 || rn.Failed != 1 {
		t.Errorf("release_notes agg: %+v", rn)
	}
}

func TestLoadEmbedded_ParsesGoldens(t *testing.T) {
	corpus, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	if len(corpus) < 3 {
		t.Errorf("expected at least 3 embedded goldens, got %d", len(corpus))
	}

	ids := make(map[string]bool)
	for _, g := range corpus {
		if !g.Category.Valid() {
			t.Errorf("invalid category on golden %q", g.ID)
		}
		if g.UserPrompt == "" {
			t.Errorf("golden %q has empty user_prompt", g.ID)
		}
		if ids[g.ID] {
			t.Errorf("duplicate id %q", g.ID)
		}
		ids[g.ID] = true
	}
}

func TestMockJudge_OverrideTakesEffect(t *testing.T) {
	j := &MockJudge{
		Default:  5,
		Override: map[string]int{"factuality": 1},
	}
	scores, _ := j.Score(context.Background(), Golden{Category: CategoryReleaseNotes, Rubric: DefaultRubric()}, "out")
	for _, s := range scores {
		if s.Criterion == "factuality" && s.Value != 1 {
			t.Errorf("expected override to apply; got %d", s.Value)
		}
		if s.Criterion != "factuality" && s.Value != 5 {
			t.Errorf("expected default 5 for non-overridden; got %d for %s", s.Value, s.Criterion)
		}
	}
}

func TestLLMJudge_NoServiceErrors(t *testing.T) {
	j := &LLMJudge{}
	if _, err := j.Score(context.Background(), Golden{Category: CategoryReleaseNotes}, "x"); err == nil {
		t.Error("expected error when LLMJudge has no Service")
	}
}

// serviceFn lets tests use a closure as Service without defining a struct.
type serviceFn func(ctx context.Context, systemPrompt, userPrompt string) (string, error)

func (f serviceFn) Complete(ctx context.Context, sp, up string) (string, error) { return f(ctx, sp, up) }
