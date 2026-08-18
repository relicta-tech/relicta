package memory

// audit_chain.go holds the audit chain's append rule once, for the two stores that keep
// their chains as a slice: the reference in-memory store and the file store.
//
// Both had to enforce the same three things — the entry follows the stored tail, its ID
// is new, and the caller cannot reach into what was stored — and a rule written twice is
// how the conformance suite ends up passing on one adapter's interpretation. The SQL
// adapters express the same rule as a query plus a constraint, which is a different
// spelling of it and not a different rule; the suite is what keeps the two honest.

import (
	"fmt"

	"github.com/relicta-tech/relicta/v4/internal/cgp/audit"
)

// appendAuditEntry returns entries with entry appended, or an error if it does not
// extend them.
//
// The stored entry is a copy. Callers hold the pointer they appended and would otherwise
// be able to edit evidence in place afterwards — for the in-memory store that is the same
// object, and a test proving tamper detection would be proving nothing if corrupting the
// caller's copy silently corrupted the store's.
func appendAuditEntry(entries []*audit.Entry, entry *audit.Entry) ([]*audit.Entry, error) {
	if entry == nil {
		return nil, fmt.Errorf("audit entry is required")
	}
	if entry.ID == "" {
		return nil, fmt.Errorf("audit entry ID is required")
	}

	for _, existing := range entries {
		if existing.ID == entry.ID {
			return nil, audit.ErrDuplicateEntry
		}
	}

	tail := ""
	if len(entries) > 0 {
		tail = entries[len(entries)-1].Hash
	}
	if entry.PreviousHash != tail {
		return nil, fmt.Errorf("%w: entry %s follows %q but the chain ends at %q",
			audit.ErrChainForked, entry.ID, entry.PreviousHash, tail)
	}

	stored := *entry
	return append(entries, &stored), nil
}

// copyAuditEntries returns a caller-owned copy of a stored chain.
//
// Deep to the entry, shallow into Details. A reader that mutated an entry it was handed
// would be editing the store's evidence through the pointer; Details is a map the reader
// could still reach into, and the alternative is a recursive clone of arbitrary JSON for
// a value every caller only reads. Verify hashes what is stored, so a reader corrupting
// its own map cannot make the stored chain fail — nor pass.
func copyAuditEntries(entries []*audit.Entry) []*audit.Entry {
	result := make([]*audit.Entry, 0, len(entries))
	for _, entry := range entries {
		copied := *entry
		result = append(result, &copied)
	}
	return result
}

// lastAuditEntry returns a copy of the tail, or nil for an empty chain.
func lastAuditEntry(entries []*audit.Entry) *audit.Entry {
	if len(entries) == 0 {
		return nil
	}
	tail := *entries[len(entries)-1]
	return &tail
}
