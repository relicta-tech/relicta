// Package blast provides blast radius analysis for monorepos.
package blast

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestGitRepo creates a temporary git repository with commits for testing.
func setupTestGitRepo(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "blast-git-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to init git: %v", err)
	}

	// Configure git user for commits
	configCmds := [][]string{
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test User"},
	}
	for _, args := range configCmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			os.RemoveAll(tmpDir)
			t.Fatalf("Failed to config git: %v", err)
		}
	}

	return tmpDir
}

// commitFile creates a file and commits it to the git repo.
func commitFile(t *testing.T, repoPath, filePath, content, message string) {
	t.Helper()

	fullPath := filepath.Join(repoPath, filePath)

	// Create directory if needed
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create dir %s: %v", dir, err)
	}

	// Write file
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file %s: %v", filePath, err)
	}

	// Git add
	cmd := exec.Command("git", "add", filePath)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}

	// Git commit
	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}
}

// getCommitHash returns the hash of the specified ref.
func getCommitHash(t *testing.T, repoPath, ref string) string {
	t.Helper()

	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get commit hash for %s: %v", ref, err)
	}

	return string(output[:len(output)-1]) // trim newline
}

func TestGetChangedFiles(t *testing.T) {
	repoPath := setupTestGitRepo(t)
	defer os.RemoveAll(repoPath)

	svc := NewService(WithRepoPath(repoPath))

	// Create initial commit
	commitFile(t, repoPath, "README.md", "# Test Project", "Initial commit")
	firstCommit := getCommitHash(t, repoPath, "HEAD")

	// Create second commit with changes
	commitFile(t, repoPath, "main.go", "package main\n\nfunc main() {}\n", "Add main.go")
	commitFile(t, repoPath, "config.yaml", "env: production\n", "Add config")
	secondCommit := getCommitHash(t, repoPath, "HEAD")

	tests := []struct {
		name          string
		fromRef       string
		toRef         string
		wantFiles     int
		wantError     bool
		checkCategory bool
	}{
		{
			name:      "between two commits",
			fromRef:   firstCommit,
			toRef:     secondCommit,
			wantFiles: 2, // main.go and config.yaml
			wantError: false,
		},
		{
			name:      "empty fromRef defaults to last commit",
			fromRef:   "",
			toRef:     "HEAD",
			wantFiles: 1, // Only config.yaml from last commit
			wantError: false,
		},
		{
			name:      "invalid fromRef",
			fromRef:   "invalid-ref",
			toRef:     "HEAD",
			wantFiles: 0,
			wantError: true,
		},
		{
			name:      "invalid toRef",
			fromRef:   firstCommit,
			toRef:     "invalid-ref",
			wantFiles: 0,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := svc.GetChangedFiles(context.Background(), tt.fromRef, tt.toRef)

			if tt.wantError {
				if err == nil {
					t.Error("GetChangedFiles() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GetChangedFiles() error = %v, want nil", err)
				return
			}

			if len(files) != tt.wantFiles {
				t.Errorf("GetChangedFiles() returned %d files, want %d", len(files), tt.wantFiles)
			}

			// Verify file categorization
			if len(files) > 0 && tt.checkCategory {
				for _, file := range files {
					if file.Category == "" {
						t.Errorf("File %s has empty category", file.Path)
					}
				}
			}
		})
	}
}

