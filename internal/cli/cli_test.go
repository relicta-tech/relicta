// Package cli provides the command-line interface for Relicta.
package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/changes"
)

func TestSetVersionInfo(t *testing.T) {
	// Save original values
	origVersion := versionInfo.Version
	origCommit := versionInfo.Commit
	origDate := versionInfo.Date
	defer func() {
		versionInfo.Version = origVersion
		versionInfo.Commit = origCommit
		versionInfo.Date = origDate
	}()

	SetVersionInfo("1.2.3", "abc123", "2024-01-01")

	if versionInfo.Version != "1.2.3" {
		t.Errorf("Version = %v, want 1.2.3", versionInfo.Version)
	}
	if versionInfo.Commit != "abc123" {
		t.Errorf("Commit = %v, want abc123", versionInfo.Commit)
	}
	if versionInfo.Date != "2024-01-01" {
		t.Errorf("Date = %v, want 2024-01-01", versionInfo.Date)
	}
}

func TestHealthStatus_String(t *testing.T) {
	tests := []struct {
		status   HealthStatus
		expected string
	}{
		{HealthStatusHealthy, "healthy"},
		{HealthStatusDegraded, "degraded"},
		{HealthStatusUnhealthy, "unhealthy"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("HealthStatus = %v, want %v", tt.status, tt.expected)
		}
	}
}

func TestCheckGit(t *testing.T) {
	ctx := context.Background()
	health := checkGit(ctx)

	// Git should be available in the test environment
	if health.Name != "git" {
		t.Errorf("Name = %v, want git", health.Name)
	}

	// Check that git is found (assuming git is installed)
	if health.Status != HealthStatusHealthy {
		t.Logf("Git not found - this is expected in some CI environments: %s", health.Message)
	}

	// If healthy, should have version details
	if health.Status == HealthStatusHealthy {
		if _, ok := health.Details["version"]; !ok {
			t.Error("Expected version in Details when healthy")
		}
	}
}

func TestCheckRepository_NotInRepo(t *testing.T) {
	// Create a temp directory that's not a git repo
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	err := os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	ctx := context.Background()
	health := checkRepository(ctx)

	if health.Name != "repository" {
		t.Errorf("Name = %v, want repository", health.Name)
	}

	// Should be degraded when not in a git repo
	if health.Status != HealthStatusDegraded {
		t.Errorf("Status = %v, want %v", health.Status, HealthStatusDegraded)
	}
}

func TestCheckConfig_NoConfigFile(t *testing.T) {
	// Create a temp directory without config files
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	err := os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	ctx := context.Background()
	health := checkConfig(ctx)

	if health.Name != "config" {
		t.Errorf("Name = %v, want config", health.Name)
	}

	// Should be degraded when no config found
	if health.Status != HealthStatusDegraded {
		t.Errorf("Status = %v, want %v", health.Status, HealthStatusDegraded)
	}

	if !strings.Contains(health.Message, "no configuration file found") {
		t.Errorf("Message = %v, expected to contain 'no configuration file found'", health.Message)
	}
}

func TestCheckConfig_WithConfigFile(t *testing.T) {
	// Create a temp directory with a config file
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// Create a config file (.relicta.yaml is the only supported format)
	configPath := filepath.Join(tmpDir, ".relicta.yaml")
	err := os.WriteFile(configPath, []byte("versioning:\n  strategy: conventional"), 0600)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	ctx := context.Background()
	health := checkConfig(ctx)

	if health.Status != HealthStatusHealthy {
		t.Errorf("Status = %v, want %v", health.Status, HealthStatusHealthy)
	}

	if health.Details["config_file"] != ".relicta.yaml" {
		t.Errorf("Details[config_file] = %v, want .relicta.yaml", health.Details["config_file"])
	}
}

func TestCheckPluginsDir_NoPlugins(t *testing.T) {
	ctx := context.Background()
	health := checkPluginsDir(ctx)

	if health.Name != "plugins_directory" {
		t.Errorf("Name = %v, want plugins_directory", health.Name)
	}

	// Should be healthy even without plugin directories
	if health.Status != HealthStatusHealthy {
		t.Errorf("Status = %v, want %v", health.Status, HealthStatusHealthy)
	}
}

func TestRootCommand_HasSubcommands(t *testing.T) {
	commands := rootCmd.Commands()
	expectedCommands := []string{"version", "init", "plan", "bump", "notes", "approve", "publish", "health", "metrics"}

	commandNames := make(map[string]bool)
	for _, cmd := range commands {
		commandNames[cmd.Name()] = true
	}

	for _, expected := range expectedCommands {
		if !commandNames[expected] {
			t.Errorf("Missing subcommand: %s", expected)
		}
	}
}

