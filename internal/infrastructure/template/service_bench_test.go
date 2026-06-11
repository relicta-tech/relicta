// Package template provides template rendering benchmarks.
package template

import (
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/domain/version"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
)

// BenchmarkService_RenderChangelog benchmarks changelog rendering.
// Target: < 100ms for typical changelog with 100 commits.
func BenchmarkService_RenderChangelog(b *testing.B) {
	b.ReportAllocs()

	svc, err := NewService()
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}

	benchCases := []struct {
		name      string
		numCommit int
	}{
		{"10_commits", 10},
		{"50_commits", 50},
		{"100_commits", 100},
		{"250_commits", 250},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			data := generateChangelogData(bc.numCommit)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := svc.Render("changelog", data)
				if err != nil {
					b.Fatalf("Render failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkService_RenderReleaseNotes benchmarks release notes rendering.
func BenchmarkService_RenderReleaseNotes(b *testing.B) {
	b.ReportAllocs()

	svc, err := NewService()
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}

	benchCases := []struct {
		name      string
		numCommit int
	}{
		{"10_commits", 10},
		{"50_commits", 50},
		{"100_commits", 100},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			changes := generateCategorizedChanges(bc.numCommit)
			data := ReleaseNotesData{
				Version:       mustParseVersion("2.0.0"),
				Date:          time.Now(),
				Changelog:     "Generated changelog content...",
				Summary:       "This release includes several new features and bug fixes.",
				Highlights:    []string{"New feature A", "Improved performance", "Bug fixes"},
				Changes:       changes,
				Contributors:  []string{"user1", "user2", "user3"},
				RepositoryURL: "https://github.com/example/repo",
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := svc.Render("release-notes", data)
				if err != nil {
					b.Fatalf("Render failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkService_RenderString benchmarks inline template rendering.
func BenchmarkService_RenderString(b *testing.B) {
	b.ReportAllocs()

	svc, err := NewService()
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}

	benchCases := []struct {
		name     string
		template string
		data     any
	}{
		{
			name:     "simple",
			template: "Hello, {{ .Name }}!",
			data:     map[string]string{"Name": "World"},
		},
		{
			name:     "with_functions",
			template: "{{ .Name | upper }} - {{ .Date | dateISO }}",
			data:     map[string]any{"Name": "test", "Date": time.Now()},
		},
		{
			name:     "with_loop",
			template: "{{ range .Items }}{{ . }}, {{ end }}",
			data:     map[string][]string{"Items": {"a", "b", "c", "d", "e"}},
		},
		{
			name:     "complex",
			template: "{{ if .Show }}{{ .Title | upper }}: {{ range .Items }}{{ . | mdCode }}, {{ end }}{{ end }}",
			data:     map[string]any{"Show": true, "Title": "Items", "Items": []string{"one", "two", "three"}},
		},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := svc.RenderString(bc.template, bc.data)
				if err != nil {
					b.Fatalf("RenderString failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkService_RegisterTemplate benchmarks template registration.
func BenchmarkService_RegisterTemplate(b *testing.B) {
	b.ReportAllocs()

	svc, err := NewService()
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}

	template := `# Release {{ .Version }}
{{ range .Items }}
- {{ . }}
{{ end }}`

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := svc.RegisterTemplate("bench-template", template)
		if err != nil {
			b.Fatalf("RegisterTemplate failed: %v", err)
		}
	}
}

// BenchmarkService_LoadTemplate benchmarks template loading.
func BenchmarkService_LoadTemplate(b *testing.B) {
	b.ReportAllocs()

	svc, err := NewService()
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := svc.LoadTemplate("changelog")
		if err != nil {
			b.Fatalf("LoadTemplate failed: %v", err)
		}
	}
}

// BenchmarkService_Concurrent benchmarks concurrent template rendering.
func BenchmarkService_Concurrent(b *testing.B) {
	b.ReportAllocs()

	svc, err := NewService()
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}

	data := generateChangelogData(50)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := svc.Render("changelog", data)
			if err != nil {
				b.Fatalf("Render failed: %v", err)
			}
		}
	})
}

// BenchmarkBufferPool benchmarks buffer pool effectiveness.
func BenchmarkBufferPool(b *testing.B) {
	b.ReportAllocs()

	svc, err := NewService()
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}

	// Small template for rapid allocation testing
	template := "Version: {{ .Version }}"
	data := map[string]string{"Version": "1.0.0"}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := svc.RenderString(template, data)
		if err != nil {
			b.Fatalf("RenderString failed: %v", err)
		}
	}
}

// Helper functions

func mustParseVersion(s string) *version.SemanticVersion {
	v, err := version.Parse(s)
	if err != nil {
		panic(err)
	}
	return &v
}

func generateChangelogData(numCommits int) *ChangelogData {
	changes := generateCategorizedChanges(numCommits)
	return &ChangelogData{
		Version:         mustParseVersion("2.0.0"),
		PreviousVersion: mustParseVersion("1.0.0"),
		Date:            time.Now(),
		Changes:         changes,
		RepositoryURL:   "https://github.com/example/repo",
		IssueURL:        "https://github.com/example/repo/issues/%s",
		CompareURL:      "https://github.com/example/repo/compare/v1.0.0...v2.0.0",
	}
}

func generateCategorizedChanges(numCommits int) *git.CategorizedChanges {
	changes := &git.CategorizedChanges{
		All: make([]git.ConventionalCommit, 0, numCommits),
	}

	// Distribute commits across categories (similar to real distribution)
	featCount := numCommits * 25 / 100
	fixCount := numCommits * 30 / 100
	perfCount := numCommits * 10 / 100
	refactorCount := numCommits * 15 / 100
	otherCount := numCommits - featCount - fixCount - perfCount - refactorCount

	// Generate features
	for i := 0; i < featCount; i++ {
		commit := generateConventionalCommit(git.CommitTypeFeat, i)
		changes.Features = append(changes.Features, commit)
		changes.All = append(changes.All, commit)
	}

	// Generate fixes
	for i := 0; i < fixCount; i++ {
		commit := generateConventionalCommit(git.CommitTypeFix, i)
		changes.Fixes = append(changes.Fixes, commit)
		changes.All = append(changes.All, commit)
	}

	// Generate performance commits
	for i := 0; i < perfCount; i++ {
		commit := generateConventionalCommit(git.CommitTypePerf, i)
		changes.Performance = append(changes.Performance, commit)
		changes.All = append(changes.All, commit)
	}

	// Generate refactoring commits
	for i := 0; i < refactorCount; i++ {
		commit := generateConventionalCommit(git.CommitTypeRefactor, i)
		changes.Refactoring = append(changes.Refactoring, commit)
		changes.All = append(changes.All, commit)
	}

	// Generate other commits
	for i := 0; i < otherCount; i++ {
		commit := generateConventionalCommit(git.CommitTypeChore, i)
		changes.Other = append(changes.Other, commit)
		changes.All = append(changes.All, commit)
	}

	// Add a breaking change
	if numCommits > 5 {
		breakingCommit := generateConventionalCommit(git.CommitTypeFeat, 999)
		breakingCommit.Breaking = true
		breakingCommit.BreakingDescription = "API changed from v1 to v2"
		changes.Breaking = append(changes.Breaking, breakingCommit)
		changes.All = append(changes.All, breakingCommit)
	}

	return changes
}

func generateConventionalCommit(commitType git.CommitType, idx int) git.ConventionalCommit {
	scopes := []string{"api", "cli", "config", "core", "plugin", "release", "template"}
	descriptions := map[git.CommitType][]string{
		git.CommitTypeFeat:     {"add user authentication", "implement plugin system", "add release notes generation"},
		git.CommitTypeFix:      {"resolve race condition", "fix version parsing", "correct changelog format"},
		git.CommitTypePerf:     {"optimize commit parsing", "improve memory usage", "speed up version lookup"},
		git.CommitTypeRefactor: {"extract commit analysis", "reorganize package structure", "simplify version calculation"},
		git.CommitTypeChore:    {"update .gitignore", "clean up unused files", "bump version"},
	}

	scope := scopes[idx%len(scopes)]
	descList := descriptions[commitType]
	if descList == nil {
		descList = []string{"some change"}
	}
	desc := descList[idx%len(descList)]

	hash := generateBenchHash(idx)

	return git.ConventionalCommit{
		Commit: git.Commit{
			Hash:      hash,
			ShortHash: hash[:7],
			Message:   string(commitType) + "(" + scope + "): " + desc,
			Subject:   string(commitType) + "(" + scope + "): " + desc,
			Author:    git.Author{Name: "Benchmark User", Email: "bench@example.com"},
			Committer: git.Author{Name: "Benchmark User", Email: "bench@example.com"},
			Date:      time.Now().Add(-time.Duration(idx) * time.Hour),
		},
		Type:           commitType,
		Scope:          scope,
		Description:    desc,
		IsConventional: true,
	}
}

func generateBenchHash(idx int) string {
	// Generate deterministic hash for benchmarks
	hash := make([]byte, 40)
	for i := range hash {
		hash[i] = "0123456789abcdef"[(idx+i)%16]
	}
	return string(hash)
}
