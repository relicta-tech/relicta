// Package workspace provides workspace detection and management for monorepos.
package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// FSDetector implements the Detector interface using the filesystem.
// It auto-detects workspace type from marker files and configuration,
// supporting Go modules (go.work), npm workspaces (package.json),
// Cargo workspaces (Cargo.toml), and manual overrides from .relicta.yaml.
type FSDetector struct{}

// NewFSDetector creates a new filesystem-based workspace detector.
func NewFSDetector() *FSDetector {
	return &FSDetector{}
}

// Detect attempts to detect a workspace starting from the given path.
// It searches the path itself and optionally upward for workspace root indicators.
func (d *FSDetector) Detect(ctx context.Context, startPath string, opts DetectionOptions) (*DetectionResult, error) {
	start := time.Now()
	result := &DetectionResult{
		CheckedPaths: make([]string, 0),
	}

	absPath, err := filepath.Abs(startPath)
	if err != nil {
		result.Error = fmt.Errorf("resolving path %s: %w", startPath, err)
		result.Duration = time.Since(start)
		return result, nil
	}

	// Search from startPath upward, respecting MaxDepth
	currentPath := absPath
	for depth := 0; depth <= opts.MaxDepth; depth++ {
		select {
		case <-ctx.Done():
			result.Error = ctx.Err()
			result.Duration = time.Since(start)
			return result, nil
		default:
		}

		result.CheckedPaths = append(result.CheckedPaths, currentPath)

		detected, err := d.DetectFromRoot(ctx, currentPath, opts)
		if err != nil {
			result.Error = err
			result.Duration = time.Since(start)
			return result, nil
		}

		if detected.Success() {
			result.Workspace = detected.Workspace
			result.Duration = time.Since(start)
			return result, nil
		}

		// Move up one directory
		parent := filepath.Dir(currentPath)
		if parent == currentPath {
			break // Reached filesystem root
		}
		currentPath = parent
	}

	// No workspace found
	result.Workspace = &Workspace{
		RootPath: absPath,
		Type:     WorkspaceTypeNone,
	}
	result.Duration = time.Since(start)
	return result, nil
}

