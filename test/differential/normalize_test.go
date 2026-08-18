package differential

// normalize_test.go proves the harness can fail.
//
// This is the load-bearing test in the package. A differential harness is only worth the
// runtime it costs if its normalizer is narrower than the differences it is meant to catch —
// normalize hard enough and every backend produces the empty string, the comparison passes
// forever, and the evidence it was built to provide is fabricated. This codebase has already
// shipped two revert-checks that silently proved nothing, so the harness is asked to
// demonstrate the opposite here: fed two transcripts that differ in a way that would matter,
// it reports the difference, and the report names the actual lines.

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

// A transcript in the shape the harness produces, with every kind of varying token in it.
func sampleTranscript(repo, runID, stamp, version, hash string) string {
	return "$ relicta plan\n[exit 0]\n" +
		"  Repository:       repo\n" +
		"  Working dir:      " + repo + "\n" +
		"  Next version:     " + version + "\n" +
		"  " + hash + " add alpha\n" +
		"Release plan saved with ID: run-" + runID + "\n" +
		"=====\n" +
		"$ relicta status\n[exit 0]\n" +
		"  Release ID: run-" + runID + "\n" +
		"  Short ID:   run-" + runID[:8] + "\n" +
		"Created: " + stamp + "\n" +
		"=====\n"
}

const (
	// Two runs of the same backend differ in exactly these: the directory, the plan hash the
	// run ID derives from, and the clock.
	repoA = "/tmp/aaaa/repo"
	repoB = "/tmp/bbbb/repo"
	// Deliberately low-entropy hex. A realistic-looking run ID is indistinguishable from a
	// leaked token to a secret scanner, and these only need the right shape — sixteen hex
	// digits — for the run-ID rules to match them.
	runA = "aaaaaaaabbbbbbbb"
	runB = "ccccccccdddddddd"
)

func normalizedA(t *testing.T, version, hash string) string {
	t.Helper()
	return newNormalizer([]string{repoA}, "").
		normalize(sampleTranscript(repoA, runA, time.Now().Format(time.RFC3339), version, hash))
}

func normalizedB(t *testing.T, version, hash string) string {
	t.Helper()
	return newNormalizer([]string{repoB}, "").
		normalize(sampleTranscript(repoB, runB, time.Now().Add(90*time.Second).Format(time.RFC3339), version, hash))
}

// TestTheNormalizerAbsorbsWhatCannotBeMadeStable is the precondition for everything below: if
// this failed, every comparison would report a difference and the harness would be noise.
func TestTheNormalizerAbsorbsWhatCannotBeMadeStable(t *testing.T) {
	a := normalizedA(t, "0.1.0", "3322237")
	b := normalizedB(t, "0.1.0", "3322237")

	if d := diffTranscripts("a", a, "b", b); d != "" {
		t.Errorf("two runs differing only in path, run ID and clock should normalize to the "+
			"same text, but they did not:\n%s", d)
	}
	for _, leaked := range []string{repoA, repoB, runA, runB} {
		if strings.Contains(a+b, leaked) {
			t.Errorf("%q survived normalization; it varies per run and would make every "+
				"comparison fail", leaked)
		}
	}
}

