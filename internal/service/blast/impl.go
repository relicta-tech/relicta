// Package blast provides blast radius analysis for monorepos.
package blast

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/felixgeelhaar/release-pilot/internal/service/git"
	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// Package-level file categorization maps for efficient lookups.
// These are defined at package level to avoid repeated allocations in categorizeFile.
var (
	// docExtensions maps documentation file extensions.
	docExtensions = map[string]bool{".md": true, ".rst": true, ".adoc": true}

	// buildFiles maps build-related file names (should be compared lowercase).
	buildFiles = map[string]bool{
		"makefile": true, "dockerfile": true, "cmakelists.txt": true,
		"build.gradle": true, "pom.xml": true, ".goreleaser.yaml": true,
		".goreleaser.yml": true, "gulpfile.js": true, "webpack.config.js": true,
	}

	// configExtensions maps configuration file extensions.
	configExtensions = map[string]bool{
		".yaml": true, ".yml": true, ".json": true, ".toml": true,
		".ini": true, ".conf": true, ".config": true, ".env": true,
	}

	// configFiles maps specific configuration file names.
	configFiles = map[string]bool{
		".gitignore": true, ".eslintrc": true, ".prettierrc": true,
		"tsconfig.json": true, ".babelrc": true,
	}

	// assetExtensions maps static asset file extensions.
	assetExtensions = map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true,
		".ico": true, ".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
		".mp3": true, ".mp4": true, ".webm": true, ".pdf": true,
	}

	// sourceExtensions maps source code file extensions.
	sourceExtensions = map[string]bool{
		".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".py": true, ".rb": true, ".rs": true, ".java": true, ".kt": true,
		".c": true, ".cpp": true, ".h": true, ".hpp": true, ".cs": true,
		".swift": true, ".scala": true, ".php": true, ".vue": true, ".svelte": true,
	}
)

// serviceImpl implements the Service interface.
type serviceImpl struct {
	config *ServiceConfig
}

// NewService creates a new blast radius analysis service.
func NewService(opts ...ServiceOption) Service {
	config := DefaultServiceConfig()
	for _, opt := range opts {
		opt(config)
	}
	return &serviceImpl{config: config}
}

// DiscoverPackages discovers all packages in the repository.
func (s *serviceImpl) DiscoverPackages(ctx context.Context, opts *AnalysisOptions) ([]*Package, error) {
	if opts == nil {
		opts = DefaultAnalysisOptions()
	}

	monorepoConfig := opts.MonorepoConfig
	if monorepoConfig == nil {
		monorepoConfig = s.config.MonorepoConfig
	}

	var packages []*Package

	// Discover packages in configured paths
	for _, pattern := range monorepoConfig.PackagePaths {
		fullPattern := filepath.Join(s.config.RepoPath, pattern)
		matches, err := filepath.Glob(fullPattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			if s.shouldExclude(match, monorepoConfig.ExcludePaths) {
				continue
			}

			info, err := os.Stat(match)
			if err != nil || !info.IsDir() {
				continue
			}

			pkg, err := s.detectPackage(ctx, match)
			if err != nil || pkg == nil {
				continue
			}

			packages = append(packages, pkg)
		}
	}

	// Check root directory if configured
	if monorepoConfig.RootPackage {
		pkg, err := s.detectPackage(ctx, s.config.RepoPath)
		if err == nil && pkg != nil {
			pkg.Name = "root"
			packages = append(packages, pkg)
		}
	}

	// Check shared directories
	for _, sharedDir := range monorepoConfig.SharedDirs {
		sharedPath := filepath.Join(s.config.RepoPath, sharedDir)
		if _, err := os.Stat(sharedPath); err == nil {
			pkg, err := s.detectPackage(ctx, sharedPath)
			if err == nil && pkg != nil {
				packages = append(packages, pkg)
			}
		}
	}

	return packages, nil
}

