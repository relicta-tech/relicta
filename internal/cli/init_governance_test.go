package cli

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// ADR-011, Option C: a new project is governed from its first release, while the
// schema default stays off so an upgrade changes nothing for anyone.
//
// This is the half that lives in init. TestGovernanceSchemaDefaultStaysOff in
// internal/config guards the other half; either alone would let the split
// collapse into "governed nowhere" or "governed everywhere".
func TestInitWritesGovernanceEnabled(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	restoreFormat := initFormat
	t.Cleanup(func() { initFormat = restoreFormat })
	initFormat = "yaml"

	if err := runInitQuick(); err != nil {
		t.Fatalf("runInitQuick: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".relicta.yaml"))
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}

	var parsed struct {
		Governance struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"governance"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse generated config: %v", err)
	}

	if !parsed.Governance.Enabled {
		t.Error("relicta init must write governance.enabled: true — without it a new " +
			"project versions and publishes but does not govern, and the setting is " +
			"something the user has to discover")
	}
}
