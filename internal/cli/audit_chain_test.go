package cli

// audit_chain_test.go covers the check that ties a signed attestation to the chain it
// claims to have been sealed over.
//
// Verifying the chain on its own is not enough. A chain can verify perfectly after
// somebody rebuilt it from scratch — every link recomputed, every entry consistent, and
// nothing left of the release that was signed for. The anchor is what makes that
// detectable, so these cases are mostly about the ways a rebuilt or trimmed chain tries to
// pass.

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/audit"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

// chainOf records n entries and returns the report a verifying reader would get.
func chainOf(t *testing.T, n int) auditChainReport {
	t.Helper()

	store := memory.NewInMemoryStore()
	recorder := audit.NewRecorder(store, "acme/widget")
	for i := range n {
		entry := audit.NewEntry(string(rune('a'+i)), audit.EventDecisionMade).
			WithProposal("run-1").
			WithActor("human:alice", cgp.ActorKindHuman).
			WithTimestamp(time.Date(2026, 5, 1, 12, i, 0, 0, time.UTC)).
			Build()
		if err := recorder.Record(context.Background(), entry); err != nil {
			t.Fatalf("recording entry %d: %v", i, err)
		}
	}

	chain, err := audit.LoadChain(context.Background(), store, "acme/widget")
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	return auditChainReport{
		Status:  chainVerified,
		Entries: chain.Len(),
		Tail:    chain.LastHash(),
		chain:   chain,
	}
}

func TestAnAttestationAnchoredInTheChainIsConfirmed(t *testing.T) {
	report := chainOf(t, 5)
	sealedOver := 3
	hash := report.chain.List()[sealedOver-1].Hash

	result, detail := checkAuditChainAnchor(report, hash, sealedOver)

	if result != anchorMatched {
		t.Errorf("an attestation naming entry %d of a chain that still holds it was "+
			"reported as %s (%s)", sealedOver-1, result, detail)
	}
}

// The case the length check alone would miss: the chain is intact and long enough, and the
// entry at the sealed position is not the entry that was sealed.
func TestAnAttestationAnchoredToARewrittenChainIsRejected(t *testing.T) {
	report := chainOf(t, 5)

	result, detail := checkAuditChainAnchor(report, "0000000000000000000000000000000000000000000000000000000000000000", 3)

	if result != anchorMismatched {
		t.Fatalf("an attestation naming a hash the chain does not hold was reported as "+
			"%s: a chain rebuilt after the release was signed would pass verification",
			result)
	}
	if detail == "" {
		t.Error("the mismatch was reported with no detail, so an operator is told the " +
			"anchor is wrong and not what changed")
	}
}

// Trimming the chain to before a release's anchor removes the evidence and leaves
// something that still verifies.
func TestAnAttestationSealedOverMoreEntriesThanRemainIsRejected(t *testing.T) {
	report := chainOf(t, 2)

	result, detail := checkAuditChainAnchor(report, report.Tail, 9)

	if result != anchorMismatched {
		t.Fatalf("an attestation sealed over 9 entries against a chain of 2 was reported "+
			"as %s: entries can be deleted and every attestation still passes", result)
	}
	if detail == "" {
		t.Error("the mismatch was reported with no detail")
	}
}

// Attestations written before the chain was appended to carry no anchor. Reporting them as
// tampered would fail every release in a repository's history on upgrade.
func TestAnAttestationWithNoRecordedPositionIsReportedAsAbsent(t *testing.T) {
	result, _ := checkAuditChainAnchor(chainOf(t, 3), "", 0)

	if result != anchorAbsent {
		t.Errorf("an attestation carrying no chain position was reported as %s, want "+
			"absent: upgrading would fail verification for every release published "+
			"before the chain existed", result)
	}
}

// An unreadable chain and a chain that disagrees are different findings. Only one is
// evidence of tampering, and `relicta verify --file` on a downloaded attestation always
// produces the other.
func TestAnUnreadableChainIsNotReportedAsTampering(t *testing.T) {
	unreadable := auditChainReport{Status: chainUnavailable, Detail: "no repository identity"}

	result, _ := checkAuditChainAnchor(unreadable, "somehash", 3)

	if result != anchorUnknown {
		t.Errorf("an attestation checked with no chain available was reported as %s, "+
			"want unknown: verifying a downloaded attestation would fail for the "+
			"absence of a repository", result)
	}
}
