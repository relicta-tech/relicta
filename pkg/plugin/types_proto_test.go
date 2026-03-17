package plugin

import (
	"testing"
)

// TestProtoMessageInterfaces exercises the ProtoMessage, Reset, and String
// methods on all proto message types to ensure coverage of the compatibility stubs.
func TestProtoMessageInterfaces(t *testing.T) {
	// ValidateRequestProto
	t.Run("ValidateRequestProto", func(t *testing.T) {
		m := &ValidateRequestProto{Config: `{"key":"value"}`}
		m.ProtoMessage()
		s := m.String()
		if s == "" {
			t.Error("String() should return non-empty")
		}
		m.Reset()
		if m.Config != "" {
			t.Error("Reset() should clear Config")
		}
	})

	// ValidateResponseProto
	t.Run("ValidateResponseProto", func(t *testing.T) {
		m := &ValidateResponseProto{Valid: true}
		m.ProtoMessage()
		s := m.String()
		if s == "" {
			t.Error("String() should return non-empty")
		}
		m.Reset()
		if m.Valid {
			t.Error("Reset() should clear Valid")
		}
	})

	// ValidationErrorProto
	t.Run("ValidationErrorProto", func(t *testing.T) {
		m := &ValidationErrorProto{Field: "name", Message: "required"}
		m.ProtoMessage()
		s := m.String()
		if s == "" {
			t.Error("String() should return non-empty")
		}
		m.Reset()
		if m.Field != "" {
			t.Error("Reset() should clear Field")
		}
	})

	// ExecuteRequestProto
	t.Run("ExecuteRequestProto", func(t *testing.T) {
		m := &ExecuteRequestProto{Hook: HookProto_HOOK_PRE_INIT}
		m.ProtoMessage()
		s := m.String()
		if s == "" {
			t.Error("String() should return non-empty")
		}
		m.Reset()
	})

	// ExecuteResponseProto
	t.Run("ExecuteResponseProto", func(t *testing.T) {
		m := &ExecuteResponseProto{Success: true, Message: "ok"}
		m.ProtoMessage()
		s := m.String()
		if s == "" {
			t.Error("String() should return non-empty")
		}
		m.Reset()
		if m.Success {
			t.Error("Reset() should clear Success")
		}
	})

	// ReleaseContextProto
	t.Run("ReleaseContextProto", func(t *testing.T) {
		m := &ReleaseContextProto{Version: "1.0.0"}
		m.ProtoMessage()
		s := m.String()
		if s == "" {
			t.Error("String() should return non-empty")
		}
		m.Reset()
		if m.Version != "" {
			t.Error("Reset() should clear Version")
		}
	})

	// CategorizedChangesProto
	t.Run("CategorizedChangesProto", func(t *testing.T) {
		m := &CategorizedChangesProto{}
		m.ProtoMessage()
		s := m.String()
		if s == "" {
			t.Error("String() should return non-empty")
		}
		m.Reset()
	})

	// ConventionalCommitProto
	t.Run("ConventionalCommitProto", func(t *testing.T) {
		m := &ConventionalCommitProto{Type: "feat", Description: "add thing"}
		m.ProtoMessage()
		s := m.String()
		if s == "" {
			t.Error("String() should return non-empty")
		}
		m.Reset()
		if m.Type != "" {
			t.Error("Reset() should clear Type")
		}
	})

	// ArtifactProto
	t.Run("ArtifactProto", func(t *testing.T) {
		m := &ArtifactProto{Name: "release.tar.gz"}
		m.ProtoMessage()
		s := m.String()
		if s == "" {
			t.Error("String() should return non-empty")
		}
		m.Reset()
		if m.Name != "" {
			t.Error("Reset() should clear Name")
		}
	})

	// Empty
	t.Run("Empty", func(t *testing.T) {
		m := &Empty{}
		m.ProtoMessage()
		s := m.String()
		if s != "" {
			t.Errorf("Empty.String() should return empty, got %q", s)
		}
		m.Reset()
	})

	// PluginInfo
	t.Run("PluginInfo", func(t *testing.T) {
		m := &PluginInfo{Name: "github", Version: "1.0.0"}
		m.ProtoMessage()
		s := m.String()
		if s == "" {
			t.Error("String() should return non-empty")
		}
		m.Reset()
		if m.Name != "" {
			t.Error("Reset() should clear Name")
		}
	})
}
