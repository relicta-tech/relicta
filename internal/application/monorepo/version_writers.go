// Package monorepo provides application services for multi-package versioning.
package monorepo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/relicta-tech/relicta/internal/domain/monorepo"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// VersionWriterRegistry provides access to version file writers by package type.
type VersionWriterRegistry struct {
	writers map[monorepo.PackageType]monorepo.VersionFileWriter
}

// NewVersionWriterRegistry creates a registry with all built-in writers.
func NewVersionWriterRegistry() *VersionWriterRegistry {
	return &VersionWriterRegistry{
		writers: map[monorepo.PackageType]monorepo.VersionFileWriter{
			monorepo.PackageTypeNPM:       &NPMVersionWriter{},
			monorepo.PackageTypeCargo:     &CargoVersionWriter{},
			monorepo.PackageTypePython:    &PythonVersionWriter{},
			monorepo.PackageTypeGoModule:  &GoModuleVersionWriter{},
			monorepo.PackageTypeMaven:     &MavenVersionWriter{},
			monorepo.PackageTypeGradle:    &GradleVersionWriter{},
			monorepo.PackageTypeComposer:  &ComposerVersionWriter{},
			monorepo.PackageTypeGem:       &GemVersionWriter{},
			monorepo.PackageTypeNuGet:     &NuGetVersionWriter{},
			monorepo.PackageTypeDirectory: &DirectoryVersionWriter{},
		},
	}
}

// GetWriter returns the writer for a package type.
func (r *VersionWriterRegistry) GetWriter(pkgType monorepo.PackageType) (monorepo.VersionFileWriter, bool) {
	w, ok := r.writers[pkgType]
	return w, ok
}

// -------------------------------------------------------------------
// NPM Version Writer (package.json)
// -------------------------------------------------------------------

// NPMVersionWriter handles versioning for NPM packages.
type NPMVersionWriter struct{}

// CanHandle returns true for NPM packages.
func (w *NPMVersionWriter) CanHandle(pkgType monorepo.PackageType) bool {
	return pkgType == monorepo.PackageTypeNPM
}

// ReadVersion reads the version from package.json.
func (w *NPMVersionWriter) ReadVersion(ctx context.Context, pkgPath string) (string, error) {
	pkgJSONPath := filepath.Join(pkgPath, "package.json")
	data, err := os.ReadFile(pkgJSONPath)
	if err != nil {
		return "", fmt.Errorf("reading package.json: %w", err)
	}

	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", fmt.Errorf("parsing package.json: %w", err)
	}

	return pkg.Version, nil
}

// WriteVersion updates the version in package.json.
func (w *NPMVersionWriter) WriteVersion(ctx context.Context, pkgPath, ver string) error {
	pkgJSONPath := filepath.Join(pkgPath, "package.json")
	data, err := os.ReadFile(pkgJSONPath)
	if err != nil {
		return fmt.Errorf("reading package.json: %w", err)
	}

	// Parse as generic map to preserve all fields
	var pkg map[string]interface{}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return fmt.Errorf("parsing package.json: %w", err)
	}

	pkg["version"] = ver

	// Write back with indentation
	output, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling package.json: %w", err)
	}

	// Append newline for POSIX compliance
	output = append(output, '\n')

	if err := os.WriteFile(pkgJSONPath, output, 0644); err != nil {
		return fmt.Errorf("writing package.json: %w", err)
	}

	return nil
}

// Files returns the files that will be modified.
func (w *NPMVersionWriter) Files(pkgPath string) []string {
	return []string{filepath.Join(pkgPath, "package.json")}
}

// -------------------------------------------------------------------
// Cargo Version Writer (Cargo.toml)
// -------------------------------------------------------------------

// CargoVersionWriter handles versioning for Rust/Cargo packages.
type CargoVersionWriter struct{}

// CanHandle returns true for Cargo packages.
func (w *CargoVersionWriter) CanHandle(pkgType monorepo.PackageType) bool {
	return pkgType == monorepo.PackageTypeCargo
}

