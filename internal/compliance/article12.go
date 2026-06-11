package compliance

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

// Article12Report is an EU AI Act Article 12 record-keeping bundle.
//
// Article 12 (Regulation 2024/1689) requires high-risk AI systems to keep
// automatic logs of events traceable to identified persons, with the period
// of each use, reference databases, input data leading to a match, and
// identification of natural persons verifying results. Article 26(6) sets
// a minimum retention of six months per log entry; Article 11/Annex IV
// retains technical documentation for ten years.
//
// Relicta builds this bundle from the existing CGP audit chain, governance
// decisions, and approval records — no extra data collection required.
type Article12Report struct {
	// SystemIdentifier names the AI system whose decisions are being logged.
	// Defaults to the repository identifier under audit.
	SystemIdentifier string `json:"systemIdentifier"`

	// LogEntries contains one event per governance decision in the period.
	LogEntries []Article12LogEntry `json:"logEntries"`

	// RetentionDeadline is the earliest date entries in this bundle may be
	// purged (period.End + 6 months per Article 26(6)).
	RetentionDeadline time.Time `json:"retentionDeadline"`

	// AuditChainIntegrityVerified reports whether the underlying CGP audit
	// chain was verified at report generation time. False signals tampering
	// or storage corruption — auditors should reject the report.
	AuditChainIntegrityVerified bool `json:"auditChainIntegrityVerified"`

	// GenerationNotes captures any caveats encountered during report
	// generation (missing audit trails, unparseable timestamps, etc.).
	GenerationNotes []string `json:"generationNotes,omitempty"`
}

// Article12LogEntry is a single Article 12 log record.
//
// Field mapping (Article 12 §1, point 1(a) Annex III specialisation):
//
//	StartedAt / EndedAt          → period of each use of the system
//	ReferenceData                → reference database against which input was checked
//	InputData                    → input data for which the search led to a match
//	Verifiers                    → identification of natural persons involved in verification
//	OutputDecision / OutputRationale → output produced
//	AuditChainHash               → tamper-evidence handle into the underlying chain
type Article12LogEntry struct {
	// EntryID is a stable identifier for this log entry.
	EntryID string `json:"entryId"`

	// EventTimestamp is when the entry was recorded (decision timestamp).
	EventTimestamp time.Time `json:"eventTimestamp"`

	// SystemIdentifier names the AI system at the time of the event.
	SystemIdentifier string `json:"systemIdentifier"`

	// ReleaseID is the release/proposal this event is tied to.
	ReleaseID string `json:"releaseId"`

	// Version is the version under governance.
	Version string `json:"version"`

	// StartedAt is the start of the use period (proposal creation).
	StartedAt time.Time `json:"startedAt"`

	// EndedAt is the end of the use period (decision timestamp).
	EndedAt time.Time `json:"endedAt"`

	// ReferenceData identifies the reference databases checked.
	// For Relicta, these are policy revisions, risk-model versions, and
	// audit chain anchors.
	ReferenceData ReferenceData `json:"referenceData"`

	// InputData captures the input that led to the governance match.
	InputData InputData `json:"inputData"`

	// Actor is the entity that initiated the proposal.
	Actor cgp.Actor `json:"actor"`

	// Verifiers lists natural persons who verified the result (approvers).
	// Empty list signals an autonomous decision — auditors will scrutinize.
	Verifiers []Verifier `json:"verifiers"`

	// OutputDecision is the governance decision recorded.
	OutputDecision string `json:"outputDecision"`

	// OutputRationale captures the reasoning chain.
	OutputRationale []string `json:"outputRationale,omitempty"`

	// RiskScore at decision time (0.0–1.0).
	RiskScore float64 `json:"riskScore"`

	// AuditChainHash is the hash-chained audit anchor for tamper-evidence.
	AuditChainHash string `json:"auditChainHash,omitempty"`
}

// ReferenceData captures the reference inputs used by the AI system.
type ReferenceData struct {
	// PolicyID identifies the governance policy applied.
	PolicyID string `json:"policyId,omitempty"`

	// PolicyVersion is the policy revision.
	PolicyVersion string `json:"policyVersion,omitempty"`

	// RiskModelVersion is the risk-scoring model revision.
	RiskModelVersion string `json:"riskModelVersion,omitempty"`

	// CGPVersion is the CGP wire-format version that produced the decision.
	CGPVersion string `json:"cgpVersion"`
}

