package cli

import (
	"os"
	"testing"

	"github.com/relicta-tech/relicta/internal/config"
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
