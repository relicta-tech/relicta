// verify-release-governance verifies the persisted governance decision recorded
// during protected approval and transferred to the publication job.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/relicta-tech/relicta/v5/internal/cgp/audit"
	"github.com/relicta-tech/relicta/v5/internal/cgp/memory"
)

func main() {
	memoryFile := flag.String("memory-file", "", "path to the transferred governance memory.json")
	repository := flag.String("repository", "", "repository identity (owner/name)")
	releaseID := flag.String("release-id", "", "Relicta release run ID")
	decision := flag.String("decision", "", "comma-separated allowed CGP decisions")
	planHash := flag.String("plan-hash", "", "expected approved release plan hash")
	reviewer := flag.String("reviewer", "", "protected human reviewer login")
	flag.Parse()

	if *memoryFile == "" || *repository == "" || *releaseID == "" || *decision == "" || *planHash == "" || *reviewer == "" {
		fatalf("memory-file, repository, release-id, decision, plan-hash, and reviewer are required")
	}
	if filepath.Base(*memoryFile) != "memory.json" {
		fatalf("governance memory file must be named memory.json")
	}

	store, err := memory.NewFileStore(filepath.Dir(*memoryFile))
	if err != nil {
		fatalf("open governance memory: %v", err)
	}
	chain, err := audit.LoadChain(context.Background(), store, *repository)
	if err != nil {
		fatalf("verify governance audit chain: %v", err)
	}

	allowed := make(map[string]struct{})
	for _, value := range strings.Split(*decision, ",") {
		if value = strings.TrimSpace(value); value != "" {
			allowed[value] = struct{}{}
		}
	}
	permittedDecision, err := verifyReleaseEvidence(chain, *releaseID, allowed, *planHash, *reviewer)
	if err != nil {
		fatalf("%v", err)
	}
	fmt.Printf("Verified governance audit chain (%d entries), %s decision, and human approval for release %s (plan %s, reviewer github:%s)\n",
		chain.Len(), permittedDecision, *releaseID, *planHash, *reviewer)
}

func verifyReleaseEvidence(chain *audit.Chain, releaseID string, allowed map[string]struct{}, planHash, reviewer string) (string, error) {
	var permittedDecision string
	approvalFound := false
	for _, entry := range chain.List() {
		if entry.ProposalID != releaseID {
			continue
		}
		if entry.EventType == audit.EventDecisionMade {
			if got, ok := entry.Details["decisionType"].(string); ok {
				if _, permitted := allowed[got]; permitted && entry.Details["planHash"] == planHash {
					permittedDecision = got
				}
			}
		}
		if entry.EventType == audit.EventApprovalGranted &&
			entry.ActorKind == "human" && entry.ActorID == "github:"+reviewer &&
			entry.Details["planHash"] == planHash && entry.Details["autoApproved"] == false {
			approvalFound = true
		}
	}
	if permittedDecision == "" {
		return "", fmt.Errorf("audit chain contains no permitted governance decision for release %s", releaseID)
	}
	if !approvalFound {
		return "", fmt.Errorf("audit chain contains no human approval for release %s matching plan %s and reviewer github:%s", releaseID, planHash, reviewer)
	}
	return permittedDecision, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "release governance verification failed: "+format+"\n", args...)
	os.Exit(1)
}
