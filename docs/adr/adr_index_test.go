// Package adr holds architecture decision records. This file contains no
// production code; it exists so the index cannot rot unnoticed.
package adr

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The ADR index linked to six documents that were never committed — ADR-001
// through ADR-006 — and carried a merge conflict marker
// ("||||||| parent of 96b6832") in the middle of the table. Both survived because
// nothing read this file except people, and a broken link looks like a link.
//
// ADR-009 argues that a governance tool's records must be verifiable. An index
// that points at files which do not exist is the smallest possible version of
// failing that, and it is the exact defect this project criticises elsewhere.

const indexFile = "README.md"

func readIndex(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(indexFile)
	if err != nil {
		t.Fatalf("read %s: %v", indexFile, err)
	}
	return string(data)
}

// TestIndexLinksResolve is the check that was missing: every ADR the index links
// to must exist on disk.
func TestIndexLinksResolve(t *testing.T) {
	linked := regexp.MustCompile(`\]\((\d{3}-[a-z0-9-]+\.md)\)`).FindAllStringSubmatch(readIndex(t), -1)
	if len(linked) == 0 {
		t.Fatal("index links to no ADR files; the table has probably been reshaped")
	}

	for _, m := range linked {
		if _, err := os.Stat(m[1]); err != nil {
			t.Errorf("index links to %s, which does not exist — either write it or list "+
				"it without a link, as ADR-001..006 are", m[1])
		}
	}
}

// TestEveryADRFileIsIndexed catches the other direction: a record written and
// never listed is a record nobody finds.
func TestEveryADRFileIsIndexed(t *testing.T) {
	index := readIndex(t)

	entries, err := filepath.Glob("[0-9][0-9][0-9]-*.md")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no ADR files found; this test is running in the wrong directory")
	}

	var unlisted []string
	for _, f := range entries {
		if !strings.Contains(index, "("+f+")") {
			unlisted = append(unlisted, f)
		}
	}
	sort.Strings(unlisted)
	if len(unlisted) > 0 {
		t.Errorf("these ADRs exist but are not linked from the index: %s",
			strings.Join(unlisted, ", "))
	}
}

// TestIndexHasNoConflictMarkers guards against the marker that was committed into
// the middle of the table. Documentation is not compiled, so nothing else would
// notice.
func TestIndexHasNoConflictMarkers(t *testing.T) {
	for i, line := range strings.Split(readIndex(t), "\n") {
		for _, marker := range []string{"<<<<<<<", "|||||||", ">>>>>>>"} {
			if strings.HasPrefix(line, marker) {
				t.Errorf("%s:%d contains a merge conflict marker: %q", indexFile, i+1, line)
			}
		}
	}
}

// TestListedADRsAreNumberedConsistently keeps the table readable: a row must name
// the number its link points at, so ADR-011 cannot link to 010.
func TestListedADRsAreNumberedConsistently(t *testing.T) {
	row := regexp.MustCompile(`\|\s*\[ADR-(\d{3})\]\((\d{3})-[a-z0-9-]+\.md\)`)
	for _, m := range row.FindAllStringSubmatch(readIndex(t), -1) {
		if m[1] != m[2] {
			t.Errorf("row labeled ADR-%s links to file %s-*", m[1], m[2])
		}
	}
}
