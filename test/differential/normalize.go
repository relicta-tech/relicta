// Package differential proves that relicta's *commands* behave identically whichever
// persistence.backend is configured.
//
// Two conformance suites already guard the ports — internal/domain/release/ports/conformance
// for release runs and internal/cgp/memory/conformance for governance memory. They prove the
// adapters agree on synthetic fixtures, and they have caught real drift. What they cannot see
// is a difference that only appears once a *command* runs: the ordering `relicta history`
// prints, a report that reaches its data through a different query, a status line assembled
// from two stores that disagree about which run is current. ADR-013 ties flipping the default
// backend to evidence, and the ports suites are not that evidence on their own.
//
// So this harness drives the built binary rather than the use cases. A test that called the
// application layer in-process would skip exactly the wiring layer this exists to check.
//
// The comparison rule is the one both conformance packages state: where behavior is
// undocumented, the file backend is the reference, because it is what every caller in the tree
// was written against. A backend that disagrees is wrong even where its answer is more
// defensible in isolation.
//
// This file holds the two pieces that make the comparison trustworthy — the normalizer that
// removes run-to-run variance, and the differ that reports what is left. Both are exercised by
// normalize_test.go, which feeds them transcripts that are deliberately different and asserts
// the difference survives. A differential harness whose normalizer quietly erased everything
// would pass forever while proving nothing, which is worse than having no harness at all.
package differential

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// normalizer rewrites the parts of a transcript that vary between two runs of the *same*
// backend, so that what survives is attributable to the backend itself.
//
// Every rule below is justified at its use site. The bar for adding one is that the value
// cannot be made stable — where it can be, the fixture makes it stable instead. That is why
// commit hashes are absent from this list: the fixture pins GIT_AUTHOR_DATE and
// GIT_COMMITTER_DATE and keeps the backend-specific config out of every commit, so the hashes
// really are identical across backends and a hash that differed would be a finding.
type normalizer struct {
	// paths are absolute directories that appear in output and differ per backend because
	// each backend needs its own repository. Longest first, so a parent never shadows a child.
	paths []string
	// dsn is the postgres connection string, which embeds a container-assigned port.
	dsn string
	// runIDs maps a run's hex to a stable ordinal. Ordinals rather than a single placeholder
	// so that identity and ordering survive: "history lists run 3 above run 2" is signal, and
	// collapsing every ID to <id> would throw it away.
	runIDs map[string]int
	// today anchors the bare-date rule, which is deliberately restricted to dates the clock
	// could have produced during this run.
	today time.Time
}

func newNormalizer(paths []string, dsn string) *normalizer {
	// Longest first: /tmp/x/repo must be replaced before /tmp/x, or the tail of the longer
	// path survives as a fragment and the two backends still look different.
	sorted := append([]string(nil), paths...)
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })
	return &normalizer{paths: sorted, dsn: dsn, runIDs: map[string]int{}, today: time.Now()}
}