func TestAnalyzeBlastRadius(t *testing.T) {
	repoPath := setupTestGitRepo(t)
	defer os.RemoveAll(repoPath)

	// Create package structure
	commitFile(t, repoPath, "go.mod", "module example.com/test\n\ngo 1.21\n", "Add go.mod")
	commitFile(t, repoPath, "packages/api/go.mod", "module example.com/test/api\n\ngo 1.21\n", "Add api package")
	commitFile(t, repoPath, "packages/web/package.json", `{"name":"web","version":"1.0.0"}`, "Add web package")
	firstCommit := getCommitHash(t, repoPath, "HEAD")

	// Make changes
	commitFile(t, repoPath, "packages/api/main.go", "package main\n\nfunc main() {}\n", "Update API")
	commitFile(t, repoPath, "packages/web/index.ts", "console.log('test');\n", "Update web")
	secondCommit := getCommitHash(t, repoPath, "HEAD")

	svc := NewService(WithRepoPath(repoPath))

	tests := []struct {
		name            string
		opts            *AnalysisOptions
		wantError       bool
		checkImpacts    bool
		checkGraph      bool
		minAffected     int
		checkRiskScores bool
		checkFiltering  bool
	}{
		{
			name: "basic analysis",
			opts: &AnalysisOptions{
				FromRef: firstCommit,
				ToRef:   secondCommit,
				MonorepoConfig: &MonorepoConfig{
					PackagePaths: []string{"packages/*"},
					ExcludePaths: []string{"node_modules", "vendor"},
				},
			},
			wantError:    false,
			checkImpacts: true,
			minAffected:  1,
		},
		{
			name: "with risk calculation",
			opts: &AnalysisOptions{
				FromRef:       firstCommit,
				ToRef:         secondCommit,
				CalculateRisk: true,
				MonorepoConfig: &MonorepoConfig{
					PackagePaths: []string{"packages/*"},
				},
			},
			wantError:       false,
			checkImpacts:    true,
			checkRiskScores: true,
		},
		{
			name: "with dependency graph",
			opts: &AnalysisOptions{
				FromRef:       firstCommit,
				ToRef:         secondCommit,
				GenerateGraph: true,
				MonorepoConfig: &MonorepoConfig{
					PackagePaths: []string{"packages/*"},
				},
			},
			wantError:  false,
			checkGraph: true,
		},
		{
			name: "exclude tests",
			opts: &AnalysisOptions{
				FromRef:      firstCommit,
				ToRef:        secondCommit,
				IncludeTests: false,
				MonorepoConfig: &MonorepoConfig{
					PackagePaths: []string{"packages/*"},
				},
			},
			wantError:      false,
			checkFiltering: true,
		},
		{
			name: "exclude docs",
			opts: &AnalysisOptions{
				FromRef:     firstCommit,
				ToRef:       secondCommit,
				IncludeDocs: false,
				MonorepoConfig: &MonorepoConfig{
					PackagePaths: []string{"packages/*"},
				},
			},
			wantError: false,
		},
		{
			name:         "nil options uses defaults",
			opts:         nil,
			wantError:    false, // Uses defaults and succeeds with existing commits
			checkImpacts: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.AnalyzeBlastRadius(context.Background(), tt.opts)

			if tt.wantError {
				if err == nil {
					t.Error("AnalyzeBlastRadius() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("AnalyzeBlastRadius() error = %v, want nil", err)
				return
			}

			if result == nil {
				t.Fatal("AnalyzeBlastRadius() returned nil result")
			}

			// Check basic structure
			if result.Summary == nil {
				t.Error("Result missing summary")
			}

			if tt.checkImpacts {
				if len(result.Impacts) < tt.minAffected {
					t.Errorf("Expected at least %d impacts, got %d", tt.minAffected, len(result.Impacts))
				}
			}

			if tt.checkRiskScores {
				for _, impact := range result.Impacts {
					if impact.RiskScore < 0 || impact.RiskScore > 100 {
						t.Errorf("Invalid risk score %d for package %s", impact.RiskScore, impact.Package.Name)
					}
					if impact.ReleaseType == "" {
						t.Errorf("Missing release type for package %s", impact.Package.Name)
					}
				}
			}

			if tt.checkGraph {
				if result.DependencyGraph == nil {
					t.Error("Expected dependency graph, got nil")
				}
			}

			if tt.checkFiltering {
				for _, file := range result.ChangedFiles {
					if file.Category == FileCategoryTest {
						t.Error("Found test file when tests should be excluded")
					}
				}
			}
		})
	}
}

func TestGetImpactedPackages(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "blast-impact-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create package structure - use relative paths like real packages
	packages := []*Package{
		{
			Name:    "api",
			Path:    "packages/api",
			Type:    PackageTypeGoModule,
			Version: "1.0.0",
		},
		{
			Name:    "web",
			Path:    "packages/web",
			Type:    PackageTypeNPM,
			Version: "1.0.0",
		},
		{
			Name:    "shared",
			Path:    "shared",
			Type:    PackageTypeGoModule,
			Version: "1.0.0",
		},
	}

	svc := NewService(WithRepoPath(tmpDir))

	tests := []struct {
		name         string
		changedFiles []ChangedFile
		wantImpacts  int
		checkLevel   bool
		expectDirect bool
	}{
		{
			name: "direct impact on api package",
			changedFiles: []ChangedFile{
				{
					Path:       "packages/api/main.go",
					Category:   FileCategorySource,
					Status:     "M",
					Insertions: 10,
					Deletions:  5,
				},
			},
			wantImpacts:  1,
			checkLevel:   true,
			expectDirect: true,
		},
		{
			name: "impact on multiple packages",
			changedFiles: []ChangedFile{
				{
					Path:     "packages/api/main.go",
					Category: FileCategorySource,
					Status:   "M",
				},
				{
					Path:     "packages/web/index.ts",
					Category: FileCategorySource,
					Status:   "M",
				},
			},
			wantImpacts: 2,
		},
		{
			name: "no impact - file outside packages",
			changedFiles: []ChangedFile{
				{
					Path:     "README.md",
					Category: FileCategoryDocs,
					Status:   "M",
				},
			},
			wantImpacts: 0,
		},
		{
			name:         "empty changed files",
			changedFiles: []ChangedFile{},
			wantImpacts:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			impacts, err := svc.GetImpactedPackages(context.Background(), tt.changedFiles, packages)
			if err != nil {
				t.Errorf("GetImpactedPackages() error = %v, want nil", err)
				return
			}

			if len(impacts) != tt.wantImpacts {
				t.Errorf("GetImpactedPackages() returned %d impacts, want %d", len(impacts), tt.wantImpacts)
			}

			if tt.checkLevel && len(impacts) > 0 {
				if tt.expectDirect && impacts[0].Level != ImpactLevelDirect {
					t.Errorf("Expected direct impact level, got %v", impacts[0].Level)
				}
			}
		})
	}
}

