package compliance

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/v5/internal/cgp"
	"github.com/relicta-tech/relicta/v5/internal/cgp/memory"
)

const article12EvidenceScope = "Release-governance evidence only: governance-decision or release-record timestamps, release-process duration when available, CGP protocol version, repository change metadata and recorded rationale. Decisions are discovered through release records in the selected period; standalone decisions without a matching release record are not enumerated. Approval-event identities and audit-chain hashes are not included, and audit-chain integrity is not checked by this export. This export does not record each AI system use or automatically generated operational logs and cannot by itself establish that Article 12 logging duties are met."

// Article12Report is a release-governance evidence export mapped to selected
// EU AI Act Article 12 concepts. It is not an Article 12 operational log.
//
// Article 12 (Regulation 2024/1689) requires high-risk AI systems to keep
// automatic logs of events traceable to identified persons, with the period
// of each use, reference databases, input data leading to a match, and
// identification of natural persons verifying results. Articles 19(1) and
// 26(6) set minimum six-month periods for certain logs retained by providers
// and deployers respectively, subject to intended purpose and other law.
//
// Relicta builds this export from governance decisions and release records. It
// does not load or verify the CGP audit chain or include approval events. It
// does not record each AI system use or
// automatically generated operational logs and cannot, by itself, establish
// that Article 12 logging duties are met.
type Article12Report struct {
	// SystemIdentifier names the target-system/repository identity associated
	// with the records.
	SystemIdentifier string `json:"systemIdentifier"`

	// LogEntries contains governance decisions, with release records filling
	// gaps when explicit decisions are unavailable.
	LogEntries []Article12LogEntry `json:"logEntries"`

	// EvidenceScope states what this export contains and what it does not cover.
	EvidenceScope string `json:"evidenceScope"`

	// SuggestedRetentionDate is six months after the report period end. It is
	// an export-based reference only; actual duties depend on role, intended
	// purpose, and other applicable law.
	SuggestedRetentionDate time.Time `json:"suggestedRetentionDate"`

	// AuditChainIntegrityStatus reports whether the underlying chain was
	// checked. This report generator does not load or verify the CGP audit chain.
	AuditChainIntegrityStatus string `json:"auditChainIntegrityStatus"`

	// GenerationNotes captures any caveats encountered during report
	// generation (missing audit trails, unparseable timestamps, etc.).
	GenerationNotes []string `json:"generationNotes,omitempty"`
}

// Article12LogEntry is a single release-governance evidence record mapped to
// selected Article 12 concepts.
//
// Relicta evidence fields (not a one-to-one mapping to statutory fields):
//
//	ReleaseProcessStartedAt      → release process start inferred from recorded duration when available
//	ReleaseRecordedAt            → release record or governance decision timestamp
//	ReferenceData                → CGP protocol version
//	InputData                    → repository and change metadata/counts, not raw AI inputs
//	Verifiers                    → not populated by the current report generator
//	OutputDecision / OutputRationale → recorded governance outcome
//	AuditChainHash               → not populated by the current report generator
type Article12LogEntry struct {
	// EvidenceScope travels with each JSONL/CSV row so an individual record
	// retains its limitation when exported or uploaded separately.
	EvidenceScope string `json:"evidenceScope"`

	// EntryID is a stable identifier for this log entry.
	EntryID string `json:"entryId"`

	// EventTimestamp is the decision timestamp when available, otherwise the
	// release-record timestamp.
	EventTimestamp time.Time `json:"eventTimestamp"`

	// SystemIdentifier names the configured target-system/repository identity.
	SystemIdentifier string `json:"systemIdentifier"`

	// ReleaseID is the release/proposal this event is tied to.
	ReleaseID string `json:"releaseId"`

	// Version is the version under governance.
	Version string `json:"version"`

	// ReleaseProcessStartedAt is inferred from release time minus recorded
	// process duration when available; otherwise it equals the record timestamp.
	// It is not the start of an AI system use or necessarily proposal creation.
	ReleaseProcessStartedAt time.Time `json:"releaseProcessStartedAt"`

	// ReleaseRecordedAt is the release record timestamp or decision timestamp.
	ReleaseRecordedAt time.Time `json:"releaseRecordedAt"`

	// ReferenceData identifies governance references included in the source
	// record. The current report generator only populates CGPVersion.
	ReferenceData ReferenceData `json:"referenceData"`

	// InputData captures repository change metadata used by release governance.
	InputData InputData `json:"inputData"`

	// Actor is the entity recorded as responsible for the governed release.
	Actor cgp.Actor `json:"actor"`

	// Verifiers can list recorded governance approvers. The current report
	// generator does not populate approval events; an empty list means unknown.
	Verifiers []Verifier `json:"verifiers,omitempty"`

	// OutputDecision is the governance decision recorded.
	OutputDecision string `json:"outputDecision"`

	// OutputRationale captures the reasoning chain.
	OutputRationale []string `json:"outputRationale,omitempty"`

	// RiskScore at decision time (0.0–1.0).
	RiskScore float64 `json:"riskScore"`

	// AuditChainHash is a source-provided hash-chain anchor, when available.
	// The current report generator does not load audit-chain entries.
	AuditChainHash string `json:"auditChainHash,omitempty"`
}

