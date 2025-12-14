// Package templates provides project detection and template management for the init wizard.
package templates

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
)

// Language represents a detected programming language.
type Language string

const (
	LanguageGo      Language = "go"
	LanguageNode    Language = "node"
	LanguagePython  Language = "python"
	LanguageRust    Language = "rust"
	LanguageRuby    Language = "ruby"
	LanguageUnknown Language = "unknown"
)

// Platform represents a detected platform or deployment target.
type Platform string

const (
	PlatformDocker     Platform = "docker"
	PlatformKubernetes Platform = "kubernetes"
	PlatformServerless Platform = "serverless"
	PlatformNative     Platform = "native"
)

// ProjectType represents the detected project type.
type ProjectType string

const (
	ProjectTypeOpenSource ProjectType = "opensource"
	ProjectTypeSaaS       ProjectType = "saas"
	ProjectTypeAPI        ProjectType = "api"
	ProjectTypeCLI        ProjectType = "cli"
	ProjectTypeLibrary    ProjectType = "library"
	ProjectTypeMobile     ProjectType = "mobile"
	ProjectTypeContainer  ProjectType = "container"
	ProjectTypeMonorepo   ProjectType = "monorepo"
	ProjectTypeUnknown    ProjectType = "unknown"
)

// Detection represents the result of project detection.
type Detection struct {
	// Language is the primary detected programming language.
	Language Language
	// LanguageConfidence is the confidence score (0-100) for the language detection.
	LanguageConfidence int
	// SecondaryLanguages are additional detected languages.
	SecondaryLanguages []Language

	// Platform is the primary detected platform.
	Platform Platform
	// PlatformConfidence is the confidence score (0-100) for the platform detection.
	PlatformConfidence int

	// ProjectType is the detected project type.
	ProjectType ProjectType
	// TypeConfidence is the confidence score (0-100) for the type detection.
	TypeConfidence int

	// GitRepository is the detected git repository URL (if available).
	GitRepository string
	// GitBranch is the current git branch.
	GitBranch string
	// IsMonorepo indicates if this is a monorepo structure.
	IsMonorepo bool
	// MonorepoRoot is the root directory if in a monorepo.
	MonorepoRoot string

	// HasDockerfile indicates presence of Dockerfile.
	HasDockerfile bool
	// HasKubernetesConfig indicates presence of Kubernetes configs.
	HasKubernetesConfig bool
	// HasCI indicates presence of CI/CD configuration.
	HasCI bool
	// CIProvider is the detected CI provider (github-actions, gitlab-ci, etc.).
	CIProvider string

	// PackageManager is the detected package manager.
	PackageManager string
	// BuildTool is the detected build tool (make, gradle, cargo, etc.).
	BuildTool string

	// SuggestedTemplate is the recommended template based on detection.
	SuggestedTemplate string
}

// ignoredDirs is a list of directories that should be excluded from scanning.
// These are typically large vendor directories, build outputs, or VCS metadata.
var ignoredDirs = map[string]bool{
	"node_modules":  true, // Node.js dependencies
	"vendor":        true, // Go/PHP dependencies
	".git":          true, // Git metadata
	".svn":          true, // SVN metadata
	".hg":           true, // Mercurial metadata
	"dist":          true, // Build output
	"build":         true, // Build output
	"target":        true, // Rust/Java build output
	".next":         true, // Next.js cache
	".nuxt":         true, // Nuxt.js cache
	"coverage":      true, // Test coverage
	".nyc_output":   true, // NYC coverage
	"venv":          true, // Python virtual env
	".venv":         true, // Python virtual env
	"__pycache__":   true, // Python cache
	".pytest_cache": true, // Pytest cache
	".tox":          true, // Tox testing
	".mypy_cache":   true, // MyPy cache
	".cache":        true, // Generic cache
	".terraform":    true, // Terraform state
	".idea":         true, // JetBrains IDE
	".vscode":       true, // VS Code
}

// Detector detects project characteristics by scanning files.
type Detector struct {
	basePath string
}

// systemDirsExact contains directories where only the exact path is blocked.
// Subdirectories of these paths are allowed (e.g., for testing in /tmp/...).
var systemDirsExact = []string{
	"/tmp",
}

// systemDirsRecursive contains directories where both the directory and all
// subdirectories are blocked for security reasons.
var systemDirsRecursive = []string{
	"/etc",
	"/sys",
	"/proc",
	"/dev",
	"/boot",
	"/root",
	"/var/log",
	"/var/run",
	"/private/etc", // macOS
	"/private/var", // macOS
	"C:\\Windows",  // Windows
	"C:\\Program Files",
	"C:\\ProgramData",
}

