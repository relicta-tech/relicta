package compliance

import (
	"fmt"
	"strings"
)

// RenderMarkdown renders a report as a Markdown document.
func RenderMarkdown(report *Report) string {
	var b strings.Builder

	writeHeader(&b, report)

	switch report.Type {
	case ReportDORA:
		writeDORA(&b, report.DORA)
	case ReportSOC2:
		writeSOC2(&b, report.SOC2)
	case ReportSummary:
		writeSummary(&b, report.Summary)
	case ReportEUAIActArticle12:
		writeArticle12(&b, report.Article12)
	case ReportEUAIActAnnexIV:
		writeAnnexIV(&b, report.AnnexIV)
	}

	writeFooter(&b, report)

	return b.String()
}

func writeArticle12(b *strings.Builder, r *Article12Report) {
	if r == nil {
		fmt.Fprintln(b, "No Article 12 data available.")
		fmt.Fprintln(b)
		return
	}

	fmt.Fprintln(b, "## EU AI Act — Article 12 Record-Keeping")
	fmt.Fprintln(b)
	fmt.Fprintf(b, "**System Identifier:** %s  \n", r.SystemIdentifier)
	fmt.Fprintf(b, "**Log Entries:** %d  \n", len(r.LogEntries))
	fmt.Fprintf(b, "**Retention Deadline (Article 26(6) — 6 months minimum):** %s  \n", r.RetentionDeadline.Format("2006-01-02"))
	fmt.Fprintf(b, "**Audit Chain Integrity:** %s  \n", boolBadge(r.AuditChainIntegrityVerified))
	fmt.Fprintln(b)

	if len(r.GenerationNotes) > 0 {
		fmt.Fprintln(b, "### Generation Notes")
		fmt.Fprintln(b)
		for _, note := range r.GenerationNotes {
			fmt.Fprintf(b, "- %s\n", note)
		}
		fmt.Fprintln(b)
	}

	if len(r.LogEntries) == 0 {
		fmt.Fprintln(b, "_No log entries in period._")
		fmt.Fprintln(b)
		return
	}

	fmt.Fprintln(b, "### Log Entries")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "| Entry ID | Started | Ended | Actor | Decision | Risk | Verifiers |")
	fmt.Fprintln(b, "|----------|---------|-------|-------|----------|------|-----------|")
	for _, e := range r.LogEntries {
		verifiers := "—"
		if len(e.Verifiers) > 0 {
			parts := make([]string, 0, len(e.Verifiers))
			for _, v := range e.Verifiers {
				parts = append(parts, fmt.Sprintf("%s:%s", v.Kind, v.ID))
			}
			verifiers = strings.Join(parts, ", ")
		}
		fmt.Fprintf(b, "| `%s` | %s | %s | %s:%s | %s | %.2f | %s |\n",
			e.EntryID,
			e.StartedAt.UTC().Format("2006-01-02 15:04Z"),
			e.EndedAt.UTC().Format("2006-01-02 15:04Z"),
			e.Actor.Kind, e.Actor.ID,
			e.OutputDecision,
			e.RiskScore,
			verifiers,
		)
	}
	fmt.Fprintln(b)

	fmt.Fprintln(b, "### Auditor Notes")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "- Each log entry corresponds to one CGP governance decision tied to a software release.")
	fmt.Fprintln(b, "- The `auditChainHash` field (where present) anchors each entry into the hash-chained CGP audit trail; tampering invalidates the chain.")
	fmt.Fprintln(b, "- Empty `Verifiers` lists indicate autonomous (auto-approved) decisions; these should be cross-referenced against the actor's autonomy budget.")
	fmt.Fprintln(b, "- For machine-readable evidence, regenerate this report with `--format jsonl` (one entry per line) or `--format csv` (regulator portal ingestion).")
	fmt.Fprintln(b)
}

func boolBadge(b bool) string {
	if b {
		return "✓ verified"
	}
	return "✗ FAILED"
}

