// Package manager provides plugin management functionality for Relicta.
package manager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TrustPolicy controls what verification an artifact must pass before it is
// installed (ADR-008). Policies fail closed: under `default`, an artifact
// without a verifiable signature is refused.
type TrustPolicy string

const (
	// TrustPermissive verifies the artifact digest only.
	TrustPermissive TrustPolicy = "permissive"
	// TrustDefault verifies the digest plus a Sigstore Cosign keyless
	// bundle (cert identity regexp + OIDC issuer from the index entry).
	TrustDefault TrustPolicy = "default"
	// TrustEnterprise verifies the digest plus a signature from an
	// operator-managed key (cosign verify-blob --key).
	TrustEnterprise TrustPolicy = "enterprise"
)

// ParseTrustPolicy validates a policy string; empty input means TrustDefault.
func ParseTrustPolicy(s string) (TrustPolicy, error) {
	switch TrustPolicy(strings.ToLower(strings.TrimSpace(s))) {
	case "":
		return TrustDefault, nil
	case TrustPermissive:
		return TrustPermissive, nil
	case TrustDefault:
		return TrustDefault, nil
	case TrustEnterprise:
		return TrustEnterprise, nil
	default:
		return "", fmt.Errorf("unknown trust policy %q (permissive | default | enterprise)", s)
	}
}

// Verifier checks downloaded plugin artifacts against a trust policy.
type Verifier struct {
	// Policy in effect.
	Policy TrustPolicy
	// KeyPath is the operator-managed public key for TrustEnterprise.
	KeyPath string
	// CosignPath overrides the cosign binary location (tests).
	CosignPath string
	// HTTPClient fetches signature bundles; defaults to a 30s-timeout client.
	HTTPClient *http.Client
	// runCosign executes the cosign CLI; replaceable in tests.
	runCosign func(ctx context.Context, args ...string) error
}

// NewVerifier constructs a Verifier for the given policy.
func NewVerifier(policy TrustPolicy) *Verifier {
	v := &Verifier{
		Policy:     policy,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
	v.runCosign = v.execCosign
	return v
}

// VerifyArtifact checks a downloaded archive at archivePath against the index
// artifact metadata under the verifier's policy. The error message lists every
// violated requirement so an operator sees the full picture at once.
func (v *Verifier) VerifyArtifact(ctx context.Context, archivePath string, artifact IndexArtifact) error {
	// Digest is mandatory under every policy; placeholder digests are refused.
	if !artifact.HasValidDigest() {
		return fmt.Errorf("artifact has no usable digest (%q): registry entry is incomplete; refusing to install", artifact.Digest)
	}
	actual, err := fileSHA256(archivePath)
	if err != nil {
		return fmt.Errorf("digest artifact: %w", err)
	}
	expected := strings.TrimPrefix(artifact.Digest, "sha256:")
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("digest mismatch: expected sha256:%s, got sha256:%s", expected, actual)
	}

	switch v.Policy {
	case TrustPermissive:
		return nil
	case TrustDefault:
		if !artifact.IsSigned() {
			return fmt.Errorf("trust policy %q requires a Cosign keyless signature, but the registry entry carries none; use --trust-policy permissive to install unsigned artifacts at your own risk", v.Policy)
		}
		return v.verifyCosignKeyless(ctx, archivePath, artifact)
	case TrustEnterprise:
		if v.KeyPath == "" {
			return fmt.Errorf("trust policy %q requires plugins.trust_key (operator public key)", v.Policy)
		}
		if artifact.CosignSigURL == "" {
			return fmt.Errorf("trust policy %q requires a signature, but the registry entry carries none", v.Policy)
		}
		return v.verifyCosignKeyed(ctx, archivePath, artifact)
	default:
		return fmt.Errorf("unknown trust policy %q", v.Policy)
	}
}

// verifyCosignKeyless downloads the Sigstore bundle and verifies the archive
// blob with cosign keyless verification, pinning the certificate identity and
// OIDC issuer declared in the registry entry.
func (v *Verifier) verifyCosignKeyless(ctx context.Context, archivePath string, artifact IndexArtifact) error {
	bundlePath, cleanup, err := v.fetchToTemp(ctx, artifact.CosignBundleURL, "relicta-cosign-bundle-*")
	if err != nil {
		return fmt.Errorf("fetch cosign bundle: %w", err)
	}
	defer cleanup()

	args := []string{
		"verify-blob",
		"--bundle", bundlePath,
		"--new-bundle-format",
		"--certificate-identity-regexp", artifact.CosignCertIdentityRegexp,
		"--certificate-oidc-issuer", artifact.CosignOIDCIssuer,
		archivePath,
	}
	if err := v.runCosign(ctx, args...); err != nil {
		return fmt.Errorf("cosign keyless verification failed: %w", err)
	}
	return nil
}

// verifyCosignKeyed verifies the archive against an operator-managed public key.
func (v *Verifier) verifyCosignKeyed(ctx context.Context, archivePath string, artifact IndexArtifact) error {
	sigPath, cleanup, err := v.fetchToTemp(ctx, artifact.CosignSigURL, "relicta-cosign-sig-*")
	if err != nil {
		return fmt.Errorf("fetch signature: %w", err)
	}
	defer cleanup()

	args := []string{
		"verify-blob",
		"--key", v.KeyPath,
		"--signature", sigPath,
		archivePath,
	}
	if err := v.runCosign(ctx, args...); err != nil {
		return fmt.Errorf("cosign keyed verification failed: %w", err)
	}
	return nil
}

// execCosign runs the cosign CLI. Cosign is a soft dependency: it is only
// required under the default/enterprise policies.
func (v *Verifier) execCosign(ctx context.Context, args ...string) error {
	bin := v.CosignPath
	if bin == "" {
		bin = "cosign"
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return fmt.Errorf("cosign not found in PATH (required by trust policy %q): install cosign or use --trust-policy permissive", v.Policy)
	}
	cmd := exec.CommandContext(ctx, path, args...) // #nosec G204 -- fixed binary, controlled args
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// fetchToTemp downloads a URL to a temp file and returns its path + cleanup.
func (v *Verifier) fetchToTemp(ctx context.Context, url, pattern string) (string, func(), error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", nil, err
	}
	client := v.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("unexpected status %d fetching %s", resp.StatusCode, url)
	}

	tmp, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, err
	}
	if _, err := io.Copy(tmp, io.LimitReader(resp.Body, 10<<20)); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", nil, err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", nil, err
	}
	name := tmp.Name()
	return name, func() { _ = os.Remove(name) }, nil
}

// fileSHA256 returns the hex SHA-256 of a file.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
