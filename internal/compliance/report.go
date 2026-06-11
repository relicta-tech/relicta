// Package compliance provides compliance report generation from governance data.
//
// It supports multiple report formats (DORA metrics, SOC 2 evidence, governance
// summaries) and output formats (Markdown, JSON). Reports are generated from
// the release memory store, which tracks releases, incidents, decisions, and
// approval workflows.
package compliance

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/memory"
)

// ReportFormat defines output formats.
type ReportFormat string

const (
	FormatMarkdown ReportFormat = "markdown"
	FormatJSON     ReportFormat = "json"
	// FormatJSONL emits one JSON object per line. Required for Article 12
	// log bundles where each entry must be independently readable.
	FormatJSONL ReportFormat = "jsonl"
	// FormatCSV emits a flat tabular view for regulator portals.
	FormatCSV ReportFormat = "csv"
)

// IsValid returns true if the format is recognized.
func (f ReportFormat) IsValid() bool {
	switch f {
	case FormatMarkdown, FormatJSON, FormatJSONL, FormatCSV:
		return true
	default:
		return false
	}
}

// ReportType defines the compliance framework.
type ReportType string

const (
	ReportDORA             ReportType = "dora"                 // DORA metrics
	ReportSOC2             ReportType = "soc2"                 // SOC 2 change management evidence
	ReportSummary          ReportType = "summary"              // General governance summary
	ReportEUAIActArticle12 ReportType = "eu-ai-act-article-12" // EU AI Act Article 12 record-keeping
	ReportEUAIActAnnexIV   ReportType = "eu-ai-act-annex-iv"   // EU AI Act Annex IV technical documentation
)

// IsValid returns true if the report type is recognized.
func (t ReportType) IsValid() bool {
	switch t {
	case ReportDORA, ReportSOC2, ReportSummary, ReportEUAIActArticle12, ReportEUAIActAnnexIV:
		return true
	default:
		return false
	}
}

// ReportConfig configures report generation.
type ReportConfig struct {
	Type       ReportType
	Format     ReportFormat
	Period     Period
	Repository string // optional: filter by repo
}

// Validate checks if the configuration is valid.
func (c *ReportConfig) Validate() error {
	if !c.Type.IsValid() {
		return fmt.Errorf("invalid report type: %q", c.Type)
	}
	if !c.Format.IsValid() {
		return fmt.Errorf("invalid report format: %q", c.Format)
	}
	if c.Period.Start.IsZero() || c.Period.End.IsZero() {
		return fmt.Errorf("period start and end are required")
	}
	if c.Period.End.Before(c.Period.Start) {
		return fmt.Errorf("period end must be after start")
	}
	return nil
}

// Period defines a time range for the report.
type Period struct {
	Start time.Time
	End   time.Time
	Label string // e.g. "2026-Q1", "March 2026"
}

// ParsePeriod parses a period string into a Period.
// Supported formats:
//   - "YYYY-QN" (quarter, e.g. "2026-Q1")
//   - "YYYY-MM-DD:YYYY-MM-DD" (date range)
func ParsePeriod(s string) (Period, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Period{}, fmt.Errorf("empty period string")
	}

	// Try quarter format: 2026-Q1
	if len(s) == 7 && s[4] == '-' && (s[5] == 'Q' || s[5] == 'q') {
		year := 0
		quarter := 0
		_, err := fmt.Sscanf(s[:4], "%d", &year)
		if err != nil {
			return Period{}, fmt.Errorf("invalid year in quarter format: %w", err)
		}
		_, err = fmt.Sscanf(s[6:], "%d", &quarter)
		if err != nil || quarter < 1 || quarter > 4 {
			return Period{}, fmt.Errorf("invalid quarter: must be Q1-Q4")
		}

		startMonth := time.Month((quarter-1)*3 + 1)
		start := time.Date(year, startMonth, 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 3, 0).Add(-time.Nanosecond)

		return Period{
			Start: start,
			End:   end,
			Label: fmt.Sprintf("%d-Q%d", year, quarter),
		}, nil
	}

	// Try date range format: 2026-03-01:2026-03-31
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 2 {
		start, err := time.Parse("2006-01-02", strings.TrimSpace(parts[0]))
		if err != nil {
			return Period{}, fmt.Errorf("invalid start date: %w", err)
		}
		end, err := time.Parse("2006-01-02", strings.TrimSpace(parts[1]))
		if err != nil {
			return Period{}, fmt.Errorf("invalid end date: %w", err)
		}
		// Set end to end of day
		end = end.Add(24*time.Hour - time.Nanosecond)

		return Period{
			Start: start,
			End:   end,
			Label: fmt.Sprintf("%s to %s", start.Format("2006-01-02"), end.Truncate(24*time.Hour).Format("2006-01-02")),
		}, nil
	}

	return Period{}, fmt.Errorf("unrecognized period format: %q (use YYYY-QN or YYYY-MM-DD:YYYY-MM-DD)", s)
}

