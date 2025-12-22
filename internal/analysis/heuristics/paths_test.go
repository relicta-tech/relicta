package heuristics

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/changes"
)

func TestPathDetector_Detect(t *testing.T) {
	detector := NewPathDetector()

	tests := []struct {
		name          string
		files         []string
		expectedType  changes.CommitType
		minConfidence float64
		shouldMatch   bool
	}{
		// Documentation files
		{
			name:          "README only",
			files:         []string{"README.md"},
			expectedType:  changes.CommitTypeDocs,
			minConfidence: 0.85,
			shouldMatch:   true,
		},
		{
			name:          "multiple docs",
			files:         []string{"README.md", "CHANGELOG.md", "CONTRIBUTING.md"},
			expectedType:  changes.CommitTypeDocs,
			minConfidence: 0.85,
			shouldMatch:   true,
		},
		{
			name:          "docs directory",
			files:         []string{"docs/guide.md", "docs/api.md"},
			expectedType:  changes.CommitTypeDocs,
			minConfidence: 0.80,
			shouldMatch:   true,
		},

		// Test files - Go
		{
			name:          "go test files",
			files:         []string{"user_test.go", "auth_test.go"},
			expectedType:  changes.CommitTypeTest,
			minConfidence: 0.85,
			shouldMatch:   true,
		},
		{
			name:          "single go test",
			files:         []string{"internal/auth/handler_test.go"},
			expectedType:  changes.CommitTypeTest,
			minConfidence: 0.85,
			shouldMatch:   true,
		},

		// Test files - TypeScript/JavaScript
		{
			name:          "ts test files",
			files:         []string{"user.test.ts", "auth.test.ts"},
			expectedType:  changes.CommitTypeTest,
			minConfidence: 0.85,
			shouldMatch:   true,
		},
		{
			name:          "spec files",
			files:         []string{"user.spec.js", "auth.spec.jsx"},
			expectedType:  changes.CommitTypeTest,
			minConfidence: 0.85,
			shouldMatch:   true,
		},
		{
			name:          "__tests__ directory",
			files:         []string{"__tests__/user.test.ts", "__tests__/auth.test.ts"},
			expectedType:  changes.CommitTypeTest,
			minConfidence: 0.90,
			shouldMatch:   true,
		},

		// Test files - Python
		{
			name:          "python test files",
			files:         []string{"test_user.py", "test_auth.py"},
			expectedType:  changes.CommitTypeTest,
			minConfidence: 0.85,
			shouldMatch:   true,
		},

		// Chore - dependencies
		{
			name:          "go.mod only",
			files:         []string{"go.mod"},
			expectedType:  changes.CommitTypeChore,
			minConfidence: 0.75,
			shouldMatch:   true,
		},
		{
			name:          "go mod and sum",
			files:         []string{"go.mod", "go.sum"},
			expectedType:  changes.CommitTypeChore,
			minConfidence: 0.80,
			shouldMatch:   true,
		},
		{
			name:          "package.json only",
			files:         []string{"package.json"},
			expectedType:  changes.CommitTypeChore,
			minConfidence: 0.70,
			shouldMatch:   true,
		},
		{
			name:          "npm lockfile",
			files:         []string{"package-lock.json"},
			expectedType:  changes.CommitTypeChore,
			minConfidence: 0.80,
			shouldMatch:   true,
		},
		{
			name:          "yarn lockfile",
			files:         []string{"yarn.lock"},
			expectedType:  changes.CommitTypeChore,
			minConfidence: 0.80,
			shouldMatch:   true,
		},
		{
			name:          "requirements.txt",
			files:         []string{"requirements.txt"},
			expectedType:  changes.CommitTypeChore,
			minConfidence: 0.75,
			shouldMatch:   true,
		},

		// Build files
		{
			name:          "Makefile",
			files:         []string{"Makefile"},
			expectedType:  changes.CommitTypeBuild,
			minConfidence: 0.80,
			shouldMatch:   true,
		},
		{
			name:          "Dockerfile",
			files:         []string{"Dockerfile"},
			expectedType:  changes.CommitTypeBuild,
			minConfidence: 0.75,
			shouldMatch:   true,
		},
		{
			name:          "docker-compose",
			files:         []string{"docker-compose.yml"},
			expectedType:  changes.CommitTypeBuild,
			minConfidence: 0.75,
			shouldMatch:   true,
		},
		{
			name:          "goreleaser",
			files:         []string{".goreleaser.yml"},
			expectedType:  changes.CommitTypeBuild,
			minConfidence: 0.80,
			shouldMatch:   true,
		},
		{
			name:          "webpack config",
			files:         []string{"webpack.config.js"},
			expectedType:  changes.CommitTypeBuild,
			minConfidence: 0.75,
			shouldMatch:   true,
		},
		{
			name:          "tsconfig",
			files:         []string{"tsconfig.json"},
			expectedType:  changes.CommitTypeBuild,
			minConfidence: 0.70,
			shouldMatch:   true,
		},

		// CI files
		{
			name:          "github workflow",
			files:         []string{".github/workflows/ci.yml"},
			expectedType:  changes.CommitTypeCI,
			minConfidence: 0.90,
			shouldMatch:   true,
		},
		{
			name:          "multiple github workflows",
			files:         []string{".github/workflows/ci.yml", ".github/workflows/release.yml"},
			expectedType:  changes.CommitTypeCI,
			minConfidence: 0.90,
			shouldMatch:   true,
		},
		{
			name:          "gitlab ci",
			files:         []string{".gitlab-ci.yml"},
			expectedType:  changes.CommitTypeCI,
			minConfidence: 0.90,
			shouldMatch:   true,
		},
		{
			name:          "travis",
			files:         []string{".travis.yml"},
			expectedType:  changes.CommitTypeCI,
			minConfidence: 0.90,
			shouldMatch:   true,
		},
		{
			name:          "dependabot config",
			files:         []string{".github/dependabot.yml"},
			expectedType:  changes.CommitTypeCI,
			minConfidence: 0.85,
			shouldMatch:   true,
		},
		{
			name:          "renovate",
			files:         []string{"renovate.json"},
			expectedType:  changes.CommitTypeCI,
			minConfidence: 0.80,
			shouldMatch:   true,
		},

		// Style files
		{
			name:          "eslint config",
			files:         []string{".eslintrc.js"},
			expectedType:  changes.CommitTypeStyle,
			minConfidence: 0.75,
			shouldMatch:   true,
		},
		{
			name:          "prettier config",
			files:         []string{".prettierrc"},
			expectedType:  changes.CommitTypeStyle,
			minConfidence: 0.75,
			shouldMatch:   true,
		},
		{
			name:          "golangci-lint",
			files:         []string{".golangci.yml"},
			expectedType:  changes.CommitTypeStyle,
			minConfidence: 0.80,
			shouldMatch:   true,
		},

		// Mixed files - should detect dominant type
		{
			name:          "mostly tests",
			files:         []string{"user_test.go", "auth_test.go", "go.mod"},
			expectedType:  changes.CommitTypeTest,
			minConfidence: 0.50,
			shouldMatch:   true,
		},
		{
			name:          "mostly docs",
			files:         []string{"README.md", "CHANGELOG.md", "main.go"},
			expectedType:  changes.CommitTypeDocs,
			minConfidence: 0.50,
			shouldMatch:   true,
		},

		// Edge cases
		{
			name:        "empty files",
			files:       []string{},
			shouldMatch: false,
		},
		{
			name:        "source files only",
			files:       []string{"main.go", "handler.go"},
			shouldMatch: false, // No specific pattern match
		},
		{
			name:        "mixed source and random",
			files:       []string{"main.go", "utils.ts", "index.py"},
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.Detect(tt.files)

			if tt.shouldMatch {
				if result == nil {
					t.Errorf("expected match but got nil for files: %v", tt.files)
					return
				}

				if result.Type != tt.expectedType {
					t.Errorf("expected type %s, got %s for files: %v",
						tt.expectedType, result.Type, tt.files)
				}

				if result.Confidence < tt.minConfidence {
					t.Errorf("expected confidence >= %.2f, got %.2f for files: %v",
						tt.minConfidence, result.Confidence, tt.files)
				}
			} else {
				if result != nil {
					t.Errorf("expected no match but got %s (%.2f) for files: %v",
						result.Type, result.Confidence, tt.files)
				}
			}
		})
	}
}