// TestTheHarnessReportsADifferenceThatMatters is the proof of failure.
//
// Each case is a difference a real backend divergence would look like. The normalizer must not
// launder any of them away, and the diff must name the offending content — "outputs differ" is
// not a usable failure message when the transcript is hundreds of lines long.
func TestTheHarnessReportsADifferenceThatMatters(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		// mustMention is content the rendered diff has to contain, so that the failure
		// message points at the divergence rather than merely announcing one.
		mustMention []string
	}{
		{
			name:        "a different version",
			a:           normalizedA(t, "0.1.0", "3322237"),
			b:           normalizedB(t, "0.2.0", "3322237"),
			mustMention: []string{"0.1.0", "0.2.0"},
		},
		{
			// Commit hashes are stable by construction — the fixture pins the commit dates
			// and keeps backend-specific content out of every commit — so a differing hash
			// is a real finding and must not be normalized.
			name:        "a different commit hash",
			a:           normalizedA(t, "0.1.0", "3322237"),
			b:           normalizedB(t, "0.1.0", "9999999"),
			mustMention: []string{"3322237", "9999999"},
		},
		{
			// The ordering `relicta history` prints is the difference the ports conformance
			// suites cannot see, and the whole reason this harness drives commands.
			name:        "a different ordering",
			a:           "$ relicta history\n[exit 0]\n✓ 0.1.1\n✓ 0.1.0\n=====\n",
			b:           "$ relicta history\n[exit 0]\n✓ 0.1.0\n✓ 0.1.1\n=====\n",
			mustMention: []string{"0.1.1", "0.1.0"},
		},
		{
			name:        "a different exit code",
			a:           "$ relicta audit\n[exit 0]\nGovernance Record\n=====\n",
			b:           "$ relicta audit\n[exit 1]\nGovernance Record\n=====\n",
			mustMention: []string{"[exit 0]", "[exit 1]"},
		},
		{
			name:        "a line only one backend prints",
			a:           "$ relicta status\n[exit 0]\nState: published\n=====\n",
			b:           "$ relicta status\n[exit 0]\nState: published\n⚠ stale run\n=====\n",
			mustMention: []string{"⚠ stale run"},
		},
		{
			name:        "a missing release in the history",
			a:           "$ relicta history\n[exit 0]\n✓ 0.1.1\n✓ 0.1.0\nSummary: 2 releases\n=====\n",
			b:           "$ relicta history\n[exit 0]\n✓ 0.1.0\nSummary: 1 releases\n=====\n",
			mustMention: []string{"0.1.1", "2 releases", "1 releases"},
		},
		{
			// Two runs the reader conflated into one would show up as the same ordinal twice.
			// Ordinals rather than a single <id> placeholder are what preserve this.
			name:        "two distinct runs collapsed into one",
			a:           newNormalizer(nil, "").normalize("run-1111111111111111 run-2222222222222222\n"),
			b:           newNormalizer(nil, "").normalize("run-1111111111111111 run-1111111111111111\n"),
			mustMention: []string{"run-<1>", "run-<2>"},
		},
		{
			name:        "a different changelog body",
			a:           "# file CHANGELOG.md\n### Features\n\n- add alpha (3322237)\n=====\n",
			b:           "# file CHANGELOG.md\n### Bug Fixes\n\n- add alpha (3322237)\n=====\n",
			mustMention: []string{"### Features", "### Bug Fixes"},
		},
		{
			name:        "a different tag set",
			a:           "$ git tag --sort=refname\n[exit 0]\nv0.1.0\nv0.1.1\n=====\n",
			b:           "$ git tag --sort=refname\n[exit 0]\nv0.1.0\n=====\n",
			mustMention: []string{"v0.1.1"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := diffTranscripts("reference", c.a, "candidate", c.b)
			if d == "" {
				t.Fatalf("the harness reported no difference between two transcripts that "+
					"differ by %s; it would pass for a backend that really behaved this "+
					"way\nreference:\n%s\ncandidate:\n%s", c.name, c.a, c.b)
			}
			for _, want := range c.mustMention {
				if !strings.Contains(d, want) {
					t.Errorf("the diff does not mention %q, so the failure message would not "+
						"say what actually differs:\n%s", want, d)
				}
			}
		})
	}
}

// TestIdenticalTranscriptsCompareEqual guards the other direction: a differ that always
// reported something would make every backend look broken and get switched off.
func TestIdenticalTranscriptsCompareEqual(t *testing.T) {
	tr := sampleTranscript(repoA, runA, time.Now().Format(time.RFC3339), "0.1.0", "3322237")
	if d := diffTranscripts("a", tr, "b", tr); d != "" {
		t.Errorf("identical transcripts were reported as differing:\n%s", d)
	}
}

// TestTheFixedReportPeriodSurvivesNormalization pins the one place the date rule is deliberately
// narrow. The changelog heading carries today's date and has to go; the report's --period is a
// fixed range the fixture chose, and a blanket YYYY-MM-DD rule would erase it along with any
// evidence that a backend reported the wrong window.
func TestTheFixedReportPeriodSurvivesNormalization(t *testing.T) {
	in := "**Period:** 2020-01-01 to 2099-12-31\n**Generated:** " +
		time.Now().UTC().Format("2006-01-02 15:04:05") + " UTC\n"

	got := newNormalizer(nil, "").normalize(in)

	if !strings.Contains(got, "2020-01-01 to 2099-12-31") {
		t.Errorf("the fixed report period was normalized away; a backend reporting a "+
			"different window would no longer be caught:\n%s", got)
	}
	if strings.Contains(got, "UTC\n") && !strings.Contains(got, "<timestamp>") {
		t.Errorf("the generated-at stamp was not normalized, so every run would differ:\n%s", got)
	}
}

// TestTheDSNNeverSurvivesNormalization matters for more than tidiness: the postgres DSN embeds
// a container-assigned port, so leaving it in would make postgres differ from file on every
// run and the harness would be permanently red.
func TestTheDSNNeverSurvivesNormalization(t *testing.T) {
	// Assembled rather than written out, for the same reason startPostgres assembles the real
	// one: a credential-bearing connection string literal in the tree is a security finding
	// whether or not the credentials are fake.
	dsn := (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword("relicta", "relicta"),
		Host:     "127.0.0.1:49173",
		Path:     "/relicta_differential",
		RawQuery: "sslmode=disable",
	}).String()

	got := newNormalizer(nil, dsn).normalize("connecting to " + dsn + "\n")

	if strings.Contains(got, "49173") || strings.Contains(got, "relicta_differential") {
		t.Errorf("the DSN survived normalization: %s", got)
	}
	if !strings.Contains(got, "<dsn>") {
		t.Errorf("the DSN was removed rather than replaced with a placeholder; a line that "+
			"vanishes cannot be compared:\n%s", got)
	}
}
