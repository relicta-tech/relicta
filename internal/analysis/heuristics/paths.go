package heuristics

import (
	"path/filepath"
	"strings"

	"github.com/relicta-tech/relicta/internal/domain/changes"
)

// PathDetector detects commit types from file paths.
type PathDetector struct {
	patterns map[changes.CommitType][]pathPattern
}

// pathPattern represents a file path matching pattern.
type pathPattern struct {
	// glob is the glob pattern to match.
	glob string

	// confidence is the base confidence when this pattern matches.
	confidence float64

	// scope is the inferred scope when this pattern matches.
	scope string
}

// NewPathDetector creates a new path detector.
func NewPathDetector() *PathDetector {
	return &PathDetector{
		patterns: initPathPatterns(),
	}
}

// Detect attempts to classify commits based on file paths.
func (d *PathDetector) Detect(files []string) *DetectionResult {
	if len(files) == 0 {
		return nil
	}

	// Count matches for each type
	typeMatches := make(map[changes.CommitType]int)
	typeConfidence := make(map[changes.CommitType]float64)
	inferredScope := ""

	for _, file := range files {
		for commitType, patterns := range d.patterns {
			for _, p := range patterns {
				if matchGlob(file, p.glob) {
					typeMatches[commitType]++
					if p.confidence > typeConfidence[commitType] {
						typeConfidence[commitType] = p.confidence
					}
					if p.scope != "" && inferredScope == "" {
						inferredScope = p.scope
					}
				}
			}
		}
	}

	// Find the dominant type
	if len(typeMatches) == 0 {
		return nil
	}

	// If all files match the same type, high confidence
	if len(typeMatches) == 1 {
		for commitType, count := range typeMatches {
			// All files matched this type
			if count == len(files) {
				return &DetectionResult{
					Type:       commitType,
					Scope:      inferredScope,
					Confidence: typeConfidence[commitType],
					Reasoning:  "all files match pattern for " + string(commitType),
				}
			}
		}
	}

	// Find the type with most matches, breaking ties by confidence
	var bestType changes.CommitType
	var maxMatches int
	var bestConfidence float64
	for commitType, count := range typeMatches {
		conf := typeConfidence[commitType]
		if count > maxMatches || (count == maxMatches && conf > bestConfidence) {
			maxMatches = count
			bestType = commitType
			bestConfidence = conf
		}
	}

	// Calculate confidence based on proportion of matching files
	proportion := float64(maxMatches) / float64(len(files))
	confidence := typeConfidence[bestType] * proportion

	// Only return if majority of files match
	if proportion >= 0.5 && confidence >= 0.5 {
		return &DetectionResult{
			Type:       bestType,
			Scope:      inferredScope,
			Confidence: confidence,
			Reasoning:  "majority of files match pattern for " + string(bestType),
		}
	}

	return nil
}

// InferScope attempts to infer a scope from file paths.
func (d *PathDetector) InferScope(files []string) string {
	if len(files) == 0 {
		return ""
	}

	// Look for common directory patterns
	scopeCounts := make(map[string]int)

	for _, file := range files {
		// Skip vendor/node_modules
		if strings.HasPrefix(file, "vendor/") || strings.HasPrefix(file, "node_modules/") {
			continue
		}

		parts := strings.Split(file, "/")
		if len(parts) >= 2 {
			// Use first meaningful directory as scope candidate
			dir := parts[0]

			// Skip common top-level dirs that aren't scopes
			switch dir {
			case "src", "lib", "pkg", "internal", "cmd", "app":
				if len(parts) >= 3 {
					dir = parts[1]
				}
			case ".github", ".gitlab", ".circleci":
				dir = "ci"
			case "docs", "doc", "documentation":
				dir = "docs"
			case "test", "tests", "spec", "specs", "__tests__":
				dir = "test"
			}

			scopeCounts[dir]++
		}
	}

	// Find most common scope
	var bestScope string
	var maxCount int
	for scope, count := range scopeCounts {
		if count > maxCount {
			maxCount = count
			bestScope = scope
		}
	}

	return bestScope
}

// matchGlob checks if a file path matches a glob pattern.
func matchGlob(file, pattern string) bool {
	// Normalize paths
	file = filepath.ToSlash(file)
	pattern = filepath.ToSlash(pattern)

	// Handle ** (any directory depth)
	if strings.Contains(pattern, "**") {
		return matchDoubleGlob(file, pattern)
	}

	// Use standard filepath.Match for simple patterns
	matched, err := filepath.Match(pattern, file)
	if err != nil {
		return false
	}
	if matched {
		return true
	}

	// Also try matching just the filename
	base := filepath.Base(file)
	matched, err = filepath.Match(pattern, base)
	return err == nil && matched
}