// AnalyzeBlastRadius performs a complete blast radius analysis.
func (s *serviceImpl) AnalyzeBlastRadius(ctx context.Context, opts *AnalysisOptions) (*BlastRadius, error) {
	if opts == nil {
		opts = DefaultAnalysisOptions()
	}

	// Get changed files
	changedFiles, err := s.GetChangedFiles(ctx, opts.FromRef, opts.ToRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	// Filter files based on options
	if !opts.IncludeTests {
		changedFiles = filterByCategory(changedFiles, FileCategoryTest, false)
	}
	if !opts.IncludeDocs {
		changedFiles = filterByCategory(changedFiles, FileCategoryDocs, false)
	}

	// Discover packages
	packages, err := s.DiscoverPackages(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to discover packages: %w", err)
	}

	// Get impacted packages
	impacts, err := s.GetImpactedPackages(ctx, changedFiles, packages)
	if err != nil {
		return nil, fmt.Errorf("failed to get impacted packages: %w", err)
	}

	// Include transitive impacts if requested
	if opts.IncludeTransitive {
		impacts = s.addTransitiveImpacts(packages, impacts, opts.MonorepoConfig)
	}

	// Calculate risk scores if requested
	if opts.CalculateRisk {
		for _, impact := range impacts {
			impact.RiskScore = s.CalculateRiskScore(impact)
			impact.ReleaseType = s.SuggestReleaseType(impact)
			impact.RequiresRelease = impact.Level != ImpactLevelNone
		}
	}

	// Build dependency graph if requested
	var graph *DependencyGraph
	if opts.GenerateGraph {
		graph, _ = s.BuildDependencyGraph(ctx, packages)
		// Mark affected nodes
		if graph != nil {
			affectedPkgs := make(map[string]ImpactLevel)
			for _, impact := range impacts {
				if impact.Package != nil {
					affectedPkgs[impact.Package.Path] = impact.Level
				}
			}
			for i := range graph.Nodes {
				if level, ok := affectedPkgs[graph.Nodes[i].ID]; ok {
					graph.Nodes[i].Affected = true
					graph.Nodes[i].ImpactLevel = level
				}
			}
		}
	}

	// Build summary
	summary := s.buildSummary(packages, impacts, changedFiles)

	return &BlastRadius{
		Packages:        packages,
		Impacts:         impacts,
		ChangedFiles:    changedFiles,
		Summary:         summary,
		DependencyGraph: graph,
		AnalyzedAt:      time.Now(),
		FromRef:         opts.FromRef,
		ToRef:           opts.ToRef,
	}, nil
}

// GetChangedFiles returns files changed between two refs.
func (s *serviceImpl) GetChangedFiles(ctx context.Context, fromRef, toRef string) ([]ChangedFile, error) {
	// Validate git references to prevent command injection
	if err := git.ValidateGitRef(fromRef); err != nil {
		return nil, fmt.Errorf("invalid fromRef: %w", err)
	}
	if err := git.ValidateGitRef(toRef); err != nil {
		return nil, fmt.Errorf("invalid toRef: %w", err)
	}

	if toRef == "" {
		toRef = "HEAD"
	}

	args := []string{"diff", "--name-status", "--numstat"}
	if fromRef != "" {
		args = append(args, fromRef+"..."+toRef)
	} else {
		// If no fromRef, get changes in the last commit
		args = append(args, toRef+"^", toRef)
	}

	// First get name-status
	statusCmd := exec.CommandContext(ctx, "git", append([]string{"diff", "--name-status"}, args[3:]...)...)
	statusCmd.Dir = s.config.RepoPath
	statusOutput, err := statusCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get file status: %w", err)
	}

	// Then get numstat for line changes
	numstatCmd := exec.CommandContext(ctx, "git", append([]string{"diff", "--numstat"}, args[3:]...)...)
	numstatCmd.Dir = s.config.RepoPath
	numstatOutput, err := numstatCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get numstat: %w", err)
	}

	// Parse results
	statusLines := strings.Split(strings.TrimSpace(string(statusOutput)), "\n")
	numstatLines := strings.Split(strings.TrimSpace(string(numstatOutput)), "\n")

	// Build numstat map
	numstatMap := make(map[string]struct {
		insertions int
		deletions  int
	})
	for _, line := range numstatLines {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			var ins, del int
			if parts[0] != "-" {
				fmt.Sscanf(parts[0], "%d", &ins)
			}
			if parts[1] != "-" {
				fmt.Sscanf(parts[1], "%d", &del)
			}
			// Handle renamed files
			path := parts[len(parts)-1]
			numstatMap[path] = struct {
				insertions int
				deletions  int
			}{ins, del}
		}
	}

	var files []ChangedFile
	for _, line := range statusLines {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		status := parts[0]
		path := parts[len(parts)-1]
		var oldPath string

		// Map git status to our status
		fileStatus := "modified"
		switch {
		case strings.HasPrefix(status, "A"):
			fileStatus = "added"
		case strings.HasPrefix(status, "D"):
			fileStatus = "deleted"
		case strings.HasPrefix(status, "R"):
			fileStatus = "renamed"
			if len(parts) >= 3 {
				oldPath = parts[1]
			}
		case strings.HasPrefix(status, "C"):
			fileStatus = "copied"
		}

		stat := numstatMap[path]
		isBinary := false
		// Binary files show "-" for insertions/deletions
		if stat.insertions == 0 && stat.deletions == 0 && fileStatus == "modified" {
			isBinary = true
		}

		files = append(files, ChangedFile{
			Path:       path,
			Status:     fileStatus,
			OldPath:    oldPath,
			Insertions: stat.insertions,
			Deletions:  stat.deletions,
			IsBinary:   isBinary,
			Category:   categorizeFile(path),
		})
	}

	return files, nil
}

