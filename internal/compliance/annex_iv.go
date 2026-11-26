package compliance

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/v5/internal/cgp/memory"
)

// AnnexIVReport is a draft of release-governance evidence mapped to Annex IV.
// It is not a complete provider technical documentation bundle.
//
// Annex IV (Regulation 2024/1689) specifies high-risk AI system technical
// documentation across eight sections. Article 18(1) requires providers to
// keep specified documentation at authorities' disposal for ten years after
// placing the system on the market or putting it into service.
// Article 12 covers logs; Annex IV covers the *system documentation* about
// what the AI system does, how it was built, how it is monitored, and how
// risks are managed.
//
// Microsoft Agent Governance Toolkit (April 2026) covers Article 12 mapping
// but does NOT generate Annex IV documentation. This is Relicta's wedge.
//
// The generator includes release-history evidence it can derive from CGP data.
// Target-system documentation, standards applicability, and declarations must
// be supplied and verified by the provider.
type AnnexIVReport struct {
	// SystemIdentifier names the AI system documented.
	SystemIdentifier string `json:"systemIdentifier"`

	// EvidenceScope describes what this report can substantiate.
	EvidenceScope string `json:"evidenceScope"`

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

	// RetentionGuidance explains the relevant Article 18(1) period. Relicta
	// does not collect the event date needed to calculate a statutory deadline.
	RetentionGuidance string `json:"retentionGuidance"`

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
	DevelopmentMethods     []string `json:"developmentMethods"`
	DesignSpecifications   []string `json:"designSpecifications"`
	SystemArchitecture     string   `json:"systemArchitecture"`
	ComputationalResources string   `json:"computationalResources,omitempty"`
	DataRequirements       []string `json:"dataRequirements,omitempty"`
	HumanOversightMeasures []string `json:"humanOversightMeasures"`
	PredeterminedChanges   []string `json:"predeterminedChanges,omitempty"`
	CGPProtocolVersion     string   `json:"cgpProtocolVersion"`
	RiskModelVersion       string   `json:"riskModelVersion,omitempty"`
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
	IdentifiedRisks        []IdentifiedRisk    `json:"identifiedRisks"`
	RiskEvaluationMethod   string              `json:"riskEvaluationMethod"`
	RiskMitigationControls []MitigationControl `json:"riskMitigationControls"`
	ResidualRiskRationale  string              `json:"residualRiskRationale,omitempty"`
}

// IdentifiedRisk summarizes a category inferred from recorded release
// governance rationales; it is not a target AI system risk assessment.
type IdentifiedRisk struct {
	Category        string  `json:"category"`
	Description     string  `json:"description"`
	Severity        string  `json:"severity"`
	OccurrenceCount int     `json:"occurrenceCount"`
	AverageScore    float64 `json:"averageScore"`
}

// MitigationControl describes a control applied to mitigate risk.
type MitigationControl struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"` // "policy", "technical", "process"
}

// LifecycleChange is a single change recorded against the system over time.
type LifecycleChange struct {
	Timestamp  time.Time `json:"timestamp"`
	Version    string    `json:"version"`
	ChangeType string    `json:"changeType"`
	Decision   string    `json:"decision"`
	Actor      string    `json:"actor"`
	Outcome    string    `json:"outcome"`
	RiskScore  float64   `json:"riskScore"`
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
	ProviderName      string   `json:"providerName,omitempty"`
	ProviderAddress   string   `json:"providerAddress,omitempty"`
	SystemIdentifier  string   `json:"systemIdentifier"`
	UniqueIdentifier  string   `json:"uniqueIdentifier,omitempty"`
	StandardsApplied  []string `json:"standardsApplied,omitempty"`
	NotifiedBody      string   `json:"notifiedBody,omitempty"`
	DateOfDeclaration string   `json:"dateOfDeclaration,omitempty"`
	Signatory         string   `json:"signatory,omitempty"`
	SignatoryRole     string   `json:"signatoryRole,omitempty"`
}

// PostMarketMonitoring is Annex IV §8 — incident response + monitoring.
type PostMarketMonitoring struct {
	MonitoringPlan       string             `json:"monitoringPlan"`
	IncidentRecords      []IncidentSummary8 `json:"incidentRecords"`
	TotalIncidents       int                `json:"totalIncidents"`
	AverageResolutionHrs float64            `json:"averageResolutionHours"`
	ChangeFailureRate    float64            `json:"changeFailureRate"`
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
// "default". Sections that cannot be derived are left blank and explained in
// GenerationNotes; the Markdown renderer marks required blanks as TODO.
func (g *Generator) buildAnnexIV(data *reportData, systemID string) *AnnexIVReport {
	if systemID == "" {
		systemID = "default"
	}

	report := &AnnexIVReport{
		SystemIdentifier:  systemID,
		EvidenceScope:     "Relicta release-governance records for the selected period; not a complete technical description, risk assessment, monitoring record, or conformity assessment of the target AI system.",
		RetentionGuidance: "EU AI Act Article 18(1) requires providers to keep specified technical documentation at competent authorities' disposal for 10 years after the system is placed on the market or put into service. Relicta does not capture that event date and does not calculate a statutory deadline; confirm applicable duties and dates with the responsible owner.",
	}

	if v := latestVersion(data.releases); v != "" {
		report.SystemVersion = v
	}

	report.GeneralDescription = buildGeneralDescription(report.SystemVersion)
	report.DetailedDescription = buildDetailedDescription(data)
	report.MonitoringControl = MonitoringControl{}
	report.RiskManagement = buildRiskManagement(data)
	report.LifecycleChanges = buildLifecycleChanges(data.releases)
	report.ConformityDeclaration = ConformityScaffold{
		SystemIdentifier: systemID,
		// Other fields left blank — provider must complete + sign.
	}
	report.PostMarketMonitoring = buildPostMarketMonitoring(data)

	if len(data.releases) == 0 {
		report.GenerationNotes = append(report.GenerationNotes,
			"no releases observed in period; Annex IV §5 (lifecycle changes) and §8 (post-market monitoring) are minimal")
	}
	report.GenerationNotes = append(report.GenerationNotes,
		"§1 intended purpose, provider, environments, deployment, interfaces, and languages are not configured or verified by Relicta; provider must complete them",
		"§2 system design, development, data, resources, oversight, and change details are not collected by Relicta; provider must complete them; CGP protocol version is included only when observed in governance decisions",
		"§3 target-system monitoring, control, and audit-trail details are not collected or verified by Relicta; provider must complete them",
		"§4 release-governance rationales are summarized as observed evidence only; this is not a target-system risk evaluation or mitigation assessment",
		"§6 standards and framework applicability are not configured or verified by Relicta; provider must identify applicable harmonized standards and document implementation status",
		"§7 conformity declaration is a scaffold; provider must complete and legally review the applicable standards, declaration date, legal entity details, notified body, and signatory",
		"§8 contains Relicta release-history metrics and incidents only; it is not a target-system post-market monitoring plan or complete incident-monitoring record")

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

func buildGeneralDescription(version string) GeneralDescription {
	return GeneralDescription{SystemVersion: version}
}

func buildDetailedDescription(data *reportData) DetailedDescription {
	cgpVersion := ""
	for _, d := range data.decisions {
		if d.CGPVersion != "" {
			cgpVersion = d.CGPVersion
			break
		}
	}
	return DetailedDescription{CGPProtocolVersion: cgpVersion}
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

	return RiskManagement{IdentifiedRisks: risks}
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
