package adapters

import "github.com/relicta-tech/relicta/v4/internal/domain/release/domain"

// ToDTO and FromDTO expose this package's run serialization to the other backends.
//
// ADR-013 puts three adapters behind one port, and each of them has to turn a
// ReleaseRun into something storable and back. The tempting shape is one converter per
// adapter, and it is how fields get lost: BaseRef already survived a second
// implementation that filled it from the branch, which produced runs that were wrong
// rather than absent and made `relicta evaluate` refuse every release. See
// run_round_trip_test.go for the whole story.
//
// So there is one conversion, here, and the sqlite and postgres adapters store the same
// DTO this package writes to disk. A field added to ReleaseRunDTO reaches all three
// backends at once, and the round trip test that guards it guards all three.
//
// These are deliberately thin: the unexported pair stays the implementation so the file
// adapter's own call sites do not change, and exporting a wrapper rather than renaming
// keeps the diff additive for the adapters landing in parallel.

// ToDTO converts a release run into the serializable form every backend stores.
func ToDTO(run *domain.ReleaseRun) *ReleaseRunDTO {
	return toDTO(run)
}

// FromDTO reconstructs a release run from its serialized form.
func FromDTO(dto *ReleaseRunDTO) (*domain.ReleaseRun, error) {
	return fromDTO(dto)
}
