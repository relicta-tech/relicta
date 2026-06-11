package compliance

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/memory"
)

func TestArticle12_GenerateBuilds(t *testing.T) {
	store := memory.NewInMemoryStore()
	now := time.Now().UTC()

	rel := &memory.ReleaseRecord{
		ID:              "rel-1",
		Repository:      "acme/payments",
		Version:         "1.4.0",
		Actor:           cgp.Actor{Kind: cgp.ActorKindAgent, ID: "claude-code-1"},
		RiskScore:       0.42,
		Decision:        cgp.DecisionApproved,
		BreakingChanges: 1,
		SecurityChanges: 0,
		FilesChanged:    12,
		LinesChanged:    340,
		Outcome:         memory.OutcomeSuccess,
		ReleasedAt:      now.Add(-1 * time.Hour),
		Duration:        45 * time.Minute,
	}
	if err := store.RecordRelease(context.Background(), rel); err != nil {
		t.Fatalf("record release: %v", err)
	}

	cfg := ReportConfig{
		Type:       ReportEUAIActArticle12,
		Format:     FormatMarkdown,
		Period:     Period{Start: now.Add(-24 * time.Hour), End: now, Label: "test-window"},
		Repository: "acme/payments",
	}

	gen := NewGenerator(store, nil)
	report, err := gen.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if report.Article12 == nil {
		t.Fatal("expected Article12 report present")
	}
	if got := report.Article12.SystemIdentifier; got != "acme/payments" {
		t.Errorf("systemIdentifier: got %q", got)
	}
	if !report.Article12.AuditChainIntegrityVerified {
		t.Errorf("expected chain integrity verified by default")
	}
	if !report.Article12.RetentionDeadline.After(cfg.Period.End) {
		t.Errorf("retention deadline must be after period end")
	}
	// Article 26(6): minimum six months retention. Calendar months vary 28-31 days,
	// so accept anything in the [175, 190] day window.
	if got := report.Article12.RetentionDeadline.Sub(cfg.Period.End).Hours() / 24; got < 175 || got > 190 {
		t.Errorf("retention deadline should be ~6 months past end, got %.0f days", got)
	}
	if len(report.Article12.LogEntries) == 0 {
		t.Errorf("expected at least one log entry from the release record")
	}

	entry := report.Article12.LogEntries[0]
	if entry.Version != "1.4.0" {
		t.Errorf("entry version: got %q", entry.Version)
	}
	if entry.Actor.Kind != cgp.ActorKindAgent {
		t.Errorf("entry actor kind: got %q", entry.Actor.Kind)
	}
	if entry.RiskScore != 0.42 {
		t.Errorf("entry risk: got %v", entry.RiskScore)
	}
	if entry.InputData.FilesChanged != 12 {
		t.Errorf("entry files changed: got %d", entry.InputData.FilesChanged)
	}
	if entry.StartedAt.Equal(entry.EndedAt) {
		t.Errorf("expected started_at != ended_at when duration is known")
	}
}

func TestArticle12_FromDecision(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()

	rel := &memory.ReleaseRecord{
		ID:           "rel-d1",
		Repository:   "acme/decisions",
		Version:      "2.0.0",
		Actor:        cgp.Actor{Kind: cgp.ActorKindHuman, ID: "alice@example.com"},
		RiskScore:    0.55,
		Decision:     cgp.DecisionApprovalRequired,
		FilesChanged: 5,
		Outcome:      memory.OutcomeSuccess,
		ReleasedAt:   now.Add(-30 * time.Minute),
		Duration:     20 * time.Minute,
	}
	if err := store.RecordRelease(ctx, rel); err != nil {
		t.Fatalf("record release: %v", err)
	}

	// Record a governance decision tied to the release.
	dec := &cgp.GovernanceDecision{
		CGPVersion: "0.1",
		ID:         "dec-1",
		ProposalID: rel.ID,
		Timestamp:  rel.ReleasedAt,
		Decision:   cgp.DecisionApprovalRequired,
		RiskScore:  rel.RiskScore,
		Rationale:  []string{"breaking change requires manual review"},
	}
	if err := store.RecordDecision(ctx, dec); err != nil {
		t.Fatalf("record decision: %v", err)
	}

	cfg := ReportConfig{
		Type:       ReportEUAIActArticle12,
		Format:     FormatJSONL,
		Period:     Period{Start: now.Add(-2 * time.Hour), End: now, Label: "test"},
		Repository: rel.Repository,
	}
	gen := NewGenerator(store, nil)
	report, err := gen.Generate(ctx, cfg)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if len(report.Article12.LogEntries) == 0 {
		t.Fatal("expected log entries from decision")
	}

	// Find the decision-derived entry (prefix art12: not art12:rel:).
	var decEntry *Article12LogEntry
	for i := range report.Article12.LogEntries {
		e := &report.Article12.LogEntries[i]
		if e.EntryID == "art12:dec-1" {
			decEntry = e
			break
		}
	}
	if decEntry == nil {
		t.Fatal("expected decision-derived entry")
	}
	if len(decEntry.OutputRationale) == 0 {
		t.Errorf("expected rationale to flow through")
	}
	if decEntry.Version != "2.0.0" {
		t.Errorf("expected version refined from release record")
	}
	if decEntry.InputData.FilesChanged != 5 {
		t.Errorf("expected files-changed refined from release record")
	}

	// JSONL output should round-trip.
	out, err := gen.Render(report, FormatJSONL)
	if err != nil {
		t.Fatalf("render jsonl: %v", err)
	}
	if !strings.Contains(out, "art12:dec-1") {
		t.Errorf("decision entry id not in jsonl output")
	}
}