var (
	// A full run ID as `relicta plan` reports it.
	reRunIDFull = regexp.MustCompile(`run-[0-9a-f]{16}`)
	// `relicta bump`, `approve` and `publish` echo the ID truncated to eight hex digits.
	// Matching 8..16 lets one rule cover both spellings.
	reRunIDAny = regexp.MustCompile(`run-[0-9a-f]{8,16}`)

	// RFC3339, as `status` and `publish` print it: 2026-08-18T11:43:20+02:00.
	reRFC3339 = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})`)
	// The report header's "Generated" line: 2026-08-18 09:38:50 UTC.
	reStampUTC = regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} UTC`)
	// The audit table's WHEN column, minute precision: 2026-08-18 11:38.
	reStampMinute = regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}`)

	// Elapsed times. Kept narrow — anchored on a digit and a known unit suffix at a word
	// boundary — so that version strings and commit subjects are untouched.
	reDuration = regexp.MustCompile(`\b\d+(?:\.\d+)?(?:ns|µs|us|ms|s|m|h)\b`)
)

// normalize applies every rule, most specific first.
func (n *normalizer) normalize(s string) string {
	// Repository path. Each backend gets its own directory because they cannot share one —
	// two backends writing the same .relicta/ would not be independent. The path leaks into
	// output directly, and indirectly through the plan hash (see run IDs below).
	for _, p := range n.paths {
		if p != "" {
			s = strings.ReplaceAll(s, p, "<repo>")
		}
	}

	// Postgres DSN. The container's host port is assigned at startup, so the DSN cannot be
	// fixed. Replaced whole rather than per-component so no credential fragment survives.
	if n.dsn != "" {
		s = strings.ReplaceAll(s, n.dsn, "<dsn>")
	}

	// Run IDs. The ID is a hash of the plan, and the plan includes the repository path, so
	// two backends necessarily produce different IDs for the same release. Assigning ordinals
	// in order of first appearance keeps the useful part: whether the same run is referred to
	// twice, and what order the readers list runs in.
	for _, id := range reRunIDFull.FindAllString(s, -1) {
		n.ordinalFor(id)
	}
	s = reRunIDAny.ReplaceAllStringFunc(s, func(m string) string {
		return fmt.Sprintf("run-<%d>", n.ordinalFor(m))
	})

	// Wall-clock timestamps. Every one of these is the time a command ran. relicta takes them
	// from the system clock with no injection point reachable from the CLI, so they cannot be
	// pinned the way the commit dates are.
	s = reRFC3339.ReplaceAllString(s, "<timestamp>")
	s = reStampUTC.ReplaceAllString(s, "<timestamp>")
	s = reStampMinute.ReplaceAllString(s, "<timestamp>")

	// Durations, for the same reason: they measure the run, not the backend. This rule runs
	// after the timestamp rules so it cannot bite into one of their matches.
	s = reDuration.ReplaceAllString(s, "<duration>")

	// Bare dates, but only ones this run could have produced. The changelog heading carries
	// the release date, which is "today"; the report's --period is a fixed range the fixture
	// chose and must stay visible, so a blanket YYYY-MM-DD rule would erase real content.
	// A day either side covers a run that straddles midnight.
	for d := -1; d <= 1; d++ {
		s = strings.ReplaceAll(s, n.today.AddDate(0, 0, d).Format("2006-01-02"), "<date>")
	}

	return s
}

// ordinalFor returns the stable ordinal for a run ID, matching the truncated eight-digit form
// back to the full ID it abbreviates.
func (n *normalizer) ordinalFor(id string) int {
	if ord, ok := n.runIDs[id]; ok {
		return ord
	}
	// A truncated ID abbreviates a full one we have usually already seen.
	for known, ord := range n.runIDs {
		if strings.HasPrefix(known, id) {
			n.runIDs[id] = ord
			return ord
		}
	}
	ord := 0
	for _, existing := range n.runIDs {
		if existing > ord {
			ord = existing
		}
	}
	ord++
	n.runIDs[id] = ord
	return ord
}

// diffTranscripts compares two normalized transcripts and renders what differs.
//
// Returns the empty string when they agree. The rendering is a unified diff rather than a
// "they differ" verdict, because the only useful failure message here is the one that shows
// which line of which command changed.
func diffTranscripts(refName, ref, gotName, got string) string {
	refLines := strings.Split(ref, "\n")
	gotLines := strings.Split(got, "\n")
	if ref == got {
		return ""
	}

	hunks := unifiedDiff(refLines, gotLines, 3)
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s (reference)\n+++ %s\n", refName, gotName)
	b.WriteString(hunks)
	return b.String()
}

// unifiedDiff renders an LCS-based diff with the given context. Small and self-contained
// because a transcript diff is the whole failure message and pulling in a dependency for it
// would be out of proportion.
func unifiedDiff(a, b []string, ctx int) string {
	lcs := longestCommonSubsequence(a, b)

	type edit struct {
		kind rune // ' ', '-', '+'
		text string
		aNum int
		bNum int
	}
	var edits []edit

	ai, bi := 0, 0
	for _, k := range lcs {
		for ai < len(a) && a[ai] != k {
			edits = append(edits, edit{'-', a[ai], ai + 1, 0})
			ai++
		}
		for bi < len(b) && b[bi] != k {
			edits = append(edits, edit{'+', b[bi], 0, bi + 1})
			bi++
		}
		edits = append(edits, edit{' ', k, ai + 1, bi + 1})
		ai++
		bi++
	}
	for ai < len(a) {
		edits = append(edits, edit{'-', a[ai], ai + 1, 0})
		ai++
	}
	for bi < len(b) {
		edits = append(edits, edit{'+', b[bi], 0, bi + 1})
		bi++
	}

	// Keep only changed lines plus ctx lines of context around them.
	keep := make([]bool, len(edits))
	for i, e := range edits {
		if e.kind == ' ' {
			continue
		}
		lo, hi := i-ctx, i+ctx
		if lo < 0 {
			lo = 0
		}
		if hi >= len(edits) {
			hi = len(edits) - 1
		}
		for j := lo; j <= hi; j++ {
			keep[j] = true
		}
	}

	var b2 strings.Builder
	gap := false
	for i, e := range edits {
		if !keep[i] {
			gap = true
			continue
		}
		if gap {
			b2.WriteString("@@\n")
			gap = false
		}
		b2.WriteString(string(e.kind))
		b2.WriteString(e.text)
		b2.WriteString("\n")
	}
	return b2.String()
}

func longestCommonSubsequence(a, b []string) []string {
	// Straight dynamic programming. Transcripts here are a few hundred lines, so the O(n*m)
	// table is cheap and the code stays obvious.
	n, m := len(a), len(b)
	table := make([][]int, n+1)
	for i := range table {
		table[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else if table[i+1][j] >= table[i][j+1] {
				table[i][j] = table[i+1][j]
			} else {
				table[i][j] = table[i][j+1]
			}
		}
	}
	var out []string
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			out = append(out, a[i])
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			i++
		default:
			j++
		}
	}
	return out
}