// Generator creates compliance reports from governance data.
type Generator struct {
	store  memory.Store
	logger *slog.Logger
}

// NewGenerator creates a new compliance report generator.
func NewGenerator(store memory.Store, logger *slog.Logger) *Generator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Generator{
		store:  store,
		logger: logger,
	}
}

// Generate creates a compliance report based on the configuration.
func (g *Generator) Generate(ctx context.Context, config ReportConfig) (*Report, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid report config: %w", err)
	}

	g.logger.Info("generating compliance report",
		"type", config.Type,
		"format", config.Format,
		"period", config.Period.Label,
	)

	// Fetch data from the store
	data, err := g.fetchData(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch report data: %w", err)
	}

	report := &Report{
		Type:        config.Type,
		Period:      config.Period,
		GeneratedAt: time.Now().UTC(),
		Repository:  config.Repository,
	}

	switch config.Type {
	case ReportDORA:
		report.DORA = g.calculateDORA(data)
	case ReportSOC2:
		report.SOC2 = g.buildSOC2(data)
	case ReportSummary:
		report.Summary = g.buildSummary(data)
	case ReportEUAIActArticle12:
		report.Article12 = g.buildArticle12(data, config.Repository)
	case ReportEUAIActAnnexIV:
		report.AnnexIV = g.buildAnnexIV(data, config.Repository)
	}

	return report, nil
}

// Render outputs the report in the requested format.
func (g *Generator) Render(report *Report, format ReportFormat) (string, error) {
	switch format {
	case FormatJSON:
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal report: %w", err)
		}
		return string(b), nil
	case FormatMarkdown:
		return RenderMarkdown(report), nil
	case FormatJSONL:
		if report.Article12 == nil {
			return "", fmt.Errorf("jsonl format requires an article12 report; use --type eu-ai-act-article-12")
		}
		return RenderJSONL(report.Article12)
	case FormatCSV:
		if report.Article12 == nil {
			return "", fmt.Errorf("csv format requires an article12 report; use --type eu-ai-act-article-12")
		}
		return RenderCSV(report.Article12)
	default:
		return "", fmt.Errorf("unsupported format: %q", format)
	}
}

// Report is the top-level report structure.
type Report struct {
	Type        ReportType `json:"type"`
	Period      Period     `json:"period"`
	GeneratedAt time.Time  `json:"generatedAt"`
	Repository  string     `json:"repository,omitempty"`

	DORA      *DORAReport      `json:"dora,omitempty"`
	SOC2      *SOC2Report      `json:"soc2,omitempty"`
	Summary   *SummaryReport   `json:"summary,omitempty"`
	Article12 *Article12Report `json:"article12,omitempty"`
	AnnexIV   *AnnexIVReport   `json:"annexIV,omitempty"`
}

// DORAReport contains DORA metrics.
type DORAReport struct {
	DeploymentFrequency DeploymentFrequency `json:"deploymentFrequency"`
	LeadTimeForChanges  LeadTimeForChanges  `json:"leadTimeForChanges"`
	MTTR                MTTRMetrics         `json:"mttr"`
	ChangeFailureRate   ChangeFailureRate   `json:"changeFailureRate"`
	Classification      string              `json:"classification"` // elite, high, medium, low
}

