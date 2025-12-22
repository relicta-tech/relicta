package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsValidPluginName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"my-plugin", true},
		{"plugin123", true},
		{"-bad", false},
		{"bad-", false},
		{"BadCaps", false},
		{"bad_name", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isValidPluginName(tt.name); got != tt.want {
			t.Errorf("isValidPluginName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestToTitleAndPascalCase(t *testing.T) {
	if got := toTitle("my-plugin"); got != "My Plugin" {
		t.Fatalf("toTitle unexpected: %q", got)
	}
	if got := toPascalCase("my-plugin"); got != "MyPlugin" {
		t.Fatalf("toPascalCase unexpected: %q", got)
	}
}

func TestGenerateFile(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "README.md")
	data := pluginTemplateData{
		Name:       "test-plugin",
		NameTitle:  "Test Plugin",
		NamePascal: "TestPlugin",
		Module:     "github.com/example/test-plugin",
		Author:     "Example",
		Hooks:      []string{"post-publish"},
		SDKVersion: "v1.0.0",
	}

	if err := generateFile(outPath, pluginReadmeTemplate, data); err != nil {
		t.Fatalf("generateFile error: %v", err)
	}

	contents, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	if !strings.Contains(string(contents), "Test Plugin") {
		t.Fatalf("generated file missing template content")
	}
}

func TestRunPluginCreateCreatesFiles(t *testing.T) {
	origOutput := createOutputDir
	origHooks := createHooks
	origAuthor := createAuthor
	origModule := createModule
	defer func() {
		createOutputDir = origOutput
		createHooks = origHooks
		createAuthor = origAuthor
		createModule = origModule
	}()

	tmpDir := t.TempDir()
	createOutputDir = tmpDir
	createHooks = []string{"post-publish"}
	createAuthor = "Tester"
	createModule = ""

	if err := runPluginCreate(nil, []string{"my-plugin"}); err != nil {
		t.Fatalf("runPluginCreate error: %v", err)
	}

	pluginDir := filepath.Join(tmpDir, "my-plugin")
	for _, name := range []string{"main.go", "go.mod", "README.md", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(pluginDir, name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}
}

func TestRunPluginCreateInvalidName(t *testing.T) {
	if err := runPluginCreate(nil, []string{"bad_name"}); err == nil {
		t.Fatal("expected error for invalid plugin name")
	}
}
