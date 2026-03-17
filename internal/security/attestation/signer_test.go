package attestation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStatement() *Statement {
	return &Statement{
		Type: StatementType,
		Subject: []Subject{
			{
				Name:   "v1.0.0",
				Digest: map[string]string{"sha256": "abc123"},
			},
		},
		PredicateType: PredicateTypeGovernance,
		Predicate: GovernancePredicate{
			Version:  "1.0.0",
			Decision: "approved",
		},
	}
}

func TestSigner_NoneMode(t *testing.T) {
	signer := NewSigner(SigningModeNone)
	stmt := newTestStatement()

	att, err := signer.Sign(context.Background(), stmt)
	require.NoError(t, err)

	assert.Equal(t, PayloadType, att.PayloadType)
	assert.NotEmpty(t, att.Payload)
	assert.Empty(t, att.Signatures)

	// Verify payload is valid JSON.
	var decoded Statement
	err = json.Unmarshal(att.Payload, &decoded)
	require.NoError(t, err)
	assert.Equal(t, StatementType, decoded.Type)
}

func TestSigner_LocalMode(t *testing.T) {
	// Generate test key.
	privPEM, _, err := GenerateTestKey()
	require.NoError(t, err)

	// Write key to temp file.
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "test.key")
	err = os.WriteFile(keyPath, privPEM, 0600)
	require.NoError(t, err)

	signer := NewSigner(SigningModeLocal, WithKeyPath(keyPath))
	stmt := newTestStatement()

	att, err := signer.Sign(context.Background(), stmt)
	require.NoError(t, err)

	assert.Equal(t, PayloadType, att.PayloadType)
	assert.NotEmpty(t, att.Payload)
	require.Len(t, att.Signatures, 1)
	assert.Equal(t, keyPath, att.Signatures[0].KeyID)
	assert.NotEmpty(t, att.Signatures[0].Sig)
}

func TestSigner_LocalMode_MissingKeyPath(t *testing.T) {
	signer := NewSigner(SigningModeLocal)
	stmt := newTestStatement()

	_, err := signer.Sign(context.Background(), stmt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key_path is required")
}

func TestSigner_LocalMode_InvalidKeyFile(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "bad.key")
	err := os.WriteFile(keyPath, []byte("not a key"), 0600)
	require.NoError(t, err)

	signer := NewSigner(SigningModeLocal, WithKeyPath(keyPath))
	stmt := newTestStatement()

	_, err = signer.Sign(context.Background(), stmt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse private key")
}

func TestSigner_LocalMode_NonexistentKeyFile(t *testing.T) {
	signer := NewSigner(SigningModeLocal, WithKeyPath("/nonexistent/path.key"))
	stmt := newTestStatement()

	_, err := signer.Sign(context.Background(), stmt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read key file")
}

func TestSigner_KeylessMode_Unsupported(t *testing.T) {
	signer := NewSigner(SigningModeKeyless)
	stmt := newTestStatement()

	_, err := signer.Sign(context.Background(), stmt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sigstore-go")
}

func TestParseSigningMode(t *testing.T) {
	tests := []struct {
		input string
		want  SigningMode
		err   bool
	}{
		{"keyless", SigningModeKeyless, false},
		{"local", SigningModeLocal, false},
		{"none", SigningModeNone, false},
		{"", SigningModeNone, false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseSigningMode(tt.input)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestGenerateTestKey(t *testing.T) {
	priv, pub, err := GenerateTestKey()
	require.NoError(t, err)
	assert.Contains(t, string(priv), "EC PRIVATE KEY")
	assert.Contains(t, string(pub), "PUBLIC KEY")
}