// DeploymentFrequency tracks how often deployments occur.
type DeploymentFrequency struct {
	TotalDeployments int     `json:"totalDeployments"`
	PerDay           float64 `json:"perDay"`
	PerWeek          float64 `json:"perWeek"`
	Classification   string  `json:"classification"` // on-demand, weekly, monthly, yearly
}

// LeadTimeForChanges tracks time from commit to release.
type LeadTimeForChanges struct {
	AverageHours   float64 `json:"averageHours"`
	MedianHours    float64 `json:"medianHours"`
	P95Hours       float64 `json:"p95Hours"`
	Classification string  `json:"classification"` // less-than-one-day, one-week, one-month, more-than-six-months
}

// MTTRMetrics tracks mean time to recovery.
type MTTRMetrics struct {
	AverageHours   float64 `json:"averageHours"`
	MedianHours    float64 `json:"medianHours"`
	TotalIncidents int     `json:"totalIncidents"`
	ResolvedCount  int     `json:"resolvedCount"`
	Classification string  `json:"classification"` // less-than-one-hour, less-than-one-day, one-week, more-than-six-months
}

// ChangeFailureRate tracks the percentage of changes causing failures.
type ChangeFailureRate struct {
	TotalChanges   int     `json:"totalChanges"`
	FailedChanges  int     `json:"failedChanges"`
	Rate           float64 `json:"rate"`           // 0.0-1.0
	Classification string  `json:"classification"` // 0-15%, 16-30%, 31-45%, 46-60%
}

// SOC2Report contains SOC 2 change management evidence.
type SOC2Report struct {
	ChangeLog        []ChangeLogEntry   `json:"changeLog"`
	ApprovalEvidence []ApprovalEvidence `json:"approvalEvidence"`
	RiskAssessments  []RiskAssessment   `json:"riskAssessments"`
	IncidentResponse []IncidentResponse `json:"incidentResponse"`
	PolicyCompliance []PolicyCompliance `json:"policyCompliance"`
}

// ChangeLogEntry is a single change request entry for SOC 2.
type ChangeLogEntry struct {
	ID        string    `json:"id"`
	Version   string    `json:"version"`
	Date      time.Time `json:"date"`
	Actor     string    `json:"actor"`
	ActorKind string    `json:"actorKind"`
	RiskScore float64   `json:"riskScore"`
	Decision  string    `json:"decision"`
	Outcome   string    `json:"outcome"`
}

// ApprovalEvidence records who approved what and when.
type ApprovalEvidence struct {
	ReleaseID    string    `json:"releaseId"`
	Version      string    `json:"version"`
	ApprovedBy   string    `json:"approvedBy"`
	ApproverKind string    `json:"approverKind"`
	ApprovedAt   time.Time `json:"approvedAt"`
	DecisionType string    `json:"decisionType"`
	RiskScore    float64   `json:"riskScore"`
}

// RiskAssessment records risk evaluation details.
type RiskAssessment struct {
	ReleaseID   string       `json:"releaseId"`
	Version     string       `json:"version"`
	RiskScore   float64      `json:"riskScore"`
	RiskLevel   string       `json:"riskLevel"` // low, medium, high, critical
	RiskFactors []RiskDetail `json:"riskFactors"`
}