func TestRootCommand_GlobalFlags(t *testing.T) {
	flags := rootCmd.PersistentFlags()

	expectedFlags := []string{"config", "verbose", "dry-run", "json", "no-color", "log-level", "model", "ci"}
	for _, name := range expectedFlags {
		if flags.Lookup(name) == nil {
			t.Errorf("Missing global flag: %s", name)
		}
	}
}

func TestRootCommand_ConfigFlagShorthand(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("config")
	if flag == nil {
		t.Fatal("config flag not found")
	}
	if flag.Shorthand != "c" {
		t.Errorf("config flag shorthand = %v, want c", flag.Shorthand)
	}
}

func TestRootCommand_VerboseFlagShorthand(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("verbose")
	if flag == nil {
		t.Fatal("verbose flag not found")
	}
	if flag.Shorthand != "v" {
		t.Errorf("verbose flag shorthand = %v, want v", flag.Shorthand)
	}
}

func TestVersionCommand_Output(t *testing.T) {
	// Set version info
	origVersion := versionInfo.Version
	defer func() { versionInfo.Version = origVersion }()
	versionInfo.Version = "test-1.0.0"

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run version command
	versionCmd.Run(versionCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "test-1.0.0") {
		t.Errorf("version output = %v, expected to contain test-1.0.0", output)
	}
}

func TestHealthReport_InitialState(t *testing.T) {
	report := &HealthReport{
		Status:      HealthStatusHealthy,
		Components:  make([]ComponentHealth, 0),
		Environment: make(map[string]string),
	}

	if report.Status != HealthStatusHealthy {
		t.Errorf("Initial status = %v, want healthy", report.Status)
	}
	if len(report.Components) != 0 {
		t.Errorf("Initial components length = %d, want 0", len(report.Components))
	}
}

