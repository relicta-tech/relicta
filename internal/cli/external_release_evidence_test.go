package cli

import (
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

func TestExternalReleaseEvidenceValidate(t *testing.T) {
	valid := externalReleaseEvidence{
		Outcome:  "published",
		Provider: "github-actions",
		RunURL:   "https://github.com/relicta-tech/relicta/actions/runs/42",
		Tag:      "v4.3.0",
		Commit:   "0123456789abcdef0123456789abcdef01234567",
		Artifacts: map[string]string{
			"checksums.txt": "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid evidence rejected: %v", err)
	}

	tests := []struct {
		name string
		edit func(*externalReleaseEvidence)
	}{
		{"unknown outcome", func(e *externalReleaseEvidence) { e.Outcome = "unknown" }},
		{"short commit", func(e *externalReleaseEvidence) { e.Commit = "abc123" }},
		{"non-https run URL", func(e *externalReleaseEvidence) { e.RunURL = "http://example.test/run" }},
		{"missing artifacts", func(e *externalReleaseEvidence) { e.Artifacts = nil }},
		{"invalid artifact digest", func(e *externalReleaseEvidence) { e.Artifacts["checksums.txt"] = "sha256:bad" }},
		{"failed without reason", func(e *externalReleaseEvidence) { e.Outcome = "failed" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evidence := valid
			evidence.Artifacts = map[string]string{"checksums.txt": valid.Artifacts["checksums.txt"]}
			tt.edit(&evidence)
			if err := evidence.Validate(); err == nil {
				t.Fatal("invalid evidence was accepted")
			}
		})
	}
}

func TestEnforceExternalPublishBudget(t *testing.T) {
	originalConfig := cfg
	t.Cleanup(func() { cfg = originalConfig })
	cfg = config.DefaultConfig()
	withEnv(t, map[string]string{
		"CI": "true", "GITHUB_ACTIONS": "true", "GITHUB_ACTOR": "release-bot", "USER": "",
	})

	if err := enforceExternalPublishBudget("published", 0.1); err == nil || !strings.Contains(err.Error(), "cosigner") {
		t.Fatalf("published external release must respect the restrictive CI budget; got %v", err)
	}
	if err := enforceExternalPublishBudget("failed", 0.1); err != nil {
		t.Fatalf("failed external release must remain recordable: %v", err)
	}
	if err := enforceExternalPublishBudgetWithCosigner("published", 0.1, true); err != nil {
		t.Fatalf("API-verified human cosigner should satisfy the publish budget: %v", err)
	}
}
