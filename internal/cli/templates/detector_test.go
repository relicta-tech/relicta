// Package templates provides project detection and template management for the init wizard.
package templates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewDetector(t *testing.T) {
	tests := []struct {
		name         string
		basePath     string
		expectedPath string
	}{
		{
			name:         "empty path defaults to current directory",
			basePath:     "",
			expectedPath: ".",
		},
		{
			name:         "relative path is preserved",
			basePath:     "./testdata",
			expectedPath: "./testdata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewDetector(tt.basePath)
			if detector == nil {
				t.Fatal("NewDetector() returned nil")
			}
			if detector.basePath != tt.expectedPath {
				t.Errorf("basePath = %v, want %v", detector.basePath, tt.expectedPath)
			}
		})
	}
}

func TestNewDetector_SystemDirectoryProtection(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		wantCwd  bool // true if should fall back to "."
	}{
		{
			name:     "etc directory blocked",
			basePath: "/etc",
			wantCwd:  true,
		},
		{
			name:     "etc subdirectory blocked",
			basePath: "/etc/passwd",
			wantCwd:  true,
		},
		{
			name:     "sys directory blocked",
			basePath: "/sys",
			wantCwd:  true,
		},
		{
			name:     "proc directory blocked",
			basePath: "/proc",
			wantCwd:  true,
		},
		{
			name:     "dev directory blocked",
			basePath: "/dev",
			wantCwd:  true,
		},
		{
			name:     "boot directory blocked",
			basePath: "/boot",
			wantCwd:  true,
		},
		{
			name:     "root directory blocked",
			basePath: "/root",
			wantCwd:  true,
		},
		{
			name:     "var log directory blocked",
			basePath: "/var/log",
			wantCwd:  true,
		},
		{
			name:     "tmp directory blocked",
			basePath: "/tmp",
			wantCwd:  true,
		},
		{
			name:     "tmp subdirectory allowed for testing",
			basePath: "/tmp/test",
			wantCwd:  false, // /tmp subdirectories allowed for test temp dirs
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewDetector(tt.basePath)
			if detector == nil {
				t.Fatal("NewDetector() returned nil")
			}
			if tt.wantCwd && detector.basePath != "." {
				t.Errorf("basePath = %v, want '.' for blocked system directory %s", detector.basePath, tt.basePath)
			}
			if !tt.wantCwd && detector.basePath == "." {
				t.Errorf("basePath = '.', want %s (path should be allowed)", tt.basePath)
			}
		})
	}
}