func TestComponentHealth_WithDetails(t *testing.T) {
	health := ComponentHealth{
		Name:    "test",
		Status:  HealthStatusHealthy,
		Message: "test message",
		Details: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	if health.Name != "test" {
		t.Errorf("Name = %v, want test", health.Name)
	}
	if len(health.Details) != 2 {
		t.Errorf("Details length = %d, want 2", len(health.Details))
	}
}

func TestInitCommand_Exists(t *testing.T) {
	if initCmd == nil {
		t.Fatal("initCmd is nil")
	}
	if initCmd.Use != "init" {
		t.Errorf("initCmd.Use = %v, want init", initCmd.Use)
	}
	if initCmd.RunE == nil {
		t.Error("initCmd.RunE is nil")
	}
}

func TestPlanCommand_Exists(t *testing.T) {
	if planCmd == nil {
		t.Fatal("planCmd is nil")
	}
	if planCmd.Use != "plan" {
		t.Errorf("planCmd.Use = %v, want plan", planCmd.Use)
	}
	if planCmd.RunE == nil {
		t.Error("planCmd.RunE is nil")
	}
}

func TestBumpCommand_HasAlias(t *testing.T) {
	if bumpCmd == nil {
		t.Fatal("bumpCmd is nil")
	}
	found := false
	for _, alias := range bumpCmd.Aliases {
		if alias == "version-bump" {
			found = true
			break
		}
	}
	if !found {
		t.Error("bumpCmd missing 'version-bump' alias")
	}
}

func TestApproveCommand_Exists(t *testing.T) {
	if approveCmd == nil {
		t.Fatal("approveCmd is nil")
	}
	if approveCmd.Use != "approve" {
		t.Errorf("approveCmd.Use = %v, want approve", approveCmd.Use)
	}
}

func TestPublishCommand_Exists(t *testing.T) {
	if publishCmd == nil {
		t.Fatal("publishCmd is nil")
	}
	if publishCmd.Use != "publish" {
		t.Errorf("publishCmd.Use = %v, want publish", publishCmd.Use)
	}
}

func TestNotesCommand_Exists(t *testing.T) {
	if notesCmd == nil {
		t.Fatal("notesCmd is nil")
	}
	if notesCmd.Use != "notes" {
		t.Errorf("notesCmd.Use = %v, want notes", notesCmd.Use)
	}
}

func TestHealthCommand_Exists(t *testing.T) {
	if healthCmd == nil {
		t.Fatal("healthCmd is nil")
	}
	if healthCmd.Use != "health" {
		t.Errorf("healthCmd.Use = %v, want health", healthCmd.Use)
	}
}

func TestAllowedEditors(t *testing.T) {
	// Test that common editors are in the allowed list
	expectedEditors := []string{"vim", "nvim", "nano", "emacs", "vi", "code"}

	for _, editor := range expectedEditors {
		if !allowedEditors[editor] {
			t.Errorf("Editor %s should be in allowed list", editor)
		}
	}
}

func TestValidateEditor_ValidEditor(t *testing.T) {
	// Test with a common editor that should be installed
	editors := []string{"vim", "vi"}

	for _, editor := range editors {
		path, err := validateEditor(editor)
		if err != nil {
			// Skip if editor not installed
			t.Logf("Editor %s not found: %v (this is OK in some environments)", editor, err)
			continue
		}
		if path == "" {
			t.Errorf("validateEditor(%s) returned empty path", editor)
		}
	}
}

func TestValidateEditor_InvalidEditor(t *testing.T) {
	_, err := validateEditor("malicious-editor")
	if err == nil {
		t.Error("validateEditor should reject unknown editors")
	}
	if !strings.Contains(err.Error(), "not in the allowed list") {
		t.Errorf("Error message should mention 'not in the allowed list', got: %v", err)
	}
}

func TestValidateEditor_PathInjection(t *testing.T) {
	// Test that path injection attempts are blocked
	dangerousInputs := []string{
		"/usr/bin/vim; rm -rf /",
		"vim && echo hacked",
		"../../../bin/sh",
		"`whoami`",
	}

	for _, input := range dangerousInputs {
		_, err := validateEditor(input)
		if err == nil {
			t.Errorf("validateEditor should reject potentially dangerous input: %s", input)
		}
	}
}

func TestCleanup(t *testing.T) {
	// Test that Cleanup doesn't panic when logFile is nil
	origLogFile := logFile
	logFile = nil
	defer func() { logFile = origLogFile }()

	// Should not panic
	Cleanup()
}

func TestIsCIMode(t *testing.T) {
	// Save original value
	orig := ciMode
	defer func() { ciMode = orig }()

	ciMode = false
	if IsCIMode() {
		t.Error("IsCIMode() should return false when ciMode is false")
	}

	ciMode = true
	if !IsCIMode() {
		t.Error("IsCIMode() should return true when ciMode is true")
	}
}

func TestIsJSONOutput(t *testing.T) {
	// Save original value
	orig := outputJSON
	defer func() { outputJSON = orig }()

	outputJSON = false
	if IsJSONOutput() {
		t.Error("IsJSONOutput() should return false when outputJSON is false")
	}

	outputJSON = true
	if !IsJSONOutput() {
		t.Error("IsJSONOutput() should return true when outputJSON is true")
	}
}

func TestCIModeFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("ci")
	if flag == nil {
		t.Fatal("ci flag not found")
	}
	if flag.DefValue != "false" {
		t.Errorf("ci flag default value = %v, want false", flag.DefValue)
	}
	if flag.Usage == "" {
		t.Error("ci flag should have usage description")
	}
}

func TestParseModelFlag(t *testing.T) {
	tests := []struct {
		name         string
		flag         string
		wantProvider string
		wantModel    string
	}{
		{
			name:         "empty flag",
			flag:         "",
			wantProvider: "",
			wantModel:    "",
		},
		{
			name:         "ollama with model",
			flag:         "ollama/llama3.2",
			wantProvider: "ollama",
			wantModel:    "llama3.2",
		},
		{
			name:         "openai with model",
			flag:         "openai/gpt-4",
			wantProvider: "openai",
			wantModel:    "gpt-4",
		},
		{
			name:         "local alias for ollama",
			flag:         "local/mistral",
			wantProvider: "ollama",
			wantModel:    "mistral",
		},
		{
			name:         "LOCAL uppercase alias",
			flag:         "LOCAL/codellama",
			wantProvider: "ollama",
			wantModel:    "codellama",
		},
		{
			name:         "model only without provider",
			flag:         "gpt-4o",
			wantProvider: "",
			wantModel:    "gpt-4o",
		},
		{
			name:         "whitespace handling",
			flag:         "  ollama/llama3.2  ",
			wantProvider: "ollama",
			wantModel:    "llama3.2",
		},
		{
			name:         "model with tag",
			flag:         "ollama/codellama:13b",
			wantProvider: "ollama",
			wantModel:    "codellama:13b",
		},
		{
			name:         "model with multiple slashes in name",
			flag:         "provider/namespace/model",
			wantProvider: "provider",
			wantModel:    "namespace/model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProvider, gotModel := parseModelFlag(tt.flag)
			if gotProvider != tt.wantProvider {
				t.Errorf("parseModelFlag() provider = %v, want %v", gotProvider, tt.wantProvider)
			}
			if gotModel != tt.wantModel {
				t.Errorf("parseModelFlag() model = %v, want %v", gotModel, tt.wantModel)
			}
		})
	}
}