// ReadVersion reads the version from Cargo.toml.
func (w *CargoVersionWriter) ReadVersion(ctx context.Context, pkgPath string) (string, error) {
	cargoPath := filepath.Join(pkgPath, "Cargo.toml")
	data, err := os.ReadFile(cargoPath)
	if err != nil {
		return "", fmt.Errorf("reading Cargo.toml: %w", err)
	}

	// Simple regex for version in [package] section
	re := regexp.MustCompile(`(?m)^\s*version\s*=\s*"([^"]+)"`)
	matches := re.FindSubmatch(data)
	if len(matches) < 2 {
		return "", fmt.Errorf("version not found in Cargo.toml")
	}

	return string(matches[1]), nil
}

// WriteVersion updates the version in Cargo.toml.
func (w *CargoVersionWriter) WriteVersion(ctx context.Context, pkgPath, ver string) error {
	cargoPath := filepath.Join(pkgPath, "Cargo.toml")
	data, err := os.ReadFile(cargoPath)
	if err != nil {
		return fmt.Errorf("reading Cargo.toml: %w", err)
	}

	// Replace version in [package] section
	re := regexp.MustCompile(`(?m)^(\s*version\s*=\s*)"[^"]+"`)
	newData := re.ReplaceAll(data, []byte(fmt.Sprintf(`${1}"%s"`, ver)))

	if err := os.WriteFile(cargoPath, newData, 0644); err != nil {
		return fmt.Errorf("writing Cargo.toml: %w", err)
	}

	return nil
}

// Files returns the files that will be modified.
func (w *CargoVersionWriter) Files(pkgPath string) []string {
	return []string{filepath.Join(pkgPath, "Cargo.toml")}
}

// -------------------------------------------------------------------
// Python Version Writer (pyproject.toml, setup.py, __version__.py)
// -------------------------------------------------------------------

// PythonVersionWriter handles versioning for Python packages.
type PythonVersionWriter struct{}

// CanHandle returns true for Python packages.
func (w *PythonVersionWriter) CanHandle(pkgType monorepo.PackageType) bool {
	return pkgType == monorepo.PackageTypePython
}

// ReadVersion reads the version from Python package files.
func (w *PythonVersionWriter) ReadVersion(ctx context.Context, pkgPath string) (string, error) {
	// Try pyproject.toml first
	pyprojectPath := filepath.Join(pkgPath, "pyproject.toml")
	if data, err := os.ReadFile(pyprojectPath); err == nil {
		re := regexp.MustCompile(`(?m)^\s*version\s*=\s*"([^"]+)"`)
		if matches := re.FindSubmatch(data); len(matches) >= 2 {
			return string(matches[1]), nil
		}
	}

	// Try setup.py
	setupPath := filepath.Join(pkgPath, "setup.py")
	if data, err := os.ReadFile(setupPath); err == nil {
		re := regexp.MustCompile(`version\s*=\s*["']([^"']+)["']`)
		if matches := re.FindSubmatch(data); len(matches) >= 2 {
			return string(matches[1]), nil
		}
	}

	// Try __version__.py
	versionPath := filepath.Join(pkgPath, "__version__.py")
	if data, err := os.ReadFile(versionPath); err == nil {
		re := regexp.MustCompile(`__version__\s*=\s*["']([^"']+)["']`)
		if matches := re.FindSubmatch(data); len(matches) >= 2 {
			return string(matches[1]), nil
		}
	}

	return "", fmt.Errorf("version not found in Python package files")
}