// GetImpactedPackages returns packages impacted by changes.
func (s *serviceImpl) GetImpactedPackages(ctx context.Context, changedFiles []ChangedFile, packages []*Package) ([]*Impact, error) {
	impacts := make(map[string]*Impact)

	for _, pkg := range packages {
		impacts[pkg.Path] = &Impact{
			Package:         pkg,
			Level:           ImpactLevelNone,
			TransitiveDepth: 0,
		}
	}

	// Map files to packages
	for _, file := range changedFiles {
		for _, pkg := range packages {
			if isFileInPackage(file.Path, pkg.Path) {
				impact := impacts[pkg.Path]
				impact.Level = ImpactLevelDirect
				impact.DirectChanges = append(impact.DirectChanges, file)
			}
		}
	}

	// Filter to only affected packages
	var result []*Impact
	for _, impact := range impacts {
		if impact.Level != ImpactLevelNone {
			// Add suggested actions
			impact.SuggestedActions = s.suggestActions(impact)
			result = append(result, impact)
		}
	}

	// Sort by package path
	sort.Slice(result, func(i, j int) bool {
		return result[i].Package.Path < result[j].Package.Path
	})

	return result, nil
}

// BuildDependencyGraph builds the dependency graph for packages.
func (s *serviceImpl) BuildDependencyGraph(ctx context.Context, packages []*Package) (*DependencyGraph, error) {
	graph := &DependencyGraph{
		Nodes: make([]GraphNode, 0, len(packages)),
		Edges: make([]GraphEdge, 0),
	}

	// Create nodes
	pkgMap := make(map[string]*Package)
	for _, pkg := range packages {
		pkgMap[pkg.Path] = pkg
		pkgMap[pkg.Name] = pkg

		graph.Nodes = append(graph.Nodes, GraphNode{
			ID:    pkg.Path,
			Label: pkg.Name,
			Type:  pkg.Type,
		})
	}

	// Create edges from dependencies
	for _, pkg := range packages {
		for _, dep := range pkg.Dependencies {
			// Try to find the dependency in our packages
			if targetPkg, ok := pkgMap[dep]; ok {
				graph.Edges = append(graph.Edges, GraphEdge{
					Source: pkg.Path,
					Target: targetPkg.Path,
					Type:   "dependency",
				})
			}
		}
	}

	return graph, nil
}

// CalculateRiskScore calculates the risk score for an impact.
func (s *serviceImpl) CalculateRiskScore(impact *Impact) int {
	if impact.Level == ImpactLevelNone {
		return 0
	}

	score := 0

	// Base score based on impact level
	switch impact.Level {
	case ImpactLevelDirect:
		score = 30
	case ImpactLevelTransitive:
		score = 15
	}

	// Add points for number of changes
	changeCount := len(impact.DirectChanges)
	score += min(changeCount*5, 30) // Max 30 points for changes

	// Add points for source code changes
	sourceChanges := 0
	configChanges := 0
	for _, change := range impact.DirectChanges {
		if change.Category == FileCategorySource {
			sourceChanges++
		}
		if change.Category == FileCategoryConfig {
			configChanges++
		}
		// High line changes increase risk
		if change.Insertions+change.Deletions > 100 {
			score += 5
		}
	}

	if sourceChanges > 0 {
		score += 15
	}
	if configChanges > 0 {
		score += 10
	}

	// Add points for transitive depth
	score += impact.TransitiveDepth * 5

	// Cap at 100
	return min(score, 100)
}

