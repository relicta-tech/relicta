package attestation

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
)

// SigningMode determines how attestations are signed.
type SigningMode string

const (
	// SigningModeKeyless uses Sigstore OIDC-based keyless signing
	// (Fulcio + Rekor). Requires ambient OIDC credentials in CI
	// or browser-based OAuth for interactive use.
	SigningModeKeyless SigningMode = "keyless"

	// SigningModeLocal uses a local private key file for signing.
	SigningModeLocal SigningMode = "local"

	// SigningModeNone produces unsigned attestations.
	SigningModeNone SigningMode = "none"
)

// ParseSigningMode parses a string into a SigningMode.
func ParseSigningMode(s string) (SigningMode, error) {
	switch s {
	case "keyless":
		return SigningModeKeyless, nil
	case "local":
		return SigningModeLocal, nil
	case "none", "":
		return SigningModeNone, nil
	default:
		return "", fmt.Errorf("unknown signing mode: %q (supported: keyless, local, none)", s)
	}
}

// Signer signs attestation statements.
type Signer struct {
	mode    SigningMode
	keyPath string
}

// SignerOption configures the Signer.
type SignerOption func(*Signer)

// WithKeyPath sets the private key file path for local signing.
func WithKeyPath(path string) SignerOption {
	return func(s *Signer) {
		s.keyPath = path
	}
}

// NewSigner creates a new Signer with the given mode and options.
func NewSigner(mode SigningMode, opts ...SignerOption) *Signer {
	s := &Signer{mode: mode}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Sign signs the given Statement and returns a SignedAttestation.
func (s *Signer) Sign(ctx context.Context, stmt *Statement) (*SignedAttestation, error) {
	payload, err := json.Marshal(stmt)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal statement: %w", err)
	}

	att := &SignedAttestation{
		Payload:     payload,
		PayloadType: PayloadType,
	}

	switch s.mode {
	case SigningModeNone:
		// No signatures
		return att, nil

	case SigningModeLocal:
		sig, err := s.signLocal(payload)
		if err != nil {
			return nil, fmt.Errorf("local signing failed: %w", err)
		}
		att.Signatures = []Signature{sig}
		return att, nil

	case SigningModeKeyless:
		return nil, fmt.Errorf("keyless signing requires sigstore-go; install with: go get github.com/sigstore/sigstore-go")

	default:
		return nil, fmt.Errorf("unsupported signing mode: %s", s.mode)
	}
}

// signLocal signs the payload with a local ECDSA private key.
func (s *Signer) signLocal(payload []byte) (Signature, error) {
	if s.keyPath == "" {
		return Signature{}, fmt.Errorf("key_path is required for local signing mode")
	}

	keyData, err := os.ReadFile(s.keyPath)
	if err != nil {
		return Signature{}, fmt.Errorf("failed to read key file %s: %w", s.keyPath, err)
	}

	privKey, err := parsePrivateKey(keyData)
	if err != nil {
		return Signature{}, fmt.Errorf("failed to parse private key: %w", err)
	}

	digest := sha256.Sum256(payload)

	sigBytes, err := privKey.Sign(rand.Reader, digest[:], crypto.SHA256)
	if err != nil {
		return Signature{}, fmt.Errorf("signing failed: %w", err)
	}

	return Signature{
		KeyID: s.keyPath,
		Sig:   base64.StdEncoding.EncodeToString(sigBytes),
	}, nil
}

// parsePrivateKey parses a PEM-encoded ECDSA private key.
func parsePrivateKey(data []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in key data")
	}

	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		pkcs8Key, pkcs8Err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if pkcs8Err != nil {
			return nil, fmt.Errorf("failed to parse as EC or PKCS8 key: EC=%w, PKCS8=%w", err, pkcs8Err)
		}
		ecKey, ok := pkcs8Key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS8 key is not ECDSA (got %T)", pkcs8Key)
		}
		return ecKey, nil
	}
	return key, nil
}

// GenerateTestKey generates an ECDSA P-256 key pair for testing.
// Returns the PEM-encoded private key and public key.
func GenerateTestKey() (privPEM, pubPEM []byte, err error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	privBytes, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return nil, nil, err
	}
	privPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privBytes,
	})

	pubBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	pubPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	return privPEM, pubPEM, nil
}