// matchDoubleGlob handles ** patterns.
func matchDoubleGlob(file, pattern string) bool {
	// Split pattern by **
	parts := strings.Split(pattern, "**")

	if len(parts) == 2 {
		prefix := parts[0]
		suffix := parts[1]

		// Remove leading slash from suffix
		suffix = strings.TrimPrefix(suffix, "/")

		// Check prefix
		if prefix != "" && !strings.HasPrefix(file, prefix) {
			return false
		}

		// Check suffix
		if suffix != "" {
			// Check if any part of the path matches the suffix
			remaining := strings.TrimPrefix(file, prefix)
			pathParts := strings.Split(remaining, "/")

			for i := range pathParts {
				testPath := strings.Join(pathParts[i:], "/")
				if matched, _ := filepath.Match(suffix, testPath); matched {
					return true
				}
				// Also try just the filename
				if i == len(pathParts)-1 {
					if matched, _ := filepath.Match(suffix, pathParts[i]); matched {
						return true
					}
				}
			}
			return false
		}

		return true
	}

	return false
}

// initPathPatterns initializes the file path patterns for each commit type.
func initPathPatterns() map[changes.CommitType][]pathPattern {
	return map[changes.CommitType][]pathPattern{
		changes.CommitTypeDocs: {
			// Documentation files
			{glob: "*.md", confidence: 0.85},
			{glob: "README*", confidence: 0.90},
			{glob: "CHANGELOG*", confidence: 0.90},
			{glob: "LICENSE*", confidence: 0.85},
			{glob: "CONTRIBUTING*", confidence: 0.88},
			{glob: "AUTHORS*", confidence: 0.85},
			{glob: "*.txt", confidence: 0.60},
			{glob: "*.rst", confidence: 0.85},
			{glob: "*.adoc", confidence: 0.85},

			// Documentation directories
			{glob: "docs/**/*", confidence: 0.88, scope: "docs"},
			{glob: "doc/**/*", confidence: 0.88, scope: "docs"},
			{glob: "documentation/**/*", confidence: 0.88, scope: "docs"},
			{glob: "wiki/**/*", confidence: 0.85, scope: "docs"},
		},

		changes.CommitTypeTest: {
			// Go tests
			{glob: "*_test.go", confidence: 0.92, scope: "test"},
			{glob: "**/*_test.go", confidence: 0.92, scope: "test"},

			// JavaScript/TypeScript tests
			{glob: "*.test.ts", confidence: 0.92, scope: "test"},
			{glob: "*.test.tsx", confidence: 0.92, scope: "test"},
			{glob: "*.test.js", confidence: 0.92, scope: "test"},
			{glob: "*.test.jsx", confidence: 0.92, scope: "test"},
			{glob: "*.spec.ts", confidence: 0.92, scope: "test"},
			{glob: "*.spec.tsx", confidence: 0.92, scope: "test"},
			{glob: "*.spec.js", confidence: 0.92, scope: "test"},
			{glob: "*.spec.jsx", confidence: 0.92, scope: "test"},
			{glob: "__tests__/**/*", confidence: 0.95, scope: "test"},

			// Python tests
			{glob: "test_*.py", confidence: 0.92, scope: "test"},
			{glob: "*_test.py", confidence: 0.92, scope: "test"},

			// Test directories
			{glob: "tests/**/*", confidence: 0.90, scope: "test"},
			{glob: "test/**/*", confidence: 0.90, scope: "test"},
			{glob: "spec/**/*", confidence: 0.90, scope: "test"},
			{glob: "specs/**/*", confidence: 0.90, scope: "test"},

			// Test fixtures and data
			{glob: "testdata/**/*", confidence: 0.85, scope: "test"},
			{glob: "fixtures/**/*", confidence: 0.85, scope: "test"},
			{glob: "__fixtures__/**/*", confidence: 0.85, scope: "test"},
			{glob: "__mocks__/**/*", confidence: 0.85, scope: "test"},
		},

		changes.CommitTypeChore: {
			// Dependency files
			{glob: "go.mod", confidence: 0.80, scope: "deps"},
			{glob: "go.sum", confidence: 0.85, scope: "deps"},
			{glob: "package.json", confidence: 0.75, scope: "deps"},
			{glob: "package-lock.json", confidence: 0.85, scope: "deps"},
			{glob: "yarn.lock", confidence: 0.85, scope: "deps"},
			{glob: "pnpm-lock.yaml", confidence: 0.85, scope: "deps"},
			{glob: "Cargo.toml", confidence: 0.75, scope: "deps"},
			{glob: "Cargo.lock", confidence: 0.85, scope: "deps"},
			{glob: "requirements.txt", confidence: 0.80, scope: "deps"},
			{glob: "poetry.lock", confidence: 0.85, scope: "deps"},
			{glob: "Pipfile.lock", confidence: 0.85, scope: "deps"},
			{glob: "composer.lock", confidence: 0.85, scope: "deps"},
			{glob: "Gemfile.lock", confidence: 0.85, scope: "deps"},

			// Configuration files
			{glob: ".editorconfig", confidence: 0.80},
			{glob: ".gitignore", confidence: 0.80},
			{glob: ".gitattributes", confidence: 0.80},
			{glob: ".npmrc", confidence: 0.80},
			{glob: ".nvmrc", confidence: 0.80},
			{glob: ".tool-versions", confidence: 0.80},
			{glob: ".envrc", confidence: 0.75},
		},

		changes.CommitTypeBuild: {
			// Build files
			{glob: "Makefile", confidence: 0.85, scope: "build"},
			{glob: "makefile", confidence: 0.85, scope: "build"},
			{glob: "GNUmakefile", confidence: 0.85, scope: "build"},
			{glob: "CMakeLists.txt", confidence: 0.85, scope: "build"},
			{glob: "BUILD", confidence: 0.80, scope: "build"},
			{glob: "BUILD.bazel", confidence: 0.85, scope: "build"},
			{glob: "WORKSPACE", confidence: 0.85, scope: "build"},

			// Docker
			{glob: "Dockerfile", confidence: 0.80, scope: "docker"},
			{glob: "Dockerfile.*", confidence: 0.80, scope: "docker"},
			{glob: "*.dockerfile", confidence: 0.80, scope: "docker"},
			{glob: "docker-compose*.yml", confidence: 0.80, scope: "docker"},
			{glob: "docker-compose*.yaml", confidence: 0.80, scope: "docker"},
			{glob: ".dockerignore", confidence: 0.80, scope: "docker"},

			// JS build tools
			{glob: "webpack.config.*", confidence: 0.82, scope: "build"},
			{glob: "vite.config.*", confidence: 0.82, scope: "build"},
			{glob: "rollup.config.*", confidence: 0.82, scope: "build"},
			{glob: "esbuild.config.*", confidence: 0.82, scope: "build"},
			{glob: "tsconfig*.json", confidence: 0.75, scope: "build"},
			{glob: "babel.config.*", confidence: 0.80, scope: "build"},

			// Go build
			{glob: ".goreleaser.yml", confidence: 0.85, scope: "build"},
			{glob: ".goreleaser.yaml", confidence: 0.85, scope: "build"},
			{glob: "goreleaser.yml", confidence: 0.85, scope: "build"},
			{glob: "goreleaser.yaml", confidence: 0.85, scope: "build"},
		},

		changes.CommitTypeCI: {
			// GitHub Actions
			{glob: ".github/workflows/*.yml", confidence: 0.92, scope: "ci"},
			{glob: ".github/workflows/*.yaml", confidence: 0.92, scope: "ci"},
			{glob: ".github/dependabot.yml", confidence: 0.88, scope: "ci"},
			{glob: ".github/dependabot.yaml", confidence: 0.88, scope: "ci"},

			// GitLab CI
			{glob: ".gitlab-ci.yml", confidence: 0.92, scope: "ci"},
			{glob: ".gitlab-ci.yaml", confidence: 0.92, scope: "ci"},

			// Other CI
			{glob: ".travis.yml", confidence: 0.92, scope: "ci"},
			{glob: ".circleci/**/*", confidence: 0.92, scope: "ci"},
			{glob: "Jenkinsfile", confidence: 0.90, scope: "ci"},
			{glob: "azure-pipelines.yml", confidence: 0.90, scope: "ci"},
			{glob: ".drone.yml", confidence: 0.90, scope: "ci"},
			{glob: "bitbucket-pipelines.yml", confidence: 0.90, scope: "ci"},

			// Renovate/Dependabot
			{glob: "renovate.json", confidence: 0.85, scope: "ci"},
			{glob: "renovate.json5", confidence: 0.85, scope: "ci"},
			{glob: ".renovaterc", confidence: 0.85, scope: "ci"},
			{glob: ".renovaterc.json", confidence: 0.85, scope: "ci"},
		},

		changes.CommitTypeStyle: {
			// Linter configs
			{glob: ".eslintrc*", confidence: 0.82, scope: "lint"},
			{glob: ".prettierrc*", confidence: 0.82, scope: "lint"},
			{glob: ".stylelintrc*", confidence: 0.82, scope: "lint"},
			{glob: "eslint.config.*", confidence: 0.82, scope: "lint"},
			{glob: "prettier.config.*", confidence: 0.82, scope: "lint"},
			{glob: ".golangci.yml", confidence: 0.85, scope: "lint"},
			{glob: ".golangci.yaml", confidence: 0.85, scope: "lint"},
			{glob: "pylintrc", confidence: 0.82, scope: "lint"},
			{glob: ".pylintrc", confidence: 0.82, scope: "lint"},
			{glob: "pyproject.toml", confidence: 0.60, scope: "config"}, // Lower, multi-purpose
			{glob: ".flake8", confidence: 0.82, scope: "lint"},
			{glob: "rustfmt.toml", confidence: 0.85, scope: "lint"},
			{glob: ".rubocop.yml", confidence: 0.85, scope: "lint"},
		},
	}
}
