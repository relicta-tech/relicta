package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	cgpsdk "github.com/relicta-tech/relicta/v4/pkg/cgp"
)

// FileProposalStore persists the CGP protocol handshake on disk.
//
// The service defaulted to inMemoryStore and WithStore was called from nowhere
// outside tests, so every server process began with an empty store and forgot
// everything when it exited. The three tools form a handshake — propose returns a
// decision, authorize records an ExecutionAuthorization against that decision,
// status reports which state a proposal reached — and the chain only held inside
// one process. Over stdio that is a single MCP session.
//
// The consequence that matters is not the inconvenience. Relicta's claim is
// verifiable governance, and a decision made through the protocol surface left no
// durable evidence: `cgp_status` answered "proposal not found" for a real earlier
// decision, indistinguishable from an ID that never existed, and nothing recorded
// that the decision had been made at all.
//
// A note on the shape, because an earlier plan for this was wrong. The backlog
// entry proposed routing proposals through the ReleaseRun aggregate so there would
// be one governance record rather than two. That looked right by analogy to the
// duplicate release store consolidated in #247, and it is not: a ChangeProposal
// carries an actor, a scope, an intent and metadata, and nothing else. It has no
// version proposal, no changeset and no release state machine. CGP governs change
// in general, and not every proposal is a release, so mapping every one onto a
// release aggregate would distort the aggregate to store data it has no meaning
// for. These are two different records because they describe two different things.
type FileProposalStore struct {
	root string
}

// cgpDirName is the subdirectory holding protocol records, alongside the
// releases/ directory the release store owns.
const cgpDirName = "cgp"

// Record kinds, each a directory. Separate directories rather than one with mixed
// prefixes, so a proposal ID and a decision ID cannot collide on the filesystem
// even though the decision is keyed by its proposal's ID.
const (
	proposalsDir      = "proposals"
	decisionsDir      = "decisions"
	authorizationsDir = "authorizations"
)

// ErrInvalidID rejects an identifier that cannot be used as a filename.
var ErrInvalidID = errors.New("invalid identifier")

// safeID matches the identifiers this protocol generates — cgp.GenerateProposalID
// and its siblings produce "prop_", "dec_" and "auth_" prefixes followed by hex
// and hyphens. Anything else is refused rather than sanitized.
//
// Refused, not cleaned: these IDs arrive from MCP tool input, so they are
// caller-controlled. Quietly rewriting "../../etc/passwd" into a plausible
// filename would store a record under a key the caller did not ask for, and the
// next read would miss it — a traversal attempt turning into silent data loss. An
// error is the honest answer.
var safeID = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

// NewFileProposalStore returns a store rooted at a repository.
//
// The root is the repository, and records live under .relicta/cgp/ — the same
// place every other piece of local governance state lives, so a repository's
// governance history is one directory rather than several.
func NewFileProposalStore(repoRoot string) *FileProposalStore {
	return &FileProposalStore{root: filepath.Join(repoRoot, ".relicta", cgpDirName)}
}

func (s *FileProposalStore) path(kind, id string) (string, error) {
	if !safeID.MatchString(id) || strings.Contains(id, "..") {
		return "", fmt.Errorf("%w: %q", ErrInvalidID, id)
	}
	return filepath.Join(s.root, kind, id+".json"), nil
}

// write stores a record atomically.
//
// Atomic because a half-written decision is worse than a missing one: absence is
// reported as "not found" and a truncated file fails to parse, which reads as
// corruption rather than as the ordinary case it is.
func (s *FileProposalStore) write(kind, id string, value any) error {
	path, err := s.path(kind, id)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create %s directory: %w", kind, err)
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", kind, err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", kind, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit %s: %w", kind, err)
	}
	return nil
}

// read loads a record. A missing record returns a "not found" error, which is what
// the service's callers already distinguish from a read failure.
func (s *FileProposalStore) read(kind, id string, into any) error {
	path, err := s.path(kind, id)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path) //nolint:gosec // path is confined by s.path
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s %s not found", strings.TrimSuffix(kind, "s"), id)
		}
		return fmt.Errorf("read %s: %w", kind, err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		return fmt.Errorf("parse stored %s %s: %w", kind, id, err)
	}
	return nil
}

func (s *FileProposalStore) SaveProposal(_ context.Context, proposal *cgpsdk.ChangeProposal) error {
	if proposal == nil {
		return fmt.Errorf("%w: nil proposal", ErrInvalidID)
	}
	return s.write(proposalsDir, proposal.ID, proposal)
}

func (s *FileProposalStore) GetProposal(_ context.Context, proposalID string) (*cgpsdk.ChangeProposal, error) {
	var proposal cgpsdk.ChangeProposal
	if err := s.read(proposalsDir, proposalID, &proposal); err != nil {
		return nil, err
	}
	return &proposal, nil
}

// SaveDecision keys the decision by the proposal it decides, matching the
// in-memory store: a proposal has one current decision, and status looks it up by
// the proposal ID the caller holds.
func (s *FileProposalStore) SaveDecision(_ context.Context, decision *cgpsdk.GovernanceDecision) error {
	if decision == nil {
		return fmt.Errorf("%w: nil decision", ErrInvalidID)
	}
	return s.write(decisionsDir, decision.ProposalID, decision)
}

func (s *FileProposalStore) GetDecision(_ context.Context, proposalID string) (*cgpsdk.GovernanceDecision, error) {
	var decision cgpsdk.GovernanceDecision
	if err := s.read(decisionsDir, proposalID, &decision); err != nil {
		return nil, err
	}
	return &decision, nil
}

func (s *FileProposalStore) SaveAuthorization(_ context.Context, auth *cgpsdk.ExecutionAuthorization) error {
	if auth == nil {
		return fmt.Errorf("%w: nil authorization", ErrInvalidID)
	}
	return s.write(authorizationsDir, auth.ProposalID, auth)
}

func (s *FileProposalStore) GetAuthorization(_ context.Context, proposalID string) (*cgpsdk.ExecutionAuthorization, error) {
	var auth cgpsdk.ExecutionAuthorization
	if err := s.read(authorizationsDir, proposalID, &auth); err != nil {
		return nil, err
	}
	return &auth, nil
}
