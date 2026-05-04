package compliance

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/memory"
)

func TestAnnexIV_GenerateBuilds(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()

	rel := &memory.ReleaseRecord{
		ID:              "rel-iv-1",
		Repository:      "acme/gov",
		Version:         "3.1.0",
		Actor:           cgp.Actor{Kind: cgp.ActorKindAgent, ID: "claude-code-1"},
		RiskScore:       0.62,
		Decision:        cgp.DecisionApprovalRequired,
		BreakingChanges: 2,
		SecurityChanges: 1,
		FilesChanged:    18,
		Outcome:         memory.OutcomeSuccess,
		ReleasedAt:      now.Add(-2 * time.Hour),
		Duration:        90 * time.Minute,
	}
	if err := store.RecordRelease(ctx, rel); err != nil {
		t.Fatalf("record release: %v", err)
	}

	dec := &cgp.GovernanceDecision{
		CGPVersion: "0.1",
		ID:         "dec-iv-1",
		ProposalID: rel.ID,
		Timestamp:  rel.ReleasedAt,
		Decision:   cgp.DecisionApprovalRequired,
		RiskScore:  rel.RiskScore,
		Rationale:  []string{"breaking changes detected", "security touch flagged"},
	}
	if err := store.RecordDecision(ctx, dec); err != nil {
		t.Fatalf("record decision: %v", err)
	}

	cfg := ReportConfig{
		Type:       ReportEUAIActAnnexIV,
		Format:     FormatMarkdown,
		Period:     Period{Start: now.Add(-24 * time.Hour), End: now, Label: "test"},
		Repository: "acme/gov",
	}

	gen := NewGenerator(store, nil)
	report, err := gen.Generate(ctx, cfg)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if report.AnnexIV == nil {
		t.Fatal("expected AnnexIV report")
	}
	r := report.AnnexIV

	// Identity
	if r.SystemIdentifier != "acme/gov" {
		t.Errorf("system id: got %q", r.SystemIdentifier)
	}
	if r.SystemVersion != "3.1.0" {
		t.Errorf("system version: got %q", r.SystemVersion)
	}

	// Retention 10y
	years := r.RetentionDeadline.Year() - cfg.Period.End.Year()
	if years < 9 || years > 11 {
		t.Errorf("retention should be ~10 years past end, got %d years diff", years)
	}

	// §1
	if r.GeneralDescription.IntendedPurpose == "" {
		t.Errorf("§1 IntendedPurpose empty")
	}
	if len(r.GeneralDescription.HardwareEnvironments) == 0 {
		t.Errorf("§1 hardware environments empty")
	}

	// §2
	if r.DetailedDescription.SystemArchitecture == "" {
		t.Errorf("§2 system architecture empty")
	}
	if len(r.DetailedDescription.HumanOversightMeasures) == 0 {
		t.Errorf("§2 human oversight measures empty")
	}
	if r.DetailedDescription.CGPProtocolVersion != "0.1" {
		t.Errorf("§2 CGP version: got %q", r.DetailedDescription.CGPProtocolVersion)
	}

	// §3
	if r.MonitoringControl.AuditChainAlgorithm == "" {
		t.Errorf("§3 audit chain algorithm empty")
	}

	// §4 — should have at least one risk identified from rationale parsing
	if r.RiskManagement.RiskEvaluationMethod == "" {
		t.Errorf("§4 evaluation method empty")
	}
	if len(r.RiskManagement.RiskMitigationControls) == 0 {
		t.Errorf("§4 mitigation controls empty")
	}
	if len(r.RiskManagement.IdentifiedRisks) == 0 {
		t.Errorf("§4 should have parsed at least one risk from decision rationale")
	}

	// §5
	if len(r.LifecycleChanges) != 1 {
		t.Errorf("§5: expected 1 lifecycle change, got %d", len(r.LifecycleChanges))
	}
	if r.LifecycleChanges[0].ChangeType != "breaking" {
		t.Errorf("§5: expected 'breaking' change type, got %q", r.LifecycleChanges[0].ChangeType)
	}

	// §6
	if len(r.HarmonizedStandards) == 0 {
		t.Errorf("§6 standards empty")
	}

	// §7 — scaffold with TODOs
	if r.ConformityDeclaration.SystemIdentifier == "" {
		t.Errorf("§7 system identifier should be filled")
	}
	if r.ConformityDeclaration.ProviderName != "" {
		t.Errorf("§7 provider name should be left blank for human completion")
	}

	// §8
	if r.PostMarketMonitoring.MonitoringPlan == "" {
		t.Errorf("§8 monitoring plan empty")
	}
}

func TestAnnexIV_EmptyPeriod(t *testing.T) {
	store := memory.NewInMemoryStore()
	now := time.Now().UTC()
	cfg := ReportConfig{
		Type:       ReportEUAIActAnnexIV,
		Format:     FormatMarkdown,
		Period:     Period{Start: now.Add(-24 * time.Hour), End: now, Label: "empty"},
		Repository: "acme/empty",
	}
	gen := NewGenerator(store, nil)
	report, err := gen.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if report.AnnexIV == nil {
		t.Fatal("expected AnnexIV present")
	}
	if len(report.AnnexIV.LifecycleChanges) != 0 {
		t.Errorf("§5: expected empty lifecycle list")
	}
	if len(report.AnnexIV.GenerationNotes) == 0 {
		t.Errorf("expected generation note for empty period")
	}
}

