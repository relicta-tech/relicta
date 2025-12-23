package manager

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
		Name:       "github",
		Version:    "v1.0.0",
		Repository: "relicta-tech/plugin-github",
	}

	archiveName := installer.getArchiveName(info)

	// Should end with .tar.gz or .zip
	if !strings.HasSuffix(archiveName, ".tar.gz") && !strings.HasSuffix(archiveName, ".zip") {
		t.Errorf("getArchiveName() = %q, should end with .tar.gz or .zip", archiveName)
	}

	// Should start with repo name (extracted from Repository field)
	if !strings.HasPrefix(archiveName, "plugin-github_") {
		t.Errorf("getArchiveName() = %q, should start with %q", archiveName, "plugin-github_")
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

func TestInstaller_GetDownloadURL(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	info := PluginInfo{
		Name:       "alpha",
		Version:    "v1.0.0",
		Repository: "relicta-tech/relicta",
	}

	url := installer.getDownloadURL(info)
	if !strings.Contains(url, "github.com/relicta-tech/relicta/releases/download/v1.0.0/") {
		t.Fatalf("getDownloadURL() = %q, want github release URL", url)
	}
}

func TestInstaller_DownloadFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("payload"))
	}))
	t.Cleanup(server.Close)

	installer := NewInstaller(t.TempDir())
	var dest bytes.Buffer

	if err := installer.downloadFile(context.Background(), server.URL, &dest); err != nil {
		t.Fatalf("downloadFile error: %v", err)
	}
	if got := dest.String(); got != "payload" {
		t.Fatalf("downloadFile content = %q, want %q", got, "payload")
	}
}

func TestInstaller_DownloadFile_NonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(server.Close)

	installer := NewInstaller(t.TempDir())
	var dest bytes.Buffer

	if err := installer.downloadFile(context.Background(), server.URL, &dest); err == nil {
		t.Fatal("expected downloadFile to fail for non-200 response")
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

func TestInstaller_InstallBinary(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "bin")
	destPath := filepath.Join(installer.pluginDir, "bin")

	if err := os.WriteFile(srcPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	if err := installer.installBinary(srcPath, destPath); err != nil {
		t.Fatalf("installBinary error: %v", err)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(data) != "binary" {
		t.Fatalf("installBinary content = %q, want %q", string(data), "binary")
	}
}

func TestInstaller_InstallBinary_ReadError(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	if err := installer.installBinary("does-not-exist", filepath.Join(t.TempDir(), "dest")); err == nil {
		t.Fatal("expected installBinary to fail for missing source file")
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

	found := installer.findBinary(extractDir, "github", "relicta-tech/plugin-github")

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

	found := installer.findBinary(extractDir, "github", "relicta-tech/plugin-github")

	if found == "" {
		t.Errorf("findBinary() returned empty string, expected to find binary at %q", binaryPath)
	}
	if found != binaryPath {
		t.Errorf("findBinary() = %q, want %q", found, binaryPath)
	}
}

func TestInstaller_FindBinary_RepoBasedName(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	extractDir := t.TempDir()

	// Create a test binary with repo-based name (e.g., plugin-github_darwin_aarch64)
	// This matches how our release workflows build binaries
	platform := GetCurrentPlatform()
	binaryName := "plugin-github_" + platform
	binaryPath := filepath.Join(extractDir, binaryName)
	if err := os.WriteFile(binaryPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Search for "github" with repository "relicta-tech/plugin-github"
	// It should find "plugin-github_*" by extracting repo name from the registry
	found := installer.findBinary(extractDir, "github", "relicta-tech/plugin-github")

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

func TestInstaller_Install_Success(t *testing.T) {
	installer := NewInstaller(t.TempDir())

	archiveData := createTarGzBytes(t, installer.getBinaryName("alpha"), []byte("binary content"))
	installer.httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(archiveData)),
				Header:     http.Header{"Content-Type": []string{"application/octet-stream"}},
			}, nil
		}),
	}

	info := PluginInfo{
		Name:       "alpha",
		Version:    "v1.0.0",
		Repository: "relicta-tech/relicta",
	}

	installed, err := installer.Install(context.Background(), info)
	if err != nil {
		t.Fatalf("Install error: %v", err)
	}
	if installed == nil || installed.BinaryPath == "" {
		t.Fatalf("Install returned invalid plugin: %+v", installed)
	}
	if _, err := os.Stat(installed.BinaryPath); err != nil {
		t.Fatalf("installed binary missing: %v", err)
	}
}

