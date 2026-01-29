package cli

import "testing"

func TestResolveDisplayAddress(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{":8080", "localhost:8080"},
		{"0.0.0.0:8080", "0.0.0.0:8080"},
		{"localhost:3000", "localhost:3000"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := resolveDisplayAddress(tt.input)
			if got != tt.want {
				t.Errorf("resolveDisplayAddress(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
