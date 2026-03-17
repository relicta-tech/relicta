package security

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		baseDir string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid simple path",
			path:    "config.yaml",
			baseDir: "",
			wantErr: false,
		},
		{
			name:    "valid nested path",
			path:    "dir/subdir/file.txt",
			baseDir: "",
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			baseDir: "",
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "path with null byte",
			path:    "config\x00.yaml",
			baseDir: "",
			wantErr: true,
			errMsg:  "null bytes",
		},
		{
			name:    "path traversal with ..",
			path:    "../etc/passwd",
			baseDir: "",
			wantErr: true,
			errMsg:  "directory traversal",
		},
		{
			name:    "path traversal in middle",
			path:    "foo/../../../etc/passwd",
			baseDir: "",
			wantErr: true,
			errMsg:  "directory traversal",
		},
		{
			name:    "double dot in filename is allowed",
			path:    "file..name.txt",
			baseDir: "",
			wantErr: false,
		},
		{
			name:    "path within base directory",
			path:    "subdir/file.txt",
			baseDir: "/home/user",
			wantErr: false,
		},
		{
			name:    "path escapes base directory via traversal",
			path:    "../other/file.txt",
			baseDir: "/home/user",
			wantErr: true,
			errMsg:  "directory traversal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidatePath(tt.path, tt.baseDir)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, result)
			}
		})
	}
}

func TestValidatePath_AbsPathWithBaseDir(t *testing.T) {
	// Absolute path within base dir
	result, err := ValidatePath("/home/user/subdir/file.txt", "/home/user")
	require.NoError(t, err)
	assert.Equal(t, "/home/user/subdir/file.txt", result)

	// Absolute path escaping base dir
	_, err = ValidatePath("/etc/passwd", "/home/user")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes base directory")

	// Path that equals base dir exactly
	result, err = ValidatePath("/home/user", "/home/user")
	require.NoError(t, err)
	assert.Equal(t, "/home/user", result)
}

func TestValidateConfigPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty path is valid",
			path:    "",
			wantErr: false,
		},
		{
			name:    "valid yaml extension",
			path:    "config.yaml",
			wantErr: false,
		},
		{
			name:    "valid yml extension",
			path:    ".relicta.yml",
			wantErr: false,
		},
		{
			name:    "valid json extension",
			path:    "config.json",
			wantErr: false,
		},
		{
			name:    "valid toml extension",
			path:    "config.toml",
			wantErr: false,
		},
		{
			name:    "no extension is valid",
			path:    "config",
			wantErr: false,
		},
		{
			name:    "invalid extension",
			path:    "config.exe",
			wantErr: true,
			errMsg:  "invalid config file extension",
		},
		{
			name:    "path traversal",
			path:    "../../../etc/passwd",
			wantErr: true,
			errMsg:  "directory traversal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateConfigPath(tt.path)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				if tt.path != "" {
					assert.Equal(t, filepath.Clean(tt.path), result)
				}
			}
		})
	}
}

func TestValidatePrerelease(t *testing.T) {
	tests := []struct {
		name       string
		prerelease string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "empty is valid",
			prerelease: "",
			wantErr:    false,
		},
		{
			name:       "alpha is valid",
			prerelease: "alpha",
			wantErr:    false,
		},
		{
			name:       "alpha.1 is valid",
			prerelease: "alpha.1",
			wantErr:    false,
		},
		{
			name:       "rc.1 is valid",
			prerelease: "rc.1",
			wantErr:    false,
		},
		{
			name:       "beta-test.2 is valid",
			prerelease: "beta-test.2",
			wantErr:    false,
		},
		{
			name:       "0.3.7 is valid",
			prerelease: "0.3.7",
			wantErr:    false,
		},
		{
			name:       "leading zero in numeric identifier",
			prerelease: "alpha.01",
			wantErr:    true,
			errMsg:     "leading zeros",
		},
		{
			name:       "invalid characters",
			prerelease: "alpha@1",
			wantErr:    true,
			errMsg:     "alphanumerics, hyphens, and dots",
		},
		{
			name:       "spaces not allowed",
			prerelease: "alpha 1",
			wantErr:    true,
			errMsg:     "alphanumerics, hyphens, and dots",
		},
		{
			name:       "too long",
			prerelease: string(make([]byte, 200)),
			wantErr:    true,
			errMsg:     "too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePrerelease(tt.prerelease)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateBuildMetadata(t *testing.T) {
	tests := []struct {
		name    string
		build   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty is valid",
			build:   "",
			wantErr: false,
		},
		{
			name:    "commit sha is valid",
			build:   "20130313144700",
			wantErr: false,
		},
		{
			name:    "build.123 is valid",
			build:   "build.123",
			wantErr: false,
		},
		{
			name:    "exp.sha.5114f85 is valid",
			build:   "exp.sha.5114f85",
			wantErr: false,
		},
		{
			name:    "leading zeros allowed in build",
			build:   "001",
			wantErr: false, // Build metadata allows leading zeros
		},
		{
			name:    "invalid characters",
			build:   "build@123",
			wantErr: true,
			errMsg:  "alphanumerics, hyphens, and dots",
		},
		{
			name:    "too long",
			build:   string(make([]byte, 200)),
			wantErr: true,
			errMsg:  "too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBuildMetadata(tt.build)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSanitizeLogMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special characters",
			input:    "normal message",
			expected: "normal message",
		},
		{
			name:     "newline injection",
			input:    "message\nINFO: fake log entry",
			expected: "message\\nINFO: fake log entry",
		},
		{
			name:     "carriage return injection",
			input:    "message\rINFO: fake",
			expected: "message\\rINFO: fake",
		},
		{
			name:     "tab character",
			input:    "col1\tcol2",
			expected: "col1\\tcol2",
		},
		{
			name:     "mixed special characters",
			input:    "line1\nline2\r\nline3",
			expected: "line1\\nline2\\r\\nline3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeLogMessage(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"0", true},
		{"abc", false},
		{"12a3", false},
		{"", false},
		{"-1", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isNumeric(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPathValidationError(t *testing.T) {
	err := &PathValidationError{
		Path:   "../etc/passwd",
		Reason: "path contains directory traversal",
	}

	assert.Contains(t, err.Error(), "../etc/passwd")
	assert.Contains(t, err.Error(), "directory traversal")
}
