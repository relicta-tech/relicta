package mcp

import (
	"testing"
)

func TestAppEntries(t *testing.T) {
	if len(appEntries) != 6 {
		t.Fatalf("expected 6 app entries, got %d", len(appEntries))
	}

	expectedURIs := []string{
		"ui://relicta/status",
		"ui://relicta/pipeline",
		"ui://relicta/risk",
		"ui://relicta/commits",
		"ui://relicta/approval",
		"ui://relicta/blast",
	}

	for i, want := range expectedURIs {
		if appEntries[i].uri != want {
			t.Errorf("appEntries[%d].uri = %q, want %q", i, appEntries[i].uri, want)
		}
	}
}

func TestDistFS_ContainsApps(t *testing.T) {
	for _, entry := range appEntries {
		data, err := distFS.ReadFile(entry.filePath)
		if err != nil {
			t.Errorf("distFS.ReadFile(%q) error: %v", entry.filePath, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("distFS.ReadFile(%q) returned empty content", entry.filePath)
		}
	}
}
