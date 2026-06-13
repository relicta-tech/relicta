package manager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeArtifact(t *testing.T, content string) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plugin.tar.gz")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	return path, hex.EncodeToString(sum[:])
}

func TestParseTrustPolicy(t *testing.T) {
	cases := map[string]struct {
		want    TrustPolicy
		wantErr bool
	}{
		"":           {want: TrustDefault},
		"default":    {want: TrustDefault},
		"permissive": {want: TrustPermissive},
		"Enterprise": {want: TrustEnterprise},
		"bogus":      {wantErr: true},
	}
	for in, c := range cases {
		got, err := ParseTrustPolicy(in)
		if c.wantErr != (err != nil) {
			t.Errorf("ParseTrustPolicy(%q) err = %v", in, err)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("ParseTrustPolicy(%q) = %q, want %q", in, got, c.want)
		}
	}
}

func TestVerifyArtifact_DigestOnly(t *testing.T) {
	path, digest := writeArtifact(t, "archive-bytes")
	v := NewVerifier(TrustPermissive)

	if err := v.VerifyArtifact(context.Background(), path, IndexArtifact{Digest: "sha256:" + digest}); err != nil {
		t.Fatalf("matching digest should pass under permissive: %v", err)
	}

	wrong := strings.Repeat("0", 64)
	err := v.VerifyArtifact(context.Background(), path, IndexArtifact{Digest: "sha256:" + wrong})
	if err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("expected digest mismatch, got %v", err)
	}
}

func TestVerifyArtifact_RefusesPlaceholderDigest(t *testing.T) {
	path, _ := writeArtifact(t, "x")
	v := NewVerifier(TrustPermissive)
	err := v.VerifyArtifact(context.Background(), path, IndexArtifact{Digest: DigestPlaceholder})
	if err == nil || !strings.Contains(err.Error(), "refusing to install") {
		t.Fatalf("placeholder digest must be refused, got %v", err)
	}
}

func TestVerifyArtifact_DefaultFailsClosedOnUnsigned(t *testing.T) {
	path, digest := writeArtifact(t, "unsigned")
	v := NewVerifier(TrustDefault)
	err := v.VerifyArtifact(context.Background(), path, IndexArtifact{Digest: "sha256:" + digest})
	if err == nil || !strings.Contains(err.Error(), "requires a Cosign keyless signature") {
		t.Fatalf("default policy must fail closed on unsigned artifacts, got %v", err)
	}
}

func TestVerifyArtifact_KeylessInvokesCosign(t *testing.T) {
	path, digest := writeArtifact(t, "signed-bytes")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"bundle":"fake"}`))
	}))
	defer srv.Close()

	var got []string
	v := NewVerifier(TrustDefault)
	v.runCosign = func(_ context.Context, args ...string) error {
		got = args
		return nil
	}

	artifact := IndexArtifact{
		Digest:                   "sha256:" + digest,
		CosignBundleURL:          srv.URL + "/checksums.txt.sigstore.json",
		CosignCertIdentityRegexp: "https://github.com/relicta-tech/plugin-x/.*",
		CosignOIDCIssuer:         "https://token.actions.githubusercontent.com",
	}
	if err := v.VerifyArtifact(context.Background(), path, artifact); err != nil {
		t.Fatalf("VerifyArtifact: %v", err)
	}

	joined := strings.Join(got, " ")
	for _, want := range []string{
		"verify-blob",
		"--new-bundle-format",
		"--certificate-identity-regexp https://github.com/relicta-tech/plugin-x/.*",
		"--certificate-oidc-issuer https://token.actions.githubusercontent.com",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("cosign args missing %q: %s", want, joined)
		}
	}
}

func TestVerifyArtifact_EnterpriseRequiresKey(t *testing.T) {
	path, digest := writeArtifact(t, "ent")
	v := NewVerifier(TrustEnterprise)
	err := v.VerifyArtifact(context.Background(), path, IndexArtifact{Digest: "sha256:" + digest, CosignSigURL: "https://example.com/sig"})
	if err == nil || !strings.Contains(err.Error(), "trust_key") {
		t.Fatalf("enterprise without key must fail, got %v", err)
	}
}

func TestInstaller_SetVerifier_DigestPathThroughVerifier(t *testing.T) {
	// With a permissive verifier attached, the installer verifies the
	// archive digest through the trust path instead of the legacy checksum
	// branch. artifactForVerification bridges PluginInfo → IndexArtifact.
	p := &PluginInfo{
		Name:      "demo",
		Checksums: map[string]string{GetCurrentPlatform(): "abc123"},
	}
	art := artifactForVerification(p)
	if art.Digest != "sha256:abc123" {
		t.Errorf("digest bridge = %q, want sha256:abc123", art.Digest)
	}
	goos, _ := splitPlatform(GetCurrentPlatform())
	if art.OS != goos {
		t.Errorf("os = %q, want %q", art.OS, goos)
	}
}

func TestInstaller_DefaultPolicyFailsClosedOnLegacyEntry(t *testing.T) {
	// A legacy registry entry carries no Cosign metadata, so the default
	// (signature-requiring) policy must refuse it — the migration pressure
	// toward signed plugins.
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.tar.gz")
	if err := os.WriteFile(archive, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("data"))

	inst := NewInstaller(dir)
	inst.SetVerifier(NewVerifier(TrustDefault))

	art := IndexArtifact{Digest: "sha256:" + hex.EncodeToString(sum[:])}
	err := inst.verifier.VerifyArtifact(context.Background(), archive, art)
	if err == nil || !strings.Contains(err.Error(), "Cosign keyless signature") {
		t.Fatalf("default policy must reject unsigned legacy artifact, got %v", err)
	}
}