func TestDetector_fileExists(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	detector := NewDetector(tmpDir)

	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{
			name:     "existing file",
			filename: "test.txt",
			want:     true,
		},
		{
			name:     "non-existing file",
			filename: "nonexistent.txt",
			want:     false,
		},
		{
			name:     "empty filename returns basePath existence",
			filename: "",
			want:     true, // Empty name joins to basePath, which exists
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.fileExists(tt.filename)
			if got != tt.want {
				t.Errorf("fileExists(%v) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestDetector_dirExists(t *testing.T) {
	// Create temp directory with subdirectories
	tmpDir := t.TempDir()

	// Create a test directory
	testDir := filepath.Join(tmpDir, "testdir")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a file (not a directory)
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	detector := NewDetector(tmpDir)

	tests := []struct {
		name    string
		dirName string
		want    bool
	}{
		{
			name:    "existing directory",
			dirName: "testdir",
			want:    true,
		},
		{
			name:    "non-existing directory",
			dirName: "nonexistent",
			want:    false,
		},
		{
			name:    "file not directory",
			dirName: "test.txt",
			want:    false,
		},
		{
			name:    "empty name returns basePath existence",
			dirName: "",
			want:    true, // Empty name joins to basePath, which exists
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.dirExists(tt.dirName)
			if got != tt.want {
				t.Errorf("dirExists(%v) = %v, want %v", tt.dirName, got, tt.want)
			}
		})
	}
}

func TestDetector_hasFilesWithExt(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()

	// Create test files with different extensions
	files := []string{
		"main.go",
		"util.go",
		"test.js",
		"index.ts",
		"README.md",
	}

	for _, file := range files {
		path := filepath.Join(tmpDir, file)
		if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	// Create subdirectory with more files
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create src directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "app.go"), []byte("test"), 0600); err != nil {
		t.Fatalf("Failed to create app.go: %v", err)
	}

	detector := NewDetector(tmpDir)

	tests := []struct {
		name string
		ext  string
		dirs []string
		want bool
	}{
		{
			name: "find .go files in current dir",
			ext:  ".go",
			dirs: []string{"."},
			want: true,
		},
		{
			name: "find .js files in current dir",
			ext:  ".js",
			dirs: []string{"."},
			want: true,
		},
		{
			name: "no .py files",
			ext:  ".py",
			dirs: []string{"."},
			want: false,
		},
		{
			name: "find .go files in src",
			ext:  ".go",
			dirs: []string{"src"},
			want: true,
		},
		{
			name: "no .rb files in src",
			ext:  ".rb",
			dirs: []string{"src"},
			want: false,
		},
		{
			name: "no .ts files in current dir",
			ext:  ".rs",
			dirs: []string{"."},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.hasFilesWithExt(tt.ext, tt.dirs...)
			if got != tt.want {
				t.Errorf("hasFilesWithExt(%v, %v) = %v, want %v", tt.ext, tt.dirs, got, tt.want)
			}
		})
	}
}

func TestDetector_hasFileContent(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()

	// Create a file with specific content
	configFile := filepath.Join(tmpDir, "package.json")
	content := `{
  "name": "test-project",
  "version": "1.0.0",
  "type": "module"
}`
	if err := os.WriteFile(configFile, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	detector := NewDetector(tmpDir)

	tests := []struct {
		name     string
		filename string
		content  string
		want     bool
	}{
		{
			name:     "content exists in file",
			filename: "package.json",
			content:  `"type": "module"`,
			want:     true,
		},
		{
			name:     "content not in file",
			filename: "package.json",
			content:  "nonexistent",
			want:     false,
		},
		{
			name:     "file does not exist",
			filename: "nonexistent.json",
			content:  "anything",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.hasFileContent(tt.filename, tt.content)
			if got != tt.want {
				t.Errorf("hasFileContent(%v, %v) = %v, want %v", tt.filename, tt.content, got, tt.want)
			}
		})
	}
}

func TestDetector_detectLanguage_Go(t *testing.T) {
	// Create temp directory with Go files
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module github.com/test/project\n"), 0600); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create main.go
	mainGo := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainGo, []byte("package main\n"), 0600); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	detector := NewDetector(tmpDir)
	detection := &Detection{}

	err := detector.detectLanguage(detection)
	if err != nil {
		t.Fatalf("detectLanguage() error = %v", err)
	}

	if detection.Language != LanguageGo {
		t.Errorf("Language = %v, want %v", detection.Language, LanguageGo)
	}

	if detection.LanguageConfidence <= 0 {
		t.Errorf("LanguageConfidence = %v, want > 0", detection.LanguageConfidence)
	}
}

