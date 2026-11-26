package container

import (
	"context"
	"strings"
	"testing"
)

// Signing is delegated to Git's configured signing backend by the infrastructure adapter.
// This case ensures a missing adapter fails explicitly before an unsigned tag can be created.
func TestSigningRequiresConfiguredGitAdapter(t *testing.T) {
	adapter := NewTagCreatorAdapter(nil, true)

	err := adapter.CreateTag(context.Background(), "v1.2.0", "release 1.2.0")
	if err == nil {
		t.Fatal("CreateTag succeeded with signing required; it would have produced an " +
			"unsigned tag under a policy that asks for a signed one")
	}

	if !strings.Contains(err.Error(), "git adapter not configured") {
		t.Errorf("error %q should identify the missing git adapter", err)
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
