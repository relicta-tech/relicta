package adr

import (
	"os"
	"strings"
	"testing"
)

// ADR-012 decides that deployment evidence and deployment gating cross a wire
// protocol rather than a module boundary: Rollops must not depend on relicta and
// relicta must not depend on Rollops.
//
// The ADR says in as many words that it is worth nothing without a checkable
// invariant, because the decision decays the first time someone reaches for a
// convenient import — and the failure is silent, since the code compiles and the
// tests pass. This is that check, on relicta's side. The mirror lives in Rollops.
func TestNoDeployerDependency(t *testing.T) {
	data, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	gomod := string(data)

	// Deployers whose evidence relicta may receive, and must not import. Named
	// rather than pattern-matched: a generic rule cannot tell a deployer from any
	// other dependency, and the point is to catch the specific import someone would
	// reach for when wiring the receiver.
	forbidden := []string{
		"klarlabs/rollops",
		"klarlabs.de/rollops",
	}

	for _, module := range forbidden {
		if strings.Contains(gomod, module) {
			t.Errorf("go.mod references %q. Deployment evidence arrives as a documented "+
				"event schema over HTTP, not as a Go type: importing a deployer couples "+
				"release cadences and obliges every relicta user to acquire it. The "+
				"receiver must accept the schema from any deployer — a CI step with curl "+
				"is a first-class client. See docs/adr/012.", module)
		}
	}
}

// The ADR is only load-bearing while the index points at it, and a decision nobody
// can find is a decision that gets remade.
func TestADR012IsIndexed(t *testing.T) {
	index, err := os.ReadFile(indexFile)
	if err != nil {
		t.Fatalf("read %s: %v", indexFile, err)
	}
	if !strings.Contains(string(index), "012-deployment-evidence-over-a-protocol.md") {
		t.Error("ADR-012 is not linked from the index")
	}
}
