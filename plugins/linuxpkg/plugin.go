// Package main implements the Linux package repository plugin for Relicta.
// Supports APT (Debian/Ubuntu) and YUM/DNF (RHEL/CentOS/Fedora) repositories.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

// LinuxPkgPlugin implements the Linux package repository plugin.
type LinuxPkgPlugin struct{}

// Config represents the Linux package plugin configuration.
type Config struct {
	// PackageName is the package name.
	PackageName string `json:"package_name,omitempty"`
	// PackageType is the package format: "deb", "rpm", or "both".
	PackageType string `json:"package_type,omitempty"`
	// Description is the package description.
	Description string `json:"description,omitempty"`
	// Maintainer is the package maintainer (email required for deb).
	Maintainer string `json:"maintainer,omitempty"`
	// Homepage is the project homepage URL.
	Homepage string `json:"homepage,omitempty"`
	// License is the package license.
	License string `json:"license,omitempty"`
	// Architecture is the target architecture (amd64, arm64, all).
	Architecture string `json:"architecture,omitempty"`
	// Section is the package section/category.
	Section string `json:"section,omitempty"`
	// Priority is the package priority (optional, default: optional).
	Priority string `json:"priority,omitempty"`
	// Dependencies lists runtime dependencies.
	Dependencies []string `json:"dependencies,omitempty"`
	// BuildDependencies lists build-time dependencies.
	BuildDependencies []string `json:"build_dependencies,omitempty"`
	// Conflicts lists conflicting packages.
	Conflicts []string `json:"conflicts,omitempty"`
	// Replaces lists packages this one replaces.
	Replaces []string `json:"replaces,omitempty"`
	// BinaryPath is the path to the binary to package.
	BinaryPath string `json:"binary_path,omitempty"`
	// InstallPath is where to install the binary (default: /usr/local/bin).
	InstallPath string `json:"install_path,omitempty"`
	// ConfigFiles lists configuration files to include.
	ConfigFiles []string `json:"config_files,omitempty"`
	// PreInstScript is the pre-installation script.
	PreInstScript string `json:"preinst_script,omitempty"`
	// PostInstScript is the post-installation script.
	PostInstScript string `json:"postinst_script,omitempty"`
	// PreRmScript is the pre-removal script.
	PreRmScript string `json:"prerm_script,omitempty"`
	// PostRmScript is the post-removal script.
	PostRmScript string `json:"postrm_script,omitempty"`
	// OutputDir is the directory for generated packages.
	OutputDir string `json:"output_dir,omitempty"`
	// APTRepository is the APT repository configuration.
	APTRepository *APTRepoConfig `json:"apt_repository,omitempty"`
	// YUMRepository is the YUM repository configuration.
	YUMRepository *YUMRepoConfig `json:"yum_repository,omitempty"`
}

// APTRepoConfig contains APT repository configuration.
type APTRepoConfig struct {
	// RepoPath is the local path to the APT repository.
	RepoPath string `json:"repo_path,omitempty"`
	// Distribution is the distribution codename (e.g., "stable", "focal").
	Distribution string `json:"distribution,omitempty"`
	// Component is the repository component (e.g., "main").
	Component string `json:"component,omitempty"`
	// GPGKeyID is the GPG key ID for signing.
	GPGKeyID string `json:"gpg_key_id,omitempty"`
	// GPGPassphrase is the GPG key passphrase.
	GPGPassphrase string `json:"gpg_passphrase,omitempty"`
}

// YUMRepoConfig contains YUM repository configuration.
type YUMRepoConfig struct {
	// RepoPath is the local path to the YUM repository.
	RepoPath string `json:"repo_path,omitempty"`
	// GPGKeyID is the GPG key ID for signing.
	GPGKeyID string `json:"gpg_key_id,omitempty"`
	// GPGPassphrase is the GPG key passphrase.
	GPGPassphrase string `json:"gpg_passphrase,omitempty"`
}

