// Package blast provides blast radius analysis for monorepos.
package blast

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCategorizeFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected FileCategory
	}{
		// Test files
		{
			name:     "Go test file",
			path:     "pkg/service/handler_test.go",
			expected: FileCategoryTest,
		},
		{
			name:     "TypeScript test file",
			path:     "src/components/Button.test.tsx",
			expected: FileCategoryTest,
		},
		{
			name:     "JavaScript spec file",
			path:     "src/utils/helpers.spec.js",
			expected: FileCategoryTest,
		},
		{
			name:     "Python test file",
			path:     "tests/test_service.py",
			expected: FileCategoryTest,
		},
		{
			name:     "Jest test directory",
			path:     "src/__tests__/Button.tsx",
			expected: FileCategoryTest,
		},

		// Documentation
		{
			name:     "Markdown file",
			path:     "README.md",
			expected: FileCategoryDocs,
		},
		{
			name:     "Docs directory",
			path:     "docs/api/endpoints.md",
			expected: FileCategoryDocs,
		},
		{
			name:     "RST file",
			path:     "docs/guide.rst",
			expected: FileCategoryDocs,
		},

		// CI/CD
		{
			name:     "GitHub workflow",
			path:     ".github/workflows/ci.yaml",
			expected: FileCategoryCI,
		},
		{
			name:     "GitLab CI",
			path:     ".gitlab-ci.yml",
			expected: FileCategoryCI,
		},
		{
			name:     "CircleCI",
			path:     ".circleci/config.yml",
			expected: FileCategoryCI,
		},

		// Build files
		{
			name:     "Makefile",
			path:     "Makefile",
			expected: FileCategoryBuild,
		},
		{
			name:     "Dockerfile",
			path:     "Dockerfile",
			expected: FileCategoryBuild,
		},
		{
			name:     "GoReleaser",
			path:     ".goreleaser.yaml",
			expected: FileCategoryBuild,
		},

		// Dependencies
		{
			name:     "package.json",
			path:     "package.json",
			expected: FileCategoryDependency,
		},
		{
			name:     "go.mod",
			path:     "go.mod",
			expected: FileCategoryDependency,
		},
		{
			name:     "go.sum",
			path:     "go.sum",
			expected: FileCategoryDependency,
		},
		{
			name:     "requirements.txt",
			path:     "requirements.txt",
			expected: FileCategoryDependency,
		},
		{
			name:     "Cargo.toml",
			path:     "Cargo.toml",
			expected: FileCategoryDependency,
		},

		// Config files
		{
			name:     "YAML config",
			path:     "config/app.yaml",
			expected: FileCategoryConfig,
		},
		{
			name:     "ESLint config",
			path:     ".eslintrc",
			expected: FileCategoryConfig,
		},
		{
			name:     "TypeScript config",
			path:     "tsconfig.json",
			expected: FileCategoryConfig,
		},

		// Generated files
		{
			name:     "Proto generated Go",
			path:     "api/v1/service.pb.go",
			expected: FileCategoryGenerated,
		},
		{
			name:     "Generated directory",
			path:     "generated/models/user.go",
			expected: FileCategoryGenerated,
		},

		// Assets
		{
			name:     "PNG image",
			path:     "assets/logo.png",
			expected: FileCategoryAsset,
		},
		{
			name:     "SVG icon",
			path:     "public/icons/menu.svg",
			expected: FileCategoryAsset,
		},
		{
			name:     "Font file",
			path:     "fonts/roboto.woff2",
			expected: FileCategoryAsset,
		},

		// Source code
		{
			name:     "Go source",
			path:     "pkg/service/handler.go",
			expected: FileCategorySource,
		},
		{
			name:     "TypeScript source",
			path:     "src/components/Button.tsx",
			expected: FileCategorySource,
		},
		{
			name:     "Python source",
			path:     "app/services/user_service.py",
			expected: FileCategorySource,
		},
		{
			name:     "Rust source",
			path:     "src/main.rs",
			expected: FileCategorySource,
		},

		// Other
		{
			name:     "License file",
			path:     "LICENSE",
			expected: FileCategoryOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := categorizeFile(tt.path)
			if result != tt.expected {
				t.Errorf("categorizeFile(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsFileInPackage(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		packagePath string
		expected    bool
	}{
		{
			name:        "file in package root",
			filePath:    "packages/api/main.go",
			packagePath: "packages/api",
			expected:    true,
		},
		{
			name:        "file in package subdir",
			filePath:    "packages/api/handlers/user.go",
			packagePath: "packages/api",
			expected:    true,
		},
		{
			name:        "file not in package",
			filePath:    "packages/web/main.go",
			packagePath: "packages/api",
			expected:    false,
		},
		{
			name:        "root package",
			filePath:    "main.go",
			packagePath: ".",
			expected:    true,
		},
		{
			name:        "similar prefix but different package",
			filePath:    "packages/api-gateway/main.go",
			packagePath: "packages/api",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isFileInPackage(tt.filePath, tt.packagePath)
			if result != tt.expected {
				t.Errorf("isFileInPackage(%q, %q) = %v, want %v",
					tt.filePath, tt.packagePath, result, tt.expected)
			}
		})
	}
}

func TestCalculateRiskScore(t *testing.T) {
	svc := NewService().(*serviceImpl)

	tests := []struct {
		name     string
		impact   *Impact
		minScore int
		maxScore int
	}{
		{
			name: "no impact",
			impact: &Impact{
				Level: ImpactLevelNone,
			},
			minScore: 0,
			maxScore: 0,
		},
		{
			name: "direct impact with source changes",
			impact: &Impact{
				Level: ImpactLevelDirect,
				DirectChanges: []ChangedFile{
					{Path: "main.go", Category: FileCategorySource, Insertions: 50, Deletions: 10},
				},
			},
			minScore: 45, // Base 30 + source 15
			maxScore: 60,
		},
		{
			name: "transitive impact",
			impact: &Impact{
				Level:           ImpactLevelTransitive,
				TransitiveDepth: 2,
			},
			minScore: 15, // Base 15 + depth 10
			maxScore: 30,
		},
		{
			name: "high change volume",
			impact: &Impact{
				Level: ImpactLevelDirect,
				DirectChanges: []ChangedFile{
					{Path: "main.go", Category: FileCategorySource, Insertions: 500, Deletions: 200},
				},
			},
			minScore: 50,
			maxScore: 70,
		},
		{
			name: "config changes",
			impact: &Impact{
				Level: ImpactLevelDirect,
				DirectChanges: []ChangedFile{
					{Path: "config.yaml", Category: FileCategoryConfig, Insertions: 5, Deletions: 2},
				},
			},
			minScore: 35, // Base 30 + config 10
			maxScore: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := svc.CalculateRiskScore(tt.impact)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("CalculateRiskScore() = %d, want between %d and %d",
					score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestSuggestReleaseType(t *testing.T) {
	svc := NewService().(*serviceImpl)

	tests := []struct {
		name     string
		impact   *Impact
		expected string
	}{
		{
			name: "no impact",
			impact: &Impact{
				Level: ImpactLevelNone,
			},
			expected: "",
		},
		{
			name: "source changes - minor",
			impact: &Impact{
				Level: ImpactLevelDirect,
				DirectChanges: []ChangedFile{
					{Path: "main.go", Category: FileCategorySource, Insertions: 200, Deletions: 50},
				},
			},
			expected: "minor",
		},
		{
			name: "small source changes - patch",
			impact: &Impact{
				Level: ImpactLevelDirect,
				DirectChanges: []ChangedFile{
					{Path: "main.go", Category: FileCategorySource, Insertions: 10, Deletions: 5},
				},
			},
			expected: "patch",
		},
		{
			name: "transitive - patch",
			impact: &Impact{
				Level: ImpactLevelTransitive,
			},
			expected: "patch",
		},
		{
			name: "breaking change path",
			impact: &Impact{
				Level: ImpactLevelDirect,
				DirectChanges: []ChangedFile{
					{Path: "migrations/breaking_change_v2.sql", Category: FileCategorySource, Insertions: 10, Deletions: 0},
				},
			},
			expected: "major",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.SuggestReleaseType(tt.impact)
			if result != tt.expected {
				t.Errorf("SuggestReleaseType() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDefaultMonorepoConfig(t *testing.T) {
	config := DefaultMonorepoConfig()

	if len(config.PackagePaths) == 0 {
		t.Error("PackagePaths should have default values")
	}

	if len(config.ExcludePaths) == 0 {
		t.Error("ExcludePaths should have default values")
	}

	// Check expected paths are present
	expectedPaths := []string{"packages/*", "plugins/*"}
	for _, expected := range expectedPaths {
		found := false
		for _, path := range config.PackagePaths {
			if path == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected %q in PackagePaths", expected)
		}
	}

	// Check expected excludes are present
	expectedExcludes := []string{"node_modules", "vendor", ".git"}
	for _, expected := range expectedExcludes {
		found := false
		for _, path := range config.ExcludePaths {
			if path == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected %q in ExcludePaths", expected)
		}
	}
}

func TestDefaultAnalysisOptions(t *testing.T) {
	opts := DefaultAnalysisOptions()

	if opts.ToRef != "HEAD" {
		t.Errorf("ToRef = %v, want HEAD", opts.ToRef)
	}

	if !opts.IncludeTransitive {
		t.Error("IncludeTransitive should be true by default")
	}

	if !opts.CalculateRisk {
		t.Error("CalculateRisk should be true by default")
	}

	if opts.GenerateGraph {
		t.Error("GenerateGraph should be false by default")
	}

	if opts.MonorepoConfig == nil {
		t.Error("MonorepoConfig should not be nil")
	}
}

func TestBuildSummary(t *testing.T) {
	svc := NewService().(*serviceImpl)

	packages := []*Package{
		{Name: "pkg1", Path: "packages/pkg1", Type: PackageTypeGoModule},
		{Name: "pkg2", Path: "packages/pkg2", Type: PackageTypeNPM},
		{Name: "pkg3", Path: "packages/pkg3", Type: PackageTypeGoModule},
	}

	impacts := []*Impact{
		{
			Package:         packages[0],
			Level:           ImpactLevelDirect,
			RiskScore:       75,
			RequiresRelease: true,
		},
		{
			Package:         packages[1],
			Level:           ImpactLevelTransitive,
			RiskScore:       30,
			RequiresRelease: true,
		},
	}

	changedFiles := []ChangedFile{
		{Path: "main.go", Category: FileCategorySource, Insertions: 100, Deletions: 50},
		{Path: "config.yaml", Category: FileCategoryConfig, Insertions: 10, Deletions: 5},
	}

	summary := svc.buildSummary(packages, impacts, changedFiles)

	if summary.TotalPackages != 3 {
		t.Errorf("TotalPackages = %d, want 3", summary.TotalPackages)
	}

	if summary.DirectlyAffected != 1 {
		t.Errorf("DirectlyAffected = %d, want 1", summary.DirectlyAffected)
	}

	if summary.TransitivelyAffected != 1 {
		t.Errorf("TransitivelyAffected = %d, want 1", summary.TransitivelyAffected)
	}

	if summary.TotalAffected != 2 {
		t.Errorf("TotalAffected = %d, want 2", summary.TotalAffected)
	}

	if summary.HighRiskCount != 1 {
		t.Errorf("HighRiskCount = %d, want 1", summary.HighRiskCount)
	}

	if summary.PackagesRequiringRelease != 2 {
		t.Errorf("PackagesRequiringRelease = %d, want 2", summary.PackagesRequiringRelease)
	}

	if summary.TotalFilesChanged != 2 {
		t.Errorf("TotalFilesChanged = %d, want 2", summary.TotalFilesChanged)
	}

	if summary.TotalInsertions != 110 {
		t.Errorf("TotalInsertions = %d, want 110", summary.TotalInsertions)
	}

	if summary.TotalDeletions != 55 {
		t.Errorf("TotalDeletions = %d, want 55", summary.TotalDeletions)
	}
}

func TestDetermineRiskLevel(t *testing.T) {
	svc := NewService().(*serviceImpl)

	tests := []struct {
		name     string
		summary  *Summary
		impacts  []*Impact
		expected RiskLevel
	}{
		{
			name: "low risk - few changes",
			summary: &Summary{
				TotalPackages:    10,
				TotalAffected:    1,
				HighRiskCount:    0,
				DirectlyAffected: 1,
			},
			impacts:  []*Impact{{RiskScore: 20}},
			expected: RiskLevelLow,
		},
		{
			name: "medium risk - multiple packages",
			summary: &Summary{
				TotalPackages:    10,
				TotalAffected:    4,
				HighRiskCount:    0,
				DirectlyAffected: 4,
			},
			impacts:  []*Impact{{RiskScore: 40}, {RiskScore: 35}, {RiskScore: 30}, {RiskScore: 25}},
			expected: RiskLevelMedium,
		},
		{
			name: "high risk - high percentage affected",
			summary: &Summary{
				TotalPackages:    10,
				TotalAffected:    6,
				HighRiskCount:    1,
				DirectlyAffected: 6,
			},
			impacts:  []*Impact{{RiskScore: 80}},
			expected: RiskLevelHigh,
		},
		{
			name: "critical risk - many high risk",
			summary: &Summary{
				TotalPackages:    10,
				TotalAffected:    5,
				HighRiskCount:    3,
				DirectlyAffected: 5,
			},
			impacts:  []*Impact{{RiskScore: 75}, {RiskScore: 80}, {RiskScore: 85}},
			expected: RiskLevelCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.determineRiskLevel(tt.summary, tt.impacts)
			if result != tt.expected {
				t.Errorf("determineRiskLevel() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFilterByCategory(t *testing.T) {
	files := []ChangedFile{
		{Path: "main.go", Category: FileCategorySource},
		{Path: "main_test.go", Category: FileCategoryTest},
		{Path: "README.md", Category: FileCategoryDocs},
		{Path: "config.yaml", Category: FileCategoryConfig},
	}

	// Include only source
	sourceOnly := filterByCategory(files, FileCategorySource, true)
	if len(sourceOnly) != 1 {
		t.Errorf("filterByCategory(include source) = %d files, want 1", len(sourceOnly))
	}

	// Exclude tests
	noTests := filterByCategory(files, FileCategoryTest, false)
	if len(noTests) != 3 {
		t.Errorf("filterByCategory(exclude tests) = %d files, want 3", len(noTests))
	}
}

func TestNewService(t *testing.T) {
	// Default service
	svc := NewService()
	if svc == nil {
		t.Error("NewService() returned nil")
	}

	// Service with options
	svc = NewService(
		WithRepoPath("/custom/path"),
		WithMonorepoConfig(&MonorepoConfig{RootPackage: true}),
	)
	if svc == nil {
		t.Error("NewService(opts) returned nil")
	}

	impl := svc.(*serviceImpl)
	if impl.config.RepoPath != "/custom/path" {
		t.Errorf("RepoPath = %v, want /custom/path", impl.config.RepoPath)
	}
	if !impl.config.MonorepoConfig.RootPackage {
		t.Error("RootPackage should be true")
	}
}

func TestDiscoverPackages(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir, err := os.MkdirTemp("", "blast-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Go module
	goModDir := filepath.Join(tmpDir, "packages", "api")
	if err := os.MkdirAll(goModDir, 0755); err != nil {
		t.Fatalf("Failed to create go mod dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goModDir, "go.mod"), []byte("module example.com/api\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create npm package
	npmDir := filepath.Join(tmpDir, "packages", "web")
	if err := os.MkdirAll(npmDir, 0755); err != nil {
		t.Fatalf("Failed to create npm dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(npmDir, "package.json"), []byte(`{"name": "web", "version": "1.0.0"}`), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	svc := NewService(WithRepoPath(tmpDir))
	packages, err := svc.DiscoverPackages(context.Background(), nil)
	if err != nil {
		t.Fatalf("DiscoverPackages() error = %v", err)
	}

	if len(packages) < 2 {
		t.Errorf("DiscoverPackages() found %d packages, want at least 2", len(packages))
	}

	// Verify package types
	foundGo := false
	foundNPM := false
	for _, pkg := range packages {
		if pkg.Type == PackageTypeGoModule {
			foundGo = true
		}
		if pkg.Type == PackageTypeNPM {
			foundNPM = true
		}
	}

	if !foundGo {
		t.Error("Expected to find a Go module package")
	}
	if !foundNPM {
		t.Error("Expected to find an NPM package")
	}
}

func TestFormatBlastRadius(t *testing.T) {
	br := &BlastRadius{
		FromRef: "v1.0.0",
		ToRef:   "HEAD",
		Packages: []*Package{
			{Name: "api", Path: "packages/api", Type: PackageTypeGoModule},
		},
		Impacts: []*Impact{
			{
				Package:   &Package{Name: "api", Path: "packages/api", Type: PackageTypeGoModule},
				Level:     ImpactLevelDirect,
				RiskScore: 50,
				DirectChanges: []ChangedFile{
					{Path: "main.go", Category: FileCategorySource, Insertions: 10, Deletions: 5},
				},
				RequiresRelease: true,
				ReleaseType:     "patch",
			},
		},
		ChangedFiles: []ChangedFile{
			{Path: "main.go", Category: FileCategorySource, Insertions: 10, Deletions: 5},
		},
		Summary: &Summary{
			TotalPackages:     1,
			DirectlyAffected:  1,
			TotalFilesChanged: 1,
			TotalInsertions:   10,
			TotalDeletions:    5,
			RiskLevel:         RiskLevelLow,
			ChangesByCategory: map[FileCategory]int{FileCategorySource: 1},
			AffectedByType:    map[PackageType]int{PackageTypeGoModule: 1},
		},
	}

	// Test non-verbose format
	output := FormatBlastRadius(br, false)
	if output == "" {
		t.Error("FormatBlastRadius() returned empty string")
	}
	if !contains(output, "v1.0.0") {
		t.Error("Output should contain FromRef")
	}
	if !contains(output, "api") {
		t.Error("Output should contain package name")
	}

	// Test verbose format
	verboseOutput := FormatBlastRadius(br, true)
	if !contains(verboseOutput, "main.go") {
		t.Error("Verbose output should contain changed file")
	}
}

func TestFormatBlastRadiusJSON(t *testing.T) {
	br := &BlastRadius{
		FromRef: "v1.0.0",
		ToRef:   "HEAD",
		Summary: &Summary{
			TotalPackages: 1,
			RiskLevel:     RiskLevelLow,
		},
	}

	output, err := FormatBlastRadiusJSON(br)
	if err != nil {
		t.Fatalf("FormatBlastRadiusJSON() error = %v", err)
	}

	if !contains(output, "v1.0.0") {
		t.Error("JSON output should contain FromRef")
	}
	if !contains(output, "total_packages") {
		t.Error("JSON output should contain total_packages")
	}
}

func TestFormatBlastRadiusYAML(t *testing.T) {
	br := &BlastRadius{
		FromRef: "v1.0.0",
		ToRef:   "HEAD",
		Summary: &Summary{
			TotalPackages: 1,
			RiskLevel:     RiskLevelLow,
		},
	}

	output, err := FormatBlastRadiusYAML(br)
	if err != nil {
		t.Fatalf("FormatBlastRadiusYAML() error = %v", err)
	}

	if !contains(output, "v1.0.0") {
		t.Error("YAML output should contain FromRef")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
