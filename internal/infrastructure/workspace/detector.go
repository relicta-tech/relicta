// Package workspace provides infrastructure implementations for workspace detection.
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/workspace"
)

var (
	// ErrNoWorkspaceFound indicates no workspace was detected.
	ErrNoWorkspaceFound = errors.New("no workspace found")

	// ErrInvalidWorkspace indicates the workspace configuration is invalid.
	ErrInvalidWorkspace = errors.New("invalid workspace configuration")
)

// FileDetector implements workspace.Detector using filesystem operations.
type FileDetector struct {
	markers []workspace.MarkerFile
}

// NewFileDetector creates a new FileDetector with default markers.
func NewFileDetector() *FileDetector {
	return &FileDetector{
		markers: workspace.DefaultMarkerFiles(),
	}
}

// NewFileDetectorWithMarkers creates a FileDetector with custom markers.
func NewFileDetectorWithMarkers(markers []workspace.MarkerFile) *FileDetector {
	return &FileDetector{
		markers: markers,
	}
}

// Detect implements workspace.Detector.
func (d *FileDetector) Detect(ctx context.Context, startPath string, opts workspace.DetectionOptions) (*workspace.DetectionResult, error) {
	start := time.Now()
	result := &workspace.DetectionResult{
		CheckedPaths: make([]string, 0),
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		result.Error = fmt.Errorf("failed to resolve path: %w", err)
		result.Duration = time.Since(start)
		return result, nil
	}

	// Search upward from the start path
	currentPath := absPath
	maxDepth := opts.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 10 // reasonable default
	}

	for depth := 0; depth < maxDepth; depth++ {
		select {
		case <-ctx.Done():
			result.Error = ctx.Err()
			result.Duration = time.Since(start)
			return result, nil
		default:
		}

		result.CheckedPaths = append(result.CheckedPaths, currentPath)

		// Check for workspace markers at this level
		ws, err := d.checkDirectory(ctx, currentPath, opts)
		if err != nil {
			result.Error = err
			result.Duration = time.Since(start)
			return result, nil
		}

		if ws != nil {
			// Found a workspace
			if opts.IncludePackages {
				packages, err := d.DiscoverPackages(ctx, ws)
				if err != nil {
					// Don't fail detection, just note the error
					ws.Packages = nil
				} else {
					ws.Packages = packages
				}
			}

			result.Workspace = ws
			result.Duration = time.Since(start)
			return result, nil
		}

		// Move up to parent directory
		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			// Reached filesystem root
			break
		}
		currentPath = parentPath
	}

	// No workspace found, return empty workspace
	result.Workspace = &workspace.Workspace{
		RootPath:   absPath,
		Type:       workspace.WorkspaceTypeNone,
		DetectedAt: time.Now(),
		Confidence: 1.0,
	}
	result.Duration = time.Since(start)
	return result, nil
}

// DetectFromRoot implements workspace.Detector.
func (d *FileDetector) DetectFromRoot(ctx context.Context, rootPath string, opts workspace.DetectionOptions) (*workspace.DetectionResult, error) {
	start := time.Now()
	result := &workspace.DetectionResult{
		CheckedPaths: []string{rootPath},
	}

	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		result.Error = fmt.Errorf("failed to resolve path: %w", err)
		result.Duration = time.Since(start)
		return result, nil
	}

	ws, err := d.checkDirectory(ctx, absPath, opts)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result, nil
	}

	if ws == nil {
		ws = &workspace.Workspace{
			RootPath:   absPath,
			Type:       workspace.WorkspaceTypeNone,
			DetectedAt: time.Now(),
			Confidence: 1.0,
		}
	} else if opts.IncludePackages {
		packages, err := d.DiscoverPackages(ctx, ws)
		if err == nil {
			ws.Packages = packages
		}
	}

	result.Workspace = ws
	result.Duration = time.Since(start)
	return result, nil
}

// DetectType implements workspace.Detector.
func (d *FileDetector) DetectType(ctx context.Context, path string) (workspace.WorkspaceType, error) {
	opts := workspace.DetectionOptions{
		MaxDepth:        5,
		IncludePackages: false,
	}

	result, err := d.Detect(ctx, path, opts)
	if err != nil {
		return workspace.WorkspaceTypeNone, err
	}

	if result.Workspace == nil {
		return workspace.WorkspaceTypeNone, nil
	}

	return result.Workspace.Type, nil
}

