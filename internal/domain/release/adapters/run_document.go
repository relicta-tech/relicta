package adapters

import (
	"encoding/json"
	"fmt"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
)

// MarshalRun encodes a run as the same JSON document the file adapter writes to
// .relicta/releases/run-<id>.json.
//
// Exported so the database adapters ADR-013 adds can store a run the way this package
// already stores one, instead of each inventing an encoding. Three adapters behind one
// port is exactly the situation in which a field goes missing in one backend only, and
// a lost field is worse than a lost run: the run still loads, and governance reads a
// record that is wrong rather than absent. ReleaseRunDTO already carries everything —
// base ref, head SHA, commits, changeset, approval, history — and
// run_round_trip_test.go is what holds it there, because that suite of fields is the
// one a lossy loader silently dropped once.
//
// The bytes are compact rather than indented. The file adapter indents because a human
// opens those files; nobody reads a TEXT column by eye, and the indentation is pure
// size in a database.
func MarshalRun(run *domain.ReleaseRun) ([]byte, error) {
	data, err := json.Marshal(toDTO(run))
	if err != nil {
		return nil, fmt.Errorf("marshaling release run %s: %w", run.ID(), err)
	}
	return data, nil
}

// UnmarshalRun rebuilds a run from bytes produced by MarshalRun or read from a file the
// file adapter wrote — they are the same document, which is what lets `relicta db
// import` copy an existing .relicta/ tree into a database rather than convert it.
func UnmarshalRun(data []byte) (*domain.ReleaseRun, error) {
	var dto ReleaseRunDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return nil, fmt.Errorf("unmarshaling release run: %w", err)
	}
	return fromDTO(&dto)
}
