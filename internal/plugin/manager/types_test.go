package manager

import (
	"runtime"
	"testing"
)

func TestGetCurrentPlatform(t *testing.T) {
	platform := GetCurrentPlatform()

	// Should return a valid platform string
	if platform == "" {
		t.Error("GetCurrentPlatform returned empty string")
	}

	// Should contain OS
	goos := runtime.GOOS
	if goos != "darwin" && goos != "linux" && goos != "windows" {
		t.Skipf("Unsupported OS: %s", goos)
	}

	// Verify format is {os}_{arch}
	expectedOS := goos
	expectedArch := "x86_64"
	if runtime.GOARCH == "arm64" {
		expectedArch = "aarch64"
	}
	expected := expectedOS + "_" + expectedArch

	if platform != expected {
		t.Errorf("GetCurrentPlatform() = %q, want %q", platform, expected)
	}
}

func TestPluginInfo_GetChecksum(t *testing.T) {
	tests := []struct {
		name     string
		info     PluginInfo
		want     string
		platform string
	}{
		{
			name: "with checksums",
			info: PluginInfo{
				Name: "test-plugin",
				Checksums: map[string]string{
					"darwin_aarch64": "abc123",
					"darwin_x86_64":  "def456",
					"linux_x86_64":   "ghi789",
				},
			},
			want:     "", // Will be checked dynamically based on platform
			platform: GetCurrentPlatform(),
		},
		{
			name: "nil checksums",
			info: PluginInfo{
				Name:      "test-plugin",
				Checksums: nil,
			},
			want:     "",
			platform: GetCurrentPlatform(),
		},
		{
			name: "empty checksums",
			info: PluginInfo{
				Name:      "test-plugin",
				Checksums: map[string]string{},
			},
			want:     "",
			platform: GetCurrentPlatform(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.info.GetChecksum()

			if len(tt.info.Checksums) == 0 {
				if got != "" {
					t.Errorf("GetChecksum() = %q, want empty string for nil/empty checksums", got)
				}
				return
			}

			expected := tt.info.Checksums[tt.platform]
			if got != expected {
				t.Errorf("GetChecksum() = %q, want %q for platform %s", got, expected, tt.platform)
			}
		})
	}
}

func TestPluginInfo_IsSDKCompatible(t *testing.T) {
	tests := []struct {
		name          string
		minSDKVersion int
		want          bool
	}{
		{
			name:          "zero version (legacy)",
			minSDKVersion: 0,
			want:          true,
		},
		{
			name:          "current version",
			minSDKVersion: CurrentSDKVersion,
			want:          true,
		},
		{
			name:          "older version",
			minSDKVersion: CurrentSDKVersion - 1,
			want:          true,
		},
		{
			name:          "newer version",
			minSDKVersion: CurrentSDKVersion + 1,
			want:          false,
		},
		{
			name:          "much newer version",
			minSDKVersion: CurrentSDKVersion + 100,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := PluginInfo{
				Name:          "test-plugin",
				MinSDKVersion: tt.minSDKVersion,
			}

			got := info.IsSDKCompatible()
			if got != tt.want {
				t.Errorf("IsSDKCompatible() = %v, want %v (minSDK=%d, currentSDK=%d)",
					got, tt.want, tt.minSDKVersion, CurrentSDKVersion)
			}
		})
	}
}

func TestUpdateResult(t *testing.T) {
	// Test that UpdateResult fields are properly set
	result := UpdateResult{
		Name:           "test-plugin",
		CurrentVersion: "v1.0.0",
		LatestVersion:  "v1.1.0",
		Updated:        true,
		Error:          "",
	}

	if result.Name != "test-plugin" {
		t.Errorf("Name = %q, want %q", result.Name, "test-plugin")
	}
	if result.CurrentVersion != "v1.0.0" {
		t.Errorf("CurrentVersion = %q, want %q", result.CurrentVersion, "v1.0.0")
	}
	if result.LatestVersion != "v1.1.0" {
		t.Errorf("LatestVersion = %q, want %q", result.LatestVersion, "v1.1.0")
	}
	if !result.Updated {
		t.Error("Updated = false, want true")
	}
	if result.Error != "" {
		t.Errorf("Error = %q, want empty string", result.Error)
	}
}

func TestPluginStatus(t *testing.T) {
	tests := []struct {
		status PluginStatus
		want   string
	}{
		{StatusNotInstalled, "not_installed"},
		{StatusInstalled, "installed"},
		{StatusEnabled, "enabled"},
		{StatusUpdateAvailable, "update_available"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("PluginStatus = %q, want %q", tt.status, tt.want)
		}
	}
}