func TestDetector_detectLanguage_Node(t *testing.T) {
	// Create temp directory with Node files
	tmpDir := t.TempDir()

	// Create package.json
	packageJSON := filepath.Join(tmpDir, "package.json")
	content := `{
  "name": "test-project",
  "version": "1.0.0"
}`
	if err := os.WriteFile(packageJSON, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	detector := NewDetector(tmpDir)
	detection := &Detection{}

	err := detector.detectLanguage(detection)
	if err != nil {
		t.Fatalf("detectLanguage() error = %v", err)
	}

	if detection.Language != LanguageNode {
		t.Errorf("Language = %v, want %v", detection.Language, LanguageNode)
	}

	if detection.LanguageConfidence <= 0 {
		t.Errorf("LanguageConfidence = %v, want > 0", detection.LanguageConfidence)
	}
}

func TestDetector_detectLanguage_Python(t *testing.T) {
	// Create temp directory with Python files
	tmpDir := t.TempDir()

	// Create requirements.txt
	requirements := filepath.Join(tmpDir, "requirements.txt")
	if err := os.WriteFile(requirements, []byte("flask==2.0.0\n"), 0600); err != nil {
		t.Fatalf("Failed to create requirements.txt: %v", err)
	}

	// Create setup.py
	setupPy := filepath.Join(tmpDir, "setup.py")
	if err := os.WriteFile(setupPy, []byte("from setuptools import setup\n"), 0600); err != nil {
		t.Fatalf("Failed to create setup.py: %v", err)
	}

	detector := NewDetector(tmpDir)
	detection := &Detection{}

	err := detector.detectLanguage(detection)
	if err != nil {
		t.Fatalf("detectLanguage() error = %v", err)
	}

	if detection.Language != LanguagePython {
		t.Errorf("Language = %v, want %v", detection.Language, LanguagePython)
	}

	if detection.LanguageConfidence <= 0 {
		t.Errorf("LanguageConfidence = %v, want > 0", detection.LanguageConfidence)
	}
}

func TestDetector_detectLanguage_Rust(t *testing.T) {
	// Create temp directory with Rust files
	tmpDir := t.TempDir()

	// Create Cargo.toml
	cargoToml := filepath.Join(tmpDir, "Cargo.toml")
	content := `[package]
name = "test"
version = "0.1.0"
`
	if err := os.WriteFile(cargoToml, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to create Cargo.toml: %v", err)
	}

	detector := NewDetector(tmpDir)
	detection := &Detection{}

	err := detector.detectLanguage(detection)
	if err != nil {
		t.Fatalf("detectLanguage() error = %v", err)
	}

	if detection.Language != LanguageRust {
		t.Errorf("Language = %v, want %v", detection.Language, LanguageRust)
	}

	if detection.LanguageConfidence <= 0 {
		t.Errorf("LanguageConfidence = %v, want > 0", detection.LanguageConfidence)
	}
}

func TestDetector_detectLanguage_Ruby(t *testing.T) {
	// Create temp directory with Ruby files
	tmpDir := t.TempDir()

	// Create Gemfile
	gemfile := filepath.Join(tmpDir, "Gemfile")
	if err := os.WriteFile(gemfile, []byte("source 'https://rubygems.org'\\ngem 'rails'"), 0600); err != nil {
		t.Fatalf("Failed to create Gemfile: %v", err)
	}

	// Create Gemfile.lock
	gemfileLock := filepath.Join(tmpDir, "Gemfile.lock")
	if err := os.WriteFile(gemfileLock, []byte("GEM\\n"), 0600); err != nil {
		t.Fatalf("Failed to create Gemfile.lock: %v", err)
	}

	// Create a .rb file
	rbFile := filepath.Join(tmpDir, "app.rb")
	if err := os.WriteFile(rbFile, []byte("puts 'Hello, World!'"), 0600); err != nil {
		t.Fatalf("Failed to create app.rb: %v", err)
	}

	detector := NewDetector(tmpDir)
	detection := &Detection{}

	err := detector.detectLanguage(detection)
	if err != nil {
		t.Fatalf("detectLanguage() error = %v", err)
	}

	if detection.Language != LanguageRuby {
		t.Errorf("Language = %v, want %v", detection.Language, LanguageRuby)
	}

	if detection.LanguageConfidence <= 0 {
		t.Errorf("LanguageConfidence = %v, want > 0", detection.LanguageConfidence)
	}
}

func TestDetector_detectLanguage_SecondaryLanguages(t *testing.T) {
	// Create temp directory with mixed Go and Node.js files
	tmpDir := t.TempDir()

	// Create go.mod (primary language)
	goMod := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module github.com/test/project\\n"), 0600); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create main.go
	mainGo := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainGo, []byte("package main\\n"), 0600); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	// Create package.json (secondary language - score >= 40)
	packageJSON := filepath.Join(tmpDir, "package.json")
	content := `{
  "name": "test-project",
  "version": "1.0.0"
}`
	if err := os.WriteFile(packageJSON, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	// Create index.js
	indexJS := filepath.Join(tmpDir, "index.js")
	if err := os.WriteFile(indexJS, []byte("console.log('Hello');"), 0600); err != nil {
		t.Fatalf("Failed to create index.js: %v", err)
	}

	detector := NewDetector(tmpDir)
	detection := &Detection{}

	err := detector.detectLanguage(detection)
	if err != nil {
		t.Fatalf("detectLanguage() error = %v", err)
	}

	// Primary language should be Go (higher score)
	if detection.Language != LanguageGo {
		t.Errorf("Language = %v, want %v", detection.Language, LanguageGo)
	}

	// Should have Node as secondary language (score >= 40)
	found := false
	for _, lang := range detection.SecondaryLanguages {
		if lang == LanguageNode {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("SecondaryLanguages should contain Node, got %v", detection.SecondaryLanguages)
	}
}

func TestDetector_detectPlatform_Serverless(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(tmpDir string) error
		expected Platform
	}{
		{
			name: "Serverless Framework",
			setup: func(tmpDir string) error {
				return os.WriteFile(filepath.Join(tmpDir, "serverless.yml"), []byte("service: my-service"), 0600)
			},
			expected: PlatformServerless,
		},
		{
			name: "Netlify",
			setup: func(tmpDir string) error {
				return os.WriteFile(filepath.Join(tmpDir, "netlify.toml"), []byte("[build]\\n"), 0600)
			},
			expected: PlatformServerless,
		},
		{
			name: "Vercel",
			setup: func(tmpDir string) error {
				return os.WriteFile(filepath.Join(tmpDir, "vercel.json"), []byte("{}"), 0600)
			},
			expected: PlatformServerless,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if err := tt.setup(tmpDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			detector := NewDetector(tmpDir)
			detection := &Detection{}

			err := detector.detectPlatform(detection)
			if err != nil {
				t.Fatalf("detectPlatform() error = %v", err)
			}

			if detection.Platform != tt.expected {
				t.Errorf("Platform = %v, want %v", detection.Platform, tt.expected)
			}
		})
	}
}

func TestDetector_detectPlatform_Docker(t *testing.T) {
	// Create temp directory with Dockerfile
	tmpDir := t.TempDir()

	// Create Dockerfile
	dockerfile := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfile, []byte("FROM alpine\n"), 0600); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}

	detector := NewDetector(tmpDir)
	detection := &Detection{}

	err := detector.detectPlatform(detection)
	if err != nil {
		t.Fatalf("detectPlatform() error = %v", err)
	}

	if !detection.HasDockerfile {
		t.Error("HasDockerfile should be true")
	}

	if detection.Platform != PlatformDocker {
		t.Errorf("Platform = %v, want %v", detection.Platform, PlatformDocker)
	}
}

