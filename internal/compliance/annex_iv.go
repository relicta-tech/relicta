package compliance

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp/memory"
)

// AnnexIVReport is the EU AI Act Annex IV technical documentation bundle.
//
// Annex IV (Regulation 2024/1689) requires high-risk AI systems to maintain
// technical documentation across eight sections, retained for ten years.
// Article 12 covers logs; Annex IV covers the *system documentation* about
// what the AI system does, how it was built, how it is monitored, and how
// risks are managed.
//
// Microsoft Agent Governance Toolkit (April 2026) covers Article 12 mapping
// but does NOT generate Annex IV documentation. This is Relicta's wedge.
//
// The generator fills sections it can derive from CGP data; sections requiring
// human signatures or external standards (Sec 7 conformity declaration) are
// emitted as scaffolded placeholders for legal/compliance team completion.
type AnnexIVReport struct {
	// SystemIdentifier names the AI system documented.
	SystemIdentifier string `json:"systemIdentifier"`

	// SystemVersion is the latest version observed in the period.
	SystemVersion string `json:"systemVersion,omitempty"`

	// GeneralDescription is Annex IV §1.
	GeneralDescription GeneralDescription `json:"generalDescription"`

	// DetailedDescription is Annex IV §2.
	DetailedDescription DetailedDescription `json:"detailedDescription"`

	// MonitoringControl is Annex IV §3.
	MonitoringControl MonitoringControl `json:"monitoringControl"`

	// RiskManagement is Annex IV §4.
	RiskManagement RiskManagement `json:"riskManagement"`

	// LifecycleChanges is Annex IV §5.
	LifecycleChanges []LifecycleChange `json:"lifecycleChanges"`

	// HarmonizedStandards is Annex IV §6.
	HarmonizedStandards []HarmonizedStandard `json:"harmonizedStandards"`

	// ConformityDeclaration is Annex IV §7. Scaffold only; legal review required.
	ConformityDeclaration ConformityScaffold `json:"conformityDeclaration"`

	// PostMarketMonitoring is Annex IV §8.
	PostMarketMonitoring PostMarketMonitoring `json:"postMarketMonitoring"`

	// RetentionDeadline is the earliest permissible purge date (period.End +
	// 10 years per Article 11).
	RetentionDeadline time.Time `json:"retentionDeadline"`

	// GenerationNotes captures caveats from the documentation pass.
	GenerationNotes []string `json:"generationNotes,omitempty"`
}

// GeneralDescription is Annex IV §1 — what the AI system is and does.
type GeneralDescription struct {
	IntendedPurpose      string   `json:"intendedPurpose"`
	ProviderName         string   `json:"providerName,omitempty"`
	SystemVersion        string   `json:"systemVersion,omitempty"`
	HardwareEnvironments []string `json:"hardwareEnvironments,omitempty"`
	DeploymentForms      []string `json:"deploymentForms,omitempty"`
	UserInterfaces       []string `json:"userInterfaces,omitempty"`
	Languages            []string `json:"languages,omitempty"`
}

// DetailedDescription is Annex IV §2 — how the system is built.
type DetailedDescription struct {
	DevelopmentMethods   []string `json:"developmentMethods"`
	DesignSpecifications []string `json:"designSpecifications"`
	SystemArchitecture   string   `json:"systemArchitecture"`
	ComputationalResources string `json:"computationalResources,omitempty"`
	DataRequirements     []string `json:"dataRequirements,omitempty"`
	HumanOversightMeasures []string `json:"humanOversightMeasures"`
	PredeterminedChanges []string `json:"predeterminedChanges,omitempty"`
	CGPProtocolVersion   string   `json:"cgpProtocolVersion"`
	RiskModelVersion     string   `json:"riskModelVersion,omitempty"`
}

// MonitoringControl is Annex IV §3 — how the system is monitored and controlled.
type MonitoringControl struct {
	MonitoringMechanisms []string `json:"monitoringMechanisms"`
	ControlInterfaces    []string `json:"controlInterfaces"`
	AuditTrailLocation   string   `json:"auditTrailLocation"`
	AuditChainAlgorithm  string   `json:"auditChainAlgorithm"`
}

