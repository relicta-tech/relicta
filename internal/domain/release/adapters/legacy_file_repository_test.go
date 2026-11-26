package adapters

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyFileReleaseRecordLoadsAndMigratesOnSave(t *testing.T) {
	repoRoot := t.TempDir()
	const head = "cccccccccccccccccccccccccccccccccccccccc"
	legacy := `{
		"id":"run-legacy",
		"state":"planned",
		"branch":"main",
		"repo_root":` + quoteJSON(t, repoRoot) + `,
		"repository_name":"acme/widget",
		"created_at":"2026-01-02T03:04:05Z",
		"updated_at":"2026-01-02T03:04:05Z",
		"plan":{
			"current_version":"1.2.0",
			"next_version":"1.3.0",
			"release_type":"minor",
			"changeset_id":"cs-legacy",
			"changeset":{
				"id":"cs-legacy",
				"from_ref":"refs/tags/v1.2.0",
				"to_ref":"` + head + `",
				"commits":[
					{"hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","type":"feat","subject":"add customer feature","date":"2026-01-02T03:04:05Z"}
				]
			}
		}
	}`

	legacyPath := filepath.Join(runsPath(repoRoot), "run-legacy.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}

	repo := NewFileReleaseRunRepository()
	loaded, err := repo.LoadFromRepo(context.Background(), repoRoot, "run-legacy")
	if err != nil {
		t.Fatalf("load legacy release record: %v", err)
	}
	if got := loaded.RepoID(); got != "acme/widget" {
		t.Errorf("RepoID = %q, want acme/widget", got)
	}
	if got := loaded.BaseRef(); got != "refs/tags/v1.2.0" {
		t.Errorf("BaseRef = %q, want refs/tags/v1.2.0", got)
	}
	if got := string(loaded.HeadSHA()); got != head {
		t.Errorf("HeadSHA = %q, want recovered target %q", got, head)
	}
	if got := loaded.VersionCurrent().String(); got != "1.2.0" {
		t.Errorf("VersionCurrent = %q, want 1.2.0", got)
	}
	if got := loaded.VersionNext().String(); got != "1.3.0" {
		t.Errorf("VersionNext = %q, want 1.3.0", got)
	}
	if !loaded.HasChangeSet() || len(loaded.ChangeSet().Commits()) != 1 {
		t.Fatalf("legacy changeset was not recovered: %#v", loaded.ChangeSet())
	}
	if got := loaded.ChangeSet().Commits()[0].Subject(); got != "add customer feature" {
		t.Errorf("legacy changeset subject = %q", got)
	}
	if got := loaded.Commits(); len(got) != 1 || string(got[0]) != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Errorf("legacy commit SHAs = %v", got)
	}

	if err := repo.Save(context.Background(), loaded); err != nil {
		t.Fatalf("save migrated release record: %v", err)
	}
	migrated, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(migrated, &fields); err != nil {
		t.Fatalf("decode migrated record: %v", err)
	}
	if _, exists := fields["plan"]; exists {
		t.Error("migrated record still uses the retired nested plan schema")
	}
	for _, field := range []string{"repo_id", "base_ref", "head_sha", "commits", "changeset"} {
		if _, exists := fields[field]; !exists {
			t.Errorf("migrated record is missing current field %q", field)
		}
	}
}

func quoteJSON(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