func TestDetector_detectPlatform_Kubernetes(t *testing.T) {
	// Create temp directory with k8s config
	tmpDir := t.TempDir()

	// Create k8s directory
	k8sDir := filepath.Join(tmpDir, "k8s")
	if err := os.Mkdir(k8sDir, 0755); err != nil {
		t.Fatalf("Failed to create k8s directory: %v", err)
	}

	// Create deployment.yaml
	deployment := filepath.Join(k8sDir, "deployment.yaml")
	if err := os.WriteFile(deployment, []byte("apiVersion: apps/v1\n"), 0600); err != nil {
		t.Fatalf("Failed to create deployment.yaml: %v", err)
	}

	detector := NewDetector(tmpDir)
	detection := &Detection{}

	err := detector.detectPlatform(detection)
	if err != nil {
		t.Fatalf("detectPlatform() error = %v", err)
	}

	if !detection.HasKubernetesConfig {
		t.Error("HasKubernetesConfig should be true")
	}

	if detection.Platform != PlatformKubernetes {
		t.Errorf("Platform = %v, want %v", detection.Platform, PlatformKubernetes)
	}
}

func TestDetector_Detect_Integration(t *testing.T) {
	// Create a complete Go CLI project
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module github.com/test/cli\n"), 0600); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create main.go
	mainGo := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainGo, []byte("package main\n\nfunc main() {}\n"), 0600); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	// Create Makefile
	makefile := filepath.Join(tmpDir, "Makefile")
	if err := os.WriteFile(makefile, []byte("build:\n\tgo build\n"), 0600); err != nil {
		t.Fatalf("Failed to create Makefile: %v", err)
	}

	detector := NewDetector(tmpDir)
	detection, err := detector.Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	// Verify language detection
	if detection.Language != LanguageGo {
		t.Errorf("Language = %v, want %v", detection.Language, LanguageGo)
	}

	// Verify build tool detection
	if detection.BuildTool != "make" {
		t.Errorf("BuildTool = %v, want make", detection.BuildTool)
	}

	// Verify suggested template
	if detection.SuggestedTemplate == "" {
		t.Error("SuggestedTemplate should not be empty")
	}
}