// RiskManagement is Annex IV §4 — how risks are identified, evaluated, mitigated.
type RiskManagement struct {
	IdentifiedRisks       []IdentifiedRisk      `json:"identifiedRisks"`
	RiskEvaluationMethod  string                `json:"riskEvaluationMethod"`
	RiskMitigationControls []MitigationControl  `json:"riskMitigationControls"`
	ResidualRiskRationale string                `json:"residualRiskRationale,omitempty"`
}

// IdentifiedRisk records a risk category recognized by the system.
type IdentifiedRisk struct {
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Severity    string  `json:"severity"`
	OccurrenceCount int `json:"occurrenceCount"`
	AverageScore float64 `json:"averageScore"`
}

// MitigationControl describes a control applied to mitigate risk.
type MitigationControl struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"` // "policy", "technical", "process"
}

// LifecycleChange is a single change recorded against the system over time.
type LifecycleChange struct {
	Timestamp   time.Time `json:"timestamp"`
	Version     string    `json:"version"`
	ChangeType  string    `json:"changeType"`
	Decision    string    `json:"decision"`
	Actor       string    `json:"actor"`
	Outcome     string    `json:"outcome"`
	RiskScore   float64   `json:"riskScore"`
}

// HarmonizedStandard records compliance with an external framework or standard.
type HarmonizedStandard struct {
	Framework string   `json:"framework"`
	Version   string   `json:"version,omitempty"`
	Controls  []string `json:"controls,omitempty"`
	Status    string   `json:"status"` // "applied", "partial", "not-applied"
}

// ConformityScaffold is Annex IV §7. Article 47 conformity declaration content
// requires legal review; Relicta emits a scaffold for the provider to complete.
type ConformityScaffold struct {
	ProviderName       string `json:"providerName,omitempty"`
	ProviderAddress    string `json:"providerAddress,omitempty"`
	SystemIdentifier   string `json:"systemIdentifier"`
	UniqueIdentifier   string `json:"uniqueIdentifier,omitempty"`
	StandardsApplied   []string `json:"standardsApplied,omitempty"`
	NotifiedBody       string `json:"notifiedBody,omitempty"`
	DateOfDeclaration  string `json:"dateOfDeclaration,omitempty"`
	Signatory          string `json:"signatory,omitempty"`
	SignatoryRole      string `json:"signatoryRole,omitempty"`
}

// PostMarketMonitoring is Annex IV §8 — incident response + monitoring.
type PostMarketMonitoring struct {
	MonitoringPlan        string             `json:"monitoringPlan"`
	IncidentRecords       []IncidentSummary8 `json:"incidentRecords"`
	TotalIncidents        int                `json:"totalIncidents"`
	AverageResolutionHrs  float64            `json:"averageResolutionHours"`
	ChangeFailureRate     float64            `json:"changeFailureRate"`
}

// IncidentSummary8 is an incident record reformatted for §8 documentation.
// Suffixed _8 to avoid colliding with the existing IncidentSummary used by SummaryReport.
type IncidentSummary8 struct {
	IncidentID    string     `json:"incidentId"`
	ReleaseID     string     `json:"releaseId"`
	Version       string     `json:"version"`
	Type          string     `json:"type"`
	Severity      string     `json:"severity"`
	DetectedAt    time.Time  `json:"detectedAt"`
	ResolvedAt    *time.Time `json:"resolvedAt,omitempty"`
	TimeToResolve string     `json:"timeToResolve,omitempty"`
}

