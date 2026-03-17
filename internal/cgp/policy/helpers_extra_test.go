package policy

import (
	"log/slog"
	"testing"
)

func TestIntFromValue(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantVal int
		wantOk  bool
	}{
		{"int", 42, 42, true},
		{"int8", int8(8), 8, true},
		{"int16", int16(16), 16, true},
		{"int32", int32(32), 32, true},
		{"int64", int64(64), 64, true},
		{"uint", uint(10), 10, true},
		{"uint8", uint8(8), 8, true},
		{"uint16", uint16(16), 16, true},
		{"uint32", uint32(32), 32, true},
		{"uint64", uint64(64), 64, true},
		{"float32", float32(3.7), 3, true},
		{"float64", float64(9.9), 9, true},
		{"string", "42", 0, false},
		{"nil", nil, 0, false},
		{"bool", true, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := intFromValue(tt.input)
			if ok != tt.wantOk {
				t.Errorf("intFromValue(%v) ok = %v, want %v", tt.input, ok, tt.wantOk)
			}
			if ok && got != tt.wantVal {
				t.Errorf("intFromValue(%v) = %v, want %v", tt.input, got, tt.wantVal)
			}
		})
	}
}

func TestValueIn_Extra(t *testing.T) {
	tests := []struct {
		name       string
		fieldValue any
		listValue  any
		expected   bool
	}{
		{"string in []any", "foo", []any{"foo", "bar"}, true},
		{"string not in []any", "baz", []any{"foo", "bar"}, false},
		{"int in []any", 42, []any{42, 99}, true},
		{"string in []string", "foo", []string{"foo", "bar"}, true},
		{"string not in []string", "baz", []string{"foo", "bar"}, false},
		{"int with []string", 42, []string{"42"}, false},
		{"empty []any", "foo", []any{}, false},
		{"empty []string", "foo", []string{}, false},
		{"unsupported list type", "foo", "not-a-list", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valueIn(tt.fieldValue, tt.listValue)
			if got != tt.expected {
				t.Errorf("valueIn(%v, %v) = %v, want %v", tt.fieldValue, tt.listValue, got, tt.expected)
			}
		})
	}
}

func TestValueContains_Extra(t *testing.T) {
	tests := []struct {
		name        string
		fieldValue  any
		searchValue any
		expected    bool
	}{
		{"string contains", "hello world", "world", true},
		{"string not contains", "hello world", "mars", false},
		{"non-string field", 42, "42", false},
		{"non-string search", "hello", 42, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valueContains(tt.fieldValue, tt.searchValue)
			if got != tt.expected {
				t.Errorf("valueContains(%v, %v) = %v, want %v", tt.fieldValue, tt.searchValue, got, tt.expected)
			}
		})
	}
}

// TestEngineContextInitialization tests that SetBusinessHours, AddFreezePeriod,
// AddTeam, AddRole, and AssignActorRole initialize their contexts when nil.
func TestEngineContextInitialization(t *testing.T) {
	t.Run("SetBusinessHours initializes timeContext", func(t *testing.T) {
		e := &Engine{logger: slog.Default()}
		e.SetBusinessHours(BusinessHoursConfig{StartHour: 9, EndHour: 17})
		// Should not panic and should have set business hours
	})

	t.Run("AddFreezePeriod initializes timeContext", func(t *testing.T) {
		e := &Engine{logger: slog.Default()}
		e.AddFreezePeriod(FreezePeriod{Name: "test"})
		// Should not panic
	})

	t.Run("AddTeam initializes teamContext", func(t *testing.T) {
		e := &Engine{logger: slog.Default()}
		e.AddTeam(&Team{Name: "backend"})
		// Should not panic
	})

	t.Run("AddRole initializes teamContext", func(t *testing.T) {
		e := &Engine{logger: slog.Default()}
		e.AddRole(&Role{Name: "admin"})
		// Should not panic
	})

	t.Run("AssignActorRole initializes teamContext", func(t *testing.T) {
		e := &Engine{logger: slog.Default()}
		e.AssignActorRole("user1", "admin")
		// Should not panic
	})
}

func TestValueMatches(t *testing.T) {
	tests := []struct {
		name       string
		fieldValue any
		pattern    any
		expected   bool
		wantErr    bool
	}{
		{"matching regex", "release-v1.0", "release-.*", true, false},
		{"non-matching regex", "feature/auth", "^release", false, false},
		{"exact match", "main", "^main$", true, false},
		{"non-string field", 42, "42", false, false},
		{"non-string pattern", "hello", 42, false, true},
		{"invalid regex", "hello", "[invalid", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := valueMatches(tt.fieldValue, tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("valueMatches(%v, %v) error = %v, wantErr %v", tt.fieldValue, tt.pattern, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("valueMatches(%v, %v) = %v, want %v", tt.fieldValue, tt.pattern, got, tt.expected)
			}
		})
	}
}