// WriteVersion updates the version in Python package files.
func (w *PythonVersionWriter) WriteVersion(ctx context.Context, pkgPath, ver string) error {
	var wrote bool

	// Update pyproject.toml if exists
	pyprojectPath := filepath.Join(pkgPath, "pyproject.toml")
	if data, err := os.ReadFile(pyprojectPath); err == nil {
		re := regexp.MustCompile(`(?m)^(\s*version\s*=\s*)"[^"]+"`)
		if re.Match(data) {
			newData := re.ReplaceAll(data, []byte(fmt.Sprintf(`${1}"%s"`, ver)))
			if err := os.WriteFile(pyprojectPath, newData, 0644); err != nil {
				return fmt.Errorf("writing pyproject.toml: %w", err)
			}
			wrote = true
		}
	}

	// Update setup.py if exists
	setupPath := filepath.Join(pkgPath, "setup.py")
	if data, err := os.ReadFile(setupPath); err == nil {
		re := regexp.MustCompile(`(version\s*=\s*)["'][^"']+["']`)
		if re.Match(data) {
			newData := re.ReplaceAll(data, []byte(fmt.Sprintf(`${1}"%s"`, ver)))
			if err := os.WriteFile(setupPath, newData, 0644); err != nil {
				return fmt.Errorf("writing setup.py: %w", err)
			}
			wrote = true
		}
	}

	// Update __version__.py if exists
	versionPath := filepath.Join(pkgPath, "__version__.py")
	if data, err := os.ReadFile(versionPath); err == nil {
		re := regexp.MustCompile(`(__version__\s*=\s*)["'][^"']+["']`)
		newData := re.ReplaceAll(data, []byte(fmt.Sprintf(`${1}"%s"`, ver)))
		if err := os.WriteFile(versionPath, newData, 0644); err != nil {
			return fmt.Errorf("writing __version__.py: %w", err)
		}
		wrote = true
	}

	if !wrote {
		return fmt.Errorf("no Python version files found to update")
	}

	return nil
}

// Files returns the files that will be modified.
func (w *PythonVersionWriter) Files(pkgPath string) []string {
	var files []string
	for _, f := range []string{"pyproject.toml", "setup.py", "__version__.py"} {
		path := filepath.Join(pkgPath, f)
		if _, err := os.Stat(path); err == nil {
			files = append(files, path)
		}
	}
	return files
}

// -------------------------------------------------------------------
// Go Module Version Writer (version.go or dedicated version file)
// -------------------------------------------------------------------

// GoModuleVersionWriter handles versioning for Go modules.
type GoModuleVersionWriter struct{}

// CanHandle returns true for Go modules.
func (w *GoModuleVersionWriter) CanHandle(pkgType monorepo.PackageType) bool {
	return pkgType == monorepo.PackageTypeGoModule
}

// ReadVersion reads the version from Go version file.
func (w *GoModuleVersionWriter) ReadVersion(ctx context.Context, pkgPath string) (string, error) {
	// Look for version.go or internal/version/version.go
	paths := []string{
		filepath.Join(pkgPath, "version.go"),
		filepath.Join(pkgPath, "internal", "version", "version.go"),
		filepath.Join(pkgPath, "pkg", "version", "version.go"),
	}

	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			re := regexp.MustCompile(`(?m)^\s*(?:const\s+)?(?:Version|version)\s*(?:string\s*)?=\s*"([^"]+)"`)
			if matches := re.FindSubmatch(data); len(matches) >= 2 {
				return string(matches[1]), nil
			}
		}
	}

	return "", fmt.Errorf("version not found in Go module files")
}

// WriteVersion updates the version in Go version file.
func (w *GoModuleVersionWriter) WriteVersion(ctx context.Context, pkgPath, ver string) error {
	// Look for existing version file
	paths := []string{
		filepath.Join(pkgPath, "version.go"),
		filepath.Join(pkgPath, "internal", "version", "version.go"),
		filepath.Join(pkgPath, "pkg", "version", "version.go"),
	}

	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			re := regexp.MustCompile(`(?m)^(\s*(?:const\s+)?(?:Version|version)\s*(?:string\s*)?=\s*)"[^"]+"`)
			if re.Match(data) {
				newData := re.ReplaceAll(data, []byte(fmt.Sprintf(`${1}"%s"`, ver)))
				if err := os.WriteFile(p, newData, 0644); err != nil {
					return fmt.Errorf("writing %s: %w", p, err)
				}
				return nil
			}
		}
	}

	return fmt.Errorf("no Go version file found to update")
}