// NewDetector creates a new project detector for the given path.
// It validates the path to prevent scanning system directories.
func NewDetector(basePath string) *Detector {
	if basePath == "" {
		basePath = "."
	}

	// Resolve to absolute path for validation
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		// If we can't resolve the path, fall back to current directory
		return &Detector{basePath: "."}
	}

	// Validate that path is not a system directory
	normalizedPath := filepath.Clean(absPath)

	// Check exact-match system directories (like /tmp)
	for _, sysDir := range systemDirsExact {
		normalizedSysDir := filepath.Clean(sysDir)
		if normalizedPath == normalizedSysDir {
			return &Detector{basePath: "."}
		}
	}

	// Check recursive system directories (block directory and subdirectories)
	for _, sysDir := range systemDirsRecursive {
		normalizedSysDir := filepath.Clean(sysDir)
		if normalizedPath == normalizedSysDir ||
			strings.HasPrefix(normalizedPath, normalizedSysDir+string(filepath.Separator)) {
			return &Detector{basePath: "."}
		}
	}

	return &Detector{
		basePath: basePath,
	}
}

// Detect scans the project directory and returns detection results.
func (d *Detector) Detect() (*Detection, error) {
	detection := &Detection{
		Language:    LanguageUnknown,
		Platform:    PlatformNative,
		ProjectType: ProjectTypeUnknown,
	}

	// Detect language
	if err := d.detectLanguage(detection); err != nil {
		return nil, err
	}

	// Detect platform
	if err := d.detectPlatform(detection); err != nil {
		return nil, err
	}

	// Detect project type
	if err := d.detectProjectType(detection); err != nil {
		return nil, err
	}

	// Detect git info (optional, don't fail on error)
	_ = d.detectGit(detection)

	// Detect tools
	if err := d.detectTools(detection); err != nil {
		return nil, err
	}

	// Suggest template
	d.suggestTemplate(detection)

	return detection, nil
}

// detectLanguage determines the primary programming language.
func (d *Detector) detectLanguage(detection *Detection) error {
	scores := make(map[Language]int)

	// Go detection
	if d.fileExists("go.mod") {
		scores[LanguageGo] += 50
	}
	if d.fileExists("go.sum") {
		scores[LanguageGo] += 10
	}
	if d.hasFilesWithExt(".go") {
		scores[LanguageGo] += 30
	}

	// Node.js detection
	if d.fileExists("package.json") {
		scores[LanguageNode] += 50
	}
	if d.fileExists("package-lock.json") || d.fileExists("yarn.lock") || d.fileExists("pnpm-lock.yaml") {
		scores[LanguageNode] += 10
	}
	if d.hasFilesWithExt(".js") || d.hasFilesWithExt(".ts") {
		scores[LanguageNode] += 30
	}

	// Python detection
	if d.fileExists("setup.py") || d.fileExists("pyproject.toml") {
		scores[LanguagePython] += 50
	}
	if d.fileExists("requirements.txt") || d.fileExists("Pipfile") {
		scores[LanguagePython] += 10
	}
	if d.hasFilesWithExt(".py") {
		scores[LanguagePython] += 30
	}

	// Rust detection
	if d.fileExists("Cargo.toml") {
		scores[LanguageRust] += 50
	}
	if d.fileExists("Cargo.lock") {
		scores[LanguageRust] += 10
	}
	if d.hasFilesWithExt(".rs") {
		scores[LanguageRust] += 30
	}

	// Ruby detection
	if d.fileExists("Gemfile") {
		scores[LanguageRuby] += 50
	}
	if d.fileExists("Gemfile.lock") {
		scores[LanguageRuby] += 10
	}
	if d.hasFilesWithExt(".rb") {
		scores[LanguageRuby] += 30
	}

	// Language priority for tiebreaking (lower index = higher priority)
	languagePriority := []Language{LanguageGo, LanguageRust, LanguageNode, LanguagePython, LanguageRuby}

	// Find language with highest score (use priority order for tiebreaking)
	maxScore := 0
	for _, lang := range languagePriority {
		score := scores[lang]
		if score > maxScore {
			maxScore = score
			detection.Language = lang
			detection.LanguageConfidence = score
		}
	}

	// Collect secondary languages (score >= 40)
	for lang, score := range scores {
		if lang != detection.Language && score >= 40 {
			detection.SecondaryLanguages = append(detection.SecondaryLanguages, lang)
		}
	}

	return nil
}

