// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/infrastructure/ai"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/ai/evals"
)

var (
	evalCategoryFlag string
	evalFailFast     bool
	evalJudgeFlag    string
	evalTimeoutFlag  time.Duration
	evalGoldenFlag   string
)

func init() {
	evalRunCmd.Flags().StringVar(&evalCategoryFlag, "category", "", "filter by category (release_notes, risk_narrative, communicate, summarize_diff)")
	evalRunCmd.Flags().BoolVar(&evalFailFast, "fail-fast", false, "stop the run on the first failed verdict")
	evalRunCmd.Flags().StringVar(&evalJudgeFlag, "judge", "deterministic", "scoring judge: deterministic (no API calls) | mock (test only)")
	evalRunCmd.Flags().DurationVar(&evalTimeoutFlag, "timeout", 30*time.Second, "per-golden timeout")
	evalRunCmd.Flags().StringVar(&evalGoldenFlag, "goldens", "", "path to a goldens directory; defaults to embedded corpus")

	evalCmd.AddCommand(evalRunCmd)
	evalCmd.AddCommand(evalListCmd)
}

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "AI evaluation harness (model regression gate)",
	Long: `Run the Relicta AI evaluation harness against a corpus of golden cases.

The harness is a non-negotiable gate for any model bump on a governance product.
Without it, prompt drift on a new model silently degrades release-notes /
risk-narrative quality between deployments.

V0 ships with deterministic heuristic scoring (no API keys needed) and a
small embedded golden corpus (release_notes, risk_narrative, communicate).
Real LLM-as-judge scoring is opt-in via build tag in a follow-up.`,
}

var evalRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run AI evals against the configured AI service",
	Long: `Drive the configured AI service against the golden corpus and score outputs.

Examples:

  # Run all goldens with deterministic scoring (no API needed for judge)
  relicta eval run

  # Run only risk-narrative cases
  relicta eval run --category risk_narrative

  # JSON output for CI ingestion
  relicta eval run --json

  # Stop at first failure (pre-commit use)
  relicta eval run --fail-fast`,
	RunE: runEval,
}

var evalListCmd = &cobra.Command{
	Use:   "list",
	Short: "List embedded golden cases",
	RunE:  listEvals,
}

func runEval(cmd *cobra.Command, args []string) error {
	corpus, err := loadEvalCorpus()
	if err != nil {
		return err
	}

	judge, err := selectJudge(evalJudgeFlag)
	if err != nil {
		return err
	}

	svc := buildEvalService(cmd.Context())

	cfg := evals.Config{
		Service:          svc,
		Judge:            judge,
		FailFast:         evalFailFast,
		PerGoldenTimeout: evalTimeoutFlag,
	}
	if evalCategoryFlag != "" {
		cfg.Categories = []evals.Category{evals.Category(evalCategoryFlag)}
	}

	runner, err := evals.NewRunner(cfg)
	if err != nil {
		return fmt.Errorf("init runner: %w", err)
	}

	run, err := runner.Run(cmd.Context(), corpus)
	if err != nil {
		return fmt.Errorf("eval run: %w", err)
	}

	if outputJSON {
		return emitJSON(run)
	}

	emitText(run)

	if !run.Passed {
		return fmt.Errorf("eval run FAILED — %d of %d goldens did not pass", run.Summary.Failed, run.Summary.Total)
	}
	return nil
}

func listEvals(_ *cobra.Command, _ []string) error {
	corpus, err := loadEvalCorpus()
	if err != nil {
		return err
	}

	if outputJSON {
		return emitJSON(corpus)
	}

	for _, g := range corpus {
		fmt.Printf("- %s [%s]: %s\n", g.ID, g.Category, g.Description)
	}
	return nil
}

func loadEvalCorpus() ([]evals.Golden, error) {
	if evalGoldenFlag != "" {
		// Future: load from disk via fs.FS. Embedded-only in V0.
		return nil, errors.New("--goldens flag is reserved for future use; V0 uses the embedded corpus")
	}
	return evals.LoadEmbedded()
}

// evalNoopService returns canned strings so the deterministic judge can run
// without API keys. Used when no AI provider is configured.
type evalNoopService struct{}

func (evalNoopService) Complete(_ context.Context, _, userPrompt string) (string, error) {
	// Return a moderately governance-flavored echo so deterministic scoring
	// has something non-empty to evaluate.
	return "Generated content responding to prompt: " + truncate(userPrompt, 80) +
		". This release follows policy and includes audit trail provenance.", nil
}

// evalAIServiceAdapter wraps the real ai.Service to expose only the
// Complete method that the eval harness needs.
type evalAIServiceAdapter struct {
	svc ai.Service
}

func (a evalAIServiceAdapter) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return a.svc.Complete(ctx, systemPrompt, userPrompt)
}

// buildEvalService returns a Service backed by the configured AI provider
// when credentials are available, otherwise falls back to evalNoopService.
//
// Detection: builds an ai.Service from env-discovered API keys + model
// override flags. If construction fails OR the service reports !IsAvailable
// (no API key), uses noop and prints a one-line notice.
func buildEvalService(_ context.Context) evals.Service {
	opts := buildEvalServiceOptions()

	svc, err := ai.NewService(opts...)
	if err != nil || svc == nil || !svc.IsAvailable() {
		printInfo("eval: no AI provider configured — using noop service. Set OPENAI_API_KEY / ANTHROPIC_API_KEY / GEMINI_API_KEY to exercise the real provider.")
		return evalNoopService{}
	}

	printInfo("eval: using configured AI provider for completion")
	return evalAIServiceAdapter{svc: svc}
}