// DiscoverPackages implements workspace.Detector.
func (d *FileDetector) DiscoverPackages(ctx context.Context, ws *workspace.Workspace) ([]*workspace.Package, error) {
	if ws == nil || ws.RootPath == "" {
		return nil, ErrInvalidWorkspace
	}

	packages := make([]*workspace.Package, 0)

	// Use package paths from workspace, or defaults based on type
	patterns := ws.PackagePaths
	if len(patterns) == 0 {
		patterns = d.defaultPackagePatterns(ws.Type)
	}

	for _, pattern := range patterns {
		// Expand glob pattern
		fullPattern := filepath.Join(ws.RootPath, pattern)
		matches, err := filepath.Glob(fullPattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			select {
			case <-ctx.Done():
				return packages, ctx.Err()
			default:
			}

			// Check for exclude patterns
			if d.shouldExclude(match, ws.ExcludePaths, ws.RootPath) {
				continue
			}

			// Try to parse package at this location
			pkg, err := d.parsePackage(ctx, match, ws)
			if err != nil {
				continue
			}

			if pkg != nil {
				packages = append(packages, pkg)
			}
		}
	}

	return packages, nil
}

// ValidateWorkspace implements workspace.Detector.
func (d *FileDetector) ValidateWorkspace(ctx context.Context, ws *workspace.Workspace) error {
	if ws == nil {
		return ErrInvalidWorkspace
	}

	if ws.RootPath == "" {
		return fmt.Errorf("%w: root path is empty", ErrInvalidWorkspace)
	}

	// Check that root path exists
	info, err := os.Stat(ws.RootPath)
	if err != nil {
		return fmt.Errorf("%w: root path does not exist: %w", ErrInvalidWorkspace, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%w: root path is not a directory", ErrInvalidWorkspace)
	}

	return nil
}

// checkDirectory checks a single directory for workspace markers.
func (d *FileDetector) checkDirectory(ctx context.Context, dirPath string, opts workspace.DetectionOptions) (*workspace.Workspace, error) {
	// Sort markers by priority (highest first)
	markers := make([]workspace.MarkerFile, len(d.markers))
	copy(markers, d.markers)
	sort.Slice(markers, func(i, j int) bool {
		return markers[i].Priority > markers[j].Priority
	})

	var bestMatch *markerMatch
	detectedPackageManager := workspace.PackageManagerUnknown

	for _, marker := range markers {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Check if preferred types filter applies
		if len(opts.PreferredTypes) > 0 && marker.Type != workspace.WorkspaceTypeNone {
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

		markerPath := filepath.Join(dirPath, marker.Name)
		if _, err := os.Stat(markerPath); err != nil {
			continue
		}

		// Found a marker file
		match := &markerMatch{
			marker:   marker,
			path:     markerPath,
			priority: marker.Priority,
		}

		// Update package manager detection
		if marker.PackageManager != workspace.PackageManagerUnknown {
			if detectedPackageManager == workspace.PackageManagerUnknown {
				detectedPackageManager = marker.PackageManager
			}
		}

		// If this marker requires content parsing
		if marker.RequiresContent {
			isWorkspace, wsType, err := d.parseMarkerContent(ctx, markerPath, marker)
			if err != nil || !isWorkspace {
				continue
			}
			match.marker.Type = wsType
		}

		// Only consider markers that indicate an actual workspace
		if match.marker.Type == workspace.WorkspaceTypeNone {
			continue
		}

		if bestMatch == nil || match.priority > bestMatch.priority {
			bestMatch = match
		}
	}

	if bestMatch == nil {
		return nil, nil
	}

	// Build the workspace
	ws := &workspace.Workspace{
		ID:             generateWorkspaceID(dirPath),
		RootPath:       dirPath,
		Type:           bestMatch.marker.Type,
		PackageManager: detectedPackageManager,
		Strategy:       workspace.StrategyIndependent,
		PackagePaths:   d.defaultPackagePatterns(bestMatch.marker.Type),
		ConfigFile:     bestMatch.path,
		DetectedAt:     time.Now(),
		Confidence:     d.calculateConfidence(bestMatch),
		Markers:        []string{bestMatch.marker.Name},
	}

	// Try to detect the package manager if not already set
	if ws.PackageManager == workspace.PackageManagerUnknown || ws.PackageManager == "" {
		ws.PackageManager = d.detectPackageManager(dirPath)
	}

	return ws, nil
}

// markerMatch represents a matched marker file.
type markerMatch struct {
	marker   workspace.MarkerFile
	path     string
	priority int
}

// parseMarkerContent parses the content of a marker file to confirm workspace status.
func (d *FileDetector) parseMarkerContent(ctx context.Context, path string, marker workspace.MarkerFile) (bool, workspace.WorkspaceType, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, workspace.WorkspaceTypeNone, err
	}

	switch marker.Name {
	case "package.json":
		return d.parsePackageJSON(content)
	case "Cargo.toml":
		return d.parseCargoToml(content)
	case "pom.xml":
		return d.parsePomXML(content)
	default:
		// If we don't know how to parse, assume it's a workspace
		return true, marker.Type, nil
	}
}

// parsePackageJSON checks if package.json has workspaces field.
func (d *FileDetector) parsePackageJSON(content []byte) (bool, workspace.WorkspaceType, error) {
	var pkg struct {
		Workspaces interface{} `json:"workspaces"`
		Private    bool        `json:"private"`
	}

	if err := json.Unmarshal(content, &pkg); err != nil {
		return false, workspace.WorkspaceTypeNone, err
	}

	if pkg.Workspaces != nil {
		// Determine if yarn or npm based on other indicators
		return true, workspace.WorkspaceTypeNpm, nil
	}

	return false, workspace.WorkspaceTypeNone, nil
}

// parseCargoToml checks if Cargo.toml has [workspace] section.
func (d *FileDetector) parseCargoToml(content []byte) (bool, workspace.WorkspaceType, error) {
	// Simple check for [workspace] section
	if strings.Contains(string(content), "[workspace]") {
		return true, workspace.WorkspaceTypeCargo, nil
	}
	return false, workspace.WorkspaceTypeNone, nil
}

// parsePomXML checks if pom.xml has modules section.
func (d *FileDetector) parsePomXML(content []byte) (bool, workspace.WorkspaceType, error) {
	// Simple check for <modules> section
	if strings.Contains(string(content), "<modules>") {
		return true, workspace.WorkspaceTypeMaven, nil
	}
	return false, workspace.WorkspaceTypeNone, nil
}

// defaultPackagePatterns returns default package glob patterns for a workspace type.
func (d *FileDetector) defaultPackagePatterns(wsType workspace.WorkspaceType) []string {
	switch wsType {
	case workspace.WorkspaceTypePnpm, workspace.WorkspaceTypeNpm, workspace.WorkspaceTypeYarn,
		workspace.WorkspaceTypeLerna, workspace.WorkspaceTypeNx, workspace.WorkspaceTypeTurborepo:
		return []string{
			"packages/*",
			"apps/*",
			"libs/*",
			"tools/*",
		}
	case workspace.WorkspaceTypeGoModule:
		return []string{
			"./",
			"cmd/*",
			"internal/*",
			"pkg/*",
		}
	case workspace.WorkspaceTypeCargo:
		return []string{
			"crates/*",
			"*/",
		}
	case workspace.WorkspaceTypeMaven:
		return []string{
			"*/",
		}
	case workspace.WorkspaceTypeGradle:
		return []string{
			"*/",
		}
	default:
		return []string{
			"packages/*",
			"*/",
		}
	}
}

// shouldExclude checks if a path matches any exclude patterns.
func (d *FileDetector) shouldExclude(path string, excludePatterns []string, rootPath string) bool {
	relPath, err := filepath.Rel(rootPath, path)
	if err != nil {
		return false
	}

	for _, pattern := range excludePatterns {
		matched, err := filepath.Match(pattern, relPath)
		if err == nil && matched {
			return true
		}

		// Also check if the path contains the pattern
		if strings.Contains(relPath, strings.TrimSuffix(strings.TrimPrefix(pattern, "**/"), "/**")) {
			return true
		}
	}

	return false
}

// parsePackage attempts to parse a package at the given path.
func (d *FileDetector) parsePackage(ctx context.Context, pkgPath string, ws *workspace.Workspace) (*workspace.Package, error) {
	info, err := os.Stat(pkgPath)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, nil
	}

	// Try different package manifest files based on workspace type
	switch ws.Type {
	case workspace.WorkspaceTypePnpm, workspace.WorkspaceTypeNpm, workspace.WorkspaceTypeYarn,
		workspace.WorkspaceTypeLerna, workspace.WorkspaceTypeNx, workspace.WorkspaceTypeTurborepo:
		return d.parseNodePackage(pkgPath, ws.RootPath)
	case workspace.WorkspaceTypeGoModule:
		return d.parseGoModule(pkgPath, ws.RootPath)
	case workspace.WorkspaceTypeCargo:
		return d.parseCargoPackage(pkgPath, ws.RootPath)
	default:
		return d.parseGenericPackage(pkgPath, ws.RootPath)
	}
}