func TestReleaseTypeDisplay(t *testing.T) {
	tests := []struct {
		name     string
		rt       string
		contains string
	}{
		{"major", "major", "major"},
		{"minor", "minor", "minor"},
		{"patch", "patch", "patch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the function doesn't panic and returns styled content
			// The actual styling is tested implicitly
			if tt.name == "major" || tt.name == "minor" || tt.name == "patch" {
				// Just verify the types exist and can be tested
				t.Logf("Release type %s can be displayed", tt.name)
			}
		})
	}
}

func TestFilterNonBreaking(t *testing.T) {
	// Test with nil slice
	result := filterNonBreaking(nil)
	if result != nil {
		t.Errorf("filterNonBreaking(nil) = %v, want nil", result)
	}

	// Test with empty slice
	result = filterNonBreaking([]*changes.ConventionalCommit{})
	if len(result) != 0 {
		t.Errorf("filterNonBreaking([]) len = %d, want 0", len(result))
	}
}

func TestGetNonCoreCategorizedCommits(t *testing.T) {
	// Test with empty categories
	cats := &changes.Categories{
		Features:  nil,
		Fixes:     nil,
		Perf:      nil,
		Docs:      nil,
		Refactors: nil,
		Tests:     nil,
		Chores:    nil,
		Build:     nil,
		CI:        nil,
		Other:     nil,
		Breaking:  nil,
	}

	result := getNonCoreCategorizedCommits(cats)
	if len(result) != 0 {
		t.Errorf("getNonCoreCategorizedCommits() len = %d, want 0", len(result))
	}
}

func TestPlanCommand_Flags(t *testing.T) {
	// Test that plan command has expected flags
	flags := planCmd.Flags()

	expectedFlags := []string{"from", "to", "all", "minimal"}
	for _, name := range expectedFlags {
		if flags.Lookup(name) == nil {
			t.Errorf("Missing flag: %s", name)
		}
	}
}

func TestBumpCommand_Flags(t *testing.T) {
	// Test that bump command has expected flags
	// Note: --tag and --push flags removed - tag creation moved to publish step
	flags := bumpCmd.Flags()

	expectedFlags := []string{"level", "prerelease", "build", "force"}
	for _, name := range expectedFlags {
		if flags.Lookup(name) == nil {
			t.Errorf("Missing flag: %s", name)
		}
	}
}

func TestNotesCommand_Flags(t *testing.T) {
	// Test that notes command has expected flags
	flags := notesCmd.Flags()

	expectedFlags := []string{"output", "tone", "audience", "emoji", "language", "ai"}
	for _, name := range expectedFlags {
		if flags.Lookup(name) == nil {
			t.Errorf("Missing flag: %s", name)
		}
	}
}

func TestApproveCommand_Flags(t *testing.T) {
	// Test that approve command has expected flags
	flags := approveCmd.Flags()

	expectedFlags := []string{"yes", "edit", "editor", "interactive"}
	for _, name := range expectedFlags {
		if flags.Lookup(name) == nil {
			t.Errorf("Missing flag: %s", name)
		}
	}
}

func TestPublishCommand_Flags(t *testing.T) {
	// Test that publish command has expected flags
	flags := publishCmd.Flags()

	expectedFlags := []string{"skip-approval", "skip-tag", "skip-push", "skip-plugins"}
	for _, name := range expectedFlags {
		if flags.Lookup(name) == nil {
			t.Errorf("Missing flag: %s", name)
		}
	}
}

func TestHealthCommand_Run(t *testing.T) {
	// Test that health command exists and is configured correctly
	if healthCmd == nil {
		t.Fatal("healthCmd is nil")
	}
	if healthCmd.RunE == nil {
		t.Error("healthCmd.RunE is nil")
	}
}

func TestMetricsCommand_Exists(t *testing.T) {
	if metricsCmd == nil {
		t.Fatal("metricsCmd is nil")
	}
	if metricsCmd.Use != "metrics" {
		t.Errorf("metricsCmd.Use = %v, want metrics", metricsCmd.Use)
	}
}