// buildAnnexIV constructs an AnnexIVReport from fetched data.
//
// The system identifier is taken from config.Repository when set, otherwise
// "default". The retention deadline is ten years past the period's end per
// Article 11. Sections that cannot be derived (e.g. provider legal name)
// are emitted as scaffolds with TODO markers.
func (g *Generator) buildAnnexIV(data *reportData, systemID string) *AnnexIVReport {
	if systemID == "" {
		systemID = "default"
	}

	report := &AnnexIVReport{
		SystemIdentifier:  systemID,
		RetentionDeadline: data.period.End.AddDate(10, 0, 0),
	}

	if v := latestVersion(data.releases); v != "" {
		report.SystemVersion = v
	}

	report.GeneralDescription = buildGeneralDescription(systemID, report.SystemVersion)
	report.DetailedDescription = buildDetailedDescription(data)
	report.MonitoringControl = buildMonitoringControl()
	report.RiskManagement = buildRiskManagement(data)
	report.LifecycleChanges = buildLifecycleChanges(data.releases)
	report.HarmonizedStandards = buildHarmonizedStandards()
	report.ConformityDeclaration = ConformityScaffold{
		SystemIdentifier:  systemID,
		StandardsApplied:  []string{"ISO/IEC 42001:2023", "OWASP LLM Top 10", "NIST AI RMF"},
		DateOfDeclaration: time.Now().UTC().Format("2006-01-02"),
		// Other fields left blank — provider must complete + sign.
	}
	report.PostMarketMonitoring = buildPostMarketMonitoring(data)

	if len(data.releases) == 0 {
		report.GenerationNotes = append(report.GenerationNotes,
			"no releases observed in period; Annex IV §5 (lifecycle changes) and §8 (post-market monitoring) are minimal")
	}
	report.GenerationNotes = append(report.GenerationNotes,
		"§7 conformity declaration is a scaffold — provider legal name, address, signatory, and notified body must be completed before submission",
		"§6 harmonized standards reflect frameworks Relicta enforces internally; cross-check against your AI Act notified body's required standards list")

	return report
}

func latestVersion(releases []*memory.ReleaseRecord) string {
	var latest string
	var latestAt time.Time
	for _, r := range releases {
		if r.ReleasedAt.After(latestAt) {
			latestAt = r.ReleasedAt
			latest = r.Version
		}
	}
	return latest
}

func buildGeneralDescription(systemID, version string) GeneralDescription {
	return GeneralDescription{
		IntendedPurpose: fmt.Sprintf(
			"%s is governed by the Relicta Change Governance Protocol (CGP). "+
				"Its intended purpose is software release governance: classifying changes, "+
				"scoring release risk, enforcing organizational policy, and producing "+
				"cryptographically attested audit evidence for AI-attributed releases.",
			systemID),
		SystemVersion:        version,
		HardwareEnvironments: []string{"Linux x86_64", "Linux arm64", "macOS arm64", "macOS x86_64"},
		DeploymentForms:      []string{"single binary", "container image", "CI/CD action"},
		UserInterfaces:       []string{"CLI (Cobra)", "MCP server (stdio + http)", "REST API (chi)", "Web dashboard (Vue 3)"},
		Languages:            []string{"en"},
	}
}

func buildDetailedDescription(data *reportData) DetailedDescription {
	cgpVersion := ""
	for _, d := range data.decisions {
		if d.CGPVersion != "" {
			cgpVersion = d.CGPVersion
			break
		}
	}
	if cgpVersion == "" {
		cgpVersion = "0.1"
	}

	return DetailedDescription{
		DevelopmentMethods: []string{
			"Domain-Driven Design (hexagonal architecture)",
			"Test-Driven Development (>=80% coverage gate via coverctl)",
			"Conventional Commits + semantic versioning",
			"Static analysis (golangci-lint, go vet)",
			"Security scanning (nox SAST/SCA)",
		},
		DesignSpecifications: []string{
			"Change Governance Protocol (CGP) wire format: pkg/cgp/",
			"Hash-chained audit trail: internal/cgp/audit/",
			"Hexagonal release aggregate: internal/domain/release/",
		},
		SystemArchitecture: "Hexagonal architecture with bounded contexts: domain (release aggregate, CGP), application (use cases), infrastructure (git, AI providers, persistence, webhooks), CLI (Cobra), MCP (server + adapters), httpserver (chi + middleware).",
		ComputationalResources: "Single-binary CLI; optional PostgreSQL persistence backend. AI provider calls outsourced to OpenAI / Anthropic / Gemini / Ollama (per .relicta.yaml).",
		DataRequirements: []string{
			"git repository commit history",
			"release tag history",
			"governance policy YAML (.relicta/*.policy)",
			"actor budget YAML (.relicta/actor-budgets.yaml)",
		},
		HumanOversightMeasures: []string{
			"RBAC roles (Admin/Approver/Viewer) enforced in middleware",
			"Approval gate before publish (CGP DecisionApprovalRequired path)",
			"Actor autonomy budgets cap agent autonomy with cosigner requirements",
			"Hash-chained audit trail enables post-hoc human review",
			"Risk score threshold triggers manual approval per policy",
		},
		PredeterminedChanges: []string{
			"semantic version bumps (major / minor / patch)",
			"pre-release channel promotion (alpha / beta / rc / stable)",
			"plugin lifecycle hooks (PreVersion / PostNotes / PostPublish)",
		},
		CGPProtocolVersion: cgpVersion,
		RiskModelVersion:   "calculator/v1+calibration",
	}
}