func TestAnnexIV_DefaultSystemIdentifier(t *testing.T) {
	store := memory.NewInMemoryStore()
	now := time.Now().UTC()
	cfg := ReportConfig{
		Type:   ReportEUAIActAnnexIV,
		Format: FormatMarkdown,
		Period: Period{Start: now.Add(-24 * time.Hour), End: now, Label: "default"},
		// Repository intentionally blank.
	}
	gen := NewGenerator(store, nil)
	report, _ := gen.Generate(context.Background(), cfg)
	if report.AnnexIV.SystemIdentifier != "default" {
		t.Errorf("expected default system identifier; got %q", report.AnnexIV.SystemIdentifier)
	}
}

func TestRenderMarkdown_AnnexIVAllSections(t *testing.T) {
	now := time.Now().UTC()
	resolved := now.Add(2 * time.Hour)
	report := &Report{
		Type:        ReportEUAIActAnnexIV,
		Period:      Period{Start: now.Add(-24 * time.Hour), End: now, Label: "test"},
		GeneratedAt: now,
		AnnexIV: &AnnexIVReport{
			SystemIdentifier:  "acme/gov",
			SystemVersion:     "1.0.0",
			RetentionDeadline: now.AddDate(10, 0, 0),
			GeneralDescription: GeneralDescription{
				IntendedPurpose:      "Release governance.",
				HardwareEnvironments: []string{"Linux x86_64"},
				DeploymentForms:      []string{"single binary"},
				UserInterfaces:       []string{"CLI"},
			},
			DetailedDescription: DetailedDescription{
				DevelopmentMethods: []string{"DDD"},
				SystemArchitecture: "hexagonal",
				HumanOversightMeasures: []string{"approval gate"},
				CGPProtocolVersion: "0.1",
			},
			MonitoringControl: MonitoringControl{
				MonitoringMechanisms: []string{"Prometheus"},
				ControlInterfaces:    []string{"relicta cancel"},
				AuditTrailLocation:   ".relicta/memory/",
				AuditChainAlgorithm:  "SHA-256",
			},
			RiskManagement: RiskManagement{
				IdentifiedRisks: []IdentifiedRisk{
					{Category: "breaking", Severity: "high", OccurrenceCount: 2, AverageScore: 0.7},
				},
				RiskEvaluationMethod: "8-factor calculator",
				RiskMitigationControls: []MitigationControl{
					{Name: "Approval Gate", Type: "policy", Description: "human approval"},
				},
				ResidualRiskRationale: "best-effort sandbox",
			},
			LifecycleChanges: []LifecycleChange{
				{Timestamp: now, Version: "1.0.0", ChangeType: "feature/fix", Decision: "approved", Actor: "human:alice", Outcome: "success", RiskScore: 0.2},
			},
			HarmonizedStandards: []HarmonizedStandard{
				{Framework: "ISO/IEC 42001", Version: "2023", Status: "applied", Controls: []string{"AI mgmt"}},
			},
			ConformityDeclaration: ConformityScaffold{
				SystemIdentifier:  "acme/gov",
				StandardsApplied:  []string{"ISO/IEC 42001:2023"},
				DateOfDeclaration: "2026-05-02",
			},
			PostMarketMonitoring: PostMarketMonitoring{
				MonitoringPlan:  "continuous",
				TotalIncidents:  1,
				ChangeFailureRate: 0.05,
				IncidentRecords: []IncidentSummary8{
					{IncidentID: "inc-1", ReleaseID: "rel-1", Version: "1.0.0", Type: "regression", Severity: "high", DetectedAt: now, ResolvedAt: &resolved, TimeToResolve: "2h"},
				},
			},
		},
	}

	out := RenderMarkdown(report)

	for _, marker := range []string{
		"§1 — General Description",
		"§2 — Detailed Description of System Elements",
		"§3 — Monitoring, Functioning, and Control",
		"§4 — Risk Management System",
		"§5 — Lifecycle Changes",
		"§6 — Harmonized Standards Applied",
		"§7 — EU Declaration of Conformity (Scaffold)",
		"§8 — Post-Market Monitoring",
		"TODO — provider must complete",
		"hexagonal",
		"SHA-256",
		"ISO/IEC 42001",
		"inc-1",
	} {
		if !strings.Contains(out, marker) {
			t.Errorf("missing %q in markdown output", marker)
		}
	}
}

func TestSeverityFromScore(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{0.9, "critical"},
		{0.7, "high"},
		{0.5, "medium"},
		{0.2, "low"},
	}
	for _, c := range cases {
		if got := severityFromScore(c.score); got != c.want {
			t.Errorf("score %.2f: got %q, want %q", c.score, got, c.want)
		}
	}
}

func TestFirstWord(t *testing.T) {
	cases := map[string]string{
		"breaking changes detected": "breaking",
		"  TRIMMED  next":           "trimmed",
		"":                          "",
		"single":                    "single",
		"with: colon":               "with",
		"comma,split":               "comma",
	}
	for in, want := range cases {
		if got := firstWord(in); got != want {
			t.Errorf("firstWord(%q): got %q, want %q", in, got, want)
		}
	}
}

func TestEmptyOrTODO(t *testing.T) {
	if emptyOrTODO("") == "" {
		t.Error("expected TODO marker for empty string")
	}
	if got := emptyOrTODO("Acme Inc"); got != "Acme Inc" {
		t.Errorf("non-empty value should pass through; got %q", got)
	}
}

func TestReportType_IsValid_AnnexIV(t *testing.T) {
	if !ReportEUAIActAnnexIV.IsValid() {
		t.Error("eu-ai-act-annex-iv should be valid")
	}
}