// SuggestReleaseType suggests a release type based on changes.
func (s *serviceImpl) SuggestReleaseType(impact *Impact) string {
	if impact.Level == ImpactLevelNone {
		return ""
	}

	hasSourceChanges := false
	hasBreakingChanges := false
	hasConfigChanges := false
	totalLines := 0

	for _, change := range impact.DirectChanges {
		totalLines += change.Insertions + change.Deletions
		if change.Category == FileCategorySource {
			hasSourceChanges = true
		}
		if change.Category == FileCategoryConfig {
			hasConfigChanges = true
		}
		// Check for breaking patterns in path
		if strings.Contains(strings.ToLower(change.Path), "breaking") ||
			strings.Contains(strings.ToLower(change.Path), "migration") {
			hasBreakingChanges = true
		}
	}

	// Suggest release type
	if hasBreakingChanges {
		return "major"
	}
	if hasSourceChanges && totalLines > 100 {
		return "minor"
	}
	if hasSourceChanges || hasConfigChanges {
		return "patch"
	}
	if impact.Level == ImpactLevelTransitive {
		return "patch"
	}

	return "patch"
}

// detectPackage detects the package type and information from a directory.
func (s *serviceImpl) detectPackage(ctx context.Context, dir string) (*Package, error) {
	relPath, err := filepath.Rel(s.config.RepoPath, dir)
	if err != nil {
		relPath = dir
	}

	// Check for Go module
	if pkg := s.detectGoModule(dir, relPath); pkg != nil {
		return pkg, nil
	}

	// Check for npm package
	if pkg := s.detectNPMPackage(dir, relPath); pkg != nil {
		return pkg, nil
	}

	// Check for Python package
	if pkg := s.detectPythonPackage(dir, relPath); pkg != nil {
		return pkg, nil
	}

	// Check for Cargo package
	if pkg := s.detectCargoPackage(dir, relPath); pkg != nil {
		return pkg, nil
	}

	// Default to directory-based package if it has source files
	if hasSourceFiles(dir) {
		return &Package{
			Name: filepath.Base(dir),
			Path: relPath,
			Type: PackageTypeDirectory,
		}, nil
	}

	return nil, nil
}

func (s *serviceImpl) detectGoModule(dir, relPath string) *Package {
	goModPath := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil
	}

	// Parse module name
	lines := strings.Split(string(data), "\n")
	var moduleName string
	var deps []string

	inRequire := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			moduleName = strings.TrimPrefix(line, "module ")
		}
		if strings.HasPrefix(line, "require (") {
			inRequire = true
			continue
		}
		if line == ")" {
			inRequire = false
			continue
		}
		if inRequire && line != "" && !strings.HasPrefix(line, "//") {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				deps = append(deps, parts[0])
			}
		}
		if strings.HasPrefix(line, "require ") && !strings.Contains(line, "(") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				deps = append(deps, parts[1])
			}
		}
	}

	return &Package{
		Name:         moduleName,
		Path:         relPath,
		Type:         PackageTypeGoModule,
		Dependencies: deps,
	}
}

