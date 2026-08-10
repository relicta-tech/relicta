package recommendation

import (
	"encoding/json"
	"fmt"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
)

// Persist stores an artifact next to the run it describes.
//
// ADR-009 says every interface returns the same artifact, and the HTTP API
// returned none — a Hub reading over HTTP got a different shape than an agent
// reading MCP. The artifact is stored rather than recomputed on read, because it
// cannot honestly be rebuilt from a stored run: risk factors and required actions
// are not persisted on the run, and Assessment.Factors serializes as `[]` rather
// than being omitted, so a reconstruction would present "no factors were
// computed" as "no factors exist". Storing what was produced also makes
// InputsDigest mean something over HTTP — it is the same artifact, not a
// same-shaped one.
//
// Shared by the CLI and MCP plan paths so both store the identical bytes. They
// assemble BuildInput differently, and having each marshal and write separately
// is how the two surfaces would drift.
//
// A repository that does not implement ports.RecommendationStore is not an error:
// artifact storage is an addition, and a caller holding a repository without it
// should keep working. Returns false when nothing was stored.
func Persist(repo any, repoRoot string, runID domain.RunID, artifact *Artifact) (stored bool, err error) {
	if artifact == nil || runID == "" || repoRoot == "" {
		return false, nil
	}

	store, ok := repo.(ports.RecommendationStore)
	if !ok {
		return false, nil
	}

	// Indented to match how runs are written, so the file is readable by whoever
	// is auditing a release by hand.
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal recommendation: %w", err)
	}

	if err := store.SaveRecommendation(repoRoot, runID, data); err != nil {
		return false, fmt.Errorf("save recommendation: %w", err)
	}
	return true, nil
}