func TestPathDetector_InferScope(t *testing.T) {
	detector := NewPathDetector()

	tests := []struct {
		name     string
		files    []string
		expected string
	}{
		{
			name:     "internal package",
			files:    []string{"internal/auth/handler.go", "internal/auth/service.go"},
			expected: "auth",
		},
		{
			name:     "pkg directory",
			files:    []string{"pkg/config/config.go"},
			expected: "config",
		},
		{
			name:     "cmd directory",
			files:    []string{"cmd/server/main.go"},
			expected: "server",
		},
		{
			name:     "src directory",
			files:    []string{"src/components/Button.tsx"},
			expected: "components",
		},
		{
			name:     "github ci",
			files:    []string{".github/workflows/ci.yml"},
			expected: "ci",
		},
		{
			name:     "docs directory",
			files:    []string{"docs/api.md"},
			expected: "docs",
		},
		{
			name:     "tests directory",
			files:    []string{"tests/unit/auth_test.go"},
			expected: "test",
		},
		{
			name:     "empty files",
			files:    []string{},
			expected: "",
		},
		{
			name:     "root level files",
			files:    []string{"main.go"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.InferScope(tt.files)
			if result != tt.expected {
				t.Errorf("InferScope(%v) = %q, want %q",
					tt.files, result, tt.expected)
			}
		})
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		file     string
		pattern  string
		expected bool
	}{
		// Simple patterns
		{"README.md", "*.md", true},
		{"main.go", "*.go", true},
		{"main.go", "*.js", false},

		// Exact matches
		{"Makefile", "Makefile", true},
		{"Dockerfile", "Dockerfile", true},
		{"Dockerfile", "Makefile", false},

		// Wildcard patterns
		{"webpack.config.js", "webpack.config.*", true},
		{"vite.config.ts", "vite.config.*", true},
		{"eslint.config.js", "eslint.config.*", true},

		// Path patterns
		{"internal/auth/handler.go", "internal/**/*.go", true},
		{"pkg/config/config.go", "pkg/**/*.go", true},
		{"src/components/Button.tsx", "src/**/*", true},

		// GitHub workflows
		{".github/workflows/ci.yml", ".github/workflows/*.yml", true},
		{".github/workflows/release.yaml", ".github/workflows/*.yaml", true},

		// Test patterns
		{"user_test.go", "*_test.go", true},
		{"handler_test.go", "*_test.go", true},
		{"user.test.ts", "*.test.ts", true},

		// Nested test patterns
		{"__tests__/user.test.ts", "__tests__/**/*", true},
		{"tests/unit/auth_test.go", "tests/**/*", true},

		// Docs patterns
		{"docs/guide.md", "docs/**/*", true},
		{"docs/api/reference.md", "docs/**/*", true},
	}

	for _, tt := range tests {
		t.Run(tt.file+"_"+tt.pattern, func(t *testing.T) {
			result := matchGlob(tt.file, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchGlob(%q, %q) = %v, want %v",
					tt.file, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestMatchDoubleGlob(t *testing.T) {
	tests := []struct {
		file     string
		pattern  string
		expected bool
	}{
		{"internal/auth/handler.go", "internal/**/*.go", true},
		{"internal/handler.go", "internal/**/*.go", true},
		{"pkg/config/config.go", "pkg/**/*.go", true},
		{"src/deep/nested/component.tsx", "src/**/*.tsx", true},
		{"docs/api/v1/reference.md", "docs/**/*.md", true},

		// Non-matching
		{"external/auth/handler.go", "internal/**/*.go", false},
		{"internal/auth/handler.ts", "internal/**/*.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.file+"_"+tt.pattern, func(t *testing.T) {
			result := matchDoubleGlob(tt.file, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchDoubleGlob(%q, %q) = %v, want %v",
					tt.file, tt.pattern, result, tt.expected)
			}
		})
	}
}