func buildMonitoringControl() MonitoringControl {
	return MonitoringControl{
		MonitoringMechanisms: []string{
			"Prometheus metrics endpoint (/metrics)",
			"Inbound webhook receiver for Alertmanager / PagerDuty / Datadog",
			"Release outcome tracking via memory store",
			"Per-actor reputation scoring",
		},
		ControlInterfaces: []string{
			"`relicta cancel` (in-progress release halt)",
			"`relicta rollback` (revert to previous version)",
			"`relicta approve --reject` (governance veto)",
			"Policy DSL (`.relicta/*.policy` declarative rules)",
		},
		AuditTrailLocation:  ".relicta/memory/ (file backend) or PostgreSQL events table (DB backend)",
		AuditChainAlgorithm: "SHA-256 hash chain over JSON-canonical CGP decisions; integrity verified at report-generation time",
	}
}

func buildRiskManagement(data *reportData) RiskManagement {
	categoryCounts := make(map[string]int)
	categoryScores := make(map[string]float64)

	for _, d := range data.decisions {
		// Each rationale is a sentence; categories are tracked via the
		// risk calculator's factor categories. We approximate by extracting
		// the first word of each rationale entry.
		for _, r := range d.Rationale {
			cat := firstWord(r)
			if cat == "" {
				continue
			}
			categoryCounts[cat]++
			categoryScores[cat] += d.RiskScore
		}
	}

	var risks []IdentifiedRisk
	for cat, count := range categoryCounts {
		avg := 0.0
		if count > 0 {
			avg = categoryScores[cat] / float64(count)
		}
		risks = append(risks, IdentifiedRisk{
			Category:        cat,
			Description:     fmt.Sprintf("Recurring risk category observed in governance decisions: %s", cat),
			Severity:        severityFromScore(avg),
			OccurrenceCount: count,
			AverageScore:    avg,
		})
	}
	sort.Slice(risks, func(i, j int) bool { return risks[i].OccurrenceCount > risks[j].OccurrenceCount })

	return RiskManagement{
		IdentifiedRisks:      risks,
		RiskEvaluationMethod: "8-factor weighted risk calculator with outcome-based calibration loop. Factors: blast radius, breaking changes, security touch, file count, line count, dependency churn, actor trust, time pressure. Scores normalized to [0,1].",
		RiskMitigationControls: []MitigationControl{
			{Name: "Approval Gate", Description: "Risk score above policy threshold requires human approval before publish.", Type: "policy"},
			{Name: "Actor Autonomy Budget", Description: "Per-actor caps on blast radius, risk, dollar cost, and required cosigners.", Type: "policy"},
			{Name: "Plugin Sandbox", Description: "Plugins run as isolated subprocesses (gRPC) with resource advisory; signed plugins required for production.", Type: "technical"},
			{Name: "Hash-Chained Audit", Description: "Every governance decision and approval is anchored in a tamper-evident hash chain.", Type: "technical"},
			{Name: "Outcome Calibration", Description: "Risk model retrains against post-release incidents; predictions adjusted over time.", Type: "process"},
		},
		ResidualRiskRationale: "Plugin sandbox is best-effort on darwin (macOS RLIMIT_AS unenforced on Apple Silicon). AI-generated release notes prose has no factuality guarantee — content is advisory, not load-bearing for governance decisions.",
	}
}