// detectPlatform determines the deployment platform.
func (d *Detector) detectPlatform(detection *Detection) error {
	scores := make(map[Platform]int)

	// Docker detection
	if d.fileExists("Dockerfile") {
		scores[PlatformDocker] += 50
		detection.HasDockerfile = true
	}
	if d.fileExists("docker-compose.yml") || d.fileExists("docker-compose.yaml") {
		scores[PlatformDocker] += 20
	}
	if d.fileExists(".dockerignore") {
		scores[PlatformDocker] += 10
	}

	// Kubernetes detection
	if d.dirExists("k8s") || d.dirExists("kubernetes") {
		scores[PlatformKubernetes] += 50
		detection.HasKubernetesConfig = true
	}
	if d.hasFilesWithExt(".yaml", "k8s", "kubernetes", "manifests") {
		scores[PlatformKubernetes] += 20
	}

	// Serverless detection
	if d.fileExists("serverless.yml") || d.fileExists("serverless.yaml") {
		scores[PlatformServerless] += 50
	}
	if d.fileExists("netlify.toml") || d.fileExists("vercel.json") {
		scores[PlatformServerless] += 30
	}

	// Find platform with highest score
	maxScore := 0
	for platform, score := range scores {
		if score > maxScore {
			maxScore = score
			detection.Platform = platform
			detection.PlatformConfidence = score
		}
	}

	return nil
}

// detectProjectType determines the project type.
func (d *Detector) detectProjectType(detection *Detection) error {
	scores := make(map[ProjectType]int)

	// CLI tool detection
	if d.dirExists("cmd") && detection.Language == LanguageGo {
		scores[ProjectTypeCLI] += 40
	}
	if d.fileExists("main.go") || d.fileExists("cmd/main.go") {
		scores[ProjectTypeCLI] += 30
	}

	// Library detection
	if detection.Language == LanguageGo && !d.fileExists("cmd/main.go") && !d.fileExists("main.go") {
		scores[ProjectTypeLibrary] += 40
	}
	if detection.Language == LanguageNode && d.hasFileContent("package.json", "\"type\": \"library\"") {
		scores[ProjectTypeLibrary] += 40
	}

	// API service detection
	if d.dirExists("api") || d.dirExists("routes") || d.dirExists("controllers") {
		scores[ProjectTypeAPI] += 40
	}
	if d.hasFileContent("package.json", "express") || d.hasFileContent("package.json", "fastify") {
		scores[ProjectTypeAPI] += 30
	}

	// SaaS detection
	if d.dirExists("frontend") && d.dirExists("backend") {
		scores[ProjectTypeSaaS] += 40
	}
	if d.dirExists("web") || d.dirExists("ui") || d.dirExists("client") {
		scores[ProjectTypeSaaS] += 30
	}

	// Monorepo detection
	if d.dirExists("packages") || d.dirExists("apps") {
		scores[ProjectTypeMonorepo] += 50
		detection.IsMonorepo = true
	}
	if d.fileExists("lerna.json") || d.fileExists("pnpm-workspace.yaml") {
		scores[ProjectTypeMonorepo] += 30
		detection.IsMonorepo = true
	}

	// Container/infrastructure detection
	if detection.HasDockerfile && detection.HasKubernetesConfig {
		scores[ProjectTypeContainer] += 50
	}

	// Open source detection (presence of common OSS files)
	opensourceScore := 0
	if d.fileExists("LICENSE") || d.fileExists("LICENSE.md") {
		opensourceScore += 20
	}
	if d.fileExists("CONTRIBUTING.md") {
		opensourceScore += 15
	}
	if d.fileExists("CODE_OF_CONDUCT.md") {
		opensourceScore += 15
	}
	if opensourceScore >= 20 {
		scores[ProjectTypeOpenSource] += opensourceScore
	}

	// Find type with highest score
	maxScore := 0
	for projectType, score := range scores {
		if score > maxScore {
			maxScore = score
			detection.ProjectType = projectType
			detection.TypeConfidence = score
		}
	}

	// Default to CLI if we have high confidence in a compiled language
	if detection.TypeConfidence < 30 && (detection.Language == LanguageGo || detection.Language == LanguageRust) {
		detection.ProjectType = ProjectTypeCLI
		detection.TypeConfidence = 50
	}

	return nil
}

// detectGit extracts git repository information.
func (d *Detector) detectGit(detection *Detection) error {
	repo, err := git.PlainOpen(d.basePath)
	if err != nil {
		return err
	}

	// Get current branch
	head, err := repo.Head()
	if err == nil {
		detection.GitBranch = head.Name().Short()
	}

	// Get remote URL
	remotes, err := repo.Remotes()
	if err == nil && len(remotes) > 0 {
		config := remotes[0].Config()
		if len(config.URLs) > 0 {
			detection.GitRepository = config.URLs[0]
		}
	}

	return nil
}