// DebControlData contains data for Debian control file.
type DebControlData struct {
	Package      string
	Version      string
	Architecture string
	Maintainer   string
	Description  string
	Homepage     string
	Section      string
	Priority     string
	Depends      string
	Conflicts    string
	Replaces     string
}

// RPMSpecData contains data for RPM spec file.
type RPMSpecData struct {
	Name        string
	Version     string
	Release     string
	Summary     string
	License     string
	URL         string
	BuildArch   string
	Requires    string
	Conflicts   string
	Description string
	InstallPath string
	BinaryName  string
	PreInst     string
	PostInst    string
	PreRm       string
	PostRm      string
}

// Debian control file template.
const debControlTemplate = `Package: {{.Package}}
Version: {{.Version}}
Architecture: {{.Architecture}}
Maintainer: {{.Maintainer}}
Description: {{.Description}}
{{- if .Homepage}}
Homepage: {{.Homepage}}
{{- end}}
{{- if .Section}}
Section: {{.Section}}
{{- end}}
{{- if .Priority}}
Priority: {{.Priority}}
{{- end}}
{{- if .Depends}}
Depends: {{.Depends}}
{{- end}}
{{- if .Conflicts}}
Conflicts: {{.Conflicts}}
{{- end}}
{{- if .Replaces}}
Replaces: {{.Replaces}}
{{- end}}
`

// RPM spec file template.
const rpmSpecTemplate = `Name:           {{.Name}}
Version:        {{.Version}}
Release:        {{.Release}}
Summary:        {{.Summary}}
License:        {{.License}}
{{- if .URL}}
URL:            {{.URL}}
{{- end}}
BuildArch:      {{.BuildArch}}
{{- if .Requires}}
Requires:       {{.Requires}}
{{- end}}
{{- if .Conflicts}}
Conflicts:      {{.Conflicts}}
{{- end}}

%description
{{.Description}}

%install
mkdir -p %{buildroot}{{.InstallPath}}
install -m 755 %{_sourcedir}/{{.BinaryName}} %{buildroot}{{.InstallPath}}/{{.BinaryName}}

%files
{{.InstallPath}}/{{.BinaryName}}
{{if .PreInst}}
%pre
{{.PreInst}}
{{end}}
{{if .PostInst}}
%post
{{.PostInst}}
{{end}}
{{if .PreRm}}
%preun
{{.PreRm}}
{{end}}
{{if .PostRm}}
%postun
{{.PostRm}}
{{end}}
`

