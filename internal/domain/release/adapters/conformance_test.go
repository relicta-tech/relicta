package adapters_test

import (
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/adapters"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports/conformance"
)

// The file adapter is the reference implementation: it is what every caller in the tree was
// written against, so where the port's documentation is silent its behavior is the contract.
// Running the suite here is what makes that claim checkable rather than asserted — a sqlite or
// postgres adapter that disagrees with this is wrong, and a change to the contract has to break
// this test first.
func TestTheFileRepositorySatisfiesTheContract(t *testing.T) {
	conformance.Run(t, func(t *testing.T) (ports.ReleaseRunRepository, string) {
		return adapters.NewFileReleaseRunRepository(), t.TempDir()
	})
}