func writeAnnexIV(b *strings.Builder, r *AnnexIVReport) {
	if r == nil {
		fmt.Fprintln(b, "No Annex IV data available.")
		fmt.Fprintln(b)
		return
	}

	fmt.Fprintln(b, "## EU AI Act — Annex IV Technical Documentation")
	fmt.Fprintln(b)
	fmt.Fprintf(b, "**System:** %s  \n", r.SystemIdentifier)
	if r.SystemVersion != "" {
		fmt.Fprintf(b, "**System Version:** %s  \n", r.SystemVersion)
	}
	fmt.Fprintf(b, "**Retention Deadline (Article 11 — 10 years):** %s  \n", r.RetentionDeadline.Format("2006-01-02"))
	fmt.Fprintln(b)

	if len(r.GenerationNotes) > 0 {
		fmt.Fprintln(b, "> **Generation Notes**")
		fmt.Fprintln(b, ">")
		for _, note := range r.GenerationNotes {
			fmt.Fprintf(b, "> - %s\n", note)
		}
		fmt.Fprintln(b)
	}

	// §1
	fmt.Fprintln(b, "### §1 — General Description")
	fmt.Fprintln(b)
	fmt.Fprintf(b, "**Intended Purpose:** %s\n\n", r.GeneralDescription.IntendedPurpose)
	listKV(b, "Hardware Environments", r.GeneralDescription.HardwareEnvironments)
	listKV(b, "Deployment Forms", r.GeneralDescription.DeploymentForms)
	listKV(b, "User Interfaces", r.GeneralDescription.UserInterfaces)
	listKV(b, "Languages Supported", r.GeneralDescription.Languages)

	// §2
	fmt.Fprintln(b, "### §2 — Detailed Description of System Elements")
	fmt.Fprintln(b)
	listKV(b, "Development Methods", r.DetailedDescription.DevelopmentMethods)
	listKV(b, "Design Specifications", r.DetailedDescription.DesignSpecifications)
	fmt.Fprintf(b, "**System Architecture:** %s\n\n", r.DetailedDescription.SystemArchitecture)
	if r.DetailedDescription.ComputationalResources != "" {
		fmt.Fprintf(b, "**Computational Resources:** %s\n\n", r.DetailedDescription.ComputationalResources)
	}
	listKV(b, "Data Requirements", r.DetailedDescription.DataRequirements)
	listKV(b, "Human Oversight Measures", r.DetailedDescription.HumanOversightMeasures)
	listKV(b, "Predetermined Changes", r.DetailedDescription.PredeterminedChanges)
	fmt.Fprintf(b, "**CGP Protocol Version:** %s  \n", r.DetailedDescription.CGPProtocolVersion)
	if r.DetailedDescription.RiskModelVersion != "" {
		fmt.Fprintf(b, "**Risk Model Version:** %s  \n", r.DetailedDescription.RiskModelVersion)
	}
	fmt.Fprintln(b)

	// §3
	fmt.Fprintln(b, "### §3 — Monitoring, Functioning, and Control")
	fmt.Fprintln(b)
	listKV(b, "Monitoring Mechanisms", r.MonitoringControl.MonitoringMechanisms)
	listKV(b, "Control Interfaces", r.MonitoringControl.ControlInterfaces)
	fmt.Fprintf(b, "**Audit Trail Location:** %s  \n", r.MonitoringControl.AuditTrailLocation)
	fmt.Fprintf(b, "**Audit Chain Algorithm:** %s  \n", r.MonitoringControl.AuditChainAlgorithm)
	fmt.Fprintln(b)

	// §4
	fmt.Fprintln(b, "### §4 — Risk Management System")
	fmt.Fprintln(b)
	fmt.Fprintf(b, "**Risk Evaluation Method:** %s\n\n", r.RiskManagement.RiskEvaluationMethod)
	if len(r.RiskManagement.IdentifiedRisks) > 0 {
		fmt.Fprintln(b, "**Identified Risks**")
		fmt.Fprintln(b)
		fmt.Fprintln(b, "| Category | Severity | Occurrences | Avg Score |")
		fmt.Fprintln(b, "|----------|----------|-------------|-----------|")
		for _, risk := range r.RiskManagement.IdentifiedRisks {
			fmt.Fprintf(b, "| %s | %s | %d | %.2f |\n", risk.Category, risk.Severity, risk.OccurrenceCount, risk.AverageScore)
		}
		fmt.Fprintln(b)
	}
	if len(r.RiskManagement.RiskMitigationControls) > 0 {
		fmt.Fprintln(b, "**Risk Mitigation Controls**")
		fmt.Fprintln(b)
		fmt.Fprintln(b, "| Control | Type | Description |")
		fmt.Fprintln(b, "|---------|------|-------------|")
		for _, c := range r.RiskManagement.RiskMitigationControls {
			fmt.Fprintf(b, "| %s | %s | %s |\n", c.Name, c.Type, c.Description)
		}
		fmt.Fprintln(b)
	}
	if r.RiskManagement.ResidualRiskRationale != "" {
		fmt.Fprintf(b, "**Residual Risk Rationale:** %s\n\n", r.RiskManagement.ResidualRiskRationale)
	}

	// §5
	fmt.Fprintln(b, "### §5 — Lifecycle Changes")
	fmt.Fprintln(b)
	if len(r.LifecycleChanges) == 0 {
		fmt.Fprintln(b, "_No lifecycle changes recorded in period._")
		fmt.Fprintln(b)
	} else {
		fmt.Fprintln(b, "| Date | Version | Change Type | Decision | Outcome | Risk | Actor |")
		fmt.Fprintln(b, "|------|---------|-------------|----------|---------|------|-------|")
		for _, c := range r.LifecycleChanges {
			fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %.2f | %s |\n",
				c.Timestamp.UTC().Format("2006-01-02 15:04Z"),
				c.Version, c.ChangeType, c.Decision, c.Outcome, c.RiskScore, c.Actor)
		}
		fmt.Fprintln(b)
	}

	// §6
	fmt.Fprintln(b, "### §6 — Harmonized Standards Applied")
	fmt.Fprintln(b)
	if len(r.HarmonizedStandards) == 0 {
		fmt.Fprintln(b, "_None._")
	} else {
		fmt.Fprintln(b, "| Framework | Version | Status | Controls |")
		fmt.Fprintln(b, "|-----------|---------|--------|----------|")
		for _, s := range r.HarmonizedStandards {
			fmt.Fprintf(b, "| %s | %s | %s | %s |\n", s.Framework, s.Version, s.Status, strings.Join(s.Controls, "; "))
		}
	}
	fmt.Fprintln(b)

	// §7
	fmt.Fprintln(b, "### §7 — EU Declaration of Conformity (Scaffold)")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "> ⚠️ **Provider must complete the fields marked TODO below before submission.** Article 47 of Regulation 2024/1689 requires a signed declaration; Relicta cannot generate signatures or legal entity details.")
	fmt.Fprintln(b)
	fmt.Fprintf(b, "- **Provider Name:** %s\n", emptyOrTODO(r.ConformityDeclaration.ProviderName))
	fmt.Fprintf(b, "- **Provider Address:** %s\n", emptyOrTODO(r.ConformityDeclaration.ProviderAddress))
	fmt.Fprintf(b, "- **System Identifier:** %s\n", r.ConformityDeclaration.SystemIdentifier)
	fmt.Fprintf(b, "- **Unique Identifier:** %s\n", emptyOrTODO(r.ConformityDeclaration.UniqueIdentifier))
	fmt.Fprintf(b, "- **Standards Applied:** %s\n", strings.Join(r.ConformityDeclaration.StandardsApplied, "; "))
	fmt.Fprintf(b, "- **Notified Body:** %s\n", emptyOrTODO(r.ConformityDeclaration.NotifiedBody))
	fmt.Fprintf(b, "- **Date of Declaration:** %s\n", r.ConformityDeclaration.DateOfDeclaration)
	fmt.Fprintf(b, "- **Signatory:** %s\n", emptyOrTODO(r.ConformityDeclaration.Signatory))
	fmt.Fprintf(b, "- **Signatory Role:** %s\n", emptyOrTODO(r.ConformityDeclaration.SignatoryRole))
	fmt.Fprintln(b)

	// §8
	fmt.Fprintln(b, "### §8 — Post-Market Monitoring")
	fmt.Fprintln(b)
	fmt.Fprintf(b, "**Monitoring Plan:** %s\n\n", r.PostMarketMonitoring.MonitoringPlan)
	fmt.Fprintln(b, "| Metric | Value |")
	fmt.Fprintln(b, "|--------|-------|")
	fmt.Fprintf(b, "| Total Incidents | %d |\n", r.PostMarketMonitoring.TotalIncidents)
	fmt.Fprintf(b, "| Average Resolution (hours) | %.2f |\n", r.PostMarketMonitoring.AverageResolutionHrs)
	fmt.Fprintf(b, "| Change Failure Rate | %.2f%% |\n", r.PostMarketMonitoring.ChangeFailureRate*100)
	fmt.Fprintln(b)

	if len(r.PostMarketMonitoring.IncidentRecords) > 0 {
		fmt.Fprintln(b, "**Incident Records**")
		fmt.Fprintln(b)
		fmt.Fprintln(b, "| Incident ID | Release | Version | Type | Severity | Detected | Resolved |")
		fmt.Fprintln(b, "|-------------|---------|---------|------|----------|----------|----------|")
		for _, inc := range r.PostMarketMonitoring.IncidentRecords {
			resolved := "—"
			if inc.ResolvedAt != nil {
				resolved = inc.ResolvedAt.UTC().Format("2006-01-02 15:04Z")
			}
			fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %s | %s |\n",
				inc.IncidentID, inc.ReleaseID, inc.Version, inc.Type, inc.Severity,
				inc.DetectedAt.UTC().Format("2006-01-02 15:04Z"), resolved)
		}
		fmt.Fprintln(b)
	}
}

