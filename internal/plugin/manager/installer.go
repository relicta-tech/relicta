package manager

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
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

// MaxPluginFileSize is the maximum allowed size for a single file in a plugin archive (100MB).
// This prevents decompression bomb attacks.
const MaxPluginFileSize = 100 * 1024 * 1024

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
	// Check SDK compatibility first
	if !pluginInfo.IsSDKCompatible() {
		return nil, fmt.Errorf("plugin %s requires SDK version %d, but host supports version %d",
			pluginInfo.Name, pluginInfo.MinSDKVersion, CurrentSDKVersion)
	}

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

	// Download the plugin archive
	if err := i.downloadFile(ctx, downloadURL, tmpFile); err != nil {
		return nil, fmt.Errorf("failed to download plugin from %s: %w", downloadURL, err)
	}

	// Close temp file before checksum verification and extraction
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	// Verify archive checksum BEFORE extraction (security: prevent installing tampered archives)
	expectedChecksum := pluginInfo.GetChecksum()
	if expectedChecksum != "" {
		archiveFile, err := os.Open(tmpFile.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to open archive for checksum verification: %w", err)
		}
		archiveChecksum, err := i.calculateChecksum(archiveFile)
		_ = archiveFile.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to calculate archive checksum: %w", err)
		}
		if !strings.EqualFold(archiveChecksum, expectedChecksum) {
			return nil, fmt.Errorf("checksum verification failed for %s: expected %s, got %s",
				pluginInfo.Name, expectedChecksum, archiveChecksum)
		}
	}

	// Create temp directory for extraction
	extractDir, err := os.MkdirTemp("", fmt.Sprintf("relicta-plugin-%s-extract-*", pluginInfo.Name))
	if err != nil {
		return nil, fmt.Errorf("failed to create extraction directory: %w", err)
	}
	defer os.RemoveAll(extractDir)

	// Extract the archive (already verified)
	archiveName := i.getArchiveName(pluginInfo)
	if strings.HasSuffix(archiveName, ".zip") {
		if err := i.extractZip(tmpFile.Name(), extractDir); err != nil {
			return nil, fmt.Errorf("failed to extract zip archive: %w", err)
		}
	} else {
		if err := i.extractTarGz(tmpFile.Name(), extractDir); err != nil {
			return nil, fmt.Errorf("failed to extract tar.gz archive: %w", err)
		}
	}

	// Find the binary in the extracted directory
	extractedBinary := i.findBinary(extractDir, pluginInfo.Name)
	if extractedBinary == "" {
		return nil, fmt.Errorf("binary not found in archive")
	}

	// Calculate checksum of the installed binary for manifest storage
	binaryFile, err := os.Open(extractedBinary)
	if err != nil {
		return nil, fmt.Errorf("failed to open extracted binary: %w", err)
	}
	binaryChecksum, err := i.calculateChecksum(binaryFile)
	_ = binaryFile.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate binary checksum: %w", err)
	}

	// Install the binary to the plugin directory
	destPath := filepath.Join(i.pluginDir, binaryName)
	if err := i.installBinary(extractedBinary, destPath); err != nil {
		return nil, fmt.Errorf("failed to install binary: %w", err)
	}

	// Create installed plugin entry
	// Store the binary checksum for later integrity verification of the installed binary
	installed := &InstalledPlugin{
		Name:        pluginInfo.Name,
		Version:     pluginInfo.Version,
		InstalledAt: time.Now(),
		BinaryPath:  destPath,
		Checksum:    binaryChecksum,
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

// getArchiveName returns the platform-specific archive name.
func (i *Installer) getArchiveName(pluginInfo PluginInfo) string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Normalize architecture names to match GoReleaser output
	switch goarch {
	case "amd64":
		goarch = "x86_64"
	case "arm64":
		goarch = "aarch64"
	}

	if goos == "windows" {
		return fmt.Sprintf("%s_%s_%s.zip", pluginInfo.Name, goos, goarch)
	}
	return fmt.Sprintf("%s_%s_%s.tar.gz", pluginInfo.Name, goos, goarch)
}

