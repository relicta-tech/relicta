// Package main implements the Homebrew formula publishing plugin for Relicta.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

// HomebrewPlugin implements the Homebrew formula publishing plugin.
type HomebrewPlugin struct{}

// Config represents the Homebrew plugin configuration.
type Config struct {
	// TapRepository is the Homebrew tap repository (e.g., "user/homebrew-tap").
	TapRepository string `json:"tap_repository,omitempty"`
	// FormulaName is the formula name (defaults to project name).
	FormulaName string `json:"formula_name,omitempty"`
	// FormulaPath is the path to the formula file in the tap repo (default: "Formula/<name>.rb").
	FormulaPath string `json:"formula_path,omitempty"`
	// Description is the formula description.
	Description string `json:"description,omitempty"`
	// Homepage is the project homepage URL.
	Homepage string `json:"homepage,omitempty"`
	// License is the project license (e.g., "MIT", "Apache-2.0").
	License string `json:"license,omitempty"`
	// DownloadURLTemplate is the URL template for release downloads.
	// Supports {{version}}, {{tag}}, {{os}}, {{arch}} placeholders.
	DownloadURLTemplate string `json:"download_url_template,omitempty"`
	// GitHubToken is the GitHub token for pushing to the tap (can use env var).
	GitHubToken string `json:"github_token,omitempty"`
	// CommitMessage is the commit message template (supports {{version}}).
	CommitMessage string `json:"commit_message,omitempty"`
	// CreatePR creates a pull request instead of pushing directly.
	CreatePR bool `json:"create_pr"`
	// PRBranch is the branch name for the PR (supports {{version}}).
	PRBranch string `json:"pr_branch,omitempty"`
	// Dependencies lists Homebrew dependencies.
	Dependencies []string `json:"dependencies,omitempty"`
	// InstallScript is a custom install script (Ruby).
	InstallScript string `json:"install_script,omitempty"`
	// TestScript is a custom test script (Ruby).
	TestScript string `json:"test_script,omitempty"`
}

// FormulaData contains data for formula template rendering.
type FormulaData struct {
	ClassName     string
	Description   string
	Homepage      string
	Version       string
	License       string
	URLX86_64     string
	SHA256X86_64  string
	URLArm64      string
	SHA256Arm64   string
	Dependencies  []string
	InstallScript string
	TestScript    string
}

// Default formula template.
const defaultFormulaTemplate = `class {{.ClassName}} < Formula
  desc "{{.Description}}"
  homepage "{{.Homepage}}"
  version "{{.Version}}"
  license "{{.License}}"

  on_macos do
    if Hardware::CPU.intel?
      url "{{.URLX86_64}}"
      sha256 "{{.SHA256X86_64}}"
    elsif Hardware::CPU.arm?
      url "{{.URLArm64}}"
      sha256 "{{.SHA256Arm64}}"
    end
  end

  on_linux do
    if Hardware::CPU.intel?
      url "{{.URLX86_64}}"
      sha256 "{{.SHA256X86_64}}"
    elsif Hardware::CPU.arm?
      url "{{.URLArm64}}"
      sha256 "{{.SHA256Arm64}}"
    end
  end
{{range .Dependencies}}
  depends_on "{{.}}"
{{end}}
  def install
    {{.InstallScript}}
  end

  test do
    {{.TestScript}}
  end
end
`

