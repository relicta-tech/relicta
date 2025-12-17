package manager

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstaller_CalculateChecksum(t *testing.T) {
	installer := NewInstaller(t.TempDir())

	// Create a test file with known content
	content := []byte("test content for checksum")
	tmpFile, err := os.CreateTemp("", "checksum-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(content); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tmpFile.Close()

	// Calculate checksum
	file, err := os.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	checksum, err := installer.calculateChecksum(file)
	if err != nil {
		t.Fatalf("calculateChecksum() error = %v", err)
	}

	// SHA256 checksum should be 64 hex characters
	if len(checksum) != 64 {
		t.Errorf("checksum length = %d, want 64", len(checksum))
	}

	// Verify checksum is consistent
	file2, err := os.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open file again: %v", err)
	}
	defer file2.Close()

	checksum2, err := installer.calculateChecksum(file2)
	if err != nil {
		t.Fatalf("calculateChecksum() second call error = %v", err)
	}

	if checksum != checksum2 {
		t.Errorf("Checksums don't match: %s != %s", checksum, checksum2)
	}
}

func TestInstaller_VerifyChecksum(t *testing.T) {
	pluginDir := t.TempDir()
	installer := NewInstaller(pluginDir)

	// Create a test binary
	binaryPath := filepath.Join(pluginDir, "test-plugin")
	content := []byte("fake binary content")
	if err := os.WriteFile(binaryPath, content, 0o755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Calculate expected checksum
	file, _ := os.Open(binaryPath)
	expectedChecksum, _ := installer.calculateChecksum(file)
	file.Close()

	tests := []struct {
		name     string
		plugin   InstalledPlugin
		wantErr  bool
		errMatch string
	}{
		{
			name: "valid checksum",
			plugin: InstalledPlugin{
				Name:       "test-plugin",
				BinaryPath: binaryPath,
				Checksum:   expectedChecksum,
			},
			wantErr: false,
		},
		{
			name: "invalid checksum",
			plugin: InstalledPlugin{
				Name:       "test-plugin",
				BinaryPath: binaryPath,
				Checksum:   "0000000000000000000000000000000000000000000000000000000000000000",
			},
			wantErr:  true,
			errMatch: "checksum mismatch",
		},
		{
			name: "non-existent binary",
			plugin: InstalledPlugin{
				Name:       "test-plugin",
				BinaryPath: "/nonexistent/path",
				Checksum:   expectedChecksum,
			},
			wantErr:  true,
			errMatch: "failed to open",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := installer.VerifyChecksum(tt.plugin)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyChecksum() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMatch != "" && !strings.Contains(err.Error(), tt.errMatch) {
				t.Errorf("VerifyChecksum() error = %v, should contain %q", err, tt.errMatch)
			}
		})
	}
}

func TestInstaller_GetArchiveName(t *testing.T) {
	installer := NewInstaller(t.TempDir())

	info := PluginInfo{
		Name:    "github",
		Version: "v1.0.0",
	}

	archiveName := installer.getArchiveName(info)

	// Should end with .tar.gz or .zip
	if !strings.HasSuffix(archiveName, ".tar.gz") && !strings.HasSuffix(archiveName, ".zip") {
		t.Errorf("getArchiveName() = %q, should end with .tar.gz or .zip", archiveName)
	}

	// Should start with plugin name
	if !strings.HasPrefix(archiveName, "github_") {
		t.Errorf("getArchiveName() = %q, should start with %q", archiveName, "github_")
	}
}

func TestInstaller_GetBinaryName(t *testing.T) {
	installer := NewInstaller(t.TempDir())

	binaryName := installer.getBinaryName("test-plugin")

	// Should not be empty
	if binaryName == "" {
		t.Error("getBinaryName() returned empty string")
	}

	// On Windows, should end with .exe
	// On other platforms, should just be the plugin name
	if strings.Contains(GetCurrentPlatform(), "windows") {
		if !strings.HasSuffix(binaryName, ".exe") {
			t.Errorf("getBinaryName() = %q, want .exe suffix on Windows", binaryName)
		}
	} else {
		if binaryName != "test-plugin" {
			t.Errorf("getBinaryName() = %q, want %q", binaryName, "test-plugin")
		}
	}
}