// detectTools identifies package managers and build tools.
func (d *Detector) detectTools(detection *Detection) error {
	// Detect CI/CD
	if d.dirExists(".github/workflows") {
		detection.HasCI = true
		detection.CIProvider = "github-actions"
	} else if d.fileExists(".gitlab-ci.yml") {
		detection.HasCI = true
		detection.CIProvider = "gitlab-ci"
	} else if d.fileExists(".circleci/config.yml") {
		detection.HasCI = true
		detection.CIProvider = "circleci"
	}

	// Detect package manager
	switch detection.Language {
	case LanguageGo:
		detection.PackageManager = "go modules"
		if d.fileExists("Makefile") {
			detection.BuildTool = "make"
		}
	case LanguageNode:
		if d.fileExists("pnpm-lock.yaml") {
			detection.PackageManager = "pnpm"
		} else if d.fileExists("yarn.lock") {
			detection.PackageManager = "yarn"
		} else {
			detection.PackageManager = "npm"
		}
	case LanguagePython:
		if d.fileExists("poetry.lock") {
			detection.PackageManager = "poetry"
		} else if d.fileExists("Pipfile") {
			detection.PackageManager = "pipenv"
		} else {
			detection.PackageManager = "pip"
		}
	case LanguageRust:
		detection.PackageManager = "cargo"
		detection.BuildTool = "cargo"
	case LanguageRuby:
		detection.PackageManager = "bundler"
	}

	return nil
}

// templateKey combines language and project type for lookup.
type templateKey struct {
	language    Language
	projectType ProjectType
}

// templateLookup maps language/project type combinations to template names.
var templateLookup = map[templateKey]string{
	{LanguageGo, ProjectTypeAPI}:      "saas-api",
	{LanguageGo, ProjectTypeSaaS}:     "saas-web",
	{LanguageNode, ProjectTypeSaaS}:   "saas-web",
	{LanguageNode, ProjectTypeAPI}:    "saas-web",
	{LanguagePython, ProjectTypeAPI}:  "opensource-python",
	{LanguagePython, ProjectTypeSaaS}: "opensource-python",
	{LanguageRust, ProjectTypeAPI}:    "opensource-rust",
	{LanguageRust, ProjectTypeSaaS}:   "opensource-rust",
}

// languageDefaultTemplates maps languages to their default templates.
var languageDefaultTemplates = map[Language]string{
	LanguageGo:     "opensource-go",
	LanguageNode:   "opensource-node",
	LanguagePython: "opensource-python",
	LanguageRust:   "opensource-rust",
	LanguageRuby:   "opensource-go", // fallback
}

// suggestTemplate recommends a template based on detection results.
func (d *Detector) suggestTemplate(detection *Detection) {
	// Priority 1: Monorepo
	if detection.IsMonorepo {
		detection.SuggestedTemplate = "monorepo"
		return
	}

	// Priority 2: Container projects
	if detection.ProjectType == ProjectTypeContainer {
		detection.SuggestedTemplate = "container"
		return
	}

	// Priority 3: Language + ProjectType combination
	key := templateKey{detection.Language, detection.ProjectType}
	if template, ok := templateLookup[key]; ok {
		detection.SuggestedTemplate = template
		return
	}

	// Priority 4: Language default
	if template, ok := languageDefaultTemplates[detection.Language]; ok {
		detection.SuggestedTemplate = template
		return
	}

	// Fallback
	detection.SuggestedTemplate = "opensource-go"
}

// Helper methods

func (d *Detector) fileExists(name string) bool {
	path := filepath.Join(d.basePath, name)
	_, err := os.Stat(path)
	return err == nil
}

func (d *Detector) dirExists(name string) bool {
	path := filepath.Join(d.basePath, name)
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (d *Detector) hasFilesWithExt(ext string, dirs ...string) bool {
	searchDirs := dirs
	if len(searchDirs) == 0 {
		searchDirs = []string{d.basePath}
	}

	for _, dir := range searchDirs {
		fullPath := filepath.Join(d.basePath, dir)
		found := false
		_ = filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			// Skip ignored directories
			if info.IsDir() && ignoredDirs[info.Name()] {
				return filepath.SkipDir
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ext) {
				found = true
				return filepath.SkipAll
			}
			return nil
		})
		if found {
			return true
		}
	}

	return false
}

func (d *Detector) hasFileContent(filename, content string) bool {
	path := filepath.Join(d.basePath, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), content)
}