func TestDetector_Detect_EmptyDirectory(t *testing.T) {
	// Create empty temp directory
	tmpDir := t.TempDir()

	detector := NewDetector(tmpDir)
	detection, err := detector.Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	// Should detect unknown language
	if detection.Language != LanguageUnknown {
		t.Errorf("Language = %v, want %v", detection.Language, LanguageUnknown)
	}

	// Should detect native platform
	if detection.Platform != PlatformNative {
		t.Errorf("Platform = %v, want %v", detection.Platform, PlatformNative)
	}

	// Should detect unknown project type
	if detection.ProjectType != ProjectTypeUnknown {
		t.Errorf("ProjectType = %v, want %v", detection.ProjectType, ProjectTypeUnknown)
	}
}

func TestDetector_detectTools_CI(t *testing.T) {
	tests := []struct {
		name             string
		setup            func(tmpDir string) error
		language         Language
		expectedCI       bool
		expectedProvider string
	}{
		{
			name: "GitHub Actions",
			setup: func(tmpDir string) error {
				dir := filepath.Join(tmpDir, ".github", "workflows")
				if err := os.MkdirAll(dir, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "ci.yml"), []byte("name: CI"), 0600)
			},
			language:         LanguageGo,
			expectedCI:       true,
			expectedProvider: "github-actions",
		},
		{
			name: "GitLab CI",
			setup: func(tmpDir string) error {
				return os.WriteFile(filepath.Join(tmpDir, ".gitlab-ci.yml"), []byte("stages: [build]"), 0600)
			},
			language:         LanguageGo,
			expectedCI:       true,
			expectedProvider: "gitlab-ci",
		},
		{
			name: "CircleCI",
			setup: func(tmpDir string) error {
				dir := filepath.Join(tmpDir, ".circleci")
				if err := os.Mkdir(dir, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "config.yml"), []byte("version: 2"), 0600)
			},
			language:         LanguageGo,
			expectedCI:       true,
			expectedProvider: "circleci",
		},
		{
			name: "No CI",
			setup: func(tmpDir string) error {
				return nil
			},
			language:         LanguageGo,
			expectedCI:       false,
			expectedProvider: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if err := tt.setup(tmpDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			detector := NewDetector(tmpDir)
			detection := &Detection{Language: tt.language}

			err := detector.detectTools(detection)
			if err != nil {
				t.Fatalf("detectTools() error = %v", err)
			}

			if detection.HasCI != tt.expectedCI {
				t.Errorf("HasCI = %v, want %v", detection.HasCI, tt.expectedCI)
			}
			if detection.CIProvider != tt.expectedProvider {
				t.Errorf("CIProvider = %v, want %v", detection.CIProvider, tt.expectedProvider)
			}
		})
	}
}

