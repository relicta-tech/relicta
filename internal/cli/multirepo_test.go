package cli

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/multirepo"
)

func TestFindGroup(t *testing.T) {
	// Set up test config with groups.
	cfg = &config.Config{
		RepositoryGroups: []multirepo.RepositoryGroup{
			{
				Name:     "platform",
				Strategy: multirepo.StrategyCoordinated,
				Repositories: []multirepo.RepoConfig{
					{Name: "core", Path: "/tmp/core"},
					{Name: "auth", Path: "/tmp/auth", Dependencies: []string{"core"}},
				},
			},
			{
				Name:     "services",
				Strategy: multirepo.StrategyIndependent,
				Repositories: []multirepo.RepoConfig{
					{Name: "svc-a", Path: "/tmp/svc-a"},
				},
			},
		},
	}

	// Test finding existing group.
	group := findGroup("platform")
	if group == nil {
		t.Fatal("expected to find group 'platform'")
	}
	if group.Name != "platform" {
		t.Errorf("expected name 'platform', got %q", group.Name)
	}
	if len(group.Repositories) != 2 {
		t.Errorf("expected 2 repos, got %d", len(group.Repositories))
	}

	// Test finding second group.
	group = findGroup("services")
	if group == nil {
		t.Fatal("expected to find group 'services'")
	}

	// Test not finding a group.
	group = findGroup("nonexistent")
	if group != nil {
		t.Error("expected nil for nonexistent group")
	}
}

func TestFindGroup_NilConfig(t *testing.T) {
	// Save and restore.
	saved := cfg
	cfg = nil
	defer func() { cfg = saved }()

	group := findGroup("anything")
	if group != nil {
		t.Error("expected nil when config is nil")
	}
}

func TestGroupCmd_Structure(t *testing.T) {
	// Verify the command hierarchy is correct.
	if groupCmd.Use != "group" {
		t.Errorf("expected use 'group', got %q", groupCmd.Use)
	}

	// Check subcommands are registered.
	subcommands := make(map[string]bool)
	for _, cmd := range groupCmd.Commands() {
		subcommands[cmd.Use] = true
	}

	expected := []string{"plan", "release", "status"}
	for _, name := range expected {
		if !subcommands[name] {
			t.Errorf("expected subcommand %q to be registered", name)
		}
	}
}

func TestGroupCmd_RequiresGroupFlag(t *testing.T) {
	// Calling setup without --group should fail.
	groupName = ""
	_, _, err := setupGroupCommand()
	if err == nil {
		t.Fatal("expected error when --group is empty")
	}
}

func TestGroupCmd_NonexistentGroup(t *testing.T) {
	cfg = &config.Config{
		RepositoryGroups: []multirepo.RepositoryGroup{},
	}
	groupName = "nonexistent"

	_, _, err := setupGroupCommand()
	if err == nil {
		t.Fatal("expected error for nonexistent group")
	}
}
