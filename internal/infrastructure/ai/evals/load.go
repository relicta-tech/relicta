package evals

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"gopkg.in/yaml.v3"
)

// EmbeddedGoldens is the default golden corpus shipped with Relicta.
// New goldens are added by dropping YAML files into `goldens/` and rebuilding.
//
//go:embed goldens/*.yaml
var EmbeddedGoldens embed.FS

// LoadGoldens parses every *.yaml file in fs (rooted at dir) into Goldens.
//
// Each YAML file may contain a single Golden or a `goldens: [...]` array.
// IDs across the loaded corpus must be unique; duplicates return an error.
func LoadGoldens(efs fs.FS, dir string) ([]Golden, error) {
	entries, err := fs.ReadDir(efs, dir)
	if err != nil {
		return nil, fmt.Errorf("read goldens dir %q: %w", dir, err)
	}

	var corpus []Golden
	seen := make(map[string]string)

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		body, err := fs.ReadFile(efs, dir+"/"+name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}

		batch, err := parseGoldenFile(body)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}

		for _, g := range batch {
			if g.ID == "" {
				return nil, fmt.Errorf("%s: golden missing id", name)
			}
			if !g.Category.Valid() {
				return nil, fmt.Errorf("%s: golden %q has invalid category %q", name, g.ID, g.Category)
			}
			if existing, dup := seen[g.ID]; dup {
				return nil, fmt.Errorf("duplicate golden id %q in %s and %s", g.ID, existing, name)
			}
			seen[g.ID] = name
			corpus = append(corpus, g)
		}
	}

	if len(corpus) == 0 {
		return nil, errors.New("no goldens found")
	}
	return corpus, nil
}

// LoadEmbedded loads the in-binary golden corpus.
func LoadEmbedded() ([]Golden, error) {
	return LoadGoldens(EmbeddedGoldens, "goldens")
}

// parseGoldenFile decodes either a single Golden or a {goldens: [...]} batch.
func parseGoldenFile(body []byte) ([]Golden, error) {
	// Try batch format first.
	var batch struct {
		Goldens []Golden `yaml:"goldens"`
	}
	if err := yaml.Unmarshal(body, &batch); err == nil && len(batch.Goldens) > 0 {
		return batch.Goldens, nil
	}

	// Fall back to single-document Golden.
	var single Golden
	if err := yaml.Unmarshal(body, &single); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if single.ID == "" {
		return nil, errors.New("no goldens parsed and single-form has no id")
	}
	return []Golden{single}, nil
}
