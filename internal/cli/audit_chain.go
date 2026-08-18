package cli

// audit_chain.go is how the CLI reads the governance audit chain, and what it says about
// what it found.
//
// The chain is evidence with one job: to make a change to the governance record
// detectable. That job is only done if something checks, and until this file existed
// nothing did — Chain.Verify had no caller anywhere in the tree, so every entry could
// have been rewritten and every attestation would still have reported "valid". Both
// `relicta verify` and `relicta audit` go through here so the two cannot disagree about
// whether a repository's evidence holds up.

import (
	"context"
	"errors"
	"fmt"

	"github.com/relicta-tech/relicta/v4/internal/cgp/audit"
)

// chainStatus is what the CLI found when it looked at the chain.
//
// The distinction between "no chain" and "could not read the chain" is the whole reason
// this is a named status and not a bool. They render as the same absence and mean
// opposite things: a repository that has recorded no governance events yet, versus one
// whose evidence is unreachable. Reporting the second as the first is how a deleted audit
// trail passes verification.
type chainStatus string

const (
	// chainVerified: the chain loaded and every link holds.
	chainVerified chainStatus = "verified"

	// chainBroken: the chain loaded and does not verify. An entry was edited,
	// removed or reordered. This is the loud one.
	chainBroken chainStatus = "broken"

	// chainUnavailable: the chain could not be read at all — no repository, no
	// governance store, an unreadable backend. Not the same as empty.
	chainUnavailable chainStatus = "unavailable"
)

// auditChainReport is one reading of a repository's chain.
//
// Not serialized: `relicta verify` flattens what it needs into VerifyOutput, which is the
// shape machine readers already parse. Adding json tags here would advertise a second
// contract for the same facts.
type auditChainReport struct {
	Status  chainStatus
	Entries int

	// Tail is the hash the chain currently ends at, which is what a new attestation
	// would anchor to. Empty for an empty chain.
	Tail string

	// Detail names the entry that broke, or why the chain could not be read. Always
	// populated when Status is not verified, because "broken" on its own tells an
	// operator nothing they can act on.
	Detail string

	chain *audit.Chain
}

// readAuditChain loads and verifies the chain for the repository the CLI is running in.
//
// It never returns an error. Every caller here has to render *something* about the chain,
// and a failure to read it is a finding rather than a reason to abandon the command —
// `relicta verify` still has an attestation signature to check, and `relicta audit` still
// has a release history to print. The finding is in Status.
func readAuditChain(ctx context.Context) auditChainReport {
	repository := getRepositoryName(ctx)
	if repository == "" {
		return auditChainReport{
			Status: chainUnavailable,
			Detail: "no repository identity: run inside a git repository",
		}
	}

	store, releaseStore, err := getMemoryStoreFunc(ctx)
	if err != nil {
		return auditChainReport{
			Status: chainUnavailable,
			Detail: fmt.Sprintf("opening the governance store: %v", err),
		}
	}
	defer releaseStore()

	chain, err := audit.LoadChain(ctx, store, repository)
	if err != nil {
		// A chain that loaded and failed verification is a different finding from one
		// that could not be read, and only the first is tampering. LoadChain wraps
		// ErrChainCorrupted for the first, so the two are told apart by the error and
		// not by guessing.
		status := chainUnavailable
		if errors.Is(err, audit.ErrChainCorrupted) {
			status = chainBroken
		}
		return auditChainReport{Status: status, Detail: err.Error()}
	}

	return auditChainReport{
		Status:  chainVerified,
		Entries: chain.Len(),
		Tail:    chain.LastHash(),
		chain:   chain,
	}
}

// anchorResult is what an attestation's recorded chain position turned out to be.
type anchorResult string

const (
	// anchorMatched: the attestation names a hash, and the chain has exactly that
	// hash at exactly that position.
	anchorMatched anchorResult = "matched"

	// anchorAbsent: the attestation records no chain position. Every attestation
	// written before the chain was appended to looks like this, so it is reported and
	// not treated as a failure — but it is not evidence of anything either.
	anchorAbsent anchorResult = "absent"

	// anchorMismatched: the attestation names a position the chain does not confirm.
	// The entry at that index hashes to something else, or the chain is now shorter
	// than the attestation says it was. Either way the record changed after it was
	// signed for.
	anchorMismatched anchorResult = "mismatched"

	// anchorUnknown: there was no readable chain to check the anchor against.
	//
	// Kept apart from mismatched because only one of them is evidence of tampering.
	// Verifying an attestation downloaded from a release page, outside any checkout, is
	// what `relicta verify --file` is for, and reporting that as a mismatch would fail
	// every such verification for the absence of a repository.
	anchorUnknown anchorResult = "unknown"
)

// checkAuditChainAnchor asks whether the chain still confirms what an attestation claimed
// about it.
//
// The attestation records the chain's tail hash and length at the moment the release was
// sealed, so the check is a prefix check: entry number count-1 must still hash to the
// recorded hash. Passing it means every entry up to that point is unchanged, because each
// one's hash feeds the next.
//
// Length alone would not do, and neither would the hash alone. A chain that lost its last
// entry and gained a new one has the same length with a different tail; a chain with the
// right hash at the wrong index has been reordered. Both are caught by pinning the hash to
// the index.
func checkAuditChainAnchor(report auditChainReport, hash string, count int) (anchorResult, string) {
	if hash == "" && count == 0 {
		return anchorAbsent, "the attestation records no audit chain position"
	}
	if report.Status != chainVerified || report.chain == nil {
		return anchorUnknown, "the audit chain could not be read, so the attestation's " +
			"anchor was not confirmed"
	}

	entries := report.chain.List()
	if count < 1 || count > len(entries) {
		return anchorMismatched, fmt.Sprintf(
			"the attestation was sealed over %d audit chain entries, and the chain now "+
				"holds %d: entries recorded before this release are missing",
			count, len(entries))
	}

	if got := entries[count-1].Hash; got != hash {
		return anchorMismatched, fmt.Sprintf(
			"audit chain entry %d (%s) hashes to %s, but the attestation was sealed "+
				"over %s: the governance record changed after the release was signed",
			count-1, entries[count-1].ID, got, hash)
	}

	return anchorMatched, ""
}
