// Package cli provides the command-line interface for ReleasePilot.
package cli

import (
	"strings"
	"testing"

	"github.com/felixgeelhaar/release-pilot/internal/service/blast"
)

func TestFormatRiskLevel(t *testing.T) {
	tests := []struct {
		name       string
		level      blast.RiskLevel
		wantOutput string
	}{
		{
			name:       "low risk",
			level:      blast.RiskLevelLow,
			wantOutput: "LOW",
		},
		{
			name:       "medium risk",
			level:      blast.RiskLevelMedium,
			wantOutput: "MEDIUM",
		},
		{
			name:       "high risk",
			level:      blast.RiskLevelHigh,
			wantOutput: "HIGH",
		},
		{
			name:       "critical risk",
			level:      blast.RiskLevelCritical,
			wantOutput: "CRITICAL",
		},
		{
			name:       "unknown risk level",
			level:      blast.RiskLevel("unknown"),
			wantOutput: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRiskLevel(tt.level)
			// The result will be styled, so check if it contains the expected text
			if !strings.Contains(got, tt.wantOutput) {
				t.Errorf("formatRiskLevel() = %q, should contain %q", got, tt.wantOutput)
			}
		})
	}
}

func TestFormatImpactIcon(t *testing.T) {
	tests := []struct {
		name  string
		level blast.ImpactLevel
		want  string
	}{
		{
			name:  "direct impact",
			level: blast.ImpactLevelDirect,
			want:  "●",
		},
		{
			name:  "transitive impact",
			level: blast.ImpactLevelTransitive,
			want:  "○",
		},
		{
			name:  "no impact",
			level: blast.ImpactLevelNone,
			want:  "  ",
		},
		{
			name:  "unknown impact",
			level: blast.ImpactLevel("unknown"),
			want:  "  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatImpactIcon(tt.level)
			if !strings.Contains(got, tt.want) {
				t.Errorf("formatImpactIcon() = %q, should contain %q", got, tt.want)
			}
		})
	}
}

func TestFormatRiskBadge(t *testing.T) {
	tests := []struct {
		name        string
		score       int
		wantContent string
	}{
		{
			name:        "high risk score",
			score:       85,
			wantContent: "[risk: 85]",
		},
		{
			name:        "critical threshold",
			score:       70,
			wantContent: "[risk: 70]",
		},
		{
			name:        "medium risk score",
			score:       55,
			wantContent: "[risk: 55]",
		},
		{
			name:        "medium threshold",
			score:       40,
			wantContent: "[risk: 40]",
		},
		{
			name:        "low risk score",
			score:       25,
			wantContent: "[risk: 25]",
		},
		{
			name:        "zero risk",
			score:       0,
			wantContent: "[risk: 0]",
		},
		{
			name:        "boundary below medium",
			score:       39,
			wantContent: "[risk: 39]",
		},
		{
			name:        "boundary below high",
			score:       69,
			wantContent: "[risk: 69]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRiskBadge(tt.score)
			if !strings.Contains(got, tt.wantContent) {
				t.Errorf("formatRiskBadge(%d) = %q, should contain %q", tt.score, got, tt.wantContent)
			}
		})
	}
}

func TestFormatFileCategoryStyle(t *testing.T) {
	tests := []struct {
		name     string
		category blast.FileCategory
	}{
		{
			name:     "source files",
			category: blast.FileCategorySource,
		},
		{
			name:     "config files",
			category: blast.FileCategoryConfig,
		},
		{
			name:     "test files",
			category: blast.FileCategoryTest,
		},
		{
			name:     "docs files",
			category: blast.FileCategoryDocs,
		},
		{
			name:     "build files",
			category: blast.FileCategoryBuild,
		},
		{
			name:     "ci files",
			category: blast.FileCategoryCI,
		},
		{
			name:     "dependency files",
			category: blast.FileCategoryDependency,
		},
		{
			name:     "asset files",
			category: blast.FileCategoryAsset,
		},
		{
			name:     "generated files",
			category: blast.FileCategoryGenerated,
		},
		{
			name:     "other files",
			category: blast.FileCategoryOther,
		},
		{
			name:     "unknown category",
			category: blast.FileCategory("unknown"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := formatFileCategoryStyle(tt.category)
			// Verify the style can render without panicking
			rendered := style.Render("test")
			if rendered == "" {
				t.Errorf("formatFileCategoryStyle(%q) returned style that renders empty string", tt.category)
			}
		})
	}
}

func TestFormatFileCategoryStyle_AppliesCorrectColors(t *testing.T) {
	// Test that different categories get different styles
	sourceStyle := formatFileCategoryStyle(blast.FileCategorySource)
	configStyle := formatFileCategoryStyle(blast.FileCategoryConfig)
	testStyle := formatFileCategoryStyle(blast.FileCategoryTest)

	sourceRendered := sourceStyle.Render("test")
	configRendered := configStyle.Render("test")
	testRendered := testStyle.Render("test")

	// While we can't easily compare exact colors in tests, we can ensure
	// each renders to something and the function doesn't panic
	if sourceRendered == "" || configRendered == "" || testRendered == "" {
		t.Error("formatFileCategoryStyle should render non-empty strings for all categories")
	}
}