// DetectFromRoot detects workspace configuration assuming the path is already the root.
func (d *FSDetector) DetectFromRoot(ctx context.Context, rootPath string, opts DetectionOptions) (*DetectionResult, error) {
	start := time.Now()
	result := &DetectionResult{
		CheckedPaths: []string{rootPath},
	}

	markers := DefaultMarkerFiles()

	// Allow custom markers to override
	if opts.CustomMarkers != nil {
		for name, wsType := range opts.CustomMarkers {
			markers = append(markers, MarkerFile{
				Name:     name,
				Type:     wsType,
				Priority: 200, // Custom markers have highest priority
			})
		}
	}

	// Sort markers by priority (highest first)
	sort.Slice(markers, func(i, j int) bool {
		return markers[i].Priority > markers[j].Priority
	})

	// Check each marker
	var bestMatch *markerMatch
	for _, marker := range markers {
		// Filter by preferred types if specified
		if len(opts.PreferredTypes) > 0 && marker.Type != WorkspaceTypeNone {
			found := false
			for _, pt := range opts.PreferredTypes {
				if pt == marker.Type {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		markerPath := filepath.Join(rootPath, marker.Name)
		if _, err := os.Stat(markerPath); err != nil {
			continue
		}

		wsType := marker.Type
		confidence := 0.8

		// For markers requiring content parsing, validate the content
		if marker.RequiresContent {
			parsed, conf, err := d.parseMarkerContent(markerPath, marker)
			if err != nil || parsed == WorkspaceTypeNone {
				continue
			}
			wsType = parsed
			confidence = conf
		}

		if wsType == WorkspaceTypeNone {
			// Lock files only indicate package manager, not workspace type
			continue
		}

		match := &markerMatch{
			wsType:         wsType,
			packageManager: marker.PackageManager,
			markerFile:     markerPath,
			priority:       marker.Priority,
			confidence:     confidence,
		}

		if bestMatch == nil || match.priority > bestMatch.priority {
			bestMatch = match
		}
	}

	if bestMatch == nil {
		result.Duration = time.Since(start)
		return result, nil
	}

	ws := &Workspace{
		ID:             filepath.Base(rootPath),
		RootPath:       rootPath,
		Type:           bestMatch.wsType,
		PackageManager: bestMatch.packageManager,
		Strategy:       StrategyIndependent,
		ConfigFile:     bestMatch.markerFile,
		DetectedAt:     time.Now(),
		Confidence:     bestMatch.confidence,
		Markers:        []string{bestMatch.markerFile},
	}

	// Detect package manager from lock files
	ws.PackageManager = d.detectPackageManager(rootPath, ws.PackageManager)

	// Discover packages if requested
	if opts.IncludePackages {
		packages, err := d.DiscoverPackages(ctx, ws)
		if err != nil {
			// Non-fatal: workspace detected but packages could not be discovered
			ws.Packages = make([]*Package, 0)
		} else {
			ws.Packages = packages
		}
	}

	result.Workspace = ws
	result.Duration = time.Since(start)
	return result, nil
}

// DetectType returns just the workspace type without full package discovery.
func (d *FSDetector) DetectType(ctx context.Context, path string) (WorkspaceType, error) {
	opts := DetectionOptions{
		MaxDepth:        0,
		IncludePackages: false,
	}
	result, err := d.DetectFromRoot(ctx, path, opts)
	if err != nil {
		return WorkspaceTypeNone, err
	}
	if result.Workspace == nil {
		return WorkspaceTypeNone, nil
	}
	return result.Workspace.Type, nil
}

// DiscoverPackages discovers all packages within a detected workspace.
func (d *FSDetector) DiscoverPackages(ctx context.Context, ws *Workspace) ([]*Package, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is nil")
	}

	switch ws.Type {
	case WorkspaceTypeGoModule:
		return d.discoverGoWorkspacePackages(ws)
	case WorkspaceTypeNpm, WorkspaceTypeYarn, WorkspaceTypePnpm:
		return d.discoverNpmWorkspacePackages(ws)
	case WorkspaceTypeCargo:
		return d.discoverCargoWorkspacePackages(ws)
	case WorkspaceTypeCustom:
		return d.discoverPackagesByGlob(ws)
	default:
		return d.discoverPackagesByGlob(ws)
	}
}

// ValidateWorkspace checks if a workspace configuration is valid and consistent.
func (d *FSDetector) ValidateWorkspace(ctx context.Context, ws *Workspace) error {
	if ws == nil {
		return fmt.Errorf("workspace is nil")
	}

	if ws.RootPath == "" {
		return fmt.Errorf("workspace root path is empty")
	}

	if _, err := os.Stat(ws.RootPath); err != nil {
		return fmt.Errorf("workspace root path does not exist: %w", err)
	}

	// Validate package paths exist
	for _, pkg := range ws.Packages {
		pkgPath := filepath.Join(ws.RootPath, pkg.Path)
		if _, err := os.Stat(pkgPath); err != nil {
			return fmt.Errorf("package path %s does not exist: %w", pkg.Path, err)
		}
	}

	// Check for duplicate package names
	names := make(map[string]bool)
	for _, pkg := range ws.Packages {
		if names[pkg.Name] {
			return fmt.Errorf("duplicate package name: %s", pkg.Name)
		}
		names[pkg.Name] = true
	}

	return nil
}

// markerMatch holds a matched workspace marker.
type markerMatch struct {
	wsType         WorkspaceType
	packageManager PackageManagerType
	markerFile     string
	priority       int
	confidence     float64
}

// parseMarkerContent parses a marker file's content to confirm workspace type.
func (d *FSDetector) parseMarkerContent(path string, marker MarkerFile) (WorkspaceType, float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return WorkspaceTypeNone, 0, err
	}

	switch marker.Name {
	case "package.json":
		return d.parsePackageJSON(data)
	case "Cargo.toml":
		return d.parseCargoToml(data)
	case "pom.xml":
		return d.parsePomXML(data)
	default:
		return WorkspaceTypeNone, 0, nil
	}
}

// parsePackageJSON checks if package.json has a workspaces field.
func (d *FSDetector) parsePackageJSON(data []byte) (WorkspaceType, float64, error) {
	var pkg struct {
		Workspaces interface{} `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return WorkspaceTypeNone, 0, err
	}

	if pkg.Workspaces == nil {
		return WorkspaceTypeNone, 0, nil
	}

	// Workspaces can be an array of strings or an object with packages field
	return WorkspaceTypeNpm, 0.9, nil
}

// parseCargoToml checks if Cargo.toml has a [workspace] section.
func (d *FSDetector) parseCargoToml(data []byte) (WorkspaceType, float64, error) {
	content := string(data)
	if strings.Contains(content, "[workspace]") {
		return WorkspaceTypeCargo, 0.95, nil
	}
	return WorkspaceTypeNone, 0, nil
}

// parsePomXML checks if pom.xml has a <modules> section.
func (d *FSDetector) parsePomXML(data []byte) (WorkspaceType, float64, error) {
	content := string(data)
	if strings.Contains(content, "<modules>") {
		return WorkspaceTypeMaven, 0.9, nil
	}
	return WorkspaceTypeNone, 0, nil
}

// detectPackageManager refines the package manager detection using lock files.
func (d *FSDetector) detectPackageManager(rootPath string, fallback PackageManagerType) PackageManagerType {
	lockFiles := map[string]PackageManagerType{
		"pnpm-lock.yaml":    PackageManagerPnpm,
		"yarn.lock":         PackageManagerYarn,
		"package-lock.json": PackageManagerNpm,
		"bun.lockb":         PackageManagerBun,
	}

	for file, pm := range lockFiles {
		if _, err := os.Stat(filepath.Join(rootPath, file)); err == nil {
			return pm
		}
	}

	return fallback
}

// discoverGoWorkspacePackages discovers Go modules from go.work.
func (d *FSDetector) discoverGoWorkspacePackages(ws *Workspace) ([]*Package, error) {
	goWorkPath := filepath.Join(ws.RootPath, "go.work")
	data, err := os.ReadFile(goWorkPath)
	if err != nil {
		return nil, fmt.Errorf("reading go.work: %w", err)
	}

	content := string(data)
	modulePaths := make([]string, 0)

	// Handle multi-line use blocks: use ( ... )
	blockRe := regexp.MustCompile(`(?s)use\s*\(\s*(.*?)\s*\)`)
	blockMatches := blockRe.FindAllStringSubmatch(content, -1)
	for _, block := range blockMatches {
		lines := strings.Split(block[1], "\n")
		for _, line := range lines {
			path := strings.TrimSpace(line)
			if path != "" && !strings.HasPrefix(path, "//") {
				modulePaths = append(modulePaths, path)
			}
		}
	}

	// Handle single-line use directives: use ./path
	// Only match if not followed by '(' (which starts a block)
	singleRe := regexp.MustCompile(`(?m)^\s*use\s+([^\s(]\S*)`)
	for _, m := range singleRe.FindAllStringSubmatch(content, -1) {
		path := strings.TrimSpace(m[1])
		if path != "" {
			modulePaths = append(modulePaths, path)
		}
	}

	// Normalize paths: strip leading "./"
	for i, p := range modulePaths {
		modulePaths[i] = strings.TrimPrefix(p, "./")
	}

	// Deduplicate
	seen := make(map[string]bool)
	var uniquePaths []string
	for _, p := range modulePaths {
		if !seen[p] {
			seen[p] = true
			uniquePaths = append(uniquePaths, p)
		}
	}

	packages := make([]*Package, 0, len(uniquePaths))
	for _, modPath := range uniquePaths {
		absModPath := filepath.Join(ws.RootPath, modPath)

		// Read go.mod for module name
		goModPath := filepath.Join(absModPath, "go.mod")
		name := filepath.Base(modPath)
		version := ""

		if goModData, err := os.ReadFile(goModPath); err == nil {
			moduleRe := regexp.MustCompile(`(?m)^module\s+(\S+)`)
			if moduleMatches := moduleRe.FindStringSubmatch(string(goModData)); len(moduleMatches) > 1 {
				name = moduleMatches[1]
			}
		}

		pkg := &Package{
			Name:         name,
			Path:         modPath,
			Version:      version,
			Dependencies: make(map[string]string),
		}

		// Parse dependencies from go.mod require blocks
		d.parseGoModDependencies(absModPath, pkg, uniquePaths)

		packages = append(packages, pkg)
	}

	// Resolve internal dependencies
	d.resolveInternalDependencies(packages)

	return packages, nil
}

// parseGoModDependencies parses go.mod for dependency information.
func (d *FSDetector) parseGoModDependencies(modPath string, pkg *Package, workspacePaths []string) {
	goModPath := filepath.Join(modPath, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return
	}

	// Parse require directives
	requireRe := regexp.MustCompile(`(?m)^\s*require\s+(\S+)\s+(\S+)`)
	for _, m := range requireRe.FindAllStringSubmatch(string(data), -1) {
		pkg.Dependencies[m[1]] = m[2]
	}

	// Parse require block
	blockRe := regexp.MustCompile(`(?s)require\s*\(\s*(.*?)\s*\)`)
	for _, block := range blockRe.FindAllStringSubmatch(string(data), -1) {
		lineRe := regexp.MustCompile(`(?m)^\s*(\S+)\s+(\S+)`)
		for _, m := range lineRe.FindAllStringSubmatch(block[1], -1) {
			pkg.Dependencies[m[1]] = m[2]
		}
	}
}

// discoverNpmWorkspacePackages discovers packages from npm/yarn/pnpm workspaces.
func (d *FSDetector) discoverNpmWorkspacePackages(ws *Workspace) ([]*Package, error) {
	var patterns []string

	switch ws.Type {
	case WorkspaceTypePnpm:
		// Read pnpm-workspace.yaml
		pnpmPath := filepath.Join(ws.RootPath, "pnpm-workspace.yaml")
		data, err := os.ReadFile(pnpmPath)
		if err == nil {
			// Simple YAML parsing for packages field
			re := regexp.MustCompile(`(?m)^\s*-\s*['"]?([^'"#\n]+)['"]?`)
			for _, m := range re.FindAllStringSubmatch(string(data), -1) {
				patterns = append(patterns, strings.TrimSpace(m[1]))
			}
		}
	default:
		// Read package.json workspaces field
		pkgJSONPath := filepath.Join(ws.RootPath, "package.json")
		data, err := os.ReadFile(pkgJSONPath)
		if err != nil {
			return nil, fmt.Errorf("reading package.json: %w", err)
		}

		var root struct {
			Workspaces interface{} `json:"workspaces"`
		}
		if err := json.Unmarshal(data, &root); err != nil {
			return nil, fmt.Errorf("parsing package.json: %w", err)
		}

		switch v := root.Workspaces.(type) {
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					patterns = append(patterns, s)
				}
			}
		case map[string]interface{}:
			if pkgs, ok := v["packages"].([]interface{}); ok {
				for _, item := range pkgs {
					if s, ok := item.(string); ok {
						patterns = append(patterns, s)
					}
				}
			}
		}
	}

	ws.PackagePaths = patterns

	// Glob each pattern to find package directories
	packages := make([]*Package, 0)
	for _, pattern := range patterns {
		globPattern := filepath.Join(ws.RootPath, pattern)
		matches, err := filepath.Glob(globPattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			// Each match should contain a package.json
			pkgJSONPath := filepath.Join(match, "package.json")
			data, err := os.ReadFile(pkgJSONPath)
			if err != nil {
				continue
			}

			var pkgJSON struct {
				Name            string            `json:"name"`
				Version         string            `json:"version"`
				Private         bool              `json:"private"`
				Dependencies    map[string]string `json:"dependencies"`
				DevDependencies map[string]string `json:"devDependencies"`
			}
			if err := json.Unmarshal(data, &pkgJSON); err != nil {
				continue
			}

			relPath, _ := filepath.Rel(ws.RootPath, match)
			pkg := &Package{
				Name:            pkgJSON.Name,
				Path:            relPath,
				Version:         pkgJSON.Version,
				Private:         pkgJSON.Private,
				Dependencies:    pkgJSON.Dependencies,
				DevDependencies: pkgJSON.DevDependencies,
			}

			packages = append(packages, pkg)
		}
	}

	// Resolve internal dependencies
	d.resolveInternalDependencies(packages)

	return packages, nil
}

// discoverCargoWorkspacePackages discovers Cargo workspace members.
func (d *FSDetector) discoverCargoWorkspacePackages(ws *Workspace) ([]*Package, error) {
	cargoPath := filepath.Join(ws.RootPath, "Cargo.toml")
	data, err := os.ReadFile(cargoPath)
	if err != nil {
		return nil, fmt.Errorf("reading Cargo.toml: %w", err)
	}

	content := string(data)

	// Extract workspace members
	membersRe := regexp.MustCompile(`(?s)\[workspace\].*?members\s*=\s*\[\s*(.*?)\s*\]`)
	membersMatch := membersRe.FindStringSubmatch(content)
	if len(membersMatch) < 2 {
		return nil, fmt.Errorf("no workspace members found in Cargo.toml")
	}

	// Parse member patterns
	memberRe := regexp.MustCompile(`"([^"]+)"`)
	memberMatches := memberRe.FindAllStringSubmatch(membersMatch[1], -1)

	packages := make([]*Package, 0)
	for _, m := range memberMatches {
		pattern := m[1]
		globPattern := filepath.Join(ws.RootPath, pattern)
		matches, err := filepath.Glob(globPattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			memberCargoPath := filepath.Join(match, "Cargo.toml")
			memberData, err := os.ReadFile(memberCargoPath)
			if err != nil {
				continue
			}

			memberContent := string(memberData)
			name := filepath.Base(match)
			version := ""

			// Extract package name
			nameRe := regexp.MustCompile(`(?m)^\s*name\s*=\s*"([^"]+)"`)
			if nameMatch := nameRe.FindStringSubmatch(memberContent); len(nameMatch) > 1 {
				name = nameMatch[1]
			}

			// Extract version
			versionRe := regexp.MustCompile(`(?m)^\s*version\s*=\s*"([^"]+)"`)
			if versionMatch := versionRe.FindStringSubmatch(memberContent); len(versionMatch) > 1 {
				version = versionMatch[1]
			}

			relPath, _ := filepath.Rel(ws.RootPath, match)
			pkg := &Package{
				Name:         name,
				Path:         relPath,
				Version:      version,
				Dependencies: make(map[string]string),
			}

			// Parse dependencies
			depRe := regexp.MustCompile(`(?m)^\s*(\S+)\s*=\s*\{.*?path\s*=`)
			for _, dep := range depRe.FindAllStringSubmatch(memberContent, -1) {
				pkg.Dependencies[dep[1]] = "workspace"
			}

			packages = append(packages, pkg)
		}
	}

	// Resolve internal dependencies
	d.resolveInternalDependencies(packages)

	return packages, nil
}

// discoverPackagesByGlob discovers packages using glob patterns from workspace config.
func (d *FSDetector) discoverPackagesByGlob(ws *Workspace) ([]*Package, error) {
	if len(ws.PackagePaths) == 0 {
		// Use default patterns
		ws.PackagePaths = []string{"packages/*", "apps/*", "libs/*"}
	}

	packages := make([]*Package, 0)
	for _, pattern := range ws.PackagePaths {
		globPattern := filepath.Join(ws.RootPath, pattern)
		matches, err := filepath.Glob(globPattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || !info.IsDir() {
				continue
			}

			// Skip excluded paths
			relPath, _ := filepath.Rel(ws.RootPath, match)
			if d.isExcluded(relPath, ws.ExcludePaths) {
				continue
			}

			name := filepath.Base(match)
			pkg := &Package{
				Name:         name,
				Path:         relPath,
				Dependencies: make(map[string]string),
			}

			packages = append(packages, pkg)
		}
	}

	return packages, nil
}

// resolveInternalDependencies identifies which dependencies are internal to the workspace.
func (d *FSDetector) resolveInternalDependencies(packages []*Package) {
	// Build a map of package names to packages
	nameMap := make(map[string]*Package)
	for _, pkg := range packages {
		nameMap[pkg.Name] = pkg
	}

	// For each package, check if any of its dependencies are internal
	for _, pkg := range packages {
		for depName := range pkg.Dependencies {
			if _, isInternal := nameMap[depName]; isInternal {
				pkg.InternalDependencies = append(pkg.InternalDependencies, depName)
			}
		}
		for depName := range pkg.DevDependencies {
			if _, isInternal := nameMap[depName]; isInternal {
				// Add if not already in internal deps
				found := false
				for _, existing := range pkg.InternalDependencies {
					if existing == depName {
						found = true
						break
					}
				}
				if !found {
					pkg.InternalDependencies = append(pkg.InternalDependencies, depName)
				}
			}
		}
	}
}

// isExcluded checks if a path matches any of the exclude patterns.
func (d *FSDetector) isExcluded(path string, excludePatterns []string) bool {
	for _, pattern := range excludePatterns {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}
	return false
}

// DefaultConfigResolver is the default implementation of ConfigResolver.
type DefaultConfigResolver struct{}

// NewDefaultConfigResolver creates a new default config resolver.
func NewDefaultConfigResolver() *DefaultConfigResolver {
	return &DefaultConfigResolver{}
}

// ResolveConfig finds and parses the workspace configuration from .relicta.yaml.
func (r *DefaultConfigResolver) ResolveConfig(ctx context.Context, rootPath string) (*WorkspaceConfig, error) {
	// This would typically be handled by the config package
	// returning nil indicates no explicit workspace config found
	return nil, nil
}

// MergeConfig merges detected configuration with explicit user configuration.
// User configuration takes precedence over detected values.
func (r *DefaultConfigResolver) MergeConfig(detected, explicit *WorkspaceConfig) *WorkspaceConfig {
	if explicit == nil {
		return detected
	}
	if detected == nil {
		return explicit
	}

	merged := *detected

	if explicit.Type != "" {
		merged.Type = explicit.Type
	}
	if explicit.Strategy != "" {
		merged.Strategy = explicit.Strategy
	}
	if len(explicit.PackagePaths) > 0 {
		merged.PackagePaths = explicit.PackagePaths
	}
	if len(explicit.ExcludePaths) > 0 {
		merged.ExcludePaths = explicit.ExcludePaths
	}
	if len(explicit.ReleaseGroups) > 0 {
		merged.ReleaseGroups = explicit.ReleaseGroups
	}

	return &merged
}
