package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/internal/analysis"
	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/domain/changes"
	domain "github.com/relicta-tech/relicta/internal/domain/release/domain"
)

func TestFormatRiskScoreDisplayStyles(t *testing.T) {
	out := formatRiskScoreDisplay(0.42, "low")
	if !strings.Contains(strings.ToLower(out), "low") {
		t.Fatalf("expected low severity to be mentioned, got %q", out)
	}

	out = formatRiskScoreDisplay(0.85, "high")
	if !strings.Contains(strings.ToLower(out), "high") {
		t.Fatalf("expected high severity to be mentioned, got %q", out)
	}
}

func TestFormatDecisionDisplay(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{"approved", "approved", "approved"},
		{"approval_required", "approval_required", "requires approval"},
		{"rejected", "rejected", "rejected"},
		{"unknown", "other", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDecisionDisplay(tt.input)
			if !strings.Contains(strings.ToLower(got), tt.contains) {
				t.Errorf("display %q missing %q: %q", tt.input, tt.contains, got)
			}
		})
	}
}

func TestFormatAutoApproveDisplay(t *testing.T) {
	if !strings.Contains(strings.ToLower(formatAutoApproveDisplay(true)), "yes") {
		t.Fatal("expected yes when auto-approve true")
	}

	if !strings.Contains(strings.ToLower(formatAutoApproveDisplay(false)), "no") {
		t.Fatal("expected no when auto-approve false")
	}
}

func TestCreateCGPActorForPlan(t *testing.T) {
	origUser := os.Getenv("USER")
	os.Setenv("USER", "plan-user")
	defer func() {
		if origUser == "" {
			os.Unsetenv("USER")
		} else {
			os.Setenv("USER", origUser)
		}
	}()

	actor := createCGPActorForPlan()
	if actor.Kind != cgp.ActorKindHuman {
		t.Fatalf("expected human actor, got %v", actor.Kind)
	}
	if !strings.Contains(actor.ID, "plan-user") {
		t.Fatalf("expected actor ID to contain user, got %s", actor.ID)
	}
}

func TestClassificationTypeLabel(t *testing.T) {
	tests := []struct {
		name  string
		input *analysis.CommitClassification
		want  string
	}{
		{"nil", nil, "unknown"},
		{"skip", &analysis.CommitClassification{ShouldSkip: true}, "skip"},
		{"empty type", &analysis.CommitClassification{}, "unknown"},
		{"feat", &analysis.CommitClassification{Type: changes.CommitTypeFeat}, "feat"},
		{"fix", &analysis.CommitClassification{Type: changes.CommitTypeFix}, "fix"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classificationTypeLabel(tt.input)
			if got != tt.want {
				t.Errorf("classificationTypeLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTrimList(t *testing.T) {
	tests := []struct {
		name  string
		items []string
		limit int
		want  int
		last  string
	}{
		{"under limit", []string{"a", "b"}, 5, 2, "b"},
		{"at limit", []string{"a", "b", "c"}, 3, 3, "c"},
		{"over limit", []string{"a", "b", "c", "d"}, 2, 3, "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimList(tt.items, tt.limit)
			if len(got) != tt.want {
				t.Errorf("trimList() len = %d, want %d", len(got), tt.want)
			}
			if got[len(got)-1] != tt.last {
				t.Errorf("trimList() last = %q, want %q", got[len(got)-1], tt.last)
			}
		})
	}
}

func TestConvertReleaseTypeToBumpKind(t *testing.T) {
	tests := []struct {
		input changes.ReleaseType
		want  domain.BumpKind
	}{
		{changes.ReleaseTypeMajor, domain.BumpMajor},
		{changes.ReleaseTypeMinor, domain.BumpMinor},
		{changes.ReleaseTypePatch, domain.BumpPatch},
		{changes.ReleaseType(""), domain.BumpNone},
	}
	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			got := convertReleaseTypeToBumpKind(tt.input)
			if got != tt.want {
				t.Errorf("convertReleaseTypeToBumpKind(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseClassificationOverride(t *testing.T) {
	current := &analysis.CommitClassification{
		CommitHash: "abc123",
		Type:       changes.CommitTypeFix,
	}

	t.Run("skip", func(t *testing.T) {
		got, err := parseClassificationOverride("skip", current)
		if err != nil {
			t.Fatal(err)
		}
		if !got.ShouldSkip {
			t.Error("expected ShouldSkip true")
		}
	})

	t.Run("skip shorthand", func(t *testing.T) {
		got, err := parseClassificationOverride("s", current)
		if err != nil {
			t.Fatal(err)
		}
		if !got.ShouldSkip {
			t.Error("expected ShouldSkip true")
		}
	})

	t.Run("feat", func(t *testing.T) {
		got, err := parseClassificationOverride("feat", current)
		if err != nil {
			t.Fatal(err)
		}
		if got.Type != changes.CommitTypeFeat {
			t.Errorf("got type %v, want feat", got.Type)
		}
		if got.IsBreaking {
			t.Error("should not be breaking")
		}
	})

	t.Run("breaking", func(t *testing.T) {
		got, err := parseClassificationOverride("feat!", current)
		if err != nil {
			t.Fatal(err)
		}
		if !got.IsBreaking {
			t.Error("expected breaking")
		}
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := parseClassificationOverride("nonsense", current)
		if err == nil {
			t.Error("expected error for invalid type")
		}
	})
}
