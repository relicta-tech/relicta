package supplychain

import (
	"encoding/json"
	"strings"
)

// ParseGoModDiff parses dependency changes from old and new go.mod contents.
// It compares require blocks to identify added, removed, and changed dependencies.
func ParseGoModDiff(oldContent, newContent string) []DependencyChange {
	oldDeps := parseGoModRequires(oldContent)
	newDeps := parseGoModRequires(newContent)

	var changes []DependencyChange

	// Detect updated and removed dependencies.
	for name, oldVersion := range oldDeps {
		if newVersion, exists := newDeps[name]; exists {
			if oldVersion != newVersion {
				changes = append(changes, DependencyChange{
					Name:       name,
					Ecosystem:  "go",
					OldVersion: oldVersion,
					NewVersion: newVersion,
					ChangeType: classifyVersionChange(oldVersion, newVersion),
				})
			}
		} else {
			changes = append(changes, DependencyChange{
				Name:       name,
				Ecosystem:  "go",
				OldVersion: oldVersion,
				ChangeType: ChangeRemoved,
			})
		}
	}

	// Detect newly added dependencies.
	for name, newVersion := range newDeps {
		if _, exists := oldDeps[name]; !exists {
			changes = append(changes, DependencyChange{
				Name:       name,
				Ecosystem:  "go",
				NewVersion: newVersion,
				ChangeType: ChangeNew,
			})
		}
	}

	return changes
}

// parseGoModRequires extracts module name -> version mappings from go.mod content.
// It handles both single-line require directives and grouped require blocks.
func parseGoModRequires(content string) map[string]string {
	deps := make(map[string]string)
	if content == "" {
		return deps
	}

	lines := strings.Split(content, "\n")
	inRequireBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect require block boundaries.
		if strings.HasPrefix(trimmed, "require (") || strings.HasPrefix(trimmed, "require(") {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && trimmed == ")" {
			inRequireBlock = false
			continue
		}

		// Single-line require: require github.com/foo/bar v1.2.3
		if strings.HasPrefix(trimmed, "require ") && !strings.Contains(trimmed, "(") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 3 {
				deps[parts[1]] = parts[2]
			}
			continue
		}

		// Inside a require block: module version // indirect
		if inRequireBlock && trimmed != "" && !strings.HasPrefix(trimmed, "//") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				name := parts[0]
				version := parts[1]
				// Check for // indirect marker.
				_ = strings.Contains(trimmed, "// indirect") // tracked for future use
				deps[name] = version
			}
		}
	}

	return deps
}

// ParsePackageJSONDiff parses dependency changes from old and new package.json contents.
// It compares both "dependencies" and "devDependencies" sections.
func ParsePackageJSONDiff(oldContent, newContent string) []DependencyChange {
	oldDeps := parsePackageJSONDeps(oldContent)
	newDeps := parsePackageJSONDeps(newContent)

	var changes []DependencyChange

	// Detect updated and removed dependencies.
	for name, oldVersion := range oldDeps {
		if newVersion, exists := newDeps[name]; exists {
			if oldVersion != newVersion {
				changes = append(changes, DependencyChange{
					Name:       name,
					Ecosystem:  "npm",
					OldVersion: oldVersion,
					NewVersion: newVersion,
					ChangeType: classifyVersionChange(
						stripSemverPrefix(oldVersion),
						stripSemverPrefix(newVersion),
					),
				})
			}
		} else {
			changes = append(changes, DependencyChange{
				Name:       name,
				Ecosystem:  "npm",
				OldVersion: oldVersion,
				ChangeType: ChangeRemoved,
			})
		}
	}

	// Detect newly added dependencies.
	for name, newVersion := range newDeps {
		if _, exists := oldDeps[name]; !exists {
			changes = append(changes, DependencyChange{
				Name:       name,
				Ecosystem:  "npm",
				NewVersion: newVersion,
				ChangeType: ChangeNew,
			})
		}
	}

	return changes
}

// parsePackageJSONDeps extracts all dependencies from a package.json file.
// It merges "dependencies" and "devDependencies" into a single map.
func parsePackageJSONDeps(content string) map[string]string {
	deps := make(map[string]string)
	if content == "" {
		return deps
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return deps
	}

	for name, version := range pkg.Dependencies {
		deps[name] = version
	}
	for name, version := range pkg.DevDependencies {
		deps[name] = version
	}

	return deps
}

// classifyVersionChange determines the change type by comparing semver components.
func classifyVersionChange(oldVersion, newVersion string) ChangeType {
	oldParts := parseSemver(oldVersion)
	newParts := parseSemver(newVersion)

	if len(oldParts) < 3 || len(newParts) < 3 {
		// Cannot parse: assume minor change as a safe default.
		return ChangeMinor
	}

	if oldParts[0] != newParts[0] {
		return ChangeMajor
	}
	if oldParts[1] != newParts[1] {
		return ChangeMinor
	}
	return ChangePatch
}

// parseSemver splits a version string into its major, minor, patch components.
// It handles the "v" prefix common in Go modules.
func parseSemver(version string) []string {
	v := stripSemverPrefix(version)

	// Handle versions with pre-release or build metadata (e.g. "1.2.3-beta.1").
	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		v = v[:idx]
	}

	parts := strings.Split(v, ".")
	return parts
}

// stripSemverPrefix removes common version prefixes like "v", "^", "~", "=", ">=".
func stripSemverPrefix(version string) string {
	v := strings.TrimLeft(version, "v^~>=<! ")
	return v
}