// parseNodePackage parses a Node.js package from package.json.
func (d *FileDetector) parseNodePackage(pkgPath, rootPath string) (*workspace.Package, error) {
	pkgJSONPath := filepath.Join(pkgPath, "package.json")
	content, err := os.ReadFile(pkgJSONPath)
	if err != nil {
		return nil, err
	}

	var pkgJSON struct {
		Name            string            `json:"name"`
		Version         string            `json:"version"`
		Private         bool              `json:"private"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	if err := json.Unmarshal(content, &pkgJSON); err != nil {
		return nil, err
	}

	relPath, _ := filepath.Rel(rootPath, pkgPath)

	pkg := &workspace.Package{
		Name:            pkgJSON.Name,
		Path:            relPath,
		Version:         pkgJSON.Version,
		Private:         pkgJSON.Private,
		Dependencies:    pkgJSON.Dependencies,
		DevDependencies: pkgJSON.DevDependencies,
	}

	return pkg, nil
}

// parseGoModule parses a Go module from go.mod.
func (d *FileDetector) parseGoModule(pkgPath, rootPath string) (*workspace.Package, error) {
	goModPath := filepath.Join(pkgPath, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, err
	}

	// Simple parsing of go.mod
	lines := strings.Split(string(content), "\n")
	var moduleName string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			moduleName = strings.TrimPrefix(line, "module ")
			break
		}
	}

	if moduleName == "" {
		return nil, fmt.Errorf("could not find module name in go.mod")
	}

	relPath, _ := filepath.Rel(rootPath, pkgPath)

	return &workspace.Package{
		Name:    moduleName,
		Path:    relPath,
		Version: "", // Go modules don't have version in go.mod
		Private: false,
	}, nil
}

// parseCargoPackage parses a Cargo package from Cargo.toml.
func (d *FileDetector) parseCargoPackage(pkgPath, rootPath string) (*workspace.Package, error) {
	cargoPath := filepath.Join(pkgPath, "Cargo.toml")
	content, err := os.ReadFile(cargoPath)
	if err != nil {
		return nil, err
	}

	// Simple TOML parsing for [package] section
	lines := strings.Split(string(content), "\n")
	inPackage := false
	var name, version string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "[package]" {
			inPackage = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inPackage = false
			continue
		}
		if inPackage {
			if strings.HasPrefix(line, "name") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					name = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				}
			}
			if strings.HasPrefix(line, "version") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					version = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				}
			}
		}
	}

	if name == "" {
		return nil, fmt.Errorf("could not find package name in Cargo.toml")
	}

	relPath, _ := filepath.Rel(rootPath, pkgPath)

	return &workspace.Package{
		Name:    name,
		Path:    relPath,
		Version: version,
		Private: false,
	}, nil
}

// parseGenericPackage creates a generic package from directory.
func (d *FileDetector) parseGenericPackage(pkgPath, rootPath string) (*workspace.Package, error) {
	relPath, _ := filepath.Rel(rootPath, pkgPath)
	name := filepath.Base(pkgPath)

	return &workspace.Package{
		Name:    name,
		Path:    relPath,
		Version: "",
		Private: false,
	}, nil
}

// detectPackageManager tries to detect the package manager from lock files.
func (d *FileDetector) detectPackageManager(dirPath string) workspace.PackageManagerType {
	lockFiles := map[string]workspace.PackageManagerType{
		"pnpm-lock.yaml":    workspace.PackageManagerPnpm,
		"yarn.lock":         workspace.PackageManagerYarn,
		"package-lock.json": workspace.PackageManagerNpm,
		"bun.lockb":         workspace.PackageManagerBun,
		"go.sum":            workspace.PackageManagerGo,
		"Cargo.lock":        workspace.PackageManagerCargo,
	}

	for lockFile, pm := range lockFiles {
		if _, err := os.Stat(filepath.Join(dirPath, lockFile)); err == nil {
			return pm
		}
	}

	return workspace.PackageManagerUnknown
}

// calculateConfidence calculates a confidence score for the detection.
func (d *FileDetector) calculateConfidence(match *markerMatch) float64 {
	// Higher priority markers get higher confidence
	if match.priority >= 90 {
		return 0.95
	}
	if match.priority >= 70 {
		return 0.85
	}
	if match.priority >= 50 {
		return 0.75
	}
	return 0.6
}

// generateWorkspaceID generates a unique ID for a workspace.
func generateWorkspaceID(rootPath string) string {
	// Use a hash of the path for a stable ID
	absPath, _ := filepath.Abs(rootPath)
	// Simple hash - take last component + hash prefix
	base := filepath.Base(absPath)
	hash := fmt.Sprintf("%x", absPath)
	if len(hash) > 8 {
		hash = hash[:8]
	}
	return fmt.Sprintf("%s-%s", base, hash)
}

// Ensure FileDetector implements workspace.Detector.
var _ workspace.Detector = (*FileDetector)(nil)