// GetInfo returns plugin metadata.
func (p *HomebrewPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "homebrew",
		Version:     "1.0.0",
		Description: "Publish Homebrew formula for releases",
		Author:      "Relicta Team",
		Hooks: []plugin.Hook{
			plugin.HookPostPublish,
		},
		ConfigSchema: `{
			"type": "object",
			"properties": {
				"tap_repository": {"type": "string", "description": "Homebrew tap repository (e.g., user/homebrew-tap)"},
				"formula_name": {"type": "string", "description": "Formula name (defaults to project name)"},
				"formula_path": {"type": "string", "description": "Path to formula in tap repo"},
				"description": {"type": "string", "description": "Formula description"},
				"homepage": {"type": "string", "description": "Project homepage URL"},
				"license": {"type": "string", "description": "Project license", "default": "MIT"},
				"download_url_template": {"type": "string", "description": "URL template for downloads"},
				"github_token": {"type": "string", "description": "GitHub token (or use HOMEBREW_GITHUB_TOKEN env)"},
				"commit_message": {"type": "string", "description": "Commit message template"},
				"create_pr": {"type": "boolean", "description": "Create PR instead of direct push", "default": false},
				"pr_branch": {"type": "string", "description": "PR branch name template"},
				"dependencies": {"type": "array", "items": {"type": "string"}, "description": "Homebrew dependencies"},
				"install_script": {"type": "string", "description": "Custom install script (Ruby)"},
				"test_script": {"type": "string", "description": "Custom test script (Ruby)"}
			},
			"required": ["tap_repository", "download_url_template"]
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *HomebrewPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	cfg := p.parseConfig(req.Config)

	switch req.Hook {
	case plugin.HookPostPublish:
		return p.publishFormula(ctx, cfg, req.Context, req.DryRun)
	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

// publishFormula generates and publishes a Homebrew formula.
func (p *HomebrewPlugin) publishFormula(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	// Determine formula name
	formulaName := cfg.FormulaName
	if formulaName == "" {
		formulaName = releaseCtx.RepositoryName
	}

	// Generate download URLs
	version := strings.TrimPrefix(releaseCtx.Version, "v")
	tag := releaseCtx.TagName

	urlX86_64 := p.resolveURL(cfg.DownloadURLTemplate, version, tag, "darwin", "amd64")
	urlArm64 := p.resolveURL(cfg.DownloadURLTemplate, version, tag, "darwin", "arm64")

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Would publish Homebrew formula",
			Outputs: map[string]any{
				"tap_repository": cfg.TapRepository,
				"formula_name":   formulaName,
				"version":        version,
				"url_x86_64":     urlX86_64,
				"url_arm64":      urlArm64,
			},
		}, nil
	}

	// Calculate SHA256 checksums
	sha256X86_64, err := p.fetchSHA256(ctx, urlX86_64)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to fetch x86_64 binary checksum: %v", err),
		}, nil
	}

	sha256Arm64, err := p.fetchSHA256(ctx, urlArm64)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to fetch arm64 binary checksum: %v", err),
		}, nil
	}

	// Generate formula content
	formulaContent, err := p.generateFormula(cfg, formulaName, version, urlX86_64, sha256X86_64, urlArm64, sha256Arm64)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to generate formula: %v", err),
		}, nil
	}

	// Clone tap, update formula, and push
	if err := p.updateTap(ctx, cfg, formulaName, version, formulaContent); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to update tap: %v", err),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Published Homebrew formula %s v%s", formulaName, version),
		Outputs: map[string]any{
			"formula_name": formulaName,
			"version":      version,
			"tap":          cfg.TapRepository,
		},
	}, nil
}

// resolveURL resolves a URL template with version and platform info.
func (p *HomebrewPlugin) resolveURL(template, version, tag, os, arch string) string {
	url := template
	url = strings.ReplaceAll(url, "{{version}}", version)
	url = strings.ReplaceAll(url, "{{tag}}", tag)
	url = strings.ReplaceAll(url, "{{os}}", os)
	url = strings.ReplaceAll(url, "{{arch}}", arch)
	return url
}

// fetchSHA256 downloads a file and calculates its SHA256 checksum.
func (p *HomebrewPlugin) fetchSHA256(ctx context.Context, url string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Minute}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, resp.Body); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// generateFormula generates the formula Ruby content.
func (p *HomebrewPlugin) generateFormula(cfg *Config, name, version, urlX86_64, sha256X86_64, urlArm64, sha256Arm64 string) (string, error) {
	// Convert name to class name (e.g., "my-tool" -> "MyTool")
	className := p.toClassName(name)

	// Default install script
	installScript := cfg.InstallScript
	if installScript == "" {
		installScript = fmt.Sprintf(`bin.install "%s"`, name)
	}

	// Default test script
	testScript := cfg.TestScript
	if testScript == "" {
		testScript = fmt.Sprintf(`system "#{bin}/%s", "--version"`, name)
	}

	data := FormulaData{
		ClassName:     className,
		Description:   cfg.Description,
		Homepage:      cfg.Homepage,
		Version:       version,
		License:       cfg.License,
		URLX86_64:     urlX86_64,
		SHA256X86_64:  sha256X86_64,
		URLArm64:      urlArm64,
		SHA256Arm64:   sha256Arm64,
		Dependencies:  cfg.Dependencies,
		InstallScript: installScript,
		TestScript:    testScript,
	}

	tmpl, err := template.New("formula").Parse(defaultFormulaTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// toClassName converts a hyphenated name to a Ruby class name.
func (p *HomebrewPlugin) toClassName(name string) string {
	parts := strings.Split(name, "-")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

// updateTap clones the tap repo, updates the formula, and pushes changes.
func (p *HomebrewPlugin) updateTap(ctx context.Context, cfg *Config, formulaName, version, formulaContent string) error {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "homebrew-tap-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// Clone tap repository
	repoURL := fmt.Sprintf("https://%s@github.com/%s.git", cfg.GitHubToken, cfg.TapRepository)
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", repoURL, tmpDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %s", string(out))
	}

	// Determine formula path
	formulaPath := cfg.FormulaPath
	if formulaPath == "" {
		formulaPath = fmt.Sprintf("Formula/%s.rb", formulaName)
	}
	fullPath := filepath.Join(tmpDir, formulaPath)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}

	// Write formula file
	if err := os.WriteFile(fullPath, []byte(formulaContent), 0644); err != nil {
		return err
	}

	// Git add
	cmd = exec.CommandContext(ctx, "git", "-C", tmpDir, "add", formulaPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %s", string(out))
	}

	// Prepare commit message
	commitMsg := cfg.CommitMessage
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("%s {{version}}", formulaName)
	}
	commitMsg = strings.ReplaceAll(commitMsg, "{{version}}", version)

	// Git commit
	cmd = exec.CommandContext(ctx, "git", "-C", tmpDir, "commit", "-m", commitMsg)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %s", string(out))
	}

	// Handle PR or direct push
	if cfg.CreatePR {
		// Create branch
		branchName := cfg.PRBranch
		if branchName == "" {
			branchName = fmt.Sprintf("update-%s-{{version}}", formulaName)
		}
		branchName = strings.ReplaceAll(branchName, "{{version}}", version)

		cmd = exec.CommandContext(ctx, "git", "-C", tmpDir, "checkout", "-b", branchName)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git checkout failed: %s", string(out))
		}

		cmd = exec.CommandContext(ctx, "git", "-C", tmpDir, "push", "-u", "origin", branchName)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git push failed: %s", string(out))
		}

		// Create PR using gh CLI if available
		cmd = exec.CommandContext(ctx, "gh", "pr", "create",
			"--repo", cfg.TapRepository,
			"--title", fmt.Sprintf("Update %s to %s", formulaName, version),
			"--body", fmt.Sprintf("Automated formula update for %s version %s", formulaName, version),
			"--head", branchName)
		cmd.Dir = tmpDir
		// Ignore PR creation errors - the push is what matters
		_ = cmd.Run()
	} else {
		// Direct push
		cmd = exec.CommandContext(ctx, "git", "-C", tmpDir, "push")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git push failed: %s", string(out))
		}
	}

	return nil
}

// parseConfig parses the plugin configuration.
func (p *HomebrewPlugin) parseConfig(raw map[string]any) *Config {
	parser := plugin.NewConfigParser(raw)

	return &Config{
		TapRepository:       parser.GetString("tap_repository"),
		FormulaName:         parser.GetString("formula_name"),
		FormulaPath:         parser.GetString("formula_path"),
		Description:         parser.GetString("description"),
		Homepage:            parser.GetString("homepage"),
		License:             parser.GetStringDefault("license", "MIT"),
		DownloadURLTemplate: parser.GetString("download_url_template"),
		GitHubToken:         parser.GetString("github_token", "HOMEBREW_GITHUB_TOKEN", "GITHUB_TOKEN"),
		CommitMessage:       parser.GetString("commit_message"),
		CreatePR:            parser.GetBool("create_pr"),
		PRBranch:            parser.GetString("pr_branch"),
		Dependencies:        parser.GetStringSlice("dependencies"),
		InstallScript:       parser.GetString("install_script"),
		TestScript:          parser.GetString("test_script"),
	}
}

// Validate validates the plugin configuration.
func (p *HomebrewPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := plugin.NewValidationBuilder()
	parser := plugin.NewConfigParser(config)

	// Validate tap repository
	tapRepo := parser.GetString("tap_repository")
	if tapRepo == "" {
		vb.AddError("tap_repository", "Homebrew tap repository is required", "required")
	} else if !strings.Contains(tapRepo, "/") {
		vb.AddFormatError("tap_repository", "must be in format 'owner/repo'")
	}

	// Validate download URL template
	downloadURL := parser.GetString("download_url_template")
	if downloadURL == "" {
		vb.AddError("download_url_template", "Download URL template is required", "required")
	}

	// Validate GitHub token
	token := parser.GetString("github_token", "HOMEBREW_GITHUB_TOKEN", "GITHUB_TOKEN")
	if token == "" {
		vb.AddWarning("github_token", "GitHub token not set - will fail on push")
	}

	return vb.Build(), nil
}