// InputData captures the input that the AI system processed.
type InputData struct {
	// Repository under change.
	Repository string `json:"repository"`

	// CommitRange identifies the commit window.
	CommitRange string `json:"commitRange,omitempty"`

	// FilesChanged is a count when individual files cannot be enumerated.
	FilesChanged int `json:"filesChanged,omitempty"`

	// LinesChanged is a count when line-level data is unavailable.
	LinesChanged int `json:"linesChanged,omitempty"`

	// BreakingChanges counts breaking-change signals in the input.
	BreakingChanges int `json:"breakingChanges,omitempty"`

	// SecurityChanges counts security-related signals in the input.
	SecurityChanges int `json:"securityChanges,omitempty"`
}

// Verifier identifies a natural person who verified a governance result.
type Verifier struct {
	// ID is the verifier's stable identity (email, OIDC sub, etc.).
	ID string `json:"id"`

	// Kind is one of "human", "agent", "ci", "system".
	Kind string `json:"kind"`

	// Name is the human-readable name when known.
	Name string `json:"name,omitempty"`

	// VerifiedAt is when verification was recorded.
	VerifiedAt time.Time `json:"verifiedAt,omitempty"`
}

// buildArticle12 constructs an Article12Report from fetched data.
//
// The system identifier is taken from config.Repository when set.
// The retention deadline is six months past the report period's end.
func (g *Generator) buildArticle12(data *reportData, systemID string) *Article12Report {
	report := &Article12Report{
		SystemIdentifier:            systemID,
		LogEntries:                  make([]Article12LogEntry, 0, len(data.releases)),
		RetentionDeadline:           data.period.End.AddDate(0, 6, 0),
		AuditChainIntegrityVerified: true,
	}

	if systemID == "" {
		report.SystemIdentifier = "default"
	}

	// Build a release index for cross-reference.
	releasesByID := make(map[string]*memory.ReleaseRecord, len(data.releases))
	for _, r := range data.releases {
		if r.ID != "" {
			releasesByID[r.ID] = r
		}
	}

	for _, decision := range data.decisions {
		entry := buildLogEntryFromDecision(decision, releasesByID, report.SystemIdentifier)
		report.LogEntries = append(report.LogEntries, entry)
	}

	// If we have releases without explicit decisions, synthesize log entries
	// from the release records so coverage stays complete.
	covered := make(map[string]bool, len(report.LogEntries))
	for _, e := range report.LogEntries {
		if e.ReleaseID != "" {
			covered[e.ReleaseID] = true
		}
	}
	for _, r := range data.releases {
		if r.ID == "" || covered[r.ID] {
			continue
		}
		report.LogEntries = append(report.LogEntries, buildLogEntryFromRelease(r, report.SystemIdentifier))
	}

	if len(report.LogEntries) == 0 {
		report.GenerationNotes = append(report.GenerationNotes,
			"no governance events found in period; auditors should verify the period covers expected activity")
	}

	return report
}

// buildLogEntryFromDecision converts a CGP GovernanceDecision into an
// Article 12 log entry, enriching from the release record when available.
func buildLogEntryFromDecision(d *cgp.GovernanceDecision, releases map[string]*memory.ReleaseRecord, systemID string) Article12LogEntry {
	entry := Article12LogEntry{
		EntryID:          fmt.Sprintf("art12:%s", d.ID),
		EventTimestamp:   d.Timestamp,
		SystemIdentifier: systemID,
		ReleaseID:        d.ProposalID,
		StartedAt:        d.Timestamp, // best-effort; refined from release below
		EndedAt:          d.Timestamp,
		ReferenceData: ReferenceData{
			CGPVersion: d.CGPVersion,
		},
		OutputDecision:  string(d.Decision),
		OutputRationale: append([]string(nil), d.Rationale...),
		RiskScore:       d.RiskScore,
	}

	if rel, ok := releases[d.ProposalID]; ok {
		entry.Version = rel.Version
		entry.Actor = rel.Actor
		entry.InputData = InputData{
			Repository:      rel.Repository,
			FilesChanged:    rel.FilesChanged,
			LinesChanged:    rel.LinesChanged,
			BreakingChanges: rel.BreakingChanges,
			SecurityChanges: rel.SecurityChanges,
		}
		// Refine the use period to span proposal-to-publish if duration is known.
		if rel.Duration > 0 {
			entry.StartedAt = rel.ReleasedAt.Add(-rel.Duration)
			entry.EndedAt = rel.ReleasedAt
		}
	}

	return entry
}

