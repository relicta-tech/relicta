//go:build darwin

package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

func TestNeedsSeatbelt(t *testing.T) {
	tests := []struct {
		name string
		caps *config.PluginCapabilities
		want bool
	}{
		{"nil caps", nil, false},
		{"fully permitted", &config.PluginCapabilities{AllowNetwork: true, AllowFilesystem: true}, false},
		{"network denied", &config.PluginCapabilities{AllowNetwork: false, AllowFilesystem: true}, true},
		{"filesystem denied", &config.PluginCapabilities{AllowNetwork: true, AllowFilesystem: false}, true},
		{"path allowlist set", &config.PluginCapabilities{AllowNetwork: true, AllowFilesystem: true, AllowedPaths: []string{"/tmp/x"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needsSeatbelt(tt.caps); got != tt.want {
				t.Fatalf("needsSeatbelt = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildSeatbeltProfile_Network(t *testing.T) {
	denied := buildSeatbeltProfile(&config.PluginCapabilities{AllowNetwork: false, AllowFilesystem: true})
	if !strings.Contains(denied, "(deny network-outbound)") {
		t.Fatalf("network-denied profile must deny outbound, got:\n%s", denied)
	}
	if !strings.Contains(denied, `(allow network-outbound (remote ip "localhost:*"))`) {
		t.Fatalf("must keep loopback for the gRPC handshake, got:\n%s", denied)
	}

	allowed := buildSeatbeltProfile(&config.PluginCapabilities{AllowNetwork: true, AllowFilesystem: true, AllowedPaths: []string{"/tmp/x"}})
	if strings.Contains(allowed, "deny network-outbound") {
		t.Fatalf("network-allowed profile must not deny outbound, got:\n%s", allowed)
	}
}

func TestBuildSeatbeltProfile_FilesystemConfinesAllowedPaths(t *testing.T) {
	profile := buildSeatbeltProfile(&config.PluginCapabilities{
		AllowNetwork:    true,
		AllowFilesystem: false,
		AllowedPaths:    []string{"/opt/relicta/data"},
	})
	if !strings.Contains(profile, "(deny file-write*)") {
		t.Fatalf("filesystem-restricted profile must deny writes, got:\n%s", profile)
	}
	if !strings.Contains(profile, `(allow file-write* (subpath "/opt/relicta/data"))`) {
		t.Fatalf("allowed path must remain writable, got:\n%s", profile)
	}
	if !strings.Contains(profile, `(subpath "/private/tmp")`) {
		t.Fatalf("temp must remain writable so the runtime works, got:\n%s", profile)
	}
}

func TestWrapWithSeatbelt_RewritesCommand(t *testing.T) {
	if !seatbeltAvailable() {
		t.Skip("sandbox-exec not available")
	}
	s := New("test-plugin", &config.PluginCapabilities{AllowNetwork: false, AllowFilesystem: false})
	cmd := exec.Command("/bin/echo", "hello")

	s.wrapWithSeatbelt(cmd)

	sbexec, _ := exec.LookPath("sandbox-exec")
	if cmd.Path != sbexec {
		t.Fatalf("cmd.Path = %s, want %s", cmd.Path, sbexec)
	}
	if len(cmd.Args) < 5 || cmd.Args[0] != sbexec || cmd.Args[1] != "-p" {
		t.Fatalf("expected sandbox-exec -p <profile> wrapping, got args: %v", cmd.Args)
	}
	// Original binary and its args must be preserved after the profile.
	if cmd.Args[3] != "/bin/echo" || cmd.Args[4] != "hello" {
		t.Fatalf("original command not preserved, got args: %v", cmd.Args)
	}
}

func TestWrapWithSeatbelt_NoOpWhenUnrestricted(t *testing.T) {
	s := New("test-plugin", &config.PluginCapabilities{AllowNetwork: true, AllowFilesystem: true})
	cmd := exec.Command("/bin/echo", "hello")
	origPath := cmd.Path

	s.wrapWithSeatbelt(cmd)

	if cmd.Path != origPath {
		t.Fatalf("unrestricted plugin must not be wrapped; cmd.Path = %s", cmd.Path)
	}
}

// TestSeatbelt_EnforcesWriteConfinement runs sandbox-exec for real and proves the
// generated profile actually denies a write outside the allowed set while
// permitting one inside temp. The "outside" target lives under HOME (normally
// writable), so a failure is attributable to the seatbelt, not to permissions.
func TestSeatbelt_EnforcesWriteConfinement(t *testing.T) {
	if !seatbeltAvailable() {
		t.Skip("sandbox-exec not available")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	outside := filepath.Join(home, ".relicta_seatbelt_test_artifact")
	t.Cleanup(func() { _ = os.Remove(outside) })

	profile := buildSeatbeltProfile(&config.PluginCapabilities{AllowNetwork: true, AllowFilesystem: false})

	// Denied: write outside temp/allowed paths.
	denyCmd := exec.Command("sandbox-exec", "-p", profile, "/bin/sh", "-c", "echo x > "+outside)
	if err := denyCmd.Run(); err == nil {
		t.Fatalf("seatbelt must deny write to %s", outside)
	}
	if _, statErr := os.Stat(outside); statErr == nil {
		t.Fatalf("denied write must not create %s", outside)
	}

	// Allowed: write inside temp.
	inside := filepath.Join(t.TempDir(), "ok.txt")
	allowCmd := exec.Command("sandbox-exec", "-p", profile, "/bin/sh", "-c", "echo x > "+inside)
	if err := allowCmd.Run(); err != nil {
		t.Fatalf("seatbelt must permit write to temp %s: %v", inside, err)
	}
}
