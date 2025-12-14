// Package main implements the Chocolatey package publishing plugin for Relicta.
package main

import (
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

// ChocolateyPlugin implements the Chocolatey package plugin.
type ChocolateyPlugin struct{}

// Config represents the Chocolatey plugin configuration.
type Config struct {
	// PackageID is the Chocolatey package identifier.
	PackageID string `json:"package_id,omitempty"`
	// PackageTitle is the human-readable package title.
	PackageTitle string `json:"package_title,omitempty"`
	// Description is the package description.
	Description string `json:"description,omitempty"`
	// Authors is a comma-separated list of authors.
	Authors string `json:"authors,omitempty"`
	// ProjectURL is the project homepage.
	ProjectURL string `json:"project_url,omitempty"`
	// LicenseURL is the URL to the license file.
	LicenseURL string `json:"license_url,omitempty"`
	// IconURL is the URL to the package icon.
	IconURL string `json:"icon_url,omitempty"`
	// Tags is a comma-separated list of tags.
	Tags string `json:"tags,omitempty"`
	// DownloadURL32 is the URL template for 32-bit download.
	DownloadURL32 string `json:"download_url_32,omitempty"`
	// DownloadURL64 is the URL template for 64-bit download.
	DownloadURL64 string `json:"download_url_64,omitempty"`
	// SilentArgs are arguments for silent installation.
	SilentArgs string `json:"silent_args,omitempty"`
	// APIKey is the Chocolatey API key for pushing packages.
	APIKey string `json:"api_key,omitempty"`
	// Source is the Chocolatey source (default: https://push.chocolatey.org/).
	Source string `json:"source,omitempty"`
	// Dependencies lists package dependencies.
	Dependencies []string `json:"dependencies,omitempty"`
	// OutputDir is the directory for generated packages.
	OutputDir string `json:"output_dir,omitempty"`
}

// NuspecData contains data for nuspec template rendering.
type NuspecData struct {
	PackageID    string
	Version      string
	Title        string
	Authors      string
	Description  string
	ProjectURL   string
	LicenseURL   string
	IconURL      string
	Tags         string
	Dependencies []DependencyData
}

// DependencyData represents a package dependency.
type DependencyData struct {
	ID      string
	Version string
}

// InstallScriptData contains data for install script rendering.
type InstallScriptData struct {
	PackageID   string
	URL32       string
	URL64       string
	Checksum32  string
	Checksum64  string
	SilentArgs  string
	FileType    string
	PackageName string
}

// Default nuspec template.
const defaultNuspecTemplate = `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://schemas.microsoft.com/packaging/2015/06/nuspec.xsd">
  <metadata>
    <id>{{.PackageID}}</id>
    <version>{{.Version}}</version>
    <title>{{.Title}}</title>
    <authors>{{.Authors}}</authors>
    <description>{{.Description}}</description>
{{- if .ProjectURL}}
    <projectUrl>{{.ProjectURL}}</projectUrl>
{{- end}}
{{- if .LicenseURL}}
    <licenseUrl>{{.LicenseURL}}</licenseUrl>
{{- end}}
{{- if .IconURL}}
    <iconUrl>{{.IconURL}}</iconUrl>
{{- end}}
{{- if .Tags}}
    <tags>{{.Tags}}</tags>
{{- end}}
{{- if .Dependencies}}
    <dependencies>
{{- range .Dependencies}}
      <dependency id="{{.ID}}"{{if .Version}} version="{{.Version}}"{{end}} />
{{- end}}
    </dependencies>
{{- end}}
  </metadata>
  <files>
    <file src="tools\**" target="tools" />
  </files>
</package>
`

// Default install script template.
const defaultInstallScriptTemplate = `$ErrorActionPreference = 'Stop'

$packageArgs = @{
  packageName    = '{{.PackageName}}'
  fileType       = '{{.FileType}}'
{{- if .URL32}}
  url            = '{{.URL32}}'
  checksum       = '{{.Checksum32}}'
  checksumType   = 'sha256'
{{- end}}
{{- if .URL64}}
  url64bit       = '{{.URL64}}'
  checksum64     = '{{.Checksum64}}'
  checksumType64 = 'sha256'
{{- end}}
{{- if .SilentArgs}}
  silentArgs     = '{{.SilentArgs}}'
{{- end}}
  validExitCodes = @(0)
}

Install-ChocolateyPackage @packageArgs
`

// GetInfo returns plugin metadata.
func (p *ChocolateyPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "chocolatey",
		Version:     "1.0.0",
		Description: "Build and publish Chocolatey packages for Windows",
		Author:      "Relicta Team",
		Hooks: []plugin.Hook{
			plugin.HookPostPublish,
		},
		ConfigSchema: `{
			"type": "object",
			"properties": {
				"package_id": {"type": "string", "description": "Chocolatey package ID"},
				"package_title": {"type": "string", "description": "Human-readable package title"},
				"description": {"type": "string", "description": "Package description"},
				"authors": {"type": "string", "description": "Comma-separated authors"},
				"project_url": {"type": "string", "description": "Project homepage URL"},
				"license_url": {"type": "string", "description": "License URL"},
				"icon_url": {"type": "string", "description": "Package icon URL"},
				"tags": {"type": "string", "description": "Comma-separated tags"},
				"download_url_32": {"type": "string", "description": "32-bit download URL template"},
				"download_url_64": {"type": "string", "description": "64-bit download URL template"},
				"silent_args": {"type": "string", "description": "Silent installation arguments"},
				"api_key": {"type": "string", "description": "Chocolatey API key (or use CHOCOLATEY_API_KEY env)"},
				"source": {"type": "string", "description": "Chocolatey source URL", "default": "https://push.chocolatey.org/"},
				"dependencies": {"type": "array", "items": {"type": "string"}, "description": "Package dependencies"},
				"output_dir": {"type": "string", "description": "Output directory for packages"}
			},
			"required": ["package_id", "description"]
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *ChocolateyPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	cfg := p.parseConfig(req.Config)

	switch req.Hook {
	case plugin.HookPostPublish:
		return p.buildAndPush(ctx, cfg, req.Context, req.DryRun)
	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

// buildAndPush builds the Chocolatey package and pushes it.
func (p *ChocolateyPlugin) buildAndPush(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	version := strings.TrimPrefix(releaseCtx.Version, "v")
	tag := releaseCtx.TagName

	// Resolve download URLs
	url32 := p.resolveURL(cfg.DownloadURL32, version, tag)
	url64 := p.resolveURL(cfg.DownloadURL64, version, tag)

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Would build and push Chocolatey package",
			Outputs: map[string]any{
				"package_id": cfg.PackageID,
				"version":    version,
				"url_32":     url32,
				"url_64":     url64,
			},
		}, nil
	}

	// Calculate checksums
	var checksum32, checksum64 string
	var err error

	if url32 != "" {
		checksum32, err = p.fetchChecksum(ctx, url32)
		if err != nil {
			return &plugin.ExecuteResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to calculate 32-bit checksum: %v", err),
			}, nil
		}
	}

	if url64 != "" {
		checksum64, err = p.fetchChecksum(ctx, url64)
		if err != nil {
			return &plugin.ExecuteResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to calculate 64-bit checksum: %v", err),
			}, nil
		}
	}

	// Create temp directory for package
	tmpDir, err := os.MkdirTemp("", "chocolatey-*")
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to create temp directory: %v", err),
		}, nil
	}
	defer os.RemoveAll(tmpDir)

	// Create tools directory
	toolsDir := filepath.Join(tmpDir, "tools")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to create tools directory: %v", err),
		}, nil
	}

	// Generate nuspec
	nuspecPath := filepath.Join(tmpDir, cfg.PackageID+".nuspec")
	if err := p.generateNuspec(cfg, version, nuspecPath); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to generate nuspec: %v", err),
		}, nil
	}

	// Generate install script
	installScriptPath := filepath.Join(toolsDir, "chocolateyInstall.ps1")
	if err := p.generateInstallScript(cfg, url32, checksum32, url64, checksum64, installScriptPath); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to generate install script: %v", err),
		}, nil
	}

	// Build package
	nupkgPath, err := p.buildPackage(ctx, tmpDir, cfg.PackageID, version)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to build package: %v", err),
		}, nil
	}

	// Copy to output directory if specified
	if cfg.OutputDir != "" {
		if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
			return &plugin.ExecuteResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to create output directory: %v", err),
			}, nil
		}
		destPath := filepath.Join(cfg.OutputDir, filepath.Base(nupkgPath))
		if err := p.copyFile(nupkgPath, destPath); err != nil {
			return &plugin.ExecuteResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to copy package: %v", err),
			}, nil
		}
		nupkgPath = destPath
	}

	// Push to Chocolatey
	if cfg.APIKey != "" {
		if err := p.pushPackage(ctx, nupkgPath, cfg.APIKey, cfg.Source); err != nil {
			return &plugin.ExecuteResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to push package: %v", err),
			}, nil
		}
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Built and pushed Chocolatey package %s v%s", cfg.PackageID, version),
		Outputs: map[string]any{
			"package_id":   cfg.PackageID,
			"version":      version,
			"package_path": nupkgPath,
		},
		Artifacts: []plugin.Artifact{
			{
				Name: fmt.Sprintf("%s.%s.nupkg", cfg.PackageID, version),
				Path: nupkgPath,
				Type: "nupkg",
			},
		},
	}, nil
}

// resolveURL resolves a URL template with version and tag.
func (p *ChocolateyPlugin) resolveURL(template, version, tag string) string {
	if template == "" {
		return ""
	}
	url := template
	url = strings.ReplaceAll(url, "{{version}}", version)
	url = strings.ReplaceAll(url, "{{tag}}", tag)
	return url
}

// fetchChecksum downloads a file and calculates its SHA256 checksum.
func (p *ChocolateyPlugin) fetchChecksum(ctx context.Context, url string) (string, error) {
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

	return fmt.Sprintf("%X", hash.Sum(nil)), nil
}

// generateNuspec generates the nuspec file.
func (p *ChocolateyPlugin) generateNuspec(cfg *Config, version, path string) error {
	data := NuspecData{
		PackageID:   cfg.PackageID,
		Version:     version,
		Title:       cfg.PackageTitle,
		Authors:     cfg.Authors,
		Description: cfg.Description,
		ProjectURL:  cfg.ProjectURL,
		LicenseURL:  cfg.LicenseURL,
		IconURL:     cfg.IconURL,
		Tags:        cfg.Tags,
	}

	if data.Title == "" {
		data.Title = cfg.PackageID
	}

	// Parse dependencies
	for _, dep := range cfg.Dependencies {
		parts := strings.SplitN(dep, ":", 2)
		d := DependencyData{ID: parts[0]}
		if len(parts) > 1 {
			d.Version = parts[1]
		}
		data.Dependencies = append(data.Dependencies, d)
	}

	tmpl, err := template.New("nuspec").Parse(defaultNuspecTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

// generateInstallScript generates the install script.
func (p *ChocolateyPlugin) generateInstallScript(cfg *Config, url32, checksum32, url64, checksum64, path string) error {
	// Determine file type from URL
	fileType := "exe"
	testURL := url64
	if testURL == "" {
		testURL = url32
	}
	if strings.HasSuffix(testURL, ".msi") {
		fileType = "msi"
	} else if strings.HasSuffix(testURL, ".zip") {
		fileType = "zip"
	}

	data := InstallScriptData{
		PackageName: cfg.PackageID,
		URL32:       url32,
		URL64:       url64,
		Checksum32:  checksum32,
		Checksum64:  checksum64,
		SilentArgs:  cfg.SilentArgs,
		FileType:    fileType,
	}

	tmpl, err := template.New("install").Parse(defaultInstallScriptTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

// buildPackage runs choco pack to build the package.
func (p *ChocolateyPlugin) buildPackage(ctx context.Context, dir, packageID, version string) (string, error) {
	cmd := exec.CommandContext(ctx, "choco", "pack", packageID+".nuspec")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return filepath.Join(dir, fmt.Sprintf("%s.%s.nupkg", packageID, version)), nil
}

// pushPackage pushes the package to Chocolatey.
// Uses environment variable for API key to prevent exposure in process listings.
func (p *ChocolateyPlugin) pushPackage(ctx context.Context, nupkgPath, apiKey, source string) error {
	if source == "" {
		source = "https://push.chocolatey.org/"
	}

	// Use --apikey with environment variable reference to avoid exposing key in process list.
	// Chocolatey CLI reads from CHOCO_API_KEY environment variable when --apikey is not provided.
	cmd := exec.CommandContext(ctx, "choco", "push", nupkgPath,
		"--source", source)

	// Pass API key via environment variable (safer than command-line args)
	cmd.Env = append(os.Environ(), "CHOCO_API_KEY="+apiKey)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// copyFile copies a file from src to dst.
func (p *ChocolateyPlugin) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// parseConfig parses the plugin configuration.
func (p *ChocolateyPlugin) parseConfig(raw map[string]any) *Config {
	parser := plugin.NewConfigParser(raw)

	return &Config{
		PackageID:     parser.GetString("package_id"),
		PackageTitle:  parser.GetString("package_title"),
		Description:   parser.GetString("description"),
		Authors:       parser.GetString("authors"),
		ProjectURL:    parser.GetString("project_url"),
		LicenseURL:    parser.GetString("license_url"),
		IconURL:       parser.GetString("icon_url"),
		Tags:          parser.GetString("tags"),
		DownloadURL32: parser.GetString("download_url_32"),
		DownloadURL64: parser.GetString("download_url_64"),
		SilentArgs:    parser.GetString("silent_args"),
		APIKey:        parser.GetString("api_key", "CHOCOLATEY_API_KEY"),
		Source:        parser.GetStringDefault("source", "https://push.chocolatey.org/"),
		Dependencies:  parser.GetStringSlice("dependencies"),
		OutputDir:     parser.GetString("output_dir"),
	}
}

// Validate validates the plugin configuration.
func (p *ChocolateyPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := plugin.NewValidationBuilder()
	parser := plugin.NewConfigParser(config)

	// Validate package ID
	packageID := parser.GetString("package_id")
	if packageID == "" {
		vb.AddError("package_id", "Chocolatey package ID is required", "required")
	}

	// Validate description
	description := parser.GetString("description")
	if description == "" {
		vb.AddError("description", "Package description is required", "required")
	}

	// Validate at least one download URL
	url32 := parser.GetString("download_url_32")
	url64 := parser.GetString("download_url_64")
	if url32 == "" && url64 == "" {
		vb.AddError("download_url", "At least one download URL (32 or 64-bit) is required", "required")
	}

	// Validate API key for pushing
	apiKey := parser.GetString("api_key", "CHOCOLATEY_API_KEY")
	if apiKey == "" {
		vb.AddWarning("api_key", "Chocolatey API key not set - package will only be built locally")
	}

	return vb.Build(), nil
}