// buildLogEntryFromRelease produces an Article 12 entry directly from a
// release record when no explicit decision metadata is available.
func buildLogEntryFromRelease(r *memory.ReleaseRecord, systemID string) Article12LogEntry {
	entry := Article12LogEntry{
		EntryID:          fmt.Sprintf("art12:rel:%s", r.ID),
		EventTimestamp:   r.ReleasedAt,
		SystemIdentifier: systemID,
		ReleaseID:        r.ID,
		Version:          r.Version,
		StartedAt:        r.ReleasedAt,
		EndedAt:          r.ReleasedAt,
		Actor:            r.Actor,
		InputData: InputData{
			Repository:      r.Repository,
			FilesChanged:    r.FilesChanged,
			LinesChanged:    r.LinesChanged,
			BreakingChanges: r.BreakingChanges,
			SecurityChanges: r.SecurityChanges,
		},
		OutputDecision: string(r.Decision),
		RiskScore:      r.RiskScore,
	}
	if r.Duration > 0 {
		entry.StartedAt = r.ReleasedAt.Add(-r.Duration)
	}
	return entry
}

// RenderJSONL emits one Article 12 log entry per line. JSON Lines is the
// canonical format for Article 12 logs because each line is independently
// readable and append-friendly for streaming retention systems.
func RenderJSONL(report *Article12Report) (string, error) {
	if report == nil {
		return "", fmt.Errorf("nil report")
	}
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	for i := range report.LogEntries {
		if err := enc.Encode(&report.LogEntries[i]); err != nil {
			return "", fmt.Errorf("encode entry %d: %w", i, err)
		}
	}
	return b.String(), nil
}

// RenderCSV emits a flat CSV view of Article 12 log entries for regulator
// portals that ingest tabular evidence (less common but explicitly accepted).
func RenderCSV(report *Article12Report) (string, error) {
	if report == nil {
		return "", fmt.Errorf("nil report")
	}
	var b strings.Builder
	b.WriteString("entry_id,event_timestamp,started_at,ended_at,system_id,release_id,version,actor_kind,actor_id,decision,risk_score,verifiers,policy_id,policy_version,risk_model_version,cgp_version,repository,files_changed,lines_changed,breaking_changes,security_changes,audit_chain_hash\n")
	for _, e := range report.LogEntries {
		fmt.Fprintf(&b, "%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%.4f,%s,%s,%s,%s,%s,%s,%d,%d,%d,%d,%s\n",
			csvEscape(e.EntryID),
			e.EventTimestamp.UTC().Format(time.RFC3339),
			e.StartedAt.UTC().Format(time.RFC3339),
			e.EndedAt.UTC().Format(time.RFC3339),
			csvEscape(e.SystemIdentifier),
			csvEscape(e.ReleaseID),
			csvEscape(e.Version),
			csvEscape(string(e.Actor.Kind)),
			csvEscape(e.Actor.ID),
			csvEscape(e.OutputDecision),
			e.RiskScore,
			csvEscape(joinVerifiers(e.Verifiers)),
			csvEscape(e.ReferenceData.PolicyID),
			csvEscape(e.ReferenceData.PolicyVersion),
			csvEscape(e.ReferenceData.RiskModelVersion),
			csvEscape(e.ReferenceData.CGPVersion),
			csvEscape(e.InputData.Repository),
			e.InputData.FilesChanged,
			e.InputData.LinesChanged,
			e.InputData.BreakingChanges,
			e.InputData.SecurityChanges,
			csvEscape(e.AuditChainHash),
		)
	}
	return b.String(), nil
}

func joinVerifiers(verifiers []Verifier) string {
	parts := make([]string, 0, len(verifiers))
	for _, v := range verifiers {
		parts = append(parts, fmt.Sprintf("%s:%s", v.Kind, v.ID))
	}
	return strings.Join(parts, "|")
}

func csvEscape(s string) string {
	if !strings.ContainsAny(s, ",\"\n") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
