package attestation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp/audit"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
)

// Generator builds attestations from release runs and audit chains.
type Generator struct {
	repoID     string
	auditChain *audit.Chain
}

// NewGenerator creates a new attestation generator.
func NewGenerator(repoID string, auditChain *audit.Chain) *Generator {
	return &Generator{
		repoID:     repoID,
		auditChain: auditChain,
	}
}

// Generate creates an in-toto Statement with a GovernancePredicate
// from the given ReleaseRun.
func (g *Generator) Generate(_ context.Context, run *domain.ReleaseRun) (*Statement, error) {
	state := run.State()
	if state != domain.StatePublishing && state != domain.StatePublished {
		return nil, fmt.Errorf("cannot generate attestation in state %s: expected publishing or published", state)
	}

	// Build subject from tag name + SHA-256 of head SHA.
	tagName := run.TagName()
	if tagName == "" {
		tagName = "v" + run.VersionNext().String()
	}

	headDigest := sha256Hex(run.HeadSHA().String())

	subject := Subject{
		Name: tagName,
		Digest: map[string]string{
			"sha256": headDigest,
		},
	}

	// Build governance predicate.
	predicate := g.buildPredicate(run)

	return &Statement{
		Type:          StatementType,
		Subject:       []Subject{subject},
		PredicateType: PredicateTypeGovernance,
		Predicate:     predicate,
	}, nil
}

// buildPredicate populates the GovernancePredicate from the run.
func (g *Generator) buildPredicate(run *domain.ReleaseRun) GovernancePredicate {
	now := time.Now().UTC()
	publishedAt := now
	if ts := run.PublishedAt(); ts != nil {
		publishedAt = *ts
	}

	pred := GovernancePredicate{
		Version:    run.VersionNext().String(),
		Tag:        run.TagName(),
		Repository: g.repoID,
		CommitSHA:  run.HeadSHA().String(),
		ReleasedAt: publishedAt,
		RiskScore:  run.RiskScore(),
		Initiator: ActorIdentity{
			ID:   run.ActorID(),
			Kind: string(run.ActorType()),
		},
	}

	if pred.Tag == "" {
		pred.Tag = "v" + run.VersionNext().String()
	}

	// Extract approval details.
	g.populateApprovals(run, &pred)

	// Extract audit chain info.
	g.populateAuditChain(&pred)

	return pred
}

// populateApprovals extracts approval records from the run.
func (g *Generator) populateApprovals(run *domain.ReleaseRun, pred *GovernancePredicate) {
	// Check for multi-level approvals first.
	if mla := run.MultiLevelApprovalStatus(); mla != nil {
		allApprovals := mla.AllApprovals()
		pred.Approvals = make([]ApprovalRecord, 0, len(allApprovals))
		for _, a := range allApprovals {
			pred.Approvals = append(pred.Approvals, ApprovalRecord{
				ApproverID:   a.ApprovedBy,
				ApproverKind: string(a.ApproverType),
				ApprovedAt:   a.ApprovedAt,
				AutoApproved: a.AutoApproved,
				Level:        string(a.Level),
			})
		}
		pred.Decision = "approved"
		pred.AutoApproved = false
		return
	}

	// Single approval.
	approval := run.Approval()
	if approval != nil {
		pred.Approvals = []ApprovalRecord{
			{
				ApproverID:   approval.ApprovedBy,
				ApproverKind: string(approval.ApproverType),
				ApprovedAt:   approval.ApprovedAt,
				AutoApproved: approval.AutoApproved,
				Level:        string(approval.Level),
			},
		}
		pred.Decision = "approved"
		pred.AutoApproved = approval.AutoApproved
		if approval.Justification != "" {
			pred.Rationale = []string{approval.Justification}
		}
	} else {
		pred.Approvals = []ApprovalRecord{}
		pred.Decision = "unknown"
	}
}

// populateAuditChain extracts audit chain info.
func (g *Generator) populateAuditChain(pred *GovernancePredicate) {
	if g.auditChain == nil {
		return
	}
	pred.AuditChainHash = g.auditChain.LastHash()
	pred.AuditEntryCount = g.auditChain.Len()
}

// sha256Hex returns the hex-encoded SHA-256 hash of s.
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