// Files returns the files that will be modified.
func (w *GoModuleVersionWriter) Files(pkgPath string) []string {
	paths := []string{
		filepath.Join(pkgPath, "version.go"),
		filepath.Join(pkgPath, "internal", "version", "version.go"),
		filepath.Join(pkgPath, "pkg", "version", "version.go"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return []string{p}
		}
	}
	return nil
}

// -------------------------------------------------------------------
// Maven Version Writer (pom.xml)
// -------------------------------------------------------------------

// MavenVersionWriter handles versioning for Maven packages.
type MavenVersionWriter struct{}

// CanHandle returns true for Maven packages.
func (w *MavenVersionWriter) CanHandle(pkgType monorepo.PackageType) bool {
	return pkgType == monorepo.PackageTypeMaven
}

// ReadVersion reads the version from pom.xml.
func (w *MavenVersionWriter) ReadVersion(ctx context.Context, pkgPath string) (string, error) {
	pomPath := filepath.Join(pkgPath, "pom.xml")
	data, err := os.ReadFile(pomPath)
	if err != nil {
		return "", fmt.Errorf("reading pom.xml: %w", err)
	}

	// Extract version from project element (not parent or dependency)
	re := regexp.MustCompile(`(?s)<project[^>]*>.*?<version>([^<]+)</version>`)
	if matches := re.FindSubmatch(data); len(matches) >= 2 {
		return string(matches[1]), nil
	}

	return "", fmt.Errorf("version not found in pom.xml")
}

// WriteVersion updates the version in pom.xml.
func (w *MavenVersionWriter) WriteVersion(ctx context.Context, pkgPath, ver string) error {
	pomPath := filepath.Join(pkgPath, "pom.xml")
	data, err := os.ReadFile(pomPath)
	if err != nil {
		return fmt.Errorf("reading pom.xml: %w", err)
	}

	// Replace version in project element
	// This is simplified - a proper implementation would use XML parsing
	re := regexp.MustCompile(`(?s)(<project[^>]*>.*?<version>)[^<]+(</version>)`)
	newData := re.ReplaceAll(data, []byte(fmt.Sprintf("${1}%s${2}", ver)))

	if err := os.WriteFile(pomPath, newData, 0644); err != nil {
		return fmt.Errorf("writing pom.xml: %w", err)
	}

	return nil
}

// Files returns the files that will be modified.
func (w *MavenVersionWriter) Files(pkgPath string) []string {
	return []string{filepath.Join(pkgPath, "pom.xml")}
}

// -------------------------------------------------------------------
// Gradle Version Writer (build.gradle, build.gradle.kts)
// -------------------------------------------------------------------

// GradleVersionWriter handles versioning for Gradle packages.
type GradleVersionWriter struct{}

// CanHandle returns true for Gradle packages.
func (w *GradleVersionWriter) CanHandle(pkgType monorepo.PackageType) bool {
	return pkgType == monorepo.PackageTypeGradle
}

// ReadVersion reads the version from build.gradle or build.gradle.kts.
func (w *GradleVersionWriter) ReadVersion(ctx context.Context, pkgPath string) (string, error) {
	// Try build.gradle.kts first (Kotlin DSL)
	ktsPath := filepath.Join(pkgPath, "build.gradle.kts")
	if data, err := os.ReadFile(ktsPath); err == nil {
		re := regexp.MustCompile(`(?m)^\s*version\s*=\s*"([^"]+)"`)
		if matches := re.FindSubmatch(data); len(matches) >= 2 {
			return string(matches[1]), nil
		}
	}

	// Try build.gradle (Groovy)
	groovyPath := filepath.Join(pkgPath, "build.gradle")
	if data, err := os.ReadFile(groovyPath); err == nil {
		re := regexp.MustCompile(`(?m)^\s*version\s*[=:]\s*['"]?([^'"]+)['"]?`)
		if matches := re.FindSubmatch(data); len(matches) >= 2 {
			return strings.TrimSpace(string(matches[1])), nil
		}
	}

	return "", fmt.Errorf("version not found in Gradle files")
}