func listKV(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "**%s:**\n\n", title)
	for _, item := range items {
		fmt.Fprintf(b, "- %s\n", item)
	}
	fmt.Fprintln(b)
}

func emptyOrTODO(s string) string {
	if s == "" {
		return "_TODO — provider must complete_"
	}
	return s
}

func writeHeader(b *strings.Builder, r *Report) {
	title := ""
	switch r.Type {
	case ReportDORA:
		title = "DORA Metrics Report"
	case ReportSOC2:
		title = "SOC 2 Change Management Evidence"
	case ReportSummary:
		title = "Governance Summary Report"
	}

	fmt.Fprintf(b, "# %s\n\n", title)
	fmt.Fprintf(b, "**Period:** %s\n\n", r.Period.Label)
	if r.Repository != "" {
		fmt.Fprintf(b, "**Repository:** %s\n\n", r.Repository)
	}
	fmt.Fprintf(b, "**Generated:** %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintln(b, "---")
	fmt.Fprintln(b)
}

func writeFooter(b *strings.Builder, r *Report) {
	fmt.Fprintln(b, "---")
	fmt.Fprintln(b)
	fmt.Fprintf(b, "*Report generated by Relicta Compliance Reporter at %s*\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC"))
}

func writeDORA(b *strings.Builder, r *DORAReport) {
	if r == nil {
		fmt.Fprintln(b, "No DORA data available.")
		fmt.Fprintln(b)
		return
	}

	fmt.Fprintf(b, "**Overall Classification: %s**\n\n", strings.ToUpper(r.Classification))

	// Deployment Frequency
	fmt.Fprintln(b, "## Deployment Frequency")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "| Metric | Value |")
	fmt.Fprintln(b, "|--------|-------|")
	fmt.Fprintf(b, "| Total Deployments | %d |\n", r.DeploymentFrequency.TotalDeployments)
	fmt.Fprintf(b, "| Per Day | %.2f |\n", r.DeploymentFrequency.PerDay)
	fmt.Fprintf(b, "| Per Week | %.2f |\n", r.DeploymentFrequency.PerWeek)
	fmt.Fprintf(b, "| Classification | %s |\n", r.DeploymentFrequency.Classification)
	fmt.Fprintln(b)

	// Lead Time
	fmt.Fprintln(b, "## Lead Time for Changes")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "| Metric | Value |")
	fmt.Fprintln(b, "|--------|-------|")
	fmt.Fprintf(b, "| Average | %.1f hours |\n", r.LeadTimeForChanges.AverageHours)
	fmt.Fprintf(b, "| Median | %.1f hours |\n", r.LeadTimeForChanges.MedianHours)
	fmt.Fprintf(b, "| P95 | %.1f hours |\n", r.LeadTimeForChanges.P95Hours)
	fmt.Fprintf(b, "| Classification | %s |\n", r.LeadTimeForChanges.Classification)
	fmt.Fprintln(b)

	// MTTR
	fmt.Fprintln(b, "## Mean Time to Recovery (MTTR)")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "| Metric | Value |")
	fmt.Fprintln(b, "|--------|-------|")
	fmt.Fprintf(b, "| Average | %.1f hours |\n", r.MTTR.AverageHours)
	fmt.Fprintf(b, "| Median | %.1f hours |\n", r.MTTR.MedianHours)
	fmt.Fprintf(b, "| Total Incidents | %d |\n", r.MTTR.TotalIncidents)
	fmt.Fprintf(b, "| Resolved | %d |\n", r.MTTR.ResolvedCount)
	fmt.Fprintf(b, "| Classification | %s |\n", r.MTTR.Classification)
	fmt.Fprintln(b)

	// Change Failure Rate
	fmt.Fprintln(b, "## Change Failure Rate")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "| Metric | Value |")
	fmt.Fprintln(b, "|--------|-------|")
	fmt.Fprintf(b, "| Total Changes | %d |\n", r.ChangeFailureRate.TotalChanges)
	fmt.Fprintf(b, "| Failed Changes | %d |\n", r.ChangeFailureRate.FailedChanges)
	fmt.Fprintf(b, "| Failure Rate | %.1f%% |\n", r.ChangeFailureRate.Rate*100)
	fmt.Fprintf(b, "| Classification | %s |\n", r.ChangeFailureRate.Classification)
	fmt.Fprintln(b)
}

