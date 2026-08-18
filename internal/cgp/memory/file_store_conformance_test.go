package memory_test

import (
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory/conformance"
)

// The file store is the reference implementation: `relicta history`, the DORA and SOC 2 reports,
// the deployment gate and hub sync were all written against it, so where the Store interface is
// silent its behavior is the contract. Running the suite here is what makes that claim checkable
// rather than asserted — a database-backed store that disagrees is wrong, and a change to the
// contract has to break this test first.
func TestTheFileStoreSatisfiesTheContract(t *testing.T) {
	conformance.Run(t, func(t *testing.T) memory.Store {
		store, err := memory.NewFileStore(t.TempDir())
		if err != nil {
			t.Fatalf("NewFileStore: %v", err)
		}
		return store
	})
}

// InMemoryStore is the other implementation already in the tree, and it is not only a test
// double: three production call sites construct one. Running the same suite against it is the
// point of having a suite — two implementations of one interface drift, and the drift is
// invisible until a caller switches between them.
func TestTheInMemoryStoreSatisfiesTheContract(t *testing.T) {
	conformance.Run(t, func(t *testing.T) memory.Store {
		return memory.NewInMemoryStore()
	})
}