func (s *serviceImpl) detectNPMPackage(dir, relPath string) *Package {
	pkgJSONPath := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(pkgJSONPath)
	if err != nil {
		return nil
	}

	var pkgJSON struct {
		Name            string            `json:"name"`
		Version         string            `json:"version"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	if err := json.Unmarshal(data, &pkgJSON); err != nil {
		return nil
	}

	var deps []string
	for dep := range pkgJSON.Dependencies {
		deps = append(deps, dep)
	}

	var devDeps []string
	for dep := range pkgJSON.DevDependencies {
		devDeps = append(devDeps, dep)
	}

	return &Package{
		Name:            pkgJSON.Name,
		Path:            relPath,
		Type:            PackageTypeNPM,
		Version:         pkgJSON.Version,
		Dependencies:    deps,
		DevDependencies: devDeps,
	}
}

func (s *serviceImpl) detectPythonPackage(dir, relPath string) *Package {
	// Check for pyproject.toml
	pyprojectPath := filepath.Join(dir, "pyproject.toml")
	if data, err := os.ReadFile(pyprojectPath); err == nil {
		var pyproject map[string]any
		if err := toml.Unmarshal(data, &pyproject); err == nil {
			name := filepath.Base(dir)
			version := ""

			if project, ok := pyproject["project"].(map[string]any); ok {
				if n, ok := project["name"].(string); ok {
					name = n
				}
				if v, ok := project["version"].(string); ok {
					version = v
				}
			}

			return &Package{
				Name:    name,
				Path:    relPath,
				Type:    PackageTypePython,
				Version: version,
			}
		}
	}

	// Check for setup.py
	setupPath := filepath.Join(dir, "setup.py")
	if _, err := os.Stat(setupPath); err == nil {
		return &Package{
			Name: filepath.Base(dir),
			Path: relPath,
			Type: PackageTypePython,
		}
	}

	return nil
}

func (s *serviceImpl) detectCargoPackage(dir, relPath string) *Package {
	cargoPath := filepath.Join(dir, "Cargo.toml")
	data, err := os.ReadFile(cargoPath)
	if err != nil {
		return nil
	}

	var cargo struct {
		Package struct {
			Name    string `toml:"name"`
			Version string `toml:"version"`
		} `toml:"package"`
		Dependencies map[string]any `toml:"dependencies"`
	}

	if err := toml.Unmarshal(data, &cargo); err != nil {
		return nil
	}

	var deps []string
	for dep := range cargo.Dependencies {
		deps = append(deps, dep)
	}

	return &Package{
		Name:         cargo.Package.Name,
		Path:         relPath,
		Type:         PackageTypeCargo,
		Version:      cargo.Package.Version,
		Dependencies: deps,
	}
}

func (s *serviceImpl) shouldExclude(path string, excludePaths []string) bool {
	for _, exclude := range excludePaths {
		if strings.Contains(path, exclude) {
			return true
		}
	}
	return false
}

func (s *serviceImpl) addTransitiveImpacts(packages []*Package, directImpacts []*Impact, config *MonorepoConfig) []*Impact {
	// Build package map and dependency graph
	pkgMap := make(map[string]*Package)
	for _, pkg := range packages {
		pkgMap[pkg.Path] = pkg
		pkgMap[pkg.Name] = pkg
	}

	// Get directly affected package names/paths
	affected := make(map[string]bool)
	for _, impact := range directImpacts {
		if impact.Package != nil {
			affected[impact.Package.Path] = true
			affected[impact.Package.Name] = true
		}
	}

	// Find packages that depend on affected packages
	impactMap := make(map[string]*Impact)
	for _, impact := range directImpacts {
		if impact.Package != nil {
			impactMap[impact.Package.Path] = impact
		}
	}

	maxDepth := config.MaxTransitiveDepth
	if maxDepth == 0 {
		maxDepth = 10 // Reasonable default
	}

	// BFS to find transitive impacts
	for depth := 1; depth <= maxDepth; depth++ {
		newAffected := make(map[string]bool)

		for _, pkg := range packages {
			if _, alreadyAffected := impactMap[pkg.Path]; alreadyAffected {
				continue
			}

			// Check if any dependency is affected
			var affectedDeps []string
			for _, dep := range pkg.Dependencies {
				if affected[dep] {
					affectedDeps = append(affectedDeps, dep)
				}
			}

			if len(affectedDeps) > 0 {
				impact := &Impact{
					Package:              pkg,
					Level:                ImpactLevelTransitive,
					AffectedDependencies: affectedDeps,
					TransitiveDepth:      depth,
				}
				impact.SuggestedActions = s.suggestActions(impact)
				impactMap[pkg.Path] = impact
				newAffected[pkg.Path] = true
				newAffected[pkg.Name] = true
			}
		}

		if len(newAffected) == 0 {
			break // No new affected packages found
		}

		// Merge new affected into affected
		for k, v := range newAffected {
			affected[k] = v
		}
	}

	// Convert map to slice
	var result []*Impact
	for _, impact := range impactMap {
		result = append(result, impact)
	}

	// Sort by path
	sort.Slice(result, func(i, j int) bool {
		return result[i].Package.Path < result[j].Package.Path
	})

	return result
}

func (s *serviceImpl) suggestActions(impact *Impact) []string {
	var actions []string

	if impact.Level == ImpactLevelDirect {
		hasSourceChanges := false
		hasTestChanges := false
		hasConfigChanges := false

		for _, change := range impact.DirectChanges {
			switch change.Category {
			case FileCategorySource:
				hasSourceChanges = true
			case FileCategoryTest:
				hasTestChanges = true
			case FileCategoryConfig:
				hasConfigChanges = true
			}
		}

		if hasSourceChanges {
			actions = append(actions, "Run unit tests")
			actions = append(actions, "Review code changes")
			if !hasTestChanges {
				actions = append(actions, "Consider adding tests for new code")
			}
		}
		if hasConfigChanges {
			actions = append(actions, "Verify configuration changes")
			actions = append(actions, "Test in staging environment")
		}
	}

	if impact.Level == ImpactLevelTransitive {
		actions = append(actions, "Run integration tests")
		actions = append(actions, "Verify compatibility with updated dependencies")
	}

	if impact.RiskScore >= 70 {
		actions = append(actions, "Consider additional review")
		actions = append(actions, "Plan rollback strategy")
	}

	return actions
}

func (s *serviceImpl) buildSummary(packages []*Package, impacts []*Impact, changedFiles []ChangedFile) *Summary {
	summary := &Summary{
		TotalPackages:     len(packages),
		TotalFilesChanged: len(changedFiles),
		ChangesByCategory: make(map[FileCategory]int),
		AffectedByType:    make(map[PackageType]int),
	}

	// Count changes
	for _, file := range changedFiles {
		summary.TotalInsertions += file.Insertions
		summary.TotalDeletions += file.Deletions
		summary.ChangesByCategory[file.Category]++
	}

	// Count impacts
	highRiskThreshold := 70
	for _, impact := range impacts {
		switch impact.Level {
		case ImpactLevelDirect:
			summary.DirectlyAffected++
		case ImpactLevelTransitive:
			summary.TransitivelyAffected++
		}

		if impact.Level != ImpactLevelNone {
			summary.TotalAffected++
			summary.AffectedByType[impact.Package.Type]++

			if impact.RequiresRelease {
				summary.PackagesRequiringRelease++
			}
		}

		if impact.RiskScore >= highRiskThreshold {
			summary.HighRiskCount++
		}
	}

	// Determine overall risk level
	summary.RiskLevel = s.determineRiskLevel(summary, impacts)
	summary.RiskFactors = s.identifyRiskFactors(summary, impacts, changedFiles)

	return summary
}

func (s *serviceImpl) determineRiskLevel(summary *Summary, impacts []*Impact) RiskLevel {
	// Critical if many high-risk impacts
	if summary.HighRiskCount >= 3 {
		return RiskLevelCritical
	}

	// High if significant percentage affected or any very high risk
	if summary.TotalPackages > 0 {
		affectedPercentage := float64(summary.TotalAffected) / float64(summary.TotalPackages)
		if affectedPercentage > 0.5 {
			return RiskLevelHigh
		}
	}

	// Check for high individual risk scores
	for _, impact := range impacts {
		if impact.RiskScore >= 80 {
			return RiskLevelHigh
		}
	}

	// Medium if multiple packages affected or moderate risk
	if summary.TotalAffected > 3 || summary.HighRiskCount > 0 {
		return RiskLevelMedium
	}

	return RiskLevelLow
}

func (s *serviceImpl) identifyRiskFactors(summary *Summary, impacts []*Impact, changedFiles []ChangedFile) []string {
	var factors []string

	if summary.TotalFilesChanged > 50 {
		factors = append(factors, fmt.Sprintf("Large change set (%d files)", summary.TotalFilesChanged))
	}

	if summary.TotalInsertions+summary.TotalDeletions > 1000 {
		factors = append(factors, fmt.Sprintf("Significant code changes (%d lines)", summary.TotalInsertions+summary.TotalDeletions))
	}

	if summary.TransitivelyAffected > summary.DirectlyAffected {
		factors = append(factors, "High transitive impact")
	}

	if summary.HighRiskCount > 0 {
		factors = append(factors, fmt.Sprintf("%d high-risk package(s)", summary.HighRiskCount))
	}

	if summary.ChangesByCategory[FileCategoryConfig] > 0 {
		factors = append(factors, "Configuration changes detected")
	}

	if summary.AffectedByType[PackageTypeGoModule] > 0 && summary.AffectedByType[PackageTypeNPM] > 0 {
		factors = append(factors, "Cross-language changes")
	}

	return factors
}

// Helper functions

func categorizeFile(path string) FileCategory {
	lowerPath := strings.ToLower(path)
	ext := strings.ToLower(filepath.Ext(path))
	baseName := filepath.Base(path)
	lowerBaseName := strings.ToLower(baseName)

	// Dependencies - check FIRST to avoid .txt matching docs
	depFiles := map[string]bool{
		"package.json": true, "package-lock.json": true, "yarn.lock": true,
		"go.mod": true, "go.sum": true, "requirements.txt": true,
		"pipfile": true, "pipfile.lock": true, "cargo.toml": true, "cargo.lock": true,
		"composer.json": true, "composer.lock": true, "gemfile": true, "gemfile.lock": true,
		"pnpm-lock.yaml": true, "poetry.lock": true, "pyproject.toml": true,
	}
	if depFiles[lowerBaseName] {
		return FileCategoryDependency
	}

	// Generated files - check BEFORE source code
	// Match: generated/, /generated/, .gen., _generated., .pb.go, _generated/
	if strings.HasPrefix(lowerPath, "generated/") || strings.Contains(lowerPath, "/generated/") ||
		strings.Contains(lowerPath, ".gen.") || strings.Contains(lowerPath, "_generated.") ||
		strings.Contains(lowerPath, ".pb.go") || strings.Contains(lowerPath, "_generated/") {
		return FileCategoryGenerated
	}

	// Test files
	testPatterns := []string{"_test.go", ".test.ts", ".test.js", ".spec.ts", ".spec.js", "_test.py", "test_"}
	for _, pattern := range testPatterns {
		if strings.Contains(lowerPath, pattern) {
			return FileCategoryTest
		}
	}
	if strings.Contains(lowerPath, "/test/") || strings.Contains(lowerPath, "/tests/") ||
		strings.Contains(lowerPath, "/__tests__/") {
		return FileCategoryTest
	}

	// Documentation (uses package-level docExtensions map)
	if docExtensions[ext] || strings.Contains(lowerPath, "/docs/") || strings.Contains(lowerPath, "/documentation/") {
		return FileCategoryDocs
	}

	// CI/CD
	if strings.Contains(lowerPath, ".github/") || strings.Contains(lowerPath, ".gitlab-ci") ||
		strings.Contains(lowerPath, "jenkinsfile") || strings.Contains(lowerPath, ".circleci/") ||
		strings.Contains(lowerPath, ".travis") {
		return FileCategoryCI
	}

	// Build files (uses package-level buildFiles map)
	if buildFiles[strings.ToLower(filepath.Base(path))] {
		return FileCategoryBuild
	}

	// Config files (uses package-level configExtensions and configFiles maps)
	if configExtensions[ext] || configFiles[filepath.Base(path)] {
		return FileCategoryConfig
	}

	// Assets (uses package-level assetExtensions map)
	if assetExtensions[ext] {
		return FileCategoryAsset
	}

	// Source code (uses package-level sourceExtensions map)
	if sourceExtensions[ext] {
		return FileCategorySource
	}

	return FileCategoryOther
}

func isFileInPackage(filePath, packagePath string) bool {
	if packagePath == "." {
		return true
	}
	// Normalize paths
	filePath = filepath.Clean(filePath)
	packagePath = filepath.Clean(packagePath)

	// Check if file is in package directory
	return strings.HasPrefix(filePath, packagePath+string(filepath.Separator)) ||
		filePath == packagePath
}

func hasSourceFiles(dir string) bool {
	sourceExtensions := []string{".go", ".ts", ".js", ".py", ".rs", ".java", ".rb"}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		for _, srcExt := range sourceExtensions {
			if ext == srcExt {
				return true
			}
		}
	}
	return false
}

func filterByCategory(files []ChangedFile, category FileCategory, include bool) []ChangedFile {
	var result []ChangedFile
	for _, file := range files {
		if include {
			if file.Category == category {
				result = append(result, file)
			}
		} else {
			if file.Category != category {
				result = append(result, file)
			}
		}
	}
	return result
}

// FormatBlastRadius formats the blast radius analysis for display.
func FormatBlastRadius(br *BlastRadius, verbose bool) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("Blast Radius Analysis (%s → %s)\n", br.FromRef, br.ToRef))
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")

	// Summary
	s := br.Summary
	sb.WriteString("Summary\n")
	sb.WriteString(strings.Repeat("-", 30) + "\n")
	sb.WriteString(fmt.Sprintf("Total Packages:     %d\n", s.TotalPackages))
	sb.WriteString(fmt.Sprintf("Directly Affected:  %d\n", s.DirectlyAffected))
	sb.WriteString(fmt.Sprintf("Transitively Affected: %d\n", s.TransitivelyAffected))
	sb.WriteString(fmt.Sprintf("Files Changed:      %d (+%d/-%d lines)\n",
		s.TotalFilesChanged, s.TotalInsertions, s.TotalDeletions))
	sb.WriteString(fmt.Sprintf("Risk Level:         %s\n", strings.ToUpper(string(s.RiskLevel))))

	if len(s.RiskFactors) > 0 {
		sb.WriteString("\nRisk Factors:\n")
		for _, factor := range s.RiskFactors {
			sb.WriteString(fmt.Sprintf("  - %s\n", factor))
		}
	}

	// Impacts
	if len(br.Impacts) > 0 {
		sb.WriteString("\n\nImpacted Packages\n")
		sb.WriteString(strings.Repeat("-", 30) + "\n")

		for _, impact := range br.Impacts {
			levelIcon := "  "
			switch impact.Level {
			case ImpactLevelDirect:
				levelIcon = "* "
			case ImpactLevelTransitive:
				levelIcon = "~ "
			}

			sb.WriteString(fmt.Sprintf("%s%s (%s)\n", levelIcon, impact.Package.Name, impact.Package.Type))
			sb.WriteString(fmt.Sprintf("   Path: %s\n", impact.Package.Path))
			sb.WriteString(fmt.Sprintf("   Impact: %s (risk: %d/100)\n", impact.Level, impact.RiskScore))

			if impact.RequiresRelease {
				sb.WriteString(fmt.Sprintf("   Suggested Release: %s\n", impact.ReleaseType))
			}

			if verbose {
				if len(impact.DirectChanges) > 0 {
					sb.WriteString("   Changed Files:\n")
					for _, change := range impact.DirectChanges {
						sb.WriteString(fmt.Sprintf("     - %s [%s] (+%d/-%d)\n",
							change.Path, change.Category, change.Insertions, change.Deletions))
					}
				}

				if len(impact.AffectedDependencies) > 0 {
					sb.WriteString("   Affected Dependencies:\n")
					for _, dep := range impact.AffectedDependencies {
						sb.WriteString(fmt.Sprintf("     - %s\n", dep))
					}
				}

				if len(impact.SuggestedActions) > 0 {
					sb.WriteString("   Suggested Actions:\n")
					for _, action := range impact.SuggestedActions {
						sb.WriteString(fmt.Sprintf("     - %s\n", action))
					}
				}
			}

			sb.WriteString("\n")
		}
	}

	// Legend
	sb.WriteString("Legend: * = Direct, ~ = Transitive\n")

	return sb.String()
}

// Regular expression for parsing Go module names.
var _ = regexp.MustCompile(`^module\s+(\S+)`) // Reserved for future use

// FormatBlastRadiusJSON formats the blast radius analysis as JSON.
func FormatBlastRadiusJSON(br *BlastRadius) (string, error) {
	data, err := json.MarshalIndent(br, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FormatBlastRadiusYAML formats the blast radius analysis as YAML.
func FormatBlastRadiusYAML(br *BlastRadius) (string, error) {
	data, err := yaml.Marshal(br)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