func writeSOC2(b *strings.Builder, r *SOC2Report) {
	if r == nil {
		fmt.Fprintln(b, "No SOC 2 data available.")
		fmt.Fprintln(b)
		return
	}

	// Change Log
	fmt.Fprintln(b, "## Change Request Log")
	fmt.Fprintln(b)
	if len(r.ChangeLog) == 0 {
		fmt.Fprintln(b, "No changes recorded in this period.")
	} else {
		fmt.Fprintln(b, "| ID | Version | Date | Actor | Risk Score | Decision | Outcome |")
		fmt.Fprintln(b, "|----|---------|------|-------|------------|----------|---------|")
		for _, c := range r.ChangeLog {
			fmt.Fprintf(b, "| %s | %s | %s | %s (%s) | %.2f | %s | %s |\n",
				c.ID, c.Version, c.Date.Format("2006-01-02"),
				c.Actor, c.ActorKind, c.RiskScore, c.Decision, c.Outcome)
		}
	}
	fmt.Fprintln(b)

	// Approval Evidence
	fmt.Fprintln(b, "## Approval Evidence")
	fmt.Fprintln(b)
	if len(r.ApprovalEvidence) == 0 {
		fmt.Fprintln(b, "No approval records in this period.")
	} else {
		fmt.Fprintln(b, "| Release | Version | Decision | Risk Score | Date |")
		fmt.Fprintln(b, "|---------|---------|----------|------------|------|")
		for _, a := range r.ApprovalEvidence {
			fmt.Fprintf(b, "| %s | %s | %s | %.2f | %s |\n",
				a.ReleaseID, a.Version, a.DecisionType, a.RiskScore,
				a.ApprovedAt.Format("2006-01-02 15:04"))
		}
	}
	fmt.Fprintln(b)

	// Risk Assessments
	fmt.Fprintln(b, "## Risk Assessment Evidence")
	fmt.Fprintln(b)
	if len(r.RiskAssessments) == 0 {
		fmt.Fprintln(b, "No risk assessments in this period.")
	} else {
		for _, ra := range r.RiskAssessments {
			fmt.Fprintf(b, "### %s (v%s) - Risk: %s (%.2f)\n\n", ra.ReleaseID, ra.Version, ra.RiskLevel, ra.RiskScore)
			if len(ra.RiskFactors) > 0 {
				fmt.Fprintln(b, "| Category | Severity | Score | Description |")
				fmt.Fprintln(b, "|----------|----------|-------|-------------|")
				for _, rf := range ra.RiskFactors {
					fmt.Fprintf(b, "| %s | %s | %.2f | %s |\n",
						rf.Category, rf.Severity, rf.Score, rf.Description)
				}
			}
			fmt.Fprintln(b)
		}
	}

	// Incident Response
	fmt.Fprintln(b, "## Incident Response")
	fmt.Fprintln(b)
	if len(r.IncidentResponse) == 0 {
		fmt.Fprintln(b, "No incidents recorded in this period.")
	} else {
		fmt.Fprintln(b, "| Incident | Release | Version | Type | Severity | Detected | Resolution Time |")
		fmt.Fprintln(b, "|----------|---------|---------|------|----------|----------|-----------------|")
		for _, inc := range r.IncidentResponse {
			resolution := "unresolved"
			if inc.TimeToResolve > 0 {
				resolution = inc.TimeToResolve.String()
			}
			fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %s | %s |\n",
				inc.IncidentID, inc.ReleaseID, inc.Version,
				inc.Type, inc.Severity,
				inc.DetectedAt.Format("2006-01-02 15:04"), resolution)
		}
	}
	fmt.Fprintln(b)

	// Policy Compliance
	fmt.Fprintln(b, "## Policy Compliance")
	fmt.Fprintln(b)
	if len(r.PolicyCompliance) == 0 {
		fmt.Fprintln(b, "No policy evaluations in this period.")
	} else {
		fmt.Fprintln(b, "| Release | Version | Decision | Risk Score | Rationale |")
		fmt.Fprintln(b, "|---------|---------|----------|------------|-----------|")
		for _, pc := range r.PolicyCompliance {
			rationale := strings.Join(pc.Rationale, "; ")
			if len(rationale) > 80 {
				rationale = rationale[:77] + "..."
			}
			fmt.Fprintf(b, "| %s | %s | %s | %.2f | %s |\n",
				pc.ReleaseID, pc.Version, pc.Decision, pc.RiskScore, rationale)
		}
	}
	fmt.Fprintln(b)
}