// buildEvalServiceOptions reads env vars + the global --model flag and
// converts them into ai.ServiceOption values. Provider precedence:
// OPENAI_API_KEY > ANTHROPIC_API_KEY > GEMINI_API_KEY > OLLAMA_HOST.
func buildEvalServiceOptions() []ai.ServiceOption {
	var opts []ai.ServiceOption

	switch {
	case os.Getenv("OPENAI_API_KEY") != "":
		opts = append(opts, ai.WithProvider("openai"), ai.WithAPIKey(os.Getenv("OPENAI_API_KEY")))
	case os.Getenv("ANTHROPIC_API_KEY") != "":
		opts = append(opts, ai.WithProvider("anthropic"), ai.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")))
	case os.Getenv("GEMINI_API_KEY") != "":
		opts = append(opts, ai.WithProvider("gemini"), ai.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	case os.Getenv("OLLAMA_HOST") != "":
		opts = append(opts, ai.WithProvider("ollama"), ai.WithBaseURL(os.Getenv("OLLAMA_HOST")))
	}

	// Honor global --model flag in `provider/model` form.
	if modelFlag != "" {
		if parts := strings.SplitN(modelFlag, "/", 2); len(parts) == 2 {
			opts = append(opts, ai.WithProvider(parts[0]), ai.WithModel(parts[1]))
		} else {
			opts = append(opts, ai.WithModel(modelFlag))
		}
	}

	return opts
}

func selectJudge(name string) (evals.Judge, error) {
	switch strings.ToLower(name) {
	case "deterministic", "":
		return evals.DeterministicJudge{}, nil
	case "mock":
		return &evals.MockJudge{Default: 4}, nil
	case "llm":
		// LLMJudge requires a configured AI service. buildEvalService
		// resolves one from env vars (OPENAI_API_KEY etc.); when none is
		// available we refuse rather than silently degrading to noop —
		// LLM-as-judge with no LLM is meaningless.
		svc := buildEvalServiceOptions()
		if len(svc) == 0 {
			return nil, errors.New("--judge llm requires a configured AI provider (set OPENAI_API_KEY / ANTHROPIC_API_KEY / GEMINI_API_KEY)")
		}
		return newLLMJudgeFromEnv()
	default:
		return nil, fmt.Errorf("unknown judge %q: use deterministic | mock | llm", name)
	}
}

// newLLMJudgeFromEnv constructs an LLMJudge from the same env-driven AI
// service builder used by buildEvalService. Refuses when no API key is set.
func newLLMJudgeFromEnv() (evals.Judge, error) {
	opts := buildEvalServiceOptions()
	if len(opts) == 0 {
		return nil, errors.New("no AI provider configured for LLM judge")
	}
	svc, err := ai.NewService(opts...)
	if err != nil {
		return nil, fmt.Errorf("init AI service for LLM judge: %w", err)
	}
	if svc == nil || !svc.IsAvailable() {
		return nil, errors.New("AI service not available for LLM judge")
	}
	return evals.NewLLMJudge(evalAIServiceAdapter{svc: svc}, "llm"), nil
}

func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func emitText(run *evals.Run) {
	if run.Summary.Total == 0 {
		printInfo("no goldens matched the filter — nothing evaluated")
		return
	}

	pass := run.Summary.Passed
	fail := run.Summary.Failed

	fmt.Printf("AI eval run — judge=%q duration=%s\n",
		runJudgeName(run), run.EndedAt.Sub(run.StartedAt).Round(time.Millisecond))
	fmt.Printf("  Total:        %d\n", run.Summary.Total)
	fmt.Printf("  Passed:       %d\n", pass)
	fmt.Printf("  Failed:       %d\n", fail)
	fmt.Printf("  OverallMean:  %.2f / 5.00\n", run.Summary.OverallMean)
	fmt.Println()

	if len(run.Summary.ByCategory) > 0 {
		fmt.Println("By category:")
		for cat, agg := range run.Summary.ByCategory {
			fmt.Printf("  %-20s total=%d passed=%d failed=%d mean=%.2f\n",
				cat, agg.Total, agg.Passed, agg.Failed, agg.Mean)
		}
		fmt.Println()
	}

	if fail == 0 {
		printSuccess("eval run PASSED")
	} else {
		fmt.Println("Failed verdicts:")
		for _, v := range run.Verdicts {
			if v.Passed {
				continue
			}
			fmt.Printf("  - %s [%s] mean=%.2f min=%d\n", v.GoldenID, v.Category, v.Mean, v.Min)
			for _, s := range v.Scores {
				if s.Value <= 2 {
					fmt.Printf("      ✗ %s (%d): %s\n", s.Criterion, s.Value, s.Rationale)
				}
			}
		}
	}
}

func runJudgeName(run *evals.Run) string {
	for _, v := range run.Verdicts {
		if v.JudgeName != "" {
			return v.JudgeName
		}
	}
	return "?"
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