func TestDetector_detectTools_PackageManager(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(tmpDir string) error
		language      Language
		expectedPM    string
		expectedBuild string
	}{
		{
			name: "Go with Makefile",
			setup: func(tmpDir string) error {
				return os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte("build:\n\tgo build"), 0600)
			},
			language:      LanguageGo,
			expectedPM:    "go modules",
			expectedBuild: "make",
		},
		{
			name: "Go without Makefile",
			setup: func(tmpDir string) error {
				return nil
			},
			language:      LanguageGo,
			expectedPM:    "go modules",
			expectedBuild: "",
		},
		{
			name: "Node with pnpm",
			setup: func(tmpDir string) error {
				return os.WriteFile(filepath.Join(tmpDir, "pnpm-lock.yaml"), []byte("lockfileVersion: 6.0"), 0600)
			},
			language:   LanguageNode,
			expectedPM: "pnpm",
		},
		{
			name: "Node with yarn",
			setup: func(tmpDir string) error {
				return os.WriteFile(filepath.Join(tmpDir, "yarn.lock"), []byte("# yarn lockfile v1"), 0600)
			},
			language:   LanguageNode,
			expectedPM: "yarn",
		},
		{
			name: "Node with npm",
			setup: func(tmpDir string) error {
				return nil
			},
			language:   LanguageNode,
			expectedPM: "npm",
		},
		{
			name: "Python with poetry",
			setup: func(tmpDir string) error {
				return os.WriteFile(filepath.Join(tmpDir, "poetry.lock"), []byte("[[package]]"), 0600)
			},
			language:   LanguagePython,
			expectedPM: "poetry",
		},
		{
			name: "Python with pipenv",
			setup: func(tmpDir string) error {
				return os.WriteFile(filepath.Join(tmpDir, "Pipfile"), []byte("[packages]"), 0600)
			},
			language:   LanguagePython,
			expectedPM: "pipenv",
		},
		{
			name: "Python with pip",
			setup: func(tmpDir string) error {
				return nil
			},
			language:   LanguagePython,
			expectedPM: "pip",
		},
		{
			name: "Rust with cargo",
			setup: func(tmpDir string) error {
				return nil
			},
			language:      LanguageRust,
			expectedPM:    "cargo",
			expectedBuild: "cargo",
		},
		{
			name: "Ruby with bundler",
			setup: func(tmpDir string) error {
				return nil
			},
			language:   LanguageRuby,
			expectedPM: "bundler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if err := tt.setup(tmpDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			detector := NewDetector(tmpDir)
			detection := &Detection{Language: tt.language}

			err := detector.detectTools(detection)
			if err != nil {
				t.Fatalf("detectTools() error = %v", err)
			}

			if detection.PackageManager != tt.expectedPM {
				t.Errorf("PackageManager = %v, want %v", detection.PackageManager, tt.expectedPM)
			}
			if detection.BuildTool != tt.expectedBuild {
				t.Errorf("BuildTool = %v, want %v", detection.BuildTool, tt.expectedBuild)
			}
		})
	}
}