func TestInstaller_Install_ChecksumMismatch(t *testing.T) {
	installer := NewInstaller(t.TempDir())

	archiveData := createTarGzBytes(t, installer.getBinaryName("alpha"), []byte("binary content"))
	installer.httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(archiveData)),
				Header:     http.Header{"Content-Type": []string{"application/octet-stream"}},
			}, nil
		}),
	}

	info := PluginInfo{
		Name:       "alpha",
		Version:    "v1.0.0",
		Repository: "relicta-tech/relicta",
		Checksums: map[string]string{
			GetCurrentPlatform(): "deadbeef",
		},
	}

	if _, err := installer.Install(context.Background(), info); err == nil {
		t.Fatal("expected Install to fail for checksum mismatch")
	}
}

func TestInstaller_Install_BinaryNotFound(t *testing.T) {
	installer := NewInstaller(t.TempDir())

	archiveData := createTarGzBytes(t, "not-alpha", []byte("binary content"))
	installer.httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(archiveData)),
				Header:     http.Header{"Content-Type": []string{"application/octet-stream"}},
			}, nil
		}),
	}

	info := PluginInfo{
		Name:       "alpha",
		Version:    "v1.0.0",
		Repository: "relicta-tech/relicta",
	}

	if _, err := installer.Install(context.Background(), info); err == nil {
		t.Fatal("expected Install to fail when binary not found")
	}
}

func TestInstaller_ExtractZip_InvalidArchive(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	destDir := t.TempDir()

	archivePath := filepath.Join(t.TempDir(), "invalid.zip")
	if err := os.WriteFile(archivePath, []byte("not-a-zip"), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	if err := installer.extractZip(archivePath, destDir); err == nil {
		t.Fatal("expected extractZip to fail for invalid archive")
	}
}

func TestInstaller_ExtractTarGz_InvalidArchive(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	destDir := t.TempDir()

	archivePath := filepath.Join(t.TempDir(), "invalid.tar.gz")
	if err := os.WriteFile(archivePath, []byte("not-a-gzip"), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	if err := installer.extractTarGz(archivePath, destDir); err == nil {
		t.Fatal("expected extractTarGz to fail for invalid archive")
	}
}

func TestInstaller_Uninstall_MissingBinary(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	err := installer.Uninstall(InstalledPlugin{
		Name:       "missing",
		BinaryPath: filepath.Join(t.TempDir(), "missing"),
	})
	if err != nil {
		t.Fatalf("expected Uninstall to ignore missing file, got %v", err)
	}
}

func TestInstaller_Install_DownloadError(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	installer.httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("network down")
		}),
	}

	info := PluginInfo{
		Name:       "alpha",
		Version:    "v1.0.0",
		Repository: "relicta-tech/relicta",
	}

	if _, err := installer.Install(context.Background(), info); err == nil {
		t.Fatal("expected Install to fail when download fails")
	}
}

func TestInstaller_ExtractZipFile_OpenError(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	archivePath := filepath.Join(t.TempDir(), "test.zip")
	createTestZip(t, archivePath, "test.txt", []byte("content"))

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("OpenReader error: %v", err)
	}
	f := reader.File[0]
	if err := reader.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	if err := installer.extractZipFile(f, filepath.Join(t.TempDir(), "out.txt")); err == nil {
		t.Fatal("expected extractZipFile to fail when archive is closed")
	}
}

func TestInstaller_ExtractZipFile_CreateError(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	archivePath := filepath.Join(t.TempDir(), "test.zip")
	createTestZip(t, archivePath, "test.txt", []byte("content"))

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("OpenReader error: %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })

	targetDir := filepath.Join(t.TempDir(), "readonly")
	if err := os.MkdirAll(targetDir, 0o555); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	if err := installer.extractZipFile(reader.File[0], filepath.Join(targetDir, "out.txt")); err == nil {
		t.Fatal("expected extractZipFile to fail when target is not writable")
	}
}

func TestInstaller_ExtractTarGz_DestDirFile(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	destDir := filepath.Join(t.TempDir(), "dest-file")
	if err := os.WriteFile(destDir, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	archivePath := filepath.Join(t.TempDir(), "test.tar.gz")
	createTestTarGz(t, archivePath, "test-binary", []byte("binary content"))

	if err := installer.extractTarGz(archivePath, destDir); err == nil {
		t.Fatal("expected extractTarGz to fail when dest is a file")
	}
}

func TestInstaller_ExtractZip_DestDirFile(t *testing.T) {
	installer := NewInstaller(t.TempDir())
	destDir := filepath.Join(t.TempDir(), "dest-file")
	if err := os.WriteFile(destDir, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	archivePath := filepath.Join(t.TempDir(), "test.zip")
	createTestZip(t, archivePath, "test-binary.exe", []byte("binary content"))

	if err := installer.extractZip(archivePath, destDir); err == nil {
		t.Fatal("expected extractZip to fail when dest is a file")
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

func createTarGzBytes(t *testing.T, filename string, content []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	header := &tar.Header{
		Name: filename,
		Mode: 0o755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("Failed to write tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("Failed to write tar content: %v", err)
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("Failed to close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("Failed to close gzip writer: %v", err)
	}

	return buf.Bytes()
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