// GetInfo returns plugin metadata.
func (p *LinuxPkgPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "linuxpkg",
		Version:     "1.0.0",
		Description: "Build and publish Linux packages (DEB/RPM) to APT/YUM repositories",
		Author:      "Relicta Team",
		Hooks: []plugin.Hook{
			plugin.HookPostPublish,
		},
		ConfigSchema: `{
			"type": "object",
			"properties": {
				"package_name": {"type": "string", "description": "Package name"},
				"package_type": {"type": "string", "enum": ["deb", "rpm", "both"], "description": "Package format", "default": "both"},
				"description": {"type": "string", "description": "Package description"},
				"maintainer": {"type": "string", "description": "Package maintainer (email for deb)"},
				"homepage": {"type": "string", "description": "Project homepage URL"},
				"license": {"type": "string", "description": "Package license"},
				"architecture": {"type": "string", "description": "Target architecture", "default": "amd64"},
				"section": {"type": "string", "description": "Package section/category"},
				"priority": {"type": "string", "description": "Package priority", "default": "optional"},
				"dependencies": {"type": "array", "items": {"type": "string"}, "description": "Runtime dependencies"},
				"conflicts": {"type": "array", "items": {"type": "string"}, "description": "Conflicting packages"},
				"replaces": {"type": "array", "items": {"type": "string"}, "description": "Packages this one replaces"},
				"binary_path": {"type": "string", "description": "Path to binary to package"},
				"install_path": {"type": "string", "description": "Installation path", "default": "/usr/local/bin"},
				"config_files": {"type": "array", "items": {"type": "string"}, "description": "Configuration files to include"},
				"preinst_script": {"type": "string", "description": "Pre-installation script"},
				"postinst_script": {"type": "string", "description": "Post-installation script"},
				"prerm_script": {"type": "string", "description": "Pre-removal script"},
				"postrm_script": {"type": "string", "description": "Post-removal script"},
				"output_dir": {"type": "string", "description": "Output directory for packages"},
				"apt_repository": {"type": "object", "description": "APT repository configuration"},
				"yum_repository": {"type": "object", "description": "YUM repository configuration"}
			},
			"required": ["package_name", "description", "binary_path"]
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *LinuxPkgPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	cfg := p.parseConfig(req.Config)

	switch req.Hook {
	case plugin.HookPostPublish:
		return p.buildPackages(ctx, cfg, req.Context, req.DryRun)
	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

// buildPackages builds the Linux packages.
func (p *LinuxPkgPlugin) buildPackages(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	version := strings.TrimPrefix(releaseCtx.Version, "v")

	if dryRun {
		packageTypes := cfg.PackageType
		if packageTypes == "" || packageTypes == "both" {
			packageTypes = "deb, rpm"
		}
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Would build Linux packages",
			Outputs: map[string]any{
				"package_name":  cfg.PackageName,
				"version":       version,
				"package_types": packageTypes,
				"architecture":  cfg.Architecture,
			},
		}, nil
	}

	var artifacts []plugin.Artifact
	var errors []string

	// Build DEB package
	if cfg.PackageType == "deb" || cfg.PackageType == "both" || cfg.PackageType == "" {
		debPath, err := p.buildDebPackage(ctx, cfg, version)
		if err != nil {
			errors = append(errors, fmt.Sprintf("DEB: %v", err))
		} else {
			artifacts = append(artifacts, plugin.Artifact{
				Name: filepath.Base(debPath),
				Path: debPath,
				Type: "deb",
			})

			// Add to APT repository if configured
			if cfg.APTRepository != nil && cfg.APTRepository.RepoPath != "" {
				if err := p.addToAPTRepo(ctx, cfg, debPath); err != nil {
					errors = append(errors, fmt.Sprintf("APT repo: %v", err))
				}
			}
		}
	}

	// Build RPM package
	if cfg.PackageType == "rpm" || cfg.PackageType == "both" || cfg.PackageType == "" {
		rpmPath, err := p.buildRPMPackage(ctx, cfg, version)
		if err != nil {
			errors = append(errors, fmt.Sprintf("RPM: %v", err))
		} else {
			artifacts = append(artifacts, plugin.Artifact{
				Name: filepath.Base(rpmPath),
				Path: rpmPath,
				Type: "rpm",
			})

			// Add to YUM repository if configured
			if cfg.YUMRepository != nil && cfg.YUMRepository.RepoPath != "" {
				if err := p.addToYUMRepo(ctx, cfg, rpmPath); err != nil {
					errors = append(errors, fmt.Sprintf("YUM repo: %v", err))
				}
			}
		}
	}

	if len(errors) > 0 && len(artifacts) == 0 {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   strings.Join(errors, "; "),
		}, nil
	}

	message := fmt.Sprintf("Built %d Linux package(s)", len(artifacts))
	if len(errors) > 0 {
		message += fmt.Sprintf(" (warnings: %s)", strings.Join(errors, "; "))
	}

	return &plugin.ExecuteResponse{
		Success:   true,
		Message:   message,
		Artifacts: artifacts,
		Outputs: map[string]any{
			"package_name": cfg.PackageName,
			"version":      version,
			"packages":     len(artifacts),
		},
	}, nil
}

// buildDebPackage builds a Debian package.
func (p *LinuxPkgPlugin) buildDebPackage(ctx context.Context, cfg *Config, version string) (string, error) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "deb-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	// Create DEBIAN directory
	debianDir := filepath.Join(tmpDir, "DEBIAN")
	if err := os.MkdirAll(debianDir, 0755); err != nil {
		return "", err
	}

	// Create install directory
	installPath := cfg.InstallPath
	if installPath == "" {
		installPath = "/usr/local/bin"
	}
	installDir := filepath.Join(tmpDir, installPath[1:]) // Remove leading /
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return "", err
	}

	// Copy binary
	binaryName := filepath.Base(cfg.BinaryPath)
	destPath := filepath.Join(installDir, binaryName)
	if err := p.copyFile(cfg.BinaryPath, destPath); err != nil {
		return "", fmt.Errorf("failed to copy binary: %w", err)
	}
	if err := os.Chmod(destPath, 0755); err != nil {
		return "", err
	}

	// Generate control file
	controlPath := filepath.Join(debianDir, "control")
	if err := p.generateDebControl(cfg, version, controlPath); err != nil {
		return "", err
	}

	// Add maintainer scripts if provided
	if cfg.PreInstScript != "" {
		if err := os.WriteFile(filepath.Join(debianDir, "preinst"), []byte(cfg.PreInstScript), 0755); err != nil {
			return "", err
		}
	}
	if cfg.PostInstScript != "" {
		if err := os.WriteFile(filepath.Join(debianDir, "postinst"), []byte(cfg.PostInstScript), 0755); err != nil {
			return "", err
		}
	}
	if cfg.PreRmScript != "" {
		if err := os.WriteFile(filepath.Join(debianDir, "prerm"), []byte(cfg.PreRmScript), 0755); err != nil {
			return "", err
		}
	}
	if cfg.PostRmScript != "" {
		if err := os.WriteFile(filepath.Join(debianDir, "postrm"), []byte(cfg.PostRmScript), 0755); err != nil {
			return "", err
		}
	}

	// Build package
	arch := cfg.Architecture
	if arch == "" {
		arch = "amd64"
	}
	debName := fmt.Sprintf("%s_%s_%s.deb", cfg.PackageName, version, arch)

	outputDir := cfg.OutputDir
	if outputDir == "" {
		outputDir = "."
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}
	debPath := filepath.Join(outputDir, debName)

	cmd := exec.CommandContext(ctx, "dpkg-deb", "--build", tmpDir, debPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("dpkg-deb failed: %w", err)
	}

	return debPath, nil
}

// generateDebControl generates the Debian control file.
func (p *LinuxPkgPlugin) generateDebControl(cfg *Config, version, path string) error {
	arch := cfg.Architecture
	if arch == "" {
		arch = "amd64"
	}

	data := DebControlData{
		Package:      cfg.PackageName,
		Version:      version,
		Architecture: arch,
		Maintainer:   cfg.Maintainer,
		Description:  cfg.Description,
		Homepage:     cfg.Homepage,
		Section:      cfg.Section,
		Priority:     cfg.Priority,
	}

	if data.Priority == "" {
		data.Priority = "optional"
	}

	if len(cfg.Dependencies) > 0 {
		data.Depends = strings.Join(cfg.Dependencies, ", ")
	}
	if len(cfg.Conflicts) > 0 {
		data.Conflicts = strings.Join(cfg.Conflicts, ", ")
	}
	if len(cfg.Replaces) > 0 {
		data.Replaces = strings.Join(cfg.Replaces, ", ")
	}

	tmpl, err := template.New("control").Parse(debControlTemplate)
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

// buildRPMPackage builds an RPM package.
func (p *LinuxPkgPlugin) buildRPMPackage(ctx context.Context, cfg *Config, version string) (string, error) {
	// Create rpmbuild directory structure
	tmpDir, err := os.MkdirTemp("", "rpm-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	// Create rpmbuild directories
	for _, dir := range []string{"BUILD", "RPMS", "SOURCES", "SPECS", "SRPMS"} {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			return "", err
		}
	}

	// Copy binary to SOURCES
	binaryName := filepath.Base(cfg.BinaryPath)
	sourcePath := filepath.Join(tmpDir, "SOURCES", binaryName)
	if err := p.copyFile(cfg.BinaryPath, sourcePath); err != nil {
		return "", fmt.Errorf("failed to copy binary: %w", err)
	}

	// Generate spec file
	specPath := filepath.Join(tmpDir, "SPECS", cfg.PackageName+".spec")
	if err := p.generateRPMSpec(cfg, version, specPath); err != nil {
		return "", err
	}

	// Build RPM
	cmd := exec.CommandContext(ctx, "rpmbuild",
		"--define", fmt.Sprintf("_topdir %s", tmpDir),
		"-bb", specPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("rpmbuild failed: %w", err)
	}

	// Find the built RPM
	arch := cfg.Architecture
	if arch == "" || arch == "amd64" {
		arch = "x86_64"
	}
	rpmDir := filepath.Join(tmpDir, "RPMS", arch)
	entries, err := os.ReadDir(rpmDir)
	if err != nil {
		// Try noarch
		rpmDir = filepath.Join(tmpDir, "RPMS", "noarch")
		entries, err = os.ReadDir(rpmDir)
		if err != nil {
			return "", fmt.Errorf("could not find built RPM: %w", err)
		}
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("no RPM found in output directory")
	}

	// Copy to output directory
	outputDir := cfg.OutputDir
	if outputDir == "" {
		outputDir = "."
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}

	srcRPM := filepath.Join(rpmDir, entries[0].Name())
	dstRPM := filepath.Join(outputDir, entries[0].Name())
	if err := p.copyFile(srcRPM, dstRPM); err != nil {
		return "", err
	}

	return dstRPM, nil
}

// generateRPMSpec generates the RPM spec file.
func (p *LinuxPkgPlugin) generateRPMSpec(cfg *Config, version, path string) error {
	arch := cfg.Architecture
	switch arch {
	case "":
		arch = "x86_64"
	case "amd64":
		arch = "x86_64"
	}

	installPath := cfg.InstallPath
	if installPath == "" {
		installPath = "/usr/local/bin"
	}

	data := RPMSpecData{
		Name:        cfg.PackageName,
		Version:     version,
		Release:     "1",
		Summary:     strings.Split(cfg.Description, ".")[0], // First sentence
		License:     cfg.License,
		URL:         cfg.Homepage,
		BuildArch:   arch,
		Description: cfg.Description,
		InstallPath: installPath,
		BinaryName:  filepath.Base(cfg.BinaryPath),
		PreInst:     cfg.PreInstScript,
		PostInst:    cfg.PostInstScript,
		PreRm:       cfg.PreRmScript,
		PostRm:      cfg.PostRmScript,
	}

	if data.License == "" {
		data.License = "MIT"
	}

	if len(cfg.Dependencies) > 0 {
		data.Requires = strings.Join(cfg.Dependencies, ", ")
	}
	if len(cfg.Conflicts) > 0 {
		data.Conflicts = strings.Join(cfg.Conflicts, ", ")
	}

	tmpl, err := template.New("spec").Parse(rpmSpecTemplate)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	return os.WriteFile(path, buf.Bytes(), 0644)
}

// addToAPTRepo adds a package to an APT repository.
func (p *LinuxPkgPlugin) addToAPTRepo(ctx context.Context, cfg *Config, debPath string) error {
	repoPath := cfg.APTRepository.RepoPath
	distribution := cfg.APTRepository.Distribution
	if distribution == "" {
		distribution = "stable"
	}
	// component := cfg.APTRepository.Component
	// if component == "" {
	// 	component = "main"
	// }

	// Use reprepro if available
	cmd := exec.CommandContext(ctx, "reprepro",
		"-b", repoPath,
		"includedeb", distribution, debPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// addToYUMRepo adds a package to a YUM repository.
func (p *LinuxPkgPlugin) addToYUMRepo(ctx context.Context, cfg *Config, rpmPath string) error {
	repoPath := cfg.YUMRepository.RepoPath

	// Copy RPM to repository
	destPath := filepath.Join(repoPath, filepath.Base(rpmPath))
	if err := p.copyFile(rpmPath, destPath); err != nil {
		return err
	}

	// Update repository metadata with createrepo
	cmd := exec.CommandContext(ctx, "createrepo", "--update", repoPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// copyFile copies a file from src to dst.
func (p *LinuxPkgPlugin) copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// parseConfig parses the plugin configuration.
func (p *LinuxPkgPlugin) parseConfig(raw map[string]any) *Config {
	parser := plugin.NewConfigParser(raw)

	cfg := &Config{
		PackageName:       parser.GetString("package_name"),
		PackageType:       parser.GetStringDefault("package_type", "both"),
		Description:       parser.GetString("description"),
		Maintainer:        parser.GetString("maintainer"),
		Homepage:          parser.GetString("homepage"),
		License:           parser.GetStringDefault("license", "MIT"),
		Architecture:      parser.GetStringDefault("architecture", "amd64"),
		Section:           parser.GetString("section"),
		Priority:          parser.GetStringDefault("priority", "optional"),
		Dependencies:      parser.GetStringSlice("dependencies"),
		BuildDependencies: parser.GetStringSlice("build_dependencies"),
		Conflicts:         parser.GetStringSlice("conflicts"),
		Replaces:          parser.GetStringSlice("replaces"),
		BinaryPath:        parser.GetString("binary_path"),
		InstallPath:       parser.GetStringDefault("install_path", "/usr/local/bin"),
		ConfigFiles:       parser.GetStringSlice("config_files"),
		PreInstScript:     parser.GetString("preinst_script"),
		PostInstScript:    parser.GetString("postinst_script"),
		PreRmScript:       parser.GetString("prerm_script"),
		PostRmScript:      parser.GetString("postrm_script"),
		OutputDir:         parser.GetString("output_dir"),
	}

	// Parse APT repository config
	if aptRepo, ok := raw["apt_repository"].(map[string]any); ok {
		aptParser := plugin.NewConfigParser(aptRepo)
		cfg.APTRepository = &APTRepoConfig{
			RepoPath:      aptParser.GetString("repo_path"),
			Distribution:  aptParser.GetStringDefault("distribution", "stable"),
			Component:     aptParser.GetStringDefault("component", "main"),
			GPGKeyID:      aptParser.GetString("gpg_key_id"),
			GPGPassphrase: aptParser.GetString("gpg_passphrase", "GPG_PASSPHRASE"),
		}
	}

	// Parse YUM repository config
	if yumRepo, ok := raw["yum_repository"].(map[string]any); ok {
		yumParser := plugin.NewConfigParser(yumRepo)
		cfg.YUMRepository = &YUMRepoConfig{
			RepoPath:      yumParser.GetString("repo_path"),
			GPGKeyID:      yumParser.GetString("gpg_key_id"),
			GPGPassphrase: yumParser.GetString("gpg_passphrase", "GPG_PASSPHRASE"),
		}
	}

	return cfg
}

// Validate validates the plugin configuration.
func (p *LinuxPkgPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := plugin.NewValidationBuilder()
	parser := plugin.NewConfigParser(config)

	// Validate package name
	packageName := parser.GetString("package_name")
	if packageName == "" {
		vb.AddError("package_name", "Package name is required", "required")
	}

	// Validate description
	description := parser.GetString("description")
	if description == "" {
		vb.AddError("description", "Package description is required", "required")
	}

	// Validate binary path
	binaryPath := parser.GetString("binary_path")
	if binaryPath == "" {
		vb.AddError("binary_path", "Binary path is required", "required")
	} else if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		vb.AddWarning("binary_path", fmt.Sprintf("Binary '%s' not found - will fail at build time", binaryPath))
	}

	// Validate package type
	packageType := parser.GetStringDefault("package_type", "both")
	validTypes := []string{"deb", "rpm", "both"}
	valid := false
	for _, t := range validTypes {
		if packageType == t {
			valid = true
			break
		}
	}
	if !valid {
		vb.AddEnumError("package_type", validTypes)
	}

	// Validate maintainer for deb packages
	if packageType == "deb" || packageType == "both" {
		maintainer := parser.GetString("maintainer")
		if maintainer == "" {
			vb.AddWarning("maintainer", "Maintainer recommended for DEB packages")
		}
	}

	return vb.Build(), nil
}