func TestDetector_detectProjectType_CLI(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cmd directory and main.go
	cmdDir := filepath.Join(tmpDir, "cmd")
	if err := os.Mkdir(cmdDir, 0755); err != nil {
		t.Fatalf("Failed to create cmd directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0600); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	detector := NewDetector(tmpDir)
	detection := &Detection{Language: LanguageGo}

	err := detector.detectProjectType(detection)
	if err != nil {
		t.Fatalf("detectProjectType() error = %v", err)
	}

	if detection.ProjectType != ProjectTypeCLI {
		t.Errorf("ProjectType = %v, want %v", detection.ProjectType, ProjectTypeCLI)
	}
	if detection.TypeConfidence <= 0 {
		t.Errorf("TypeConfidence = %v, want > 0", detection.TypeConfidence)
	}
}

func TestDetector_detectProjectType_Library(t *testing.T) {
	tmpDir := t.TempDir()

	// Create lib.go without main.go
	if err := os.WriteFile(filepath.Join(tmpDir, "lib.go"), []byte("package mylib"), 0600); err != nil {
		t.Fatalf("Failed to create lib.go: %v", err)
	}

	detector := NewDetector(tmpDir)
	detection := &Detection{Language: LanguageGo}

	err := detector.detectProjectType(detection)
	if err != nil {
		t.Fatalf("detectProjectType() error = %v", err)
	}

	if detection.ProjectType != ProjectTypeLibrary {
		t.Errorf("ProjectType = %v, want %v", detection.ProjectType, ProjectTypeLibrary)
	}
}

func TestDetector_detectProjectType_API(t *testing.T) {
	tmpDir := t.TempDir()

	// Create api directory to score for API
	apiDir := filepath.Join(tmpDir, "api")
	if err := os.Mkdir(apiDir, 0755); err != nil {
		t.Fatalf("Failed to create api directory: %v", err)
	}
	// Create main.go to prevent Library detection
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0600); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	detector := NewDetector(tmpDir)
	detection := &Detection{Language: LanguageGo}

	err := detector.detectProjectType(detection)
	if err != nil {
		t.Fatalf("detectProjectType() error = %v", err)
	}

	if detection.ProjectType != ProjectTypeAPI {
		t.Errorf("ProjectType = %v, want %v", detection.ProjectType, ProjectTypeAPI)
	}
}

func TestDetector_detectProjectType_SaaS(t *testing.T) {
	tmpDir := t.TempDir()

	// Create frontend and backend directories
	if err := os.Mkdir(filepath.Join(tmpDir, "frontend"), 0755); err != nil {
		t.Fatalf("Failed to create frontend directory: %v", err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "backend"), 0755); err != nil {
		t.Fatalf("Failed to create backend directory: %v", err)
	}

	detector := NewDetector(tmpDir)
	detection := &Detection{Language: LanguageNode}

	err := detector.detectProjectType(detection)
	if err != nil {
		t.Fatalf("detectProjectType() error = %v", err)
	}

	if detection.ProjectType != ProjectTypeSaaS {
		t.Errorf("ProjectType = %v, want %v", detection.ProjectType, ProjectTypeSaaS)
	}
}

func TestDetector_detectProjectType_Monorepo(t *testing.T) {
	tmpDir := t.TempDir()

	// Create packages directory
	if err := os.Mkdir(filepath.Join(tmpDir, "packages"), 0755); err != nil {
		t.Fatalf("Failed to create packages directory: %v", err)
	}

	detector := NewDetector(tmpDir)
	detection := &Detection{Language: LanguageNode}

	err := detector.detectProjectType(detection)
	if err != nil {
		t.Fatalf("detectProjectType() error = %v", err)
	}

	if detection.ProjectType != ProjectTypeMonorepo {
		t.Errorf("ProjectType = %v, want %v", detection.ProjectType, ProjectTypeMonorepo)
	}
	if !detection.IsMonorepo {
		t.Error("IsMonorepo should be true")
	}
}

func TestDetector_detectProjectType_Container(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Dockerfile and k8s config
	if err := os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte("FROM alpine"), 0600); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}
	k8sDir := filepath.Join(tmpDir, "k8s")
	if err := os.Mkdir(k8sDir, 0755); err != nil {
		t.Fatalf("Failed to create k8s directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(k8sDir, "deployment.yaml"), []byte("apiVersion: apps/v1"), 0600); err != nil {
		t.Fatalf("Failed to create deployment.yaml: %v", err)
	}

	detector := NewDetector(tmpDir)
	detection := &Detection{
		Language:            LanguageGo,
		HasDockerfile:       true,
		HasKubernetesConfig: true,
	}

	err := detector.detectProjectType(detection)
	if err != nil {
		t.Fatalf("detectProjectType() error = %v", err)
	}

	if detection.ProjectType != ProjectTypeContainer {
		t.Errorf("ProjectType = %v, want %v", detection.ProjectType, ProjectTypeContainer)
	}
}

