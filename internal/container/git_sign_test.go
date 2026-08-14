package container

import (
	"context"
	"strings"
	"testing"
)

// `versioning.git_sign: true` produced an ordinary unsigned tag and said nothing. The chain
// was dead at every link: versioning.git_sign was read by no code at all, TagOptions.Sign is
// declared and never read by CreateTag, and ServiceConfig.GPGSign is written only by
// WithGPGSign, which nothing calls.
//
// Verified against the old binary: with git_sign set, publish created v0.1.0 and
// `git tag -v` answered "error: no signature found" — 0 PGP signature blocks in the object.
//
// For most settings, silently doing nothing is a bug. For this one it is a false integrity
// claim: the signature is the evidence, and a release policy that requires signed tags was
// being satisfied on paper by tags nobody signed. So it refuses rather than tags.
//
// Refusing only affects someone who deliberately set it — the default is false and the wizard
// writes false — which is exactly the set of people who were being misled.
func TestSigningIsRefusedRatherThanSilentlySkipped(t *testing.T) {
	adapter := NewTagCreatorAdapter(nil, true)

	err := adapter.CreateTag(context.Background(), "v1.2.0", "release 1.2.0")
	if err == nil {
		t.Fatal("CreateTag succeeded with signing required; it would have produced an " +
			"unsigned tag under a policy that asks for a signed one")
	}

	// The message has to name the setting, or the operator cannot act on it.
	if !strings.Contains(err.Error(), "git_sign") {
		t.Errorf("refusal %q does not name versioning.git_sign", err)
	}
	// And the tag, so it is clear which release stopped.
	if !strings.Contains(err.Error(), "v1.2.0") {
		t.Errorf("refusal %q does not name the tag it refused to create", err)
	}
}

// Without the setting, tagging is unaffected: this must not become a new failure mode for
// everyone who never asked for signing.
func TestTaggingIsUnaffectedWhenSigningIsNotRequested(t *testing.T) {
	adapter := NewTagCreatorAdapter(nil, false)

	err := adapter.CreateTag(context.Background(), "v1.2.0", "release 1.2.0")
	if err == nil {
		t.Fatal("expected the nil-adapter error, so this test is exercising the path at all")
	}
	if strings.Contains(err.Error(), "git_sign") {
		t.Errorf("tagging without signing requested failed for a signing reason: %v", err)
	}
}
