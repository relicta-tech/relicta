package drata

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/compliance"
)

// EvidenceType categorizes the artifact kind for Drata's evidence taxonomy.
type EvidenceType string

const (
	EvidenceTypeChangeManagement EvidenceType = "change_management"
	EvidenceTypeAccessReview     EvidenceType = "access_review"
	EvidenceTypeAuditLog         EvidenceType = "audit_log"
	EvidenceTypeRiskAssessment   EvidenceType = "risk_assessment"
)

// FrameworkMapping links Relicta evidence to Drata framework controls.
type FrameworkMapping struct {
	Framework string   `json:"framework"`
	Controls  []string `json:"controls"`
}

// Evidence is the Drata evidence payload.
type Evidence struct {
	Title            string             `json:"title"`
	Description      string             `json:"description"`
	Type             EvidenceType       `json:"type"`
	CollectedAt      time.Time          `json:"collectedAt"`
	Source           string             `json:"source"`
	SystemIdentifier string             `json:"systemIdentifier"`
	Frameworks       []FrameworkMapping `json:"frameworks,omitempty"`
	Data             map[string]any     `json:"data"`
	IntegrityHash    string             `json:"integrityHash,omitempty"`
}

// Validate ensures required fields are populated before push.
func (e *Evidence) Validate() error {
	if strings.TrimSpace(e.Title) == "" {
		return errors.New("title is required")
	}
	if e.Type == "" {
		return errors.New("type is required")
	}
	if e.CollectedAt.IsZero() {
		return errors.New("collectedAt is required")
	}
	if e.SystemIdentifier == "" {
		return errors.New("systemIdentifier is required")
	}
	return nil
}

// MapArticle12LogEntries converts an Article 12 report into Drata Evidence
// records. One entry per log entry to preserve the per-decision granularity
// auditors expect for Article 12 walkthroughs.
func MapArticle12LogEntries(report *compliance.Article12Report) []Evidence {
	if report == nil || len(report.LogEntries) == 0 {
		return nil
	}
	frameworks := defaultArticle12Frameworks()
	out := make([]Evidence, 0, len(report.LogEntries))
	for _, entry := range report.LogEntries {
		ev := Evidence{
			Title:            fmt.Sprintf("Governance Decision %s — %s", entry.Version, entry.OutputDecision),
			Description:      describeEntry(entry),
			Type:             EvidenceTypeAuditLog,
			CollectedAt:      entry.EventTimestamp,
			Source:           "relicta",
			SystemIdentifier: entry.SystemIdentifier,
			Frameworks:       frameworks,
			IntegrityHash:    entry.AuditChainHash,
			Data: map[string]any{
				"entryId":         entry.EntryID,
				"releaseId":       entry.ReleaseID,
				"version":         entry.Version,
				"startedAt":       entry.StartedAt,
				"endedAt":         entry.EndedAt,
				"actor":           entry.Actor,
				"verifiers":       entry.Verifiers,
				"outputDecision":  entry.OutputDecision,
				"outputRationale": entry.OutputRationale,
				"riskScore":       entry.RiskScore,
				"referenceData":   entry.ReferenceData,
				"inputData":       entry.InputData,
			},
		}
		out = append(out, ev)
	}
	return out
}

// MapSOC2 converts a SOC 2 compliance report into Drata Evidence records
// grouped by control area for SOC 2 walkthroughs.
func MapSOC2(report *compliance.Report) []Evidence {
	if report == nil || report.SOC2 == nil {
		return nil
	}

	systemID := report.Repository
	if systemID == "" {
		systemID = "default"
	}

	soc2Frameworks := []FrameworkMapping{
		{Framework: "SOC2", Controls: []string{"CC6.1", "CC7.1", "CC7.4", "CC8.1"}},
	}

	return []Evidence{
		{
			Title:            fmt.Sprintf("Change Log %s — %s", systemID, report.Period.Label),
			Description:      "Aggregated change log entries with actor attribution and decision outcomes for the reporting period.",
			Type:             EvidenceTypeChangeManagement,
			CollectedAt:      report.GeneratedAt,
			Source:           "relicta",
			SystemIdentifier: systemID,
			Frameworks:       soc2Frameworks,
			Data: map[string]any{
				"period":    report.Period,
				"changeLog": report.SOC2.ChangeLog,
			},
		},
		{
			Title:            fmt.Sprintf("Approval Evidence %s — %s", systemID, report.Period.Label),
			Description:      "Approver identity, timestamps, and decision types for releases in the reporting period.",
			Type:             EvidenceTypeAccessReview,
			CollectedAt:      report.GeneratedAt,
			Source:           "relicta",
			SystemIdentifier: systemID,
			Frameworks:       soc2Frameworks,
			Data: map[string]any{
				"period":    report.Period,
				"approvals": report.SOC2.ApprovalEvidence,
			},
		},
		{
			Title:            fmt.Sprintf("Risk Assessment %s — %s", systemID, report.Period.Label),
			Description:      "Per-release risk evaluation details with factor-level breakdown.",
			Type:             EvidenceTypeRiskAssessment,
			CollectedAt:      report.GeneratedAt,
			Source:           "relicta",
			SystemIdentifier: systemID,
			Frameworks:       soc2Frameworks,
			Data: map[string]any{
				"period":      report.Period,
				"assessments": report.SOC2.RiskAssessments,
			},
		},
	}
}

// describeEntry produces a short human-readable description for an Article 12
// log entry suitable for display in Drata's evidence detail view.
func describeEntry(entry compliance.Article12LogEntry) string {
	verifierCount := len(entry.Verifiers)
	verifierText := "no verifiers (autonomous decision)"
	switch verifierCount {
	case 1:
		verifierText = "1 verifier"
	default:
		if verifierCount > 1 {
			verifierText = fmt.Sprintf("%d verifiers", verifierCount)
		}
	}
	return fmt.Sprintf(
		"Article 12 governance log entry. Actor: %s:%s. Decision: %s. Risk: %.2f. %s.",
		entry.Actor.Kind, entry.Actor.ID,
		entry.OutputDecision, entry.RiskScore,
		verifierText,
	)
}

// defaultArticle12Frameworks returns the framework mapping applied to every
// Article 12 log entry.
func defaultArticle12Frameworks() []FrameworkMapping {
	return []FrameworkMapping{
		{Framework: "EU-AI-Act", Controls: []string{"Article 12", "Article 26(6)"}},
		{Framework: "SOC2", Controls: []string{"CC7.1", "CC8.1"}},
		{Framework: "ISO-27001", Controls: []string{"A.5.10", "A.8.32"}},
	}
}