func TestInstaller_ExtractTarGz(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	destDir := t.TempDir()

	// Create a test tar.gz archive
	archivePath := filepath.Join(t.TempDir(), "test.tar.gz")
	createTestTarGz(t, archivePath, "test-binary", []byte("binary content"))

	// Extract
	if err := installer.extractTarGz(archivePath, destDir); err != nil {
		t.Fatalf("extractTarGz() error = %v", err)
	}

	// Verify file was extracted
	extractedPath := filepath.Join(destDir, "test-binary")
	if _, err := os.Stat(extractedPath); os.IsNotExist(err) {
		t.Error("extractTarGz() did not extract the file")
	}

	// Verify content
	content, err := os.ReadFile(extractedPath)
	if err != nil {
		t.Fatalf("Failed to read extracted file: %v", err)
	}
	if string(content) != "binary content" {
		t.Errorf("Extracted content = %q, want %q", string(content), "binary content")
	}
}

func TestInstaller_ExtractZip(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	destDir := t.TempDir()

	// Create a test zip archive
	archivePath := filepath.Join(t.TempDir(), "test.zip")
	createTestZip(t, archivePath, "test-binary.exe", []byte("binary content"))

	// Extract
	if err := installer.extractZip(archivePath, destDir); err != nil {
		t.Fatalf("extractZip() error = %v", err)
	}

	// Verify file was extracted
	extractedPath := filepath.Join(destDir, "test-binary.exe")
	if _, err := os.Stat(extractedPath); os.IsNotExist(err) {
		t.Error("extractZip() did not extract the file")
	}
}

func TestInstaller_ExtractTarGz_PathTraversal(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	destDir := t.TempDir()

	// Create a malicious tar.gz with path traversal
	archivePath := filepath.Join(t.TempDir(), "malicious.tar.gz")
	createTestTarGz(t, archivePath, "../../../evil", []byte("malicious"))

	// Extraction should fail
	err := installer.extractTarGz(archivePath, destDir)
	if err == nil {
		t.Error("extractTarGz() should fail for path traversal")
	}
	if !strings.Contains(err.Error(), "invalid file path") {
		t.Errorf("extractTarGz() error = %v, should mention invalid file path", err)
	}
}

func TestInstaller_ExtractZip_PathTraversal(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	destDir := t.TempDir()

	// Create a malicious zip with path traversal
	archivePath := filepath.Join(t.TempDir(), "malicious.zip")
	createTestZip(t, archivePath, "../../../evil.exe", []byte("malicious"))

	// Extraction should fail
	err := installer.extractZip(archivePath, destDir)
	if err == nil {
		t.Error("extractZip() should fail for path traversal")
	}
	if !strings.Contains(err.Error(), "invalid file path") {
		t.Errorf("extractZip() error = %v, should mention invalid file path", err)
	}
}

func TestInstaller_FindBinary(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	extractDir := t.TempDir()

	// Create a test binary with platform-specific name
	platform := GetCurrentPlatform()
	binaryName := "github_" + platform
	binaryPath := filepath.Join(extractDir, binaryName)
	if err := os.WriteFile(binaryPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	found := installer.findBinary(extractDir, "github")

	if found == "" {
		t.Errorf("findBinary() returned empty string, expected to find binary at %q", binaryPath)
	}
	if found != binaryPath {
		t.Errorf("findBinary() = %q, want %q", found, binaryPath)
	}
}

func TestInstaller_FindBinary_SimpleName(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	extractDir := t.TempDir()

	// Create a test binary with simple name (no platform suffix)
	binaryPath := filepath.Join(extractDir, "github")
	if err := os.WriteFile(binaryPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	found := installer.findBinary(extractDir, "github")

	if found == "" {
		t.Errorf("findBinary() returned empty string, expected to find binary at %q", binaryPath)
	}
	if found != binaryPath {
		t.Errorf("findBinary() = %q, want %q", found, binaryPath)
	}
}

func TestInstaller_Install_SDKIncompatible(t *testing.T) {
	installer := NewInstaller(t.TempDir())

	pluginInfo := PluginInfo{
		Name:          "test-plugin",
		Version:       "v1.0.0",
		MinSDKVersion: CurrentSDKVersion + 100, // Future incompatible version
	}

	_, err := installer.Install(context.Background(), pluginInfo)
	if err == nil {
		t.Error("Install() should fail for incompatible SDK version")
	}
	if !strings.Contains(err.Error(), "requires SDK version") {
		t.Errorf("Install() error = %v, should mention SDK version", err)
	}
}

// Helper functions

func createTestTarGz(t *testing.T, path, filename string, content []byte) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create tar.gz: %v", err)
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	header := &tar.Header{
		Name: filename,
		Mode: 0o755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("Failed to write tar header: %v", err)
	}
	if _, err := io.Copy(tw, strings.NewReader(string(content))); err != nil {
		t.Fatalf("Failed to write tar content: %v", err)
	}
}

func createTestZip(t *testing.T, path, filename string, content []byte) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create zip: %v", err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer zw.Close()

	fw, err := zw.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create zip entry: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("Failed to write zip content: %v", err)
	}
}
