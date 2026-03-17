package dsl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExamplePolicies(t *testing.T) {
	// Find the examples directory relative to the repo root
	examplesDir := filepath.Join("..", "..", "..", "..", "examples", "policies")

	// Check if the directory exists
	if _, err := os.Stat(examplesDir); os.IsNotExist(err) {
		t.Skip("examples/policies directory not found, skipping")
	}

	files, err := filepath.Glob(filepath.Join(examplesDir, "*.policy"))
	require.NoError(t, err, "failed to glob policy files")
	require.NotEmpty(t, files, "no policy files found in examples/policies")

	loader := NewLoader(LoaderOptions{})

	for _, file := range files {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			pol, err := loader.LoadFile(file)
			require.NoError(t, err, "failed to parse %s", name)
			assert.NotEmpty(t, pol.Rules, "policy %s has no rules", name)

			for _, rule := range pol.Rules {
				assert.NotEmpty(t, rule.ID, "rule has empty ID")
				assert.NotEmpty(t, rule.Name, "rule has empty name")
				assert.NotEmpty(t, rule.Conditions, "rule %s has no conditions", rule.Name)
			}
		})
	}
}