func TestDetector_detectProjectType_OpenSource(t *testing.T) {
	tmpDir := t.TempDir()

	// Create OSS files (all three to maximize score > 40)
	if err := os.WriteFile(filepath.Join(tmpDir, "LICENSE"), []byte("MIT License"), 0600); err != nil {
		t.Fatalf("Failed to create LICENSE: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "CONTRIBUTING.md"), []byte("# Contributing"), 0600); err != nil {
		t.Fatalf("Failed to create CONTRIBUTING.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "CODE_OF_CONDUCT.md"), []byte("# Code of Conduct"), 0600); err != nil {
		t.Fatalf("Failed to create CODE_OF_CONDUCT.md: %v", err)
	}

	detector := NewDetector(tmpDir)
	detection := &Detection{Language: LanguageGo}

	err := detector.detectProjectType(detection)
	if err != nil {
		t.Fatalf("detectProjectType() error = %v", err)
	}

	if detection.ProjectType != ProjectTypeOpenSource {
		t.Errorf("ProjectType = %v, want %v", detection.ProjectType, ProjectTypeOpenSource)
	}
}

func TestDetector_suggestTemplate(t *testing.T) {
	tests := []struct {
		name             string
		detection        Detection
		expectedTemplate string
	}{
		{
			name: "Monorepo",
			detection: Detection{
				IsMonorepo: true,
				Language:   LanguageNode,
			},
			expectedTemplate: "monorepo",
		},
		{
			name: "Container",
			detection: Detection{
				Language:    LanguageGo,
				ProjectType: ProjectTypeContainer,
			},
			expectedTemplate: "container",
		},
		{
			name: "Go API",
			detection: Detection{
				Language:    LanguageGo,
				ProjectType: ProjectTypeAPI,
			},
			expectedTemplate: "saas-api",
		},
		{
			name: "Go SaaS",
			detection: Detection{
				Language:    LanguageGo,
				ProjectType: ProjectTypeSaaS,
			},
			expectedTemplate: "saas-web",
		},
		{
			name: "Go default (open source)",
			detection: Detection{
				Language:    LanguageGo,
				ProjectType: ProjectTypeCLI,
			},
			expectedTemplate: "opensource-go",
		},
		{
			name: "Node SaaS",
			detection: Detection{
				Language:    LanguageNode,
				ProjectType: ProjectTypeSaaS,
			},
			expectedTemplate: "saas-web",
		},
		{
			name: "Node API",
			detection: Detection{
				Language:    LanguageNode,
				ProjectType: ProjectTypeAPI,
			},
			expectedTemplate: "saas-web",
		},
		{
			name: "Node default (open source)",
			detection: Detection{
				Language:    LanguageNode,
				ProjectType: ProjectTypeLibrary,
			},
			expectedTemplate: "opensource-node",
		},
		{
			name: "Python",
			detection: Detection{
				Language:    LanguagePython,
				ProjectType: ProjectTypeCLI,
			},
			expectedTemplate: "opensource-python",
		},
		{
			name: "Rust",
			detection: Detection{
				Language:    LanguageRust,
				ProjectType: ProjectTypeCLI,
			},
			expectedTemplate: "opensource-rust",
		},
		{
			name: "Unknown language",
			detection: Detection{
				Language:    LanguageUnknown,
				ProjectType: ProjectTypeCLI,
			},
			expectedTemplate: "opensource-go", // Default fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewDetector(".")
			detection := tt.detection
			detector.suggestTemplate(&detection)

			if detection.SuggestedTemplate != tt.expectedTemplate {
				t.Errorf("SuggestedTemplate = %v, want %v", detection.SuggestedTemplate, tt.expectedTemplate)
			}
		})
	}
}
