package cli

import (
	"os"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/config"
)

func cleanupEnv(keys ...string) func() {
	original := make(map[string]string)
	for _, key := range keys {
		original[key] = os.Getenv(key)
	}
	return func() {
		for _, key := range keys {
			if val, ok := original[key]; ok && val != "" {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
	}
}

func TestCreateCGPActorHuman(t *testing.T) {
	cfg = config.DefaultConfig()
	defer cleanupEnv("CI", "GITHUB_ACTOR", "GITHUB_ACTIONS", "GITLAB_CI", "JENKINS_URL", "USER")()

	os.Unsetenv("CI")
	os.Unsetenv("GITHUB_ACTOR")
	os.Unsetenv("GITHUB_ACTIONS")
	os.Unsetenv("GITLAB_CI")
	os.Unsetenv("JENKINS_URL")
	os.Setenv("USER", "tester")
	cfg.Governance.TrustedActors = []string{"tester"}

	actor := createCGPActor()

	if actor.Kind != "human" {
		t.Fatalf("expected human kind, got %s", actor.Kind)
	}
	if actor.TrustLevel != 3 { // TrustLevelFull
		t.Fatalf("expected full trust level for trusted human, got %d", actor.TrustLevel)
	}
}

func TestCreateCGPActorCI(t *testing.T) {
	cfg = config.DefaultConfig()
	defer cleanupEnv("CI", "GITHUB_ACTOR", "GITHUB_ACTIONS", "GITLAB_CI", "JENKINS_URL", "USER")()

	os.Setenv("CI", "true")
	os.Setenv("GITHUB_ACTOR", "ci-user")

	actor := createCGPActor()

	if actor.Kind != "ci" {
		t.Fatalf("expected ci kind, got %s", actor.Kind)
	}
	if actor.TrustLevel == 3 {
		t.Fatal("expected ci actor not to have full trust level")
	}
}

// TestCreateCGPActorUntrustedHumanIsLimited locks in the trust-spoofing fix:
// a human actor that is NOT in the trusted-actors allowlist must stay Limited
// and must NOT be able to auto-approve. Previously any invocation without CI
// markers was granted TrustLevelTrusted, so clearing CI=true unlocked
// auto-approval.
func TestCreateCGPActorUntrustedHumanIsLimited(t *testing.T) {
	cfg = config.DefaultConfig()
	defer cleanupEnv("CI", "GITHUB_ACTOR", "GITHUB_ACTIONS", "GITLAB_CI", "JENKINS_URL", "USER")()

	os.Unsetenv("CI")
	os.Unsetenv("GITHUB_ACTOR")
	os.Unsetenv("GITHUB_ACTIONS")
	os.Unsetenv("GITLAB_CI")
	os.Unsetenv("JENKINS_URL")
	os.Setenv("USER", "stranger")
	cfg.Governance.TrustedActors = nil // empty allowlist

	actor := createCGPActor()

	if actor.Kind != "human" {
		t.Fatalf("expected human kind, got %s", actor.Kind)
	}
	if actor.TrustLevel != cgp.TrustLevelLimited {
		t.Fatalf("untrusted human must be Limited, got %d", actor.TrustLevel)
	}
	if actor.TrustLevel.CanAutoApprove() {
		t.Fatal("untrusted human must NOT be able to auto-approve")
	}
}

// TestCreateCGPActorTrustScopedByKind verifies a kind-scoped allowlist entry
// (human:alice) does not also trust a CI actor running under the same identity
// (GITHUB_ACTOR=alice), closing a same-name escalation path.
func TestCreateCGPActorTrustScopedByKind(t *testing.T) {
	cfg = config.DefaultConfig()
	defer cleanupEnv("CI", "GITHUB_ACTOR", "GITHUB_ACTIONS", "GITLAB_CI", "JENKINS_URL", "USER")()

	cfg.Governance.TrustedActors = []string{"human:alice"}

	os.Setenv("CI", "true")
	os.Setenv("GITHUB_ACTOR", "alice")
	ciActor := createCGPActor()
	if ciActor.TrustLevel == cgp.TrustLevelFull {
		t.Fatal("ci:alice must not match a human:alice trust entry")
	}

	os.Unsetenv("CI")
	os.Unsetenv("GITHUB_ACTIONS")
	os.Unsetenv("GITHUB_ACTOR")
	os.Setenv("USER", "alice")
	humanActor := createCGPActor()
	if humanActor.TrustLevel != cgp.TrustLevelFull {
		t.Fatalf("human:alice must match its trust entry, got %d", humanActor.TrustLevel)
	}
}