func TestArticle12_EmptyPeriod(t *testing.T) {
	store := memory.NewInMemoryStore()
	now := time.Now().UTC()

	cfg := ReportConfig{
		Type:       ReportEUAIActArticle12,
		Format:     FormatMarkdown,
		Period:     Period{Start: now.Add(-24 * time.Hour), End: now, Label: "empty"},
		Repository: "acme/empty",
	}
	gen := NewGenerator(store, nil)
	report, err := gen.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if report.Article12 == nil {
		t.Fatal("expected Article12 report present even when empty")
	}
	if len(report.Article12.LogEntries) != 0 {
		t.Errorf("expected no entries; got %d", len(report.Article12.LogEntries))
	}
	if len(report.Article12.GenerationNotes) == 0 {
		t.Errorf("expected a generation note for empty periods")
	}
}

func TestRenderJSONL_OnePerLine(t *testing.T) {
	r := &Article12Report{
		LogEntries: []Article12LogEntry{
			{EntryID: "a", Version: "1.0.0"},
			{EntryID: "b", Version: "1.0.1"},
		},
	}
	out, err := RenderJSONL(r)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	for i, line := range lines {
		var e Article12LogEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Errorf("line %d not valid JSON: %v", i, err)
		}
	}
}

func TestRenderJSONL_NilReport(t *testing.T) {
	if _, err := RenderJSONL(nil); err == nil {
		t.Error("expected error on nil report")
	}
}

func TestRenderCSV_HeaderAndRow(t *testing.T) {
	r := &Article12Report{
		LogEntries: []Article12LogEntry{
			{
				EntryID:        "x,y",
				EventTimestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
				Actor:          cgp.Actor{Kind: cgp.ActorKindHuman, ID: "alice@example.com"},
				OutputDecision: "approved",
				RiskScore:      0.31,
				InputData:      InputData{Repository: "acme/widgets", FilesChanged: 4},
			},
		},
	}
	out, err := RenderCSV(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "entry_id,") {
		t.Errorf("expected csv header first; got %q", out[:60])
	}
	if !strings.Contains(out, `"x,y"`) {
		t.Errorf("expected commas in entry_id to be quoted")
	}
	if !strings.Contains(out, "human") {
		t.Errorf("expected actor kind in row")
	}
}

func TestRenderCSV_NilReport(t *testing.T) {
	if _, err := RenderCSV(nil); err == nil {
		t.Error("expected error on nil report")
	}
}

func TestGenerator_Render_JSONLRequiresArticle12(t *testing.T) {
	gen := NewGenerator(memory.NewInMemoryStore(), nil)
	report := &Report{
		Type:   ReportSOC2,
		Period: Period{Start: time.Now(), End: time.Now().Add(time.Hour), Label: "x"},
		SOC2:   &SOC2Report{},
	}
	if _, err := gen.Render(report, FormatJSONL); err == nil {
		t.Error("expected error: jsonl format requires article12 report")
	}
	if _, err := gen.Render(report, FormatCSV); err == nil {
		t.Error("expected error: csv format requires article12 report")
	}
}

func TestRenderMarkdown_Article12(t *testing.T) {
	report := &Report{
		Type:        ReportEUAIActArticle12,
		Period:      Period{Start: time.Now(), End: time.Now().Add(time.Hour), Label: "x"},
		GeneratedAt: time.Now().UTC(),
		Article12: &Article12Report{
			SystemIdentifier:            "acme/payments",
			AuditChainIntegrityVerified: true,
			RetentionDeadline:           time.Now().AddDate(0, 6, 0),
			LogEntries: []Article12LogEntry{
				{
					EntryID:        "art12:1",
					Actor:          cgp.Actor{Kind: cgp.ActorKindAgent, ID: "claude-code-1"},
					OutputDecision: "approved",
					RiskScore:      0.4,
					Verifiers:      []Verifier{{Kind: "human", ID: "alice@example.com"}},
				},
			},
		},
	}

	out := RenderMarkdown(report)
	if !strings.Contains(out, "EU AI Act — Article 12 Record-Keeping") {
		t.Errorf("missing Article 12 heading")
	}
	if !strings.Contains(out, "claude-code-1") {
		t.Errorf("missing actor in markdown")
	}
	if !strings.Contains(out, "Audit Chain Integrity:** ✓ verified") {
		t.Errorf("missing chain integrity badge")
	}
	if !strings.Contains(out, "alice@example.com") {
		t.Errorf("missing verifier in markdown")
	}
}

func TestReportFormat_IsValid_NewFormats(t *testing.T) {
	for _, f := range []ReportFormat{FormatMarkdown, FormatJSON, FormatJSONL, FormatCSV} {
		if !f.IsValid() {
			t.Errorf("%q should be valid", f)
		}
	}
	if ReportFormat("xml").IsValid() {
		t.Errorf("xml should not be valid")
	}
}

func TestReportType_IsValid_Article12(t *testing.T) {
	if !ReportEUAIActArticle12.IsValid() {
		t.Errorf("eu-ai-act-article-12 should be a valid report type")
	}
}