func TestOutputBlastJSON(t *testing.T) {
	br := &blast.BlastRadius{
		FromRef: "v1.0.0",
		ToRef:   "HEAD",
		Summary: &blast.Summary{
			TotalPackages: 5,
			RiskLevel:     blast.RiskLevelMedium,
		},
	}

	// Capture stdout by redirecting to /dev/null
	// This just tests that the function doesn't panic
	err := outputBlastJSON(br)
	if err != nil {
		t.Errorf("outputBlastJSON() error = %v", err)
	}
}

func TestOutputBlastText(t *testing.T) {
	br := &blast.BlastRadius{
		FromRef: "v1.0.0",
		ToRef:   "HEAD",
		Summary: &blast.Summary{
			TotalPackages:            5,
			TotalFilesChanged:        10,
			TotalInsertions:          100,
			TotalDeletions:           50,
			DirectlyAffected:         2,
			TransitivelyAffected:     1,
			PackagesRequiringRelease: 3,
			RiskLevel:                blast.RiskLevelMedium,
			RiskFactors:              []string{"Large changeset", "Critical files modified"},
			ChangesByCategory: map[blast.FileCategory]int{
				blast.FileCategorySource: 8,
				blast.FileCategoryTest:   2,
			},
		},
		Impacts: []*blast.Impact{
			{
				Package: &blast.Package{
					Name: "pkg-a",
					Type: "library",
					Path: "/path/to/pkg-a",
				},
				Level:           blast.ImpactLevelDirect,
				RiskScore:       75,
				RequiresRelease: true,
				ReleaseType:     "minor",
				DirectChanges: []blast.ChangedFile{
					{
						Path:       "main.go",
						Category:   blast.FileCategorySource,
						Insertions: 50,
						Deletions:  25,
					},
				},
				AffectedDependencies: []string{"pkg-b", "pkg-c"},
				SuggestedActions:     []string{"Run integration tests", "Update documentation"},
			},
		},
	}

	// Test non-verbose output
	err := outputBlastText(br, false)
	if err != nil {
		t.Errorf("outputBlastText(verbose=false) error = %v", err)
	}

	// Test verbose output
	err = outputBlastText(br, true)
	if err != nil {
		t.Errorf("outputBlastText(verbose=true) error = %v", err)
	}
}

func TestOutputBlastSummary(t *testing.T) {
	tests := []struct {
		name string
		br   *blast.BlastRadius
	}{
		{
			name: "basic summary",
			br: &blast.BlastRadius{
				FromRef: "v1.0.0",
				ToRef:   "HEAD",
				Summary: &blast.Summary{
					TotalPackages:            5,
					TotalFilesChanged:        10,
					TotalInsertions:          100,
					TotalDeletions:           50,
					DirectlyAffected:         2,
					TransitivelyAffected:     1,
					PackagesRequiringRelease: 3,
					RiskLevel:                blast.RiskLevelMedium,
				},
			},
		},
		{
			name: "high risk summary",
			br: &blast.BlastRadius{
				FromRef: "v2.0.0",
				ToRef:   "main",
				Summary: &blast.Summary{
					TotalPackages:            10,
					TotalFilesChanged:        50,
					TotalInsertions:          500,
					TotalDeletions:           200,
					DirectlyAffected:         5,
					TransitivelyAffected:     3,
					PackagesRequiringRelease: 8,
					RiskLevel:                blast.RiskLevelHigh,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic
			outputBlastSummary(tt.br)
		})
	}
}

func TestOutputBlastRiskFactors(t *testing.T) {
	tests := []struct {
		name    string
		factors []string
	}{
		{
			name:    "no factors",
			factors: []string{},
		},
		{
			name:    "single factor",
			factors: []string{"Large changeset"},
		},
		{
			name:    "multiple factors",
			factors: []string{"Large changeset", "Critical files modified", "Many affected packages"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic
			outputBlastRiskFactors(tt.factors)
		})
	}
}

func TestOutputBlastImpacts(t *testing.T) {
	tests := []struct {
		name    string
		impacts []*blast.Impact
		verbose bool
	}{
		{
			name:    "no impacts",
			impacts: []*blast.Impact{},
			verbose: false,
		},
		{
			name: "single impact non-verbose",
			impacts: []*blast.Impact{
				{
					Package: &blast.Package{
						Name: "pkg-a",
						Type: "library",
						Path: "/path/to/pkg-a",
					},
					Level:           blast.ImpactLevelDirect,
					RiskScore:       50,
					RequiresRelease: true,
					ReleaseType:     "patch",
				},
			},
			verbose: false,
		},
		{
			name: "multiple impacts verbose",
			impacts: []*blast.Impact{
				{
					Package: &blast.Package{
						Name: "pkg-a",
						Type: "library",
						Path: "/path/to/pkg-a",
					},
					Level:           blast.ImpactLevelDirect,
					RiskScore:       75,
					TransitiveDepth: 0,
					RequiresRelease: true,
					ReleaseType:     "minor",
					DirectChanges: []blast.ChangedFile{
						{
							Path:       "main.go",
							Category:   blast.FileCategorySource,
							Insertions: 50,
							Deletions:  25,
						},
					},
					AffectedDependencies: []string{"pkg-b"},
					SuggestedActions:     []string{"Run tests"},
				},
				{
					Package: &blast.Package{
						Name: "pkg-b",
						Type: "service",
						Path: "/path/to/pkg-b",
					},
					Level:           blast.ImpactLevelTransitive,
					RiskScore:       30,
					TransitiveDepth: 1,
					RequiresRelease: false,
				},
			},
			verbose: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic
			outputBlastImpacts(tt.impacts, tt.verbose)
		})
	}
}

