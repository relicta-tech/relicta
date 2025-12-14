package manager

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Installer handles plugin installation operations.
type Installer struct {
	httpClient *http.Client
	pluginDir  string
}

// NewInstaller creates a new plugin installer.
func NewInstaller(pluginDir string) *Installer {
	return &Installer{
		httpClient: &http.Client{
			Timeout: 5 * time.Minute, // Larger timeout for downloads
		},
		pluginDir: pluginDir,
	}
}

// Install downloads and installs a plugin binary.
func (i *Installer) Install(ctx context.Context, pluginInfo PluginInfo) (*InstalledPlugin, error) {
	// Determine platform-specific binary name
	binaryName := i.getBinaryName(pluginInfo.Name)
	downloadURL := i.getDownloadURL(pluginInfo)

	// Create temporary download location
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("relicta-plugin-%s-*", pluginInfo.Name))
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Download the plugin binary
	if err := i.downloadFile(ctx, downloadURL, tmpFile); err != nil {
		return nil, fmt.Errorf("failed to download plugin: %w", err)
	}

	// Reset file pointer to beginning for checksum calculation
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek to beginning of file: %w", err)
	}

	// Calculate checksum
	checksum, err := i.calculateChecksum(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// Close temp file before moving
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	// Install the binary to the plugin directory
	destPath := filepath.Join(i.pluginDir, binaryName)
	if err := i.installBinary(tmpFile.Name(), destPath); err != nil {
		return nil, fmt.Errorf("failed to install binary: %w", err)
	}

	// Create installed plugin entry
	installed := &InstalledPlugin{
		Name:        pluginInfo.Name,
		Version:     pluginInfo.Version,
		InstalledAt: time.Now(),
		BinaryPath:  destPath,
		Checksum:    checksum,
		Enabled:     false, // Installed but not enabled by default
	}

	return installed, nil
}

// Uninstall removes a plugin binary and its manifest entry.
func (i *Installer) Uninstall(plugin InstalledPlugin) error {
	// Remove binary file
	if err := os.Remove(plugin.BinaryPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plugin binary: %w", err)
	}

	return nil
}

// getBinaryName returns the platform-specific binary name.
func (i *Installer) getBinaryName(pluginName string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("%s.exe", pluginName)
	}
	return pluginName
}

// getDownloadURL constructs the GitHub release download URL for the plugin.
func (i *Installer) getDownloadURL(pluginInfo PluginInfo) string {
	// Format: https://github.com/{owner}/{repo}/releases/download/{version}/{plugin}_{os}_{arch}
	// Example: https://github.com/relicta-tech/relicta/releases/download/v1.1.0/github_darwin_arm64

	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Normalize architecture names to match GoReleaser output
	switch goarch {
	case "amd64":
		goarch = "x86_64"
	case "arm64":
		goarch = "aarch64"
	}

	binary := fmt.Sprintf("%s_%s_%s", pluginInfo.Name, goos, goarch)
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	return fmt.Sprintf(
		"https://github.com/%s/releases/download/%s/%s",
		pluginInfo.Repository,
		pluginInfo.Version,
		binary,
	)
}

// downloadFile downloads a file from URL to the destination writer.
func (i *Installer) downloadFile(ctx context.Context, url string, dest io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	_, err = io.Copy(dest, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// calculateChecksum computes SHA256 checksum of the file.
func (i *Installer) calculateChecksum(file io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to compute checksum: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// installBinary moves the downloaded binary to the plugin directory and sets permissions.
func (i *Installer) installBinary(srcPath, destPath string) error {
	// Read the source file
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Write to destination with executable permissions
	if err := os.WriteFile(destPath, data, 0o755); err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	return nil
}

// VerifyChecksum verifies the checksum of an installed plugin.
func (i *Installer) VerifyChecksum(plugin InstalledPlugin) error {
	file, err := os.Open(plugin.BinaryPath)
	if err != nil {
		return fmt.Errorf("failed to open plugin binary: %w", err)
	}
	defer file.Close()

	checksum, err := i.calculateChecksum(file)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	if !strings.EqualFold(checksum, plugin.Checksum) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", plugin.Checksum, checksum)
	}

	return nil
}
