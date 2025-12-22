package manager

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func writeRegistryConfig(t *testing.T, configDir string, registries []RegistryEntry) {
	t.Helper()

	cfg := RegistryConfig{
		Version:    "1.0",
		Registries: registries,
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal registry config error: %v", err)
	}

	path := filepath.Join(configDir, RegistryConfigFile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write registry config error: %v", err)
	}
}

func writeRegistryCache(t *testing.T, cacheDir, name string, registry Registry) {
	t.Helper()

	if registry.UpdatedAt.IsZero() {
		registry.UpdatedAt = time.Now()
	}

	data, err := yaml.Marshal(registry)
	if err != nil {
		t.Fatalf("marshal registry cache error: %v", err)
	}

	path := filepath.Join(cacheDir, "registry-"+name+".yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write registry cache error: %v", err)
	}
}

func yamlMarshal(value any) ([]byte, error) {
	return yaml.Marshal(value)
}
