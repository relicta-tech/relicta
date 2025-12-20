package analysis

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/infrastructure/ai"
	"github.com/relicta-tech/relicta/internal/infrastructure/git"
)

type stubAIService struct {
	response string
	err      error
	ready    bool
}

func (s *stubAIService) GenerateChangelog(_ context.Context, _ *git.CategorizedChanges, _ ai.GenerateOptions) (string, error) {
	return "", nil
}

func (s *stubAIService) GenerateReleaseNotes(_ context.Context, _ string, _ ai.GenerateOptions) (string, error) {
	return "", nil
}

func (s *stubAIService) GenerateMarketingBlurb(_ context.Context, _ string, _ ai.GenerateOptions) (string, error) {
	return "", nil
}

func (s *stubAIService) SummarizeChanges(_ context.Context, _ *git.CategorizedChanges, _ ai.GenerateOptions) (string, error) {
	return "", nil
}

func (s *stubAIService) Complete(_ context.Context, _ string, _ string) (string, error) {
	return s.response, s.err
}

func (s *stubAIService) IsAvailable() bool {
	return s.ready
}

func TestAIClassifier_ParsesResponse(t *testing.T) {
	service := &stubAIService{
		ready:    true,
		response: `{"type":"fix","scope":"api","confidence":0.91,"reasoning":"keyword match","is_breaking":false,"breaking_reason":"","should_skip":false,"skip_reason":""}`,
	}
	classifier := NewAIClassifier(service)
	commit := CommitInfo{
		Hash:    sourcecontrol.CommitHash("abc123"),
		Subject: "fix api bug",
	}

	result, err := classifier.Classify(context.Background(), commit)
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if result.Method != MethodAI {
		t.Errorf("Method = %s, want %s", result.Method, MethodAI)
	}
	if result.Scope != "api" {
		t.Errorf("Scope = %q, want %q", result.Scope, "api")
	}
	if result.Confidence != 0.91 {
		t.Errorf("Confidence = %.2f, want 0.91", result.Confidence)
	}
}

func TestAIClassifier_SkipResult(t *testing.T) {
	service := &stubAIService{
		ready:    true,
		response: `{"type":"","scope":"","confidence":0.2,"reasoning":"merge commit","is_breaking":false,"breaking_reason":"","should_skip":true,"skip_reason":"merge commit"}`,
	}
	classifier := NewAIClassifier(service)
	commit := CommitInfo{
		Hash:    sourcecontrol.CommitHash("abc123"),
		Subject: "Merge branch",
	}

	result, err := classifier.Classify(context.Background(), commit)
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if result.Method != MethodSkipped {
		t.Errorf("Method = %s, want %s", result.Method, MethodSkipped)
	}
	if !result.ShouldSkip {
		t.Error("ShouldSkip = false, want true")
	}
}

func TestAIClassifier_Unavailable(t *testing.T) {
	service := &stubAIService{ready: false}
	classifier := NewAIClassifier(service)
	commit := CommitInfo{Hash: sourcecontrol.CommitHash("abc")}

	_, err := classifier.Classify(context.Background(), commit)
	if err == nil {
		t.Fatal("expected error for unavailable service")
	}
}

func TestAIClassifier_ErrorResponse(t *testing.T) {
	service := &stubAIService{ready: true, err: errors.New("api down")}
	classifier := NewAIClassifier(service)
	commit := CommitInfo{Hash: sourcecontrol.CommitHash("abc")}

	_, err := classifier.Classify(context.Background(), commit)
	if err == nil {
		t.Fatal("expected error for AI failure")
	}
}

func TestParseAIResponse_Invalid(t *testing.T) {
	if _, err := parseAIResponse(""); err == nil {
		t.Fatal("expected error for empty response")
	}
	if _, err := parseAIResponse("no json"); err == nil {
		t.Fatal("expected error for missing json")
	}
}

func TestClamp(t *testing.T) {
	if got := clamp(-1, 0, 1); got != 0 {
		t.Errorf("clamp(-1) = %v, want 0", got)
	}
	if got := clamp(2, 0, 1); got != 1 {
		t.Errorf("clamp(2) = %v, want 1", got)
	}
	if got := clamp(0.5, 0, 1); got != 0.5 {
		t.Errorf("clamp(0.5) = %v, want 0.5", got)
	}
}

func TestBuildUserPrompt_IncludesDetails(t *testing.T) {
	commit := CommitInfo{
		Hash:    sourcecontrol.CommitHash("abc123"),
		Message: "update doc\n\nmore detail",
		Subject: "update doc",
		Files:   []string{"README.md"},
		Stats: DiffStats{
			FilesChanged: 1,
			Additions:    2,
			Deletions:    1,
		},
		Diff: "diff --git a/README.md b/README.md\n+added",
	}

	prompt := buildUserPrompt(commit)
	if !strings.Contains(prompt, "README.md") {
		t.Errorf("prompt missing file list")
	}
	if !strings.Contains(prompt, "Stats: 1 files") {
		t.Errorf("prompt missing stats")
	}
	if !strings.Contains(prompt, "diff --git") {
		t.Errorf("prompt missing diff")
	}
}