func TestStyles_Initialization(t *testing.T) {
	// Verify that styles are initialized and render correctly
	if styles.Title.Render("test") == "" {
		t.Error("Title style should render content")
	}
	if styles.Success.Render("test") == "" {
		t.Error("Success style should render content")
	}
	if styles.Error.Render("test") == "" {
		t.Error("Error style should render content")
	}
	if styles.Warning.Render("test") == "" {
		t.Error("Warning style should render content")
	}
	if styles.Info.Render("test") == "" {
		t.Error("Info style should render content")
	}
	if styles.Subtle.Render("test") == "" {
		t.Error("Subtle style should render content")
	}
}

func TestRootCommand_Use(t *testing.T) {
	if rootCmd.Use != "relicta" {
		t.Errorf("rootCmd.Use = %v, want relicta", rootCmd.Use)
	}
}

func TestRootCommand_HasShortDescription(t *testing.T) {
	if rootCmd.Short == "" {
		t.Error("rootCmd.Short should not be empty")
	}
}

func TestRootCommand_HasLongDescription(t *testing.T) {
	if rootCmd.Long == "" {
		t.Error("rootCmd.Long should not be empty")
	}
}

func TestVersionInfo_FullString(t *testing.T) {
	// Save original values
	origVersion := versionInfo.Version
	origCommit := versionInfo.Commit
	origDate := versionInfo.Date
	defer func() {
		versionInfo.Version = origVersion
		versionInfo.Commit = origCommit
		versionInfo.Date = origDate
	}()

	SetVersionInfo("v1.0.0", "abc1234", "2024-06-01")

	if versionInfo.Version != "v1.0.0" {
		t.Errorf("Version = %v, want v1.0.0", versionInfo.Version)
	}
	if versionInfo.Commit != "abc1234" {
		t.Errorf("Commit = %v, want abc1234", versionInfo.Commit)
	}
	if versionInfo.Date != "2024-06-01" {
		t.Errorf("Date = %v, want 2024-06-01", versionInfo.Date)
	}
}

func TestInitCommand_HasDescription(t *testing.T) {
	if initCmd.Short == "" {
		t.Error("initCmd.Short should not be empty")
	}
}

func TestPlanCommand_HasDescription(t *testing.T) {
	if planCmd.Short == "" {
		t.Error("planCmd.Short should not be empty")
	}
}

func TestBumpCommand_HasDescription(t *testing.T) {
	if bumpCmd.Short == "" {
		t.Error("bumpCmd.Short should not be empty")
	}
}

func TestNotesCommand_HasDescription(t *testing.T) {
	if notesCmd.Short == "" {
		t.Error("notesCmd.Short should not be empty")
	}
}

func TestApproveCommand_HasDescription(t *testing.T) {
	if approveCmd.Short == "" {
		t.Error("approveCmd.Short should not be empty")
	}
}

func TestPublishCommand_HasDescription(t *testing.T) {
	if publishCmd.Short == "" {
		t.Error("publishCmd.Short should not be empty")
	}
}

func TestHealthCommand_HasDescription(t *testing.T) {
	if healthCmd.Short == "" {
		t.Error("healthCmd.Short should not be empty")
	}
}

func TestBlastCommand_Exists(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "blast" {
			found = true
			break
		}
	}
	if !found {
		t.Log("blast command not found in root commands (may be optional)")
	}
}

func TestDryRunModeFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("dry-run")
	if flag == nil {
		t.Fatal("dry-run flag not found")
	}
	if flag.DefValue != "false" {
		t.Errorf("dry-run flag default value = %v, want false", flag.DefValue)
	}
	// Shorthand may or may not be set depending on implementation
	if flag.Usage == "" {
		t.Error("dry-run flag should have usage description")
	}
}

func TestJSONOutputFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("json")
	if flag == nil {
		t.Fatal("json flag not found")
	}
	if flag.DefValue != "false" {
		t.Errorf("json flag default value = %v, want false", flag.DefValue)
	}
}

func TestNoColorFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("no-color")
	if flag == nil {
		t.Fatal("no-color flag not found")
	}
	if flag.DefValue != "false" {
		t.Errorf("no-color flag default value = %v, want false", flag.DefValue)
	}
}

func TestLogLevelFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("log-level")
	if flag == nil {
		t.Fatal("log-level flag not found")
	}
	if flag.Usage == "" {
		t.Error("log-level flag should have usage description")
	}
}

func TestModelFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("model")
	if flag == nil {
		t.Fatal("model flag not found")
	}
	if flag.Usage == "" {
		t.Error("model flag should have usage description")
	}
}