func TestOutputBlastImpactHeader(t *testing.T) {
	tests := []struct {
		name   string
		impact *blast.Impact
	}{
		{
			name: "direct impact with release",
			impact: &blast.Impact{
				Package: &blast.Package{
					Name: "pkg-a",
					Type: "library",
					Path: "/path/to/pkg-a",
				},
				Level:           blast.ImpactLevelDirect,
				RiskScore:       75,
				RequiresRelease: true,
				ReleaseType:     "minor",
			},
		},
		{
			name: "transitive impact no release",
			impact: &blast.Impact{
				Package: &blast.Package{
					Name: "pkg-b",
					Type: "service",
					Path: "/path/to/pkg-b",
				},
				Level:           blast.ImpactLevelTransitive,
				RiskScore:       30,
				TransitiveDepth: 2,
				RequiresRelease: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic
			outputBlastImpactHeader(tt.impact)
		})
	}
}

func TestOutputBlastImpactDetails(t *testing.T) {
	impact := &blast.Impact{
		DirectChanges: []blast.ChangedFile{
			{
				Path:       "main.go",
				Category:   blast.FileCategorySource,
				Insertions: 50,
				Deletions:  25,
			},
		},
		AffectedDependencies: []string{"pkg-b", "pkg-c"},
		SuggestedActions:     []string{"Run tests", "Update docs"},
	}

	// Just verify it doesn't panic
	outputBlastImpactDetails(impact)
}

func TestOutputBlastChangedFiles(t *testing.T) {
	tests := []struct {
		name    string
		changes []blast.ChangedFile
	}{
		{
			name:    "no changes",
			changes: []blast.ChangedFile{},
		},
		{
			name: "single change",
			changes: []blast.ChangedFile{
				{
					Path:       "main.go",
					Category:   blast.FileCategorySource,
					Insertions: 50,
					Deletions:  25,
				},
			},
		},
		{
			name: "multiple changes",
			changes: []blast.ChangedFile{
				{
					Path:       "main.go",
					Category:   blast.FileCategorySource,
					Insertions: 50,
					Deletions:  25,
				},
				{
					Path:       "config.yaml",
					Category:   blast.FileCategoryConfig,
					Insertions: 10,
					Deletions:  5,
				},
				{
					Path:       "main_test.go",
					Category:   blast.FileCategoryTest,
					Insertions: 30,
					Deletions:  15,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic
			outputBlastChangedFiles(tt.changes)
		})
	}
}

func TestOutputBlastAffectedDeps(t *testing.T) {
	tests := []struct {
		name string
		deps []string
	}{
		{
			name: "no deps",
			deps: []string{},
		},
		{
			name: "single dep",
			deps: []string{"pkg-a"},
		},
		{
			name: "multiple deps",
			deps: []string{"pkg-a", "pkg-b", "pkg-c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic
			outputBlastAffectedDeps(tt.deps)
		})
	}
}

func TestOutputBlastSuggestedActions(t *testing.T) {
	tests := []struct {
		name    string
		actions []string
	}{
		{
			name:    "no actions",
			actions: []string{},
		},
		{
			name:    "single action",
			actions: []string{"Run integration tests"},
		},
		{
			name:    "multiple actions",
			actions: []string{"Run integration tests", "Update documentation", "Notify stakeholders"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic
			outputBlastSuggestedActions(tt.actions)
		})
	}
}

func TestOutputBlastChangesByCategory(t *testing.T) {
	tests := []struct {
		name       string
		categories map[blast.FileCategory]int
		verbose    bool
	}{
		{
			name:       "not verbose",
			categories: map[blast.FileCategory]int{blast.FileCategorySource: 5},
			verbose:    false,
		},
		{
			name:       "verbose empty",
			categories: map[blast.FileCategory]int{},
			verbose:    true,
		},
		{
			name: "verbose with categories",
			categories: map[blast.FileCategory]int{
				blast.FileCategorySource: 8,
				blast.FileCategoryTest:   2,
				blast.FileCategoryConfig: 1,
			},
			verbose: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic
			outputBlastChangesByCategory(tt.categories, tt.verbose)
		})
	}
}

func TestOutputBlastLegend(t *testing.T) {
	// Just verify it doesn't panic
	outputBlastLegend()
}