// getDownloadURL constructs the GitHub release download URL for the plugin.
func (i *Installer) getDownloadURL(pluginInfo PluginInfo) string {
	// Format: https://github.com/{owner}/{repo}/releases/download/{version}/{plugin}_{os}_{arch}.tar.gz
	// Example: https://github.com/relicta-tech/relicta/releases/download/v2.2.0/github_darwin_aarch64.tar.gz

	archiveName := i.getArchiveName(pluginInfo)

	return fmt.Sprintf(
		"https://github.com/%s/releases/download/%s/%s",
		pluginInfo.Repository,
		pluginInfo.Version,
		archiveName,
	)
}

// downloadFile downloads a file from URL to the destination writer.
func (i *Installer) downloadFile(ctx context.Context, url string, dest io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent to avoid GitHub blocking
	req.Header.Set("User-Agent", "relicta-plugin-installer")
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d for URL: %s", resp.StatusCode, url)
	}

	_, err = io.Copy(dest, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// extractTarGz extracts a .tar.gz archive to the destination directory.
func (i *Installer) extractTarGz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Sanitize the path to prevent path traversal
		target := filepath.Join(destDir, filepath.Clean(header.Name))
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}
			// Use LimitReader to prevent decompression bomb attacks (G110)
			limitedReader := io.LimitReader(tr, MaxPluginFileSize)
			written, copyErr := io.Copy(outFile, limitedReader)
			closeErr := outFile.Close()
			if copyErr != nil {
				return fmt.Errorf("failed to write file: %w", copyErr)
			}
			if closeErr != nil {
				return fmt.Errorf("failed to close file: %w", closeErr)
			}
			if written == MaxPluginFileSize {
				return fmt.Errorf("file %s exceeds maximum allowed size", header.Name)
			}
		}
	}

	return nil
}

// extractZip extracts a .zip archive to the destination directory.
func (i *Installer) extractZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open zip archive: %w", err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		// Sanitize the path to prevent path traversal
		target := filepath.Join(destDir, filepath.Clean(f.Name))
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path in archive: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		if err := i.extractZipFile(f, target); err != nil {
			return err
		}
	}

	return nil
}

// extractZipFile extracts a single file from a zip archive.
func (i *Installer) extractZipFile(f *zip.File, target string) error {
	outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	rc, err := f.Open()
	if err != nil {
		_ = outFile.Close()
		return fmt.Errorf("failed to open file in archive: %w", err)
	}

	// Use LimitReader to prevent decompression bomb attacks (G110)
	limitedReader := io.LimitReader(rc, MaxPluginFileSize)
	written, copyErr := io.Copy(outFile, limitedReader)

	// Close both handles, track errors
	rcCloseErr := rc.Close()
	outCloseErr := outFile.Close()

	if copyErr != nil {
		return fmt.Errorf("failed to write file: %w", copyErr)
	}
	if rcCloseErr != nil {
		return fmt.Errorf("failed to close archive file: %w", rcCloseErr)
	}
	if outCloseErr != nil {
		return fmt.Errorf("failed to close output file: %w", outCloseErr)
	}
	if written == MaxPluginFileSize {
		return fmt.Errorf("file %s exceeds maximum allowed size", f.Name)
	}

	return nil
}

// findBinary searches for the plugin binary in the extracted directory.
func (i *Installer) findBinary(extractDir, pluginName string) string {
	// Build list of possible binary names
	possibleNames := []string{pluginName}

	// Add platform-specific name (e.g., github_darwin_aarch64)
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	switch goarch {
	case "amd64":
		goarch = "x86_64"
	case "arm64":
		goarch = "aarch64"
	}
	platformName := fmt.Sprintf("%s_%s_%s", pluginName, goos, goarch)
	possibleNames = append(possibleNames, platformName)

	// Add .exe suffix for Windows
	if runtime.GOOS == "windows" {
		possibleNames = []string{
			pluginName + ".exe",
			platformName + ".exe",
		}
	}

	var foundPath string
	_ = filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		// Check if filename matches any possible binary name
		for _, name := range possibleNames {
			if info.Name() == name {
				foundPath = path
				return filepath.SkipAll
			}
		}
		return nil
	})

	return foundPath
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
