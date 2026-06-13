package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

func withEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		old, had := os.LookupEnv(k)
		_ = os.Setenv(k, v)
		t.Cleanup(func() {
			if had {
				_ = os.Setenv(k, old)
			} else {
				_ = os.Unsetenv(k)
			}
		})
	}
}

func TestEnforceActorBudget_HumanPermissiveByDefault(t *testing.T) {
	origCfg := cfg
	t.Cleanup(func() { cfg = origCfg })
	cfg = config.DefaultConfig()

	// Clear CI markers so the actor resolves as human.
	withEnv(t, map[string]string{
		"CI": "", "GITHUB_ACTIONS": "", "GITLAB_CI": "", "JENKINS_URL": "",
		"USER": "alice", "GITHUB_ACTOR": "",
	})

	if err := enforceActorBudget("publish", 0.95); err != nil {
		t.Fatalf("human should be permitted a high-risk publish by default: %v", err)
	}
}

func TestEnforceActorBudget_CIRestrictedOnHighRisk(t *testing.T) {
	origCfg := cfg
	t.Cleanup(func() { cfg = origCfg })
	cfg = config.DefaultConfig()

	// GITHUB_ACTIONS marks the actor as CI → restrictive default budget.
	withEnv(t, map[string]string{
		"CI": "true", "GITHUB_ACTIONS": "true", "GITHUB_ACTOR": "ci-bot", "USER": "",
	})

	err := enforceActorBudget("publish", 0.95)
	if err == nil {
		t.Fatal("CI actor must be blocked from a critical-risk publish by the restrictive default budget")
	}
	if !strings.Contains(err.Error(), "autonomy budget") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnforceActorBudget_CIRequiresCosignEvenLowRisk(t *testing.T) {
	origCfg := cfg
	t.Cleanup(func() { cfg = origCfg })
	cfg = config.DefaultConfig()

	withEnv(t, map[string]string{
		"CI": "true", "GITHUB_ACTIONS": "true", "GITHUB_ACTOR": "ci-bot", "USER": "",
	})

	// The restrictive default budget requires a human cosigner for
	// publish/approve/rollback regardless of risk, so even a low-risk CI
	// publish is blocked — agents/CI cannot self-authorize privileged ops.
	if err := enforceActorBudget("publish", 0.1); err == nil {
		t.Fatal("CI publish must require a cosigner under the restrictive default budget")
	}
}