func buildLifecycleChanges(releases []*memory.ReleaseRecord) []LifecycleChange {
	changes := make([]LifecycleChange, 0, len(releases))
	for _, r := range releases {
		changes = append(changes, LifecycleChange{
			Timestamp:  r.ReleasedAt,
			Version:    r.Version,
			ChangeType: classifyChangeType(r),
			Decision:   string(r.Decision),
			Actor:      fmt.Sprintf("%s:%s", r.Actor.Kind, r.Actor.ID),
			Outcome:    string(r.Outcome),
			RiskScore:  r.RiskScore,
		})
	}
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Timestamp.Before(changes[j].Timestamp)
	})
	return changes
}

func classifyChangeType(r *memory.ReleaseRecord) string {
	switch {
	case r.BreakingChanges > 0:
		return "breaking"
	case r.SecurityChanges > 0:
		return "security"
	default:
		return "feature/fix"
	}
}

func buildHarmonizedStandards() []HarmonizedStandard {
	return []HarmonizedStandard{
		{Framework: "ISO/IEC 42001", Version: "2023", Controls: []string{"AI management system requirements"}, Status: "applied"},
		{Framework: "ISO/IEC 27001", Version: "2022", Controls: []string{"A.5", "A.8", "A.12"}, Status: "applied"},
		{Framework: "SOC 2 Type II", Controls: []string{"CC6.1", "CC7.1", "CC7.4", "CC8.1"}, Status: "applied"},
		{Framework: "OWASP LLM Top 10", Version: "2025", Controls: []string{"LLM06 Sensitive Information Disclosure"}, Status: "applied"},
		{Framework: "OWASP Agentic Top 10", Version: "2026", Controls: []string{"AG04 Data Exfiltration"}, Status: "partial"},
		{Framework: "NIST SP 800-53", Controls: []string{"AU-3 Content of Audit Records", "AU-12 Audit Generation"}, Status: "applied"},
		{Framework: "NIST AI RMF", Version: "1.0", Controls: []string{"GOVERN", "MAP", "MEASURE", "MANAGE"}, Status: "partial"},
	}
}

func buildPostMarketMonitoring(data *reportData) PostMarketMonitoring {
	incidents := make([]IncidentSummary8, 0, len(data.incidents))
	totalResolutionHours := 0.0
	resolvedCount := 0

	for _, inc := range data.incidents {
		summary := IncidentSummary8{
			IncidentID: inc.ID,
			ReleaseID:  inc.ReleaseID,
			Version:    inc.Version,
			Type:       string(inc.Type),
			Severity:   string(inc.Severity),
			DetectedAt: inc.DetectedAt,
		}
		if inc.ResolvedAt != nil {
			summary.ResolvedAt = inc.ResolvedAt
			delta := inc.ResolvedAt.Sub(inc.DetectedAt)
			summary.TimeToResolve = delta.String()
			totalResolutionHours += delta.Hours()
			resolvedCount++
		}
		incidents = append(incidents, summary)
	}

	avgResolution := 0.0
	if resolvedCount > 0 {
		avgResolution = totalResolutionHours / float64(resolvedCount)
	}

	failureRate := 0.0
	if len(data.releases) > 0 {
		failed := 0
		for _, r := range data.releases {
			if r.Outcome.IsNegative() {
				failed++
			}
		}
		failureRate = float64(failed) / float64(len(data.releases))
	}

	return PostMarketMonitoring{
		MonitoringPlan: "Continuous post-deployment outcome tracking via release memory store. " +
			"Inbound webhooks from observability systems (Alertmanager, PagerDuty, Datadog) " +
			"correlate incidents with releases. Risk model recalibration runs against " +
			"observed outcomes monthly.",
		IncidentRecords:      incidents,
		TotalIncidents:       len(incidents),
		AverageResolutionHrs: avgResolution,
		ChangeFailureRate:    failureRate,
	}
}

// firstWord returns the leading whitespace-delimited token of s, lowercased
// for stable categorization. Empty input returns empty string.
func firstWord(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	idx := strings.IndexFunc(s, func(r rune) bool { return r == ' ' || r == '\t' || r == ':' || r == ',' })
	if idx < 0 {
		return strings.ToLower(s)
	}
	return strings.ToLower(s[:idx])
}

func severityFromScore(score float64) string {
	switch {
	case score >= 0.8:
		return "critical"
	case score >= 0.6:
		return "high"
	case score >= 0.4:
		return "medium"
	default:
		return "low"
	}
}
