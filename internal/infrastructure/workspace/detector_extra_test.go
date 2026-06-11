package workspace

import (
	"testing"

	ws "github.com/relicta-tech/relicta/v4/internal/domain/workspace"
)

func TestParseCargoToml(t *testing.T) {
	d := NewFileDetector()

	tests := []struct {
		name     string
		content  string
		isWS     bool
		wantType ws.WorkspaceType
	}{
		{
			name:     "has workspace section",
			content:  "[package]\nname = \"my-crate\"\n\n[workspace]\nmembers = [\"crates/*\"]",
			isWS:     true,
			wantType: ws.WorkspaceTypeCargo,
		},
		{
			name:     "no workspace section",
			content:  "[package]\nname = \"my-crate\"\nversion = \"0.1.0\"",
			isWS:     false,
			wantType: ws.WorkspaceTypeNone,
		},
		{
			name:     "empty content",
			content:  "",
			isWS:     false,
			wantType: ws.WorkspaceTypeNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isWS, wsType, err := d.parseCargoToml([]byte(tt.content))
			if err != nil {
				t.Fatalf("parseCargoToml() error = %v", err)
			}
			if isWS != tt.isWS {
				t.Errorf("isWorkspace = %v, want %v", isWS, tt.isWS)
			}
			if wsType != tt.wantType {
				t.Errorf("type = %v, want %v", wsType, tt.wantType)
			}
		})
	}
}

func TestParsePomXML(t *testing.T) {
	d := NewFileDetector()

	tests := []struct {
		name     string
		content  string
		isWS     bool
		wantType ws.WorkspaceType
	}{
		{
			name:     "has modules section",
			content:  "<project>\n<modules>\n<module>core</module>\n</modules>\n</project>",
			isWS:     true,
			wantType: ws.WorkspaceTypeMaven,
		},
		{
			name:     "no modules section",
			content:  "<project><groupId>com.example</groupId></project>",
			isWS:     false,
			wantType: ws.WorkspaceTypeNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isWS, wsType, err := d.parsePomXML([]byte(tt.content))
			if err != nil {
				t.Fatalf("parsePomXML() error = %v", err)
			}
			if isWS != tt.isWS {
				t.Errorf("isWorkspace = %v, want %v", isWS, tt.isWS)
			}
			if wsType != tt.wantType {
				t.Errorf("type = %v, want %v", wsType, tt.wantType)
			}
		})
	}
}

func TestShouldExclude(t *testing.T) {
	d := NewFileDetector()

	tests := []struct {
		name     string
		path     string
		root     string
		patterns []string
		expected bool
	}{
		{"matching pattern", "/root/node_modules", "/root", []string{"node_modules"}, true},
		{"non-matching", "/root/src", "/root", []string{"node_modules"}, false},
		{"no patterns", "/root/anything", "/root", nil, false},
		{"glob pattern", "/root/test-pkg", "/root", []string{"test-*"}, true},
		{"nested path", "/root/packages/foo", "/root", []string{"packages/*"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.shouldExclude(tt.path, tt.patterns, tt.root)
			if got != tt.expected {
				t.Errorf("shouldExclude(%q, %v, %q) = %v, want %v", tt.path, tt.patterns, tt.root, got, tt.expected)
			}
		})
	}
}

func TestDefaultPackagePatterns(t *testing.T) {
	d := NewFileDetector()

	tests := []struct {
		name   string
		wsType ws.WorkspaceType
		minLen int
	}{
		{"npm", ws.WorkspaceTypeNpm, 1},
		{"go module", ws.WorkspaceTypeGoModule, 1},
		{"cargo", ws.WorkspaceTypeCargo, 1},
		{"maven", ws.WorkspaceTypeMaven, 1},
		{"custom", ws.WorkspaceTypeCustom, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := d.defaultPackagePatterns(tt.wsType)
			if len(patterns) < tt.minLen {
				t.Errorf("defaultPackagePatterns(%v) returned %d patterns, want >= %d", tt.wsType, len(patterns), tt.minLen)
			}
		})
	}
}