func writeSummary(b *strings.Builder, r *SummaryReport) {
	if r == nil {
		fmt.Fprintln(b, "No summary data available.")
		fmt.Fprintln(b)
		return
	}

	fmt.Fprintf(b, "## Overview\n\n")
	fmt.Fprintf(b, "**Total Releases:** %d\n\n", r.TotalReleases)

	// Risk Distribution
	fmt.Fprintln(b, "## Risk Score Distribution")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "| Level | Count |")
	fmt.Fprintln(b, "|-------|-------|")
	fmt.Fprintf(b, "| Low (< 0.4) | %d |\n", r.RiskDistribution.Low)
	fmt.Fprintf(b, "| Medium (0.4-0.6) | %d |\n", r.RiskDistribution.Medium)
	fmt.Fprintf(b, "| High (0.6-0.8) | %d |\n", r.RiskDistribution.High)
	fmt.Fprintf(b, "| Critical (>= 0.8) | %d |\n", r.RiskDistribution.Critical)
	fmt.Fprintln(b)

	// Approval Breakdown
	fmt.Fprintln(b, "## Approval Breakdown")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "| Type | Count |")
	fmt.Fprintln(b, "|------|-------|")
	fmt.Fprintf(b, "| Auto-Approved | %d |\n", r.ApprovalBreakdown.AutoApproved)
	fmt.Fprintf(b, "| Human-Approved | %d |\n", r.ApprovalBreakdown.HumanApproved)
	fmt.Fprintf(b, "| Rejected | %d |\n", r.ApprovalBreakdown.Rejected)
	fmt.Fprintln(b)

	// Top Risk Factors
	if len(r.TopRiskFactors) > 0 {
		fmt.Fprintln(b, "## Top Risk Factors")
		fmt.Fprintln(b)
		fmt.Fprintln(b, "| Category | Occurrences | Avg Score |")
		fmt.Fprintln(b, "|----------|-------------|-----------|")
		for _, rf := range r.TopRiskFactors {
			fmt.Fprintf(b, "| %s | %d | %.2f |\n", rf.Category, rf.Count, rf.AvgScore)
		}
		fmt.Fprintln(b)
	}

	// Actor Activity
	if len(r.ActorActivity) > 0 {
		fmt.Fprintln(b, "## Actor Activity")
		fmt.Fprintln(b)
		fmt.Fprintln(b, "| Actor | Kind | Releases | Success Rate | Avg Risk |")
		fmt.Fprintln(b, "|-------|------|----------|--------------|----------|")
		for _, a := range r.ActorActivity {
			fmt.Fprintf(b, "| %s | %s | %d | %.0f%% | %.2f |\n",
				a.ActorID, a.ActorKind, a.ReleaseCount,
				a.SuccessRate*100, a.AvgRiskScore)
		}
		fmt.Fprintln(b)
	}

	// Incident Summary
	fmt.Fprintln(b, "## Incident Summary")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "| Metric | Value |")
	fmt.Fprintln(b, "|--------|-------|")
	fmt.Fprintf(b, "| Total Incidents | %d |\n", r.IncidentSummary.TotalIncidents)
	fmt.Fprintf(b, "| Avg Resolution Time | %.1f hours |\n", r.IncidentSummary.AvgResolutionHrs)
	fmt.Fprintf(b, "| Incidents per Release | %.2f |\n", r.IncidentSummary.CorrelationRate)
	fmt.Fprintln(b)
}