// WriteVersion updates the version in Gradle build files.
func (w *GradleVersionWriter) WriteVersion(ctx context.Context, pkgPath, ver string) error {
	// Try build.gradle.kts first
	ktsPath := filepath.Join(pkgPath, "build.gradle.kts")
	if data, err := os.ReadFile(ktsPath); err == nil {
		re := regexp.MustCompile(`(?m)^(\s*version\s*=\s*)"[^"]+"`)
		if re.Match(data) {
			newData := re.ReplaceAll(data, []byte(fmt.Sprintf(`${1}"%s"`, ver)))
			if err := os.WriteFile(ktsPath, newData, 0644); err != nil {
				return fmt.Errorf("writing build.gradle.kts: %w", err)
			}
			return nil
		}
	}

	// Try build.gradle
	groovyPath := filepath.Join(pkgPath, "build.gradle")
	if data, err := os.ReadFile(groovyPath); err == nil {
		re := regexp.MustCompile(`(?m)^(\s*version\s*[=:]\s*)['"]?[^'"]+['"]?`)
		if re.Match(data) {
			newData := re.ReplaceAll(data, []byte(fmt.Sprintf(`${1}'%s'`, ver)))
			if err := os.WriteFile(groovyPath, newData, 0644); err != nil {
				return fmt.Errorf("writing build.gradle: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("no Gradle build file found to update")
}

// Files returns the files that will be modified.
func (w *GradleVersionWriter) Files(pkgPath string) []string {
	ktsPath := filepath.Join(pkgPath, "build.gradle.kts")
	if _, err := os.Stat(ktsPath); err == nil {
		return []string{ktsPath}
	}
	groovyPath := filepath.Join(pkgPath, "build.gradle")
	if _, err := os.Stat(groovyPath); err == nil {
		return []string{groovyPath}
	}
	return nil
}

// -------------------------------------------------------------------
// Composer Version Writer (composer.json)
// -------------------------------------------------------------------

// ComposerVersionWriter handles versioning for PHP/Composer packages.
type ComposerVersionWriter struct{}

// CanHandle returns true for Composer packages.
func (w *ComposerVersionWriter) CanHandle(pkgType monorepo.PackageType) bool {
	return pkgType == monorepo.PackageTypeComposer
}

// ReadVersion reads the version from composer.json.
func (w *ComposerVersionWriter) ReadVersion(ctx context.Context, pkgPath string) (string, error) {
	composerPath := filepath.Join(pkgPath, "composer.json")
	data, err := os.ReadFile(composerPath)
	if err != nil {
		return "", fmt.Errorf("reading composer.json: %w", err)
	}

	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", fmt.Errorf("parsing composer.json: %w", err)
	}

	return pkg.Version, nil
}

// WriteVersion updates the version in composer.json.
func (w *ComposerVersionWriter) WriteVersion(ctx context.Context, pkgPath, ver string) error {
	composerPath := filepath.Join(pkgPath, "composer.json")
	data, err := os.ReadFile(composerPath)
	if err != nil {
		return fmt.Errorf("reading composer.json: %w", err)
	}

	var pkg map[string]interface{}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return fmt.Errorf("parsing composer.json: %w", err)
	}

	pkg["version"] = ver

	output, err := json.MarshalIndent(pkg, "", "    ")
	if err != nil {
		return fmt.Errorf("marshaling composer.json: %w", err)
	}

	output = append(output, '\n')

	if err := os.WriteFile(composerPath, output, 0644); err != nil {
		return fmt.Errorf("writing composer.json: %w", err)
	}

	return nil
}

// Files returns the files that will be modified.
func (w *ComposerVersionWriter) Files(pkgPath string) []string {
	return []string{filepath.Join(pkgPath, "composer.json")}
}

// -------------------------------------------------------------------
// Gem Version Writer (*.gemspec, lib/*/version.rb)
// -------------------------------------------------------------------

// GemVersionWriter handles versioning for Ruby gems.
type GemVersionWriter struct{}

// CanHandle returns true for Ruby gems.
func (w *GemVersionWriter) CanHandle(pkgType monorepo.PackageType) bool {
	return pkgType == monorepo.PackageTypeGem
}

// ReadVersion reads the version from gemspec or version.rb.
func (w *GemVersionWriter) ReadVersion(ctx context.Context, pkgPath string) (string, error) {
	// Try version.rb first (preferred location)
	versionFiles, _ := filepath.Glob(filepath.Join(pkgPath, "lib", "*", "version.rb"))
	for _, vf := range versionFiles {
		if data, err := os.ReadFile(vf); err == nil {
			re := regexp.MustCompile(`VERSION\s*=\s*["']([^"']+)["']`)
			if matches := re.FindSubmatch(data); len(matches) >= 2 {
				return string(matches[1]), nil
			}
		}
	}

	// Try gemspec
	gemspecs, _ := filepath.Glob(filepath.Join(pkgPath, "*.gemspec"))
	for _, gs := range gemspecs {
		if data, err := os.ReadFile(gs); err == nil {
			re := regexp.MustCompile(`\.version\s*=\s*["']([^"']+)["']`)
			if matches := re.FindSubmatch(data); len(matches) >= 2 {
				return string(matches[1]), nil
			}
		}
	}

	return "", fmt.Errorf("version not found in gem files")
}

// WriteVersion updates the version in gem files.
func (w *GemVersionWriter) WriteVersion(ctx context.Context, pkgPath, ver string) error {
	var wrote bool

	// Update version.rb if exists
	versionFiles, _ := filepath.Glob(filepath.Join(pkgPath, "lib", "*", "version.rb"))
	for _, vf := range versionFiles {
		if data, err := os.ReadFile(vf); err == nil {
			re := regexp.MustCompile(`(VERSION\s*=\s*)["'][^"']+["']`)
			if re.Match(data) {
				newData := re.ReplaceAll(data, []byte(fmt.Sprintf(`${1}"%s"`, ver)))
				if err := os.WriteFile(vf, newData, 0644); err != nil {
					return fmt.Errorf("writing %s: %w", vf, err)
				}
				wrote = true
			}
		}
	}

	if !wrote {
		return fmt.Errorf("no gem version file found to update")
	}

	return nil
}

// Files returns the files that will be modified.
func (w *GemVersionWriter) Files(pkgPath string) []string {
	versionFiles, _ := filepath.Glob(filepath.Join(pkgPath, "lib", "*", "version.rb"))
	return versionFiles
}

// -------------------------------------------------------------------
// NuGet Version Writer (*.csproj)
// -------------------------------------------------------------------

// NuGetVersionWriter handles versioning for .NET/NuGet packages.
type NuGetVersionWriter struct{}

// CanHandle returns true for NuGet packages.
func (w *NuGetVersionWriter) CanHandle(pkgType monorepo.PackageType) bool {
	return pkgType == monorepo.PackageTypeNuGet
}

// ReadVersion reads the version from .csproj file.
func (w *NuGetVersionWriter) ReadVersion(ctx context.Context, pkgPath string) (string, error) {
	csproj, _ := filepath.Glob(filepath.Join(pkgPath, "*.csproj"))
	if len(csproj) == 0 {
		return "", fmt.Errorf("no .csproj file found")
	}

	data, err := os.ReadFile(csproj[0])
	if err != nil {
		return "", fmt.Errorf("reading .csproj: %w", err)
	}

	re := regexp.MustCompile(`<Version>([^<]+)</Version>`)
	if matches := re.FindSubmatch(data); len(matches) >= 2 {
		return string(matches[1]), nil
	}

	return "", fmt.Errorf("version not found in .csproj")
}

// WriteVersion updates the version in .csproj file.
func (w *NuGetVersionWriter) WriteVersion(ctx context.Context, pkgPath, ver string) error {
	csproj, _ := filepath.Glob(filepath.Join(pkgPath, "*.csproj"))
	if len(csproj) == 0 {
		return fmt.Errorf("no .csproj file found")
	}

	data, err := os.ReadFile(csproj[0])
	if err != nil {
		return fmt.Errorf("reading .csproj: %w", err)
	}

	re := regexp.MustCompile(`(<Version>)[^<]+(</Version>)`)
	newData := re.ReplaceAll(data, []byte(fmt.Sprintf("${1}%s${2}", ver)))

	if err := os.WriteFile(csproj[0], newData, 0644); err != nil {
		return fmt.Errorf("writing .csproj: %w", err)
	}

	return nil
}

// Files returns the files that will be modified.
func (w *NuGetVersionWriter) Files(pkgPath string) []string {
	csproj, _ := filepath.Glob(filepath.Join(pkgPath, "*.csproj"))
	return csproj
}

// -------------------------------------------------------------------
// Directory Version Writer (VERSION file)
// -------------------------------------------------------------------

// DirectoryVersionWriter handles versioning for plain directories using a VERSION file.
type DirectoryVersionWriter struct{}

// CanHandle returns true for directory type packages.
func (w *DirectoryVersionWriter) CanHandle(pkgType monorepo.PackageType) bool {
	return pkgType == monorepo.PackageTypeDirectory
}

// ReadVersion reads the version from VERSION file.
func (w *DirectoryVersionWriter) ReadVersion(ctx context.Context, pkgPath string) (string, error) {
	versionPath := filepath.Join(pkgPath, "VERSION")
	data, err := os.ReadFile(versionPath)
	if err != nil {
		return "", fmt.Errorf("reading VERSION file: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}

// WriteVersion updates the version in VERSION file.
func (w *DirectoryVersionWriter) WriteVersion(ctx context.Context, pkgPath, ver string) error {
	versionPath := filepath.Join(pkgPath, "VERSION")
	if err := os.WriteFile(versionPath, []byte(ver+"\n"), 0644); err != nil {
		return fmt.Errorf("writing VERSION file: %w", err)
	}
	return nil
}

// Files returns the files that will be modified.
func (w *DirectoryVersionWriter) Files(pkgPath string) []string {
	return []string{filepath.Join(pkgPath, "VERSION")}
}

// -------------------------------------------------------------------
// Composite Version Writer (combines multiple writers)
// -------------------------------------------------------------------

// CompositeVersionWriter applies multiple writers based on package type.
type CompositeVersionWriter struct {
	registry *VersionWriterRegistry
}

// NewCompositeVersionWriter creates a composite writer with all built-in writers.
func NewCompositeVersionWriter() *CompositeVersionWriter {
	return &CompositeVersionWriter{
		registry: NewVersionWriterRegistry(),
	}
}

// CanHandle returns true if any registered writer can handle the type.
func (w *CompositeVersionWriter) CanHandle(pkgType monorepo.PackageType) bool {
	_, ok := w.registry.GetWriter(pkgType)
	return ok
}

// ReadVersion reads the version using the appropriate writer.
func (w *CompositeVersionWriter) ReadVersion(ctx context.Context, pkgPath string, pkgType monorepo.PackageType) (version.SemanticVersion, error) {
	writer, ok := w.registry.GetWriter(pkgType)
	if !ok {
		return version.Zero, fmt.Errorf("no version writer for package type: %s", pkgType)
	}

	verStr, err := writer.ReadVersion(ctx, pkgPath)
	if err != nil {
		return version.Zero, err
	}

	return version.Parse(verStr)
}

// WriteVersion updates the version using the appropriate writer.
func (w *CompositeVersionWriter) WriteVersion(ctx context.Context, pkgPath string, pkgType monorepo.PackageType, ver version.SemanticVersion) error {
	writer, ok := w.registry.GetWriter(pkgType)
	if !ok {
		return fmt.Errorf("no version writer for package type: %s", pkgType)
	}

	return writer.WriteVersion(ctx, pkgPath, ver.String())
}

// Files returns the files that will be modified.
func (w *CompositeVersionWriter) Files(pkgPath string, pkgType monorepo.PackageType) []string {
	writer, ok := w.registry.GetWriter(pkgType)
	if !ok {
		return nil
	}
	return writer.Files(pkgPath)
}