// RiskDetail captures a single risk factor.
type RiskDetail struct {
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`
	Severity    string  `json:"severity"`
}

// IncidentResponse records incident handling.
type IncidentResponse struct {
	IncidentID    string        `json:"incidentId"`
	ReleaseID     string        `json:"releaseId"`
	Version       string        `json:"version"`
	Type          string        `json:"type"`
	Severity      string        `json:"severity"`
	DetectedAt    time.Time     `json:"detectedAt"`
	ResolvedAt    *time.Time    `json:"resolvedAt,omitempty"`
	TimeToResolve time.Duration `json:"timeToResolve,omitempty"`
}

// PolicyCompliance records policy evaluation results.
type PolicyCompliance struct {
	ReleaseID string   `json:"releaseId"`
	Version   string   `json:"version"`
	Decision  string   `json:"decision"`
	RiskScore float64  `json:"riskScore"`
	Rationale []string `json:"rationale"`
}

// SummaryReport is a general governance summary.
type SummaryReport struct {
	TotalReleases     int                    `json:"totalReleases"`
	RiskDistribution  RiskDistribution       `json:"riskDistribution"`
	ApprovalBreakdown ApprovalBreakdown      `json:"approvalBreakdown"`
	TopRiskFactors    []RiskFactorSummary    `json:"topRiskFactors"`
	ActorActivity     []ActorActivitySummary `json:"actorActivity"`
	IncidentSummary   IncidentSummary        `json:"incidentSummary"`
}

// RiskDistribution categorizes releases by risk level.
type RiskDistribution struct {
	Low      int `json:"low"`      // < 0.4
	Medium   int `json:"medium"`   // 0.4-0.6
	High     int `json:"high"`     // 0.6-0.8
	Critical int `json:"critical"` // >= 0.8
}

// ApprovalBreakdown shows approval statistics.
type ApprovalBreakdown struct {
	AutoApproved  int `json:"autoApproved"`
	HumanApproved int `json:"humanApproved"`
	Rejected      int `json:"rejected"`
}

// RiskFactorSummary captures recurring risk factors.
type RiskFactorSummary struct {
	Category string  `json:"category"`
	Count    int     `json:"count"`
	AvgScore float64 `json:"avgScore"`
}

// ActorActivitySummary captures per-actor release activity.
type ActorActivitySummary struct {
	ActorID      string  `json:"actorId"`
	ActorKind    string  `json:"actorKind"`
	ReleaseCount int     `json:"releaseCount"`
	SuccessRate  float64 `json:"successRate"`
	AvgRiskScore float64 `json:"avgRiskScore"`
}

// IncidentSummary provides incident statistics.
type IncidentSummary struct {
	TotalIncidents   int     `json:"totalIncidents"`
	AvgResolutionHrs float64 `json:"avgResolutionHours"`
	CorrelationRate  float64 `json:"correlationRate"` // incidents per release
}

// reportData holds fetched data used across report types.
type reportData struct {
	releases  []*memory.ReleaseRecord
	incidents []*memory.IncidentRecord
	decisions []*cgp.GovernanceDecision
	period    Period
}

// fetchData retrieves all relevant data from the store for the period.
func (g *Generator) fetchData(ctx context.Context, config ReportConfig) (*reportData, error) {
	repo := config.Repository
	if repo == "" {
		repo = "default"
	}

	// Fetch a large batch; we filter by period in memory.
	releases, err := g.store.GetReleaseHistory(ctx, repo, 10000)
	if err != nil {
		return nil, fmt.Errorf("failed to get release history: %w", err)
	}

	incidents, err := g.store.GetIncidentHistory(ctx, repo, 10000)
	if err != nil {
		return nil, fmt.Errorf("failed to get incident history: %w", err)
	}

	// Filter releases by period
	var filtered []*memory.ReleaseRecord
	for _, r := range releases {
		if !r.ReleasedAt.Before(config.Period.Start) && !r.ReleasedAt.After(config.Period.End) {
			filtered = append(filtered, r)
		}
	}

	// Filter incidents by period
	var filteredIncidents []*memory.IncidentRecord
	for _, inc := range incidents {
		if !inc.DetectedAt.Before(config.Period.Start) && !inc.DetectedAt.After(config.Period.End) {
			filteredIncidents = append(filteredIncidents, inc)
		}
	}

	// Gather decisions for filtered releases
	var decisions []*cgp.GovernanceDecision
	seen := make(map[string]bool)
	for _, r := range filtered {
		if r.ID == "" || seen[r.ID] {
			continue
		}
		seen[r.ID] = true
		trail, err := g.store.GetAuditTrail(ctx, r.ID)
		if err != nil {
			// Not all releases may have audit trails
			g.logger.Debug("no audit trail for release", "id", r.ID, "err", err)
			continue
		}
		decisions = append(decisions, trail.Decisions...)
	}

	return &reportData{
		releases:  filtered,
		incidents: filteredIncidents,
		decisions: decisions,
		period:    config.Period,
	}, nil
}

// calculateDORA computes DORA metrics from release and incident data.
func (g *Generator) calculateDORA(data *reportData) *DORAReport {
	report := &DORAReport{}

	// Deployment Frequency
	report.DeploymentFrequency = g.calcDeploymentFrequency(data)

	// Lead Time for Changes
	report.LeadTimeForChanges = g.calcLeadTime(data)

	// MTTR
	report.MTTR = g.calcMTTR(data)

	// Change Failure Rate
	report.ChangeFailureRate = g.calcChangeFailureRate(data)

	// Overall classification
	report.Classification = classifyDORA(report)

	return report
}

func (g *Generator) calcDeploymentFrequency(data *reportData) DeploymentFrequency {
	total := len(data.releases)
	days := data.period.End.Sub(data.period.Start).Hours() / 24
	if days < 1 {
		days = 1
	}

	perDay := float64(total) / days
	perWeek := perDay * 7

	classification := "yearly"
	switch {
	case perDay >= 1:
		classification = "on-demand"
	case perWeek >= 1:
		classification = "weekly"
	case perDay*30 >= 1:
		classification = "monthly"
	}

	return DeploymentFrequency{
		TotalDeployments: total,
		PerDay:           perDay,
		PerWeek:          perWeek,
		Classification:   classification,
	}
}

func (g *Generator) calcLeadTime(data *reportData) LeadTimeForChanges {
	if len(data.releases) == 0 {
		return LeadTimeForChanges{Classification: "more-than-six-months"}
	}

	var durations []float64
	for _, r := range data.releases {
		if r.Duration > 0 {
			durations = append(durations, r.Duration.Hours())
		}
	}

	if len(durations) == 0 {
		return LeadTimeForChanges{Classification: "more-than-six-months"}
	}

	sort.Float64s(durations)

	avg := 0.0
	for _, d := range durations {
		avg += d
	}
	avg /= float64(len(durations))

	median := percentile(durations, 50)
	p95 := percentile(durations, 95)

	classification := "more-than-six-months"
	switch {
	case median < 24:
		classification = "less-than-one-day"
	case median < 168: // 7 days
		classification = "one-week"
	case median < 720: // 30 days
		classification = "one-month"
	}

	return LeadTimeForChanges{
		AverageHours:   avg,
		MedianHours:    median,
		P95Hours:       p95,
		Classification: classification,
	}
}

func (g *Generator) calcMTTR(data *reportData) MTTRMetrics {
	if len(data.incidents) == 0 {
		return MTTRMetrics{Classification: "less-than-one-hour"}
	}

	var resolutionHours []float64
	resolved := 0
	for _, inc := range data.incidents {
		if inc.TimeToResolve > 0 {
			resolutionHours = append(resolutionHours, inc.TimeToResolve.Hours())
			resolved++
		}
	}

	if len(resolutionHours) == 0 {
		return MTTRMetrics{
			TotalIncidents: len(data.incidents),
			Classification: "more-than-six-months",
		}
	}

	sort.Float64s(resolutionHours)

	avg := 0.0
	for _, h := range resolutionHours {
		avg += h
	}
	avg /= float64(len(resolutionHours))

	median := percentile(resolutionHours, 50)

	classification := "more-than-six-months"
	switch {
	case median < 1:
		classification = "less-than-one-hour"
	case median < 24:
		classification = "less-than-one-day"
	case median < 168:
		classification = "one-week"
	}

	return MTTRMetrics{
		AverageHours:   avg,
		MedianHours:    median,
		TotalIncidents: len(data.incidents),
		ResolvedCount:  resolved,
		Classification: classification,
	}
}

func (g *Generator) calcChangeFailureRate(data *reportData) ChangeFailureRate {
	total := len(data.releases)
	if total == 0 {
		return ChangeFailureRate{Classification: "0-15%"}
	}

	// Count releases with negative outcomes
	failed := 0
	for _, r := range data.releases {
		if r.Outcome.IsNegative() {
			failed++
		}
	}

	// Also count releases that had associated incidents
	releaseIDs := make(map[string]bool)
	for _, r := range data.releases {
		releaseIDs[r.ID] = true
	}
	for _, inc := range data.incidents {
		if releaseIDs[inc.ReleaseID] {
			// Only count once per release
			delete(releaseIDs, inc.ReleaseID)
			failed++
		}
	}

	// Cap at total
	if failed > total {
		failed = total
	}

	rate := float64(failed) / float64(total)

	classification := "0-15%"
	switch {
	case rate > 0.45:
		classification = "46-60%"
	case rate > 0.30:
		classification = "31-45%"
	case rate > 0.15:
		classification = "16-30%"
	}

	return ChangeFailureRate{
		TotalChanges:   total,
		FailedChanges:  failed,
		Rate:           rate,
		Classification: classification,
	}
}

// classifyDORA returns the overall DORA classification.
func classifyDORA(r *DORAReport) string {
	scores := map[string]int{
		"elite":  0,
		"high":   0,
		"medium": 0,
		"low":    0,
	}

	// Map each metric classification to a DORA level
	switch r.DeploymentFrequency.Classification {
	case "on-demand":
		scores["elite"]++
	case "weekly":
		scores["high"]++
	case "monthly":
		scores["medium"]++
	default:
		scores["low"]++
	}

	switch r.LeadTimeForChanges.Classification {
	case "less-than-one-day":
		scores["elite"]++
	case "one-week":
		scores["high"]++
	case "one-month":
		scores["medium"]++
	default:
		scores["low"]++
	}

	switch r.MTTR.Classification {
	case "less-than-one-hour":
		scores["elite"]++
	case "less-than-one-day":
		scores["high"]++
	case "one-week":
		scores["medium"]++
	default:
		scores["low"]++
	}

	switch r.ChangeFailureRate.Classification {
	case "0-15%":
		scores["elite"]++
	case "16-30%":
		scores["high"]++
	case "31-45%":
		scores["medium"]++
	default:
		scores["low"]++
	}

	// Return the most frequent classification
	best := "low"
	bestCount := 0
	for level, count := range scores {
		if count > bestCount {
			best = level
			bestCount = count
		}
	}
	return best
}

// buildSOC2 generates SOC 2 change management evidence.
func (g *Generator) buildSOC2(data *reportData) *SOC2Report {
	report := &SOC2Report{}

	// Change Log
	for _, r := range data.releases {
		report.ChangeLog = append(report.ChangeLog, ChangeLogEntry{
			ID:        r.ID,
			Version:   r.Version,
			Date:      r.ReleasedAt,
			Actor:     r.Actor.ID,
			ActorKind: string(r.Actor.Kind),
			RiskScore: r.RiskScore,
			Decision:  string(r.Decision),
			Outcome:   string(r.Outcome),
		})
	}

	// Approval Evidence from decisions
	for _, d := range data.decisions {
		report.ApprovalEvidence = append(report.ApprovalEvidence, ApprovalEvidence{
			ReleaseID:    d.ProposalID,
			Version:      d.RecommendedVersion,
			DecisionType: string(d.Decision),
			RiskScore:    d.RiskScore,
			ApprovedAt:   d.Timestamp,
		})
	}

	// Risk Assessments
	for _, d := range data.decisions {
		ra := RiskAssessment{
			ReleaseID: d.ProposalID,
			Version:   d.RecommendedVersion,
			RiskScore: d.RiskScore,
			RiskLevel: riskLevel(d.RiskScore),
		}
		for _, rf := range d.RiskFactors {
			ra.RiskFactors = append(ra.RiskFactors, RiskDetail{
				Category:    rf.Category,
				Description: rf.Description,
				Score:       rf.Score,
				Severity:    string(rf.Severity),
			})
		}
		report.RiskAssessments = append(report.RiskAssessments, ra)
	}

	// Incident Response
	for _, inc := range data.incidents {
		report.IncidentResponse = append(report.IncidentResponse, IncidentResponse{
			IncidentID:    inc.ID,
			ReleaseID:     inc.ReleaseID,
			Version:       inc.Version,
			Type:          string(inc.Type),
			Severity:      string(inc.Severity),
			DetectedAt:    inc.DetectedAt,
			ResolvedAt:    inc.ResolvedAt,
			TimeToResolve: inc.TimeToResolve,
		})
	}

	// Policy Compliance from decisions
	for _, d := range data.decisions {
		report.PolicyCompliance = append(report.PolicyCompliance, PolicyCompliance{
			ReleaseID: d.ProposalID,
			Version:   d.RecommendedVersion,
			Decision:  string(d.Decision),
			RiskScore: d.RiskScore,
			Rationale: d.Rationale,
		})
	}

	return report
}

// buildSummary generates a governance summary report.
func (g *Generator) buildSummary(data *reportData) *SummaryReport {
	report := &SummaryReport{
		TotalReleases: len(data.releases),
	}

	// Risk Distribution
	for _, r := range data.releases {
		switch {
		case r.RiskScore >= 0.8:
			report.RiskDistribution.Critical++
		case r.RiskScore >= 0.6:
			report.RiskDistribution.High++
		case r.RiskScore >= 0.4:
			report.RiskDistribution.Medium++
		default:
			report.RiskDistribution.Low++
		}
	}

	// Approval Breakdown
	for _, r := range data.releases {
		switch r.Decision {
		case cgp.DecisionApproved:
			report.ApprovalBreakdown.AutoApproved++
		case cgp.DecisionApprovalRequired:
			report.ApprovalBreakdown.HumanApproved++
		case cgp.DecisionRejected:
			report.ApprovalBreakdown.Rejected++
		}
	}

	// Top Risk Factors from decisions
	factorCounts := make(map[string]*RiskFactorSummary)
	for _, d := range data.decisions {
		for _, rf := range d.RiskFactors {
			s, ok := factorCounts[rf.Category]
			if !ok {
				s = &RiskFactorSummary{Category: rf.Category}
				factorCounts[rf.Category] = s
			}
			s.Count++
			s.AvgScore += rf.Score
		}
	}
	for _, s := range factorCounts {
		if s.Count > 0 {
			s.AvgScore /= float64(s.Count)
		}
		report.TopRiskFactors = append(report.TopRiskFactors, *s)
	}
	sort.Slice(report.TopRiskFactors, func(i, j int) bool {
		return report.TopRiskFactors[i].Count > report.TopRiskFactors[j].Count
	})

	// Actor Activity
	actorMap := make(map[string]*ActorActivitySummary)
	for _, r := range data.releases {
		a, ok := actorMap[r.Actor.ID]
		if !ok {
			a = &ActorActivitySummary{
				ActorID:   r.Actor.ID,
				ActorKind: string(r.Actor.Kind),
			}
			actorMap[r.Actor.ID] = a
		}
		a.ReleaseCount++
		a.AvgRiskScore += r.RiskScore
		if r.Outcome == memory.OutcomeSuccess {
			a.SuccessRate++
		}
	}
	for _, a := range actorMap {
		if a.ReleaseCount > 0 {
			a.AvgRiskScore /= float64(a.ReleaseCount)
			a.SuccessRate /= float64(a.ReleaseCount)
		}
		report.ActorActivity = append(report.ActorActivity, *a)
	}
	sort.Slice(report.ActorActivity, func(i, j int) bool {
		return report.ActorActivity[i].ReleaseCount > report.ActorActivity[j].ReleaseCount
	})

	// Incident Summary
	report.IncidentSummary.TotalIncidents = len(data.incidents)
	if len(data.incidents) > 0 {
		var totalResHours float64
		resolvedCount := 0
		for _, inc := range data.incidents {
			if inc.TimeToResolve > 0 {
				totalResHours += inc.TimeToResolve.Hours()
				resolvedCount++
			}
		}
		if resolvedCount > 0 {
			report.IncidentSummary.AvgResolutionHrs = totalResHours / float64(resolvedCount)
		}
		if len(data.releases) > 0 {
			report.IncidentSummary.CorrelationRate = float64(len(data.incidents)) / float64(len(data.releases))
		}
	}

	return report
}

// percentile returns the p-th percentile of a sorted slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}

	rank := (p / 100) * float64(len(sorted)-1)
	lower := int(rank)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	weight := rank - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

// riskLevel returns a human-readable risk level from a score.
func riskLevel(score float64) string {
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
