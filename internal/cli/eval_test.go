package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/internal/infrastructure/ai/evals"
)

func TestSelectJudge(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOk bool
	}{
		{"deterministic", "deterministic", true},
		{"", "deterministic", true},
		{"DETERMINISTIC", "deterministic", true},
		{"mock", "mock", true},
		// "llm" depends on env var presence; tested separately. Skipping here.
		{"bogus", "", false},
	}

	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			j, err := selectJudge(c.in)
			if c.wantOk && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !c.wantOk && err == nil {
				t.Fatal("expected error")
			}
			if c.wantOk && j.Name() != c.want {
				t.Errorf("name: got %q, want %q", j.Name(), c.want)
			}
		})
	}
}

func TestEvalNoopService_NonEmpty(t *testing.T) {
	out, err := evalNoopService{}.Complete(context.Background(), "sys", "user prompt here")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "user prompt here") {
		t.Errorf("expected user prompt to flow through; got %q", out)
	}
	if !strings.Contains(out, "policy") {
		t.Errorf("expected governance vocabulary in echo; got %q", out)
	}
}

func TestTruncate(t *testing.T) {
	cases := map[string]string{
		"short":             "short",
		"   spaced   ":      "spaced",
	}
	for in, want := range cases {
		if got := truncate(in, 100); got != want {
			t.Errorf("truncate(%q, 100) = %q, want %q", in, got, want)
		}
	}
	if got := truncate("aaaaaaaaaaaaaaaa", 5); got != "aaaaa…" {
		t.Errorf("truncate did not cut to 5 + ellipsis; got %q", got)
	}
}

func TestRunJudgeName_FallsBackOnEmptyVerdicts(t *testing.T) {
	if got := runJudgeName(&evals.Run{}); got != "?" {
		t.Errorf("expected fallback name '?', got %q", got)
	}
}

func TestLoadEvalCorpus_RejectsCustomPath(t *testing.T) {
	prev := evalGoldenFlag
	defer func() { evalGoldenFlag = prev }()
	evalGoldenFlag = "/tmp/x"
	if _, err := loadEvalCorpus(); err == nil {
		t.Error("expected V0 to reject --goldens path; reserved for future use")
	}
}

func TestLoadEvalCorpus_DefaultLoadsEmbedded(t *testing.T) {
	prev := evalGoldenFlag
	defer func() { evalGoldenFlag = prev }()
	evalGoldenFlag = ""
	corpus, err := loadEvalCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus) < 3 {
		t.Errorf("expected embedded corpus with ≥3 goldens; got %d", len(corpus))
	}
}