func TestBuildDependencyGraph(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "blast-graph-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packages := []*Package{
		{
			Name:         "api",
			Path:         filepath.Join(tmpDir, "packages/api"),
			Type:         PackageTypeGoModule,
			Dependencies: []string{"shared"},
		},
		{
			Name:         "web",
			Path:         filepath.Join(tmpDir, "packages/web"),
			Type:         PackageTypeNPM,
			Dependencies: []string{"shared"},
		},
		{
			Name: "shared",
			Path: filepath.Join(tmpDir, "shared"),
			Type: PackageTypeGoModule,
		},
	}

	svc := NewService(WithRepoPath(tmpDir))

	tests := []struct {
		name      string
		packages  []*Package
		wantNodes int
		wantEdges int
		wantError bool
	}{
		{
			name:      "build graph with dependencies",
			packages:  packages,
			wantNodes: 3,
			wantEdges: 2, // api->shared, web->shared
			wantError: false,
		},
		{
			name:      "empty packages",
			packages:  []*Package{},
			wantNodes: 0,
			wantEdges: 0,
			wantError: false,
		},
		{
			name: "package without dependencies",
			packages: []*Package{
				{
					Name: "standalone",
					Path: filepath.Join(tmpDir, "standalone"),
					Type: PackageTypeGoModule,
				},
			},
			wantNodes: 1,
			wantEdges: 0,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph, err := svc.BuildDependencyGraph(context.Background(), tt.packages)

			if tt.wantError {
				if err == nil {
					t.Error("BuildDependencyGraph() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("BuildDependencyGraph() error = %v, want nil", err)
				return
			}

			if graph == nil {
				t.Fatal("BuildDependencyGraph() returned nil graph")
			}

			if len(graph.Nodes) != tt.wantNodes {
				t.Errorf("Graph has %d nodes, want %d", len(graph.Nodes), tt.wantNodes)
			}

			if len(graph.Edges) != tt.wantEdges {
				t.Errorf("Graph has %d edges, want %d", len(graph.Edges), tt.wantEdges)
			}
		})
	}
}

func TestAddTransitiveImpacts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "blast-transitive-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packages := []*Package{
		{
			Name:         "api",
			Path:         "packages/api",
			Dependencies: []string{"shared"},
		},
		{
			Name:         "web",
			Path:         "packages/web",
			Dependencies: []string{"api"},
		},
		{
			Name: "shared",
			Path: "shared",
		},
	}

	svc := NewService(WithRepoPath(tmpDir)).(*serviceImpl)

	tests := []struct {
		name            string
		directImpacts   []*Impact
		config          *MonorepoConfig
		wantMin         int
		checkTransitive bool
	}{
		{
			name: "transitive impact propagation",
			directImpacts: []*Impact{
				{
					Package: &Package{Name: "shared", Path: "shared"},
					Level:   ImpactLevelDirect,
				},
			},
			config: &MonorepoConfig{
				MaxTransitiveDepth: 10,
			},
			wantMin:         2, // api and web are transitively affected
			checkTransitive: true,
		},
		{
			name: "no transitive impacts",
			directImpacts: []*Impact{
				{
					Package: &Package{Name: "web", Path: "packages/web"},
					Level:   ImpactLevelDirect,
				},
			},
			config: &MonorepoConfig{
				MaxTransitiveDepth: 10,
			},
			wantMin: 1, // Only web itself
		},
		{
			name:          "empty direct impacts",
			directImpacts: []*Impact{},
			config: &MonorepoConfig{
				MaxTransitiveDepth: 10,
			},
			wantMin: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			impacts := svc.addTransitiveImpacts(packages, tt.directImpacts, tt.config)

			if len(impacts) < tt.wantMin {
				t.Errorf("addTransitiveImpacts() returned %d impacts, want at least %d", len(impacts), tt.wantMin)
			}

			if tt.checkTransitive {
				foundTransitive := false
				for _, impact := range impacts {
					if impact.Level == ImpactLevelTransitive {
						foundTransitive = true
						if len(impact.AffectedDependencies) == 0 {
							t.Error("Transitive impact should have affected dependencies")
						}
						if impact.TransitiveDepth == 0 {
							t.Error("Transitive impact should have depth > 0")
						}
					}
				}
				if !foundTransitive && len(impacts) > 1 {
					t.Error("Expected at least one transitive impact")
				}
			}
		})
	}
}

