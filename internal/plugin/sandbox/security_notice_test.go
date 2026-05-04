package sandbox

import (
	"runtime"
	"strings"
	"testing"
)

func TestCurrentPosture_PlatformFields(t *testing.T) {
	p := CurrentPosture()

	if p.Platform != runtime.GOOS {
		t.Errorf("Platform: got %q, want %q", p.Platform, runtime.GOOS)
	}
	if p.Architecture != runtime.GOARCH {
		t.Errorf("Architecture: got %q, want %q", p.Architecture, runtime.GOARCH)
	}
	if p.SignatureVerification {
		t.Error("SignatureVerification must be false until signing infrastructure ships")
	}
}

func TestCurrentPosture_LevelByPlatform(t *testing.T) {
	p := CurrentPosture()
	switch runtime.GOOS {
	case "linux":
		if p.Level != EnforcementStrict {
			t.Errorf("linux should be strict; got %q", p.Level)
		}
		if !p.MemoryEnforced {
			t.Error("linux should report memory as enforced")
		}
	case "darwin":
		if p.Level != EnforcementBestEffort {
			t.Errorf("darwin should be best-effort; got %q", p.Level)
		}
		if p.MemoryEnforced {
			t.Error("darwin must NOT claim memory enforcement (RLIMIT_AS ignored on Apple Silicon)")
		}
		if p.CPUEnforced {
			t.Error("darwin must NOT claim CPU enforcement")
		}
	default:
		if p.Level != EnforcementNone {
			t.Errorf("non-linux/darwin should be none; got %q", p.Level)
		}
	}

	if len(p.Caveats) == 0 {
		t.Error("every posture should declare at least one caveat — sandboxing is hard")
	}
}

func TestSecurityNotice_IncludesKeyMarkers(t *testing.T) {
	out := SecurityNotice()

	for _, marker := range []string{
		"Plugin Sandbox Security Posture",
		"Platform:",
		"Enforcement:",
		"Memory limits:",
		"CPU limits:",
		"Plugin signatures:",
	} {
		if !strings.Contains(out, marker) {
			t.Errorf("notice missing marker %q. Output:\n%s", marker, out)
		}
	}

	// Until signing ships, the notice MUST surface that fact prominently.
	if !strings.Contains(out, "NOT verified") {
		t.Errorf("notice must disclose unverified signatures while feature is missing")
	}
}

func TestSecurityNotice_DarwinCallsOutAppleSilicon(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-specific assertion")
	}
	out := SecurityNotice()
	if !strings.Contains(out, "Apple Silicon") {
		t.Error("darwin notice should call out Apple Silicon RLIMIT_AS limitation")
	}
}

func TestRequireExplicitTrust(t *testing.T) {
	got := RequireExplicitTrust()
	switch runtime.GOOS {
	case "linux":
		if got {
			t.Error("linux strict enforcement should not require explicit trust opt-in")
		}
	case "darwin":
		if !got {
			t.Error("darwin best-effort + no signing should require explicit trust opt-in")
		}
	}
}

func TestEnforcementLevel_Values(t *testing.T) {
	if EnforcementStrict == EnforcementBestEffort || EnforcementStrict == EnforcementNone {
		t.Error("enforcement level constants must be distinct")
	}
}
