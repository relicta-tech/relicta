package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/internal/cgp"
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