func TestSuggestActions(t *testing.T) {
	svc := NewService().(*serviceImpl)

	tests := []struct {
		name        string
		impact      *Impact
		wantMin     int
		wantActions []string
	}{
		{
			name: "direct impact with source changes",
			impact: &Impact{
				Level: ImpactLevelDirect,
				DirectChanges: []ChangedFile{
					{Category: FileCategorySource},
				},
				RiskScore: 50,
			},
			wantMin:     2,
			wantActions: []string{"Run unit tests", "Review code changes"},
		},
		{
			name: "direct impact with config changes",
			impact: &Impact{
				Level: ImpactLevelDirect,
				DirectChanges: []ChangedFile{
					{Category: FileCategoryConfig},
				},
				RiskScore: 40,
			},
			wantMin:     2,
			wantActions: []string{"Verify configuration changes"},
		},
		{
			name: "source changes without tests",
			impact: &Impact{
				Level: ImpactLevelDirect,
				DirectChanges: []ChangedFile{
					{Category: FileCategorySource},
				},
				RiskScore: 30,
			},
			wantMin:     3,
			wantActions: []string{"Consider adding tests for new code"},
		},
		{
			name: "transitive impact",
			impact: &Impact{
				Level:     ImpactLevelTransitive,
				RiskScore: 30,
			},
			wantMin:     2,
			wantActions: []string{"Run integration tests", "Verify compatibility with updated dependencies"},
		},
		{
			name: "high risk score",
			impact: &Impact{
				Level:     ImpactLevelDirect,
				RiskScore: 80,
			},
			wantMin:     2,
			wantActions: []string{"Consider additional review", "Plan rollback strategy"},
		},
		{
			name: "no impact",
			impact: &Impact{
				Level:     ImpactLevelNone,
				RiskScore: 0,
			},
			wantMin: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := svc.suggestActions(tt.impact)

			if len(actions) < tt.wantMin {
				t.Errorf("suggestActions() returned %d actions, want at least %d", len(actions), tt.wantMin)
			}

			for _, wantAction := range tt.wantActions {
				found := false
				for _, action := range actions {
					if action == wantAction {
						found = true
						break
					}
				}
				if !found && len(tt.wantActions) > 0 {
					t.Errorf("Expected action '%s' not found in %v", wantAction, actions)
				}
			}
		})
	}
}

func TestAnalyzeBlastRadiusWithTransitiveImpacts(t *testing.T) {
	repoPath := setupTestGitRepo(t)
	defer os.RemoveAll(repoPath)

	// Create packages with dependencies
	commitFile(t, repoPath, "shared/go.mod", "module example.com/shared\n\ngo 1.21\n", "Add shared")
	commitFile(t, repoPath, "api/go.mod", "module example.com/api\n\ngo 1.21\n\nrequire example.com/shared v1.0.0\n", "Add api")
	firstCommit := getCommitHash(t, repoPath, "HEAD")

	// Change shared package
	commitFile(t, repoPath, "shared/lib.go", "package shared\n\nfunc Lib() {}\n", "Update shared")
	secondCommit := getCommitHash(t, repoPath, "HEAD")

	svc := NewService(WithRepoPath(repoPath))

	opts := &AnalysisOptions{
		FromRef:           firstCommit,
		ToRef:             secondCommit,
		IncludeTransitive: true,
		CalculateRisk:     true,
		MonorepoConfig: &MonorepoConfig{
			PackagePaths:       []string{"*"},
			MaxTransitiveDepth: 5,
		},
	}

	result, err := svc.AnalyzeBlastRadius(context.Background(), opts)
	if err != nil {
		t.Fatalf("AnalyzeBlastRadius() error = %v", err)
	}

	if result == nil {
		t.Fatal("AnalyzeBlastRadius() returned nil result")
	}

	// Should have at least the direct impact on shared
	if len(result.Impacts) < 1 {
		t.Error("Expected at least one impact")
	}

	// Check that transitive impacts have suggested actions
	for _, impact := range result.Impacts {
		if impact.Level == ImpactLevelTransitive {
			if len(impact.SuggestedActions) == 0 {
				t.Error("Transitive impact should have suggested actions")
			}
		}
	}
}
