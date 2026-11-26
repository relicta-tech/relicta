package multirepo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEffectiveTagPrefix(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		fallback string
		want     string
		wantErr  bool
	}{
		{name: "no config uses fallback", fallback: "release-", want: "release-"},
		{name: "no config defaults to v", want: "v"},
		{name: "configured prefix wins", config: "versioning:\n  tag_prefix: rel-\n", fallback: "v", want: "rel-"},
		{name: "empty configured prefix uses fallback", config: "versioning:\n  tag_prefix: \"\"\n", fallback: "rel-", want: "rel-"},
		{name: "invalid config errors", config: "versioning: [broken\n", fallback: "v", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.config != "" {
				if err := os.WriteFile(filepath.Join(dir, ".relicta.yaml"), []byte(tt.config), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			got, err := effectiveTagPrefix(dir, tt.fallback)
			if (err != nil) != tt.wantErr {
				t.Fatalf("effectiveTagPrefix() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("effectiveTagPrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}