// ReferenceData captures references used by release governance.
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

// InputData captures repository and change metadata, not raw AI-system input.
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
// The export-based suggested date is six months past the report period end;
// this is not a determination of a particular entity's legal obligations.
func (g *Generator) buildArticle12(data *reportData, systemID string) *Article12Report {
	report := &Article12Report{
		SystemIdentifier:          systemID,
		LogEntries:                make([]Article12LogEntry, 0, len(data.releases)),
		EvidenceScope:             article12EvidenceScope,
		SuggestedRetentionDate:    data.period.End.AddDate(0, 6, 0),
		AuditChainIntegrityStatus: "not_checked",
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
	for i := range report.LogEntries {
		report.LogEntries[i].EvidenceScope = report.EvidenceScope
	}
	report.GenerationNotes = append(report.GenerationNotes,
		"CGP audit-chain integrity is not checked by this report; run `relicta audit verify` separately",
		"approval-event identities and audit-chain hashes are not included; an empty verifier list does not establish an autonomous decision")

	if len(report.LogEntries) == 0 {
		report.GenerationNotes = append(report.GenerationNotes,
			"no governance events found in period; auditors should verify the period covers expected activity")
	}

	return report
}

// buildLogEntryFromDecision converts a CGP GovernanceDecision into a
// release-governance evidence record, enriching from the release when available.
func buildLogEntryFromDecision(d *cgp.GovernanceDecision, releases map[string]*memory.ReleaseRecord, systemID string) Article12LogEntry {
	entry := Article12LogEntry{
		EntryID:                 fmt.Sprintf("art12:%s", d.ID),
		EventTimestamp:          d.Timestamp,
		SystemIdentifier:        systemID,
		ReleaseID:               d.ProposalID,
		ReleaseProcessStartedAt: d.Timestamp,
		ReleaseRecordedAt:       d.Timestamp,
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
		// Derive the release-process start from recorded duration when known.
		if rel.Duration > 0 {
			entry.ReleaseProcessStartedAt = rel.ReleasedAt.Add(-rel.Duration)
			entry.ReleaseRecordedAt = rel.ReleasedAt
		}
	}

	return entry
}

// buildLogEntryFromRelease produces a governance evidence record directly from a
// release record when no explicit decision metadata is available.
func buildLogEntryFromRelease(r *memory.ReleaseRecord, systemID string) Article12LogEntry {
	entry := Article12LogEntry{
		EntryID:                 fmt.Sprintf("art12:rel:%s", r.ID),
		EventTimestamp:          r.ReleasedAt,
		SystemIdentifier:        systemID,
		ReleaseID:               r.ID,
		Version:                 r.Version,
		ReleaseProcessStartedAt: r.ReleasedAt,
		ReleaseRecordedAt:       r.ReleasedAt,
		Actor:                   r.Actor,
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
		entry.ReleaseProcessStartedAt = r.ReleasedAt.Add(-r.Duration)
	}
	return entry
}

// RenderJSONL emits one release-governance evidence record per line. Each row
// carries its evidence scope so it is not mistaken for an operational AI log.
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

// RenderCSV emits a flat CSV view of release-governance evidence records.
func RenderCSV(report *Article12Report) (string, error) {
	if report == nil {
		return "", fmt.Errorf("nil report")
	}
	var b strings.Builder
	b.WriteString("evidence_scope,entry_id,event_timestamp,release_process_started_at,release_recorded_at,system_id,release_id,version,actor_kind,actor_id,decision,risk_score,cgp_version,repository,files_changed,lines_changed,breaking_changes,security_changes\n")
	for _, e := range report.LogEntries {
		fmt.Fprintf(&b, "%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%.4f,%s,%s,%d,%d,%d,%d\n",
			csvEscape(e.EvidenceScope),
			csvEscape(e.EntryID),
			e.EventTimestamp.UTC().Format(time.RFC3339),
			e.ReleaseProcessStartedAt.UTC().Format(time.RFC3339),
			e.ReleaseRecordedAt.UTC().Format(time.RFC3339),
			csvEscape(e.SystemIdentifier),
			csvEscape(e.ReleaseID),
			csvEscape(e.Version),
			csvEscape(string(e.Actor.Kind)),
			csvEscape(e.Actor.ID),
			csvEscape(e.OutputDecision),
			e.RiskScore,
			csvEscape(e.ReferenceData.CGPVersion),
			csvEscape(e.InputData.Repository),
			e.InputData.FilesChanged,
			e.InputData.LinesChanged,
			e.InputData.BreakingChanges,
			e.InputData.SecurityChanges,
		)
	}
	return b.String(), nil
}

func csvEscape(s string) string {
	if !strings.ContainsAny(s, ",\"\n") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
