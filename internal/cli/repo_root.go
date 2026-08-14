package cli

// repo_root.go: one answer to "which repository am I in".

import (
	"context"

	gitservice "github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
)

// repositoryRoot returns the root of the repository containing the working directory, or "."
// when there is no repository.
//
// Commands kept answering this with "." or os.Getwd(), which is the working directory rather
// than the repository — so running from a subdirectory silently changed what the command
// operated on. It has been fixed one command at a time: the release store (#246, which
// reported "no release run found" while printing the correct root), the governance memory
// store, and the dashboard. `relicta blast` was still doing it, and blast radius feeds the
// risk score, so a subdirectory produced a plausible answer computed over the wrong file set.
//
// git itself works from anywhere in a tree, and so should this.
func repositoryRoot(ctx context.Context) string {
	svc, err := gitservice.NewService()
	if err != nil {
		return "."
	}
	info, err := gitservice.NewAdapter(svc).GetInfo(ctx)
	if err != nil || info.Path == "" {
		return "."
	}
	return info.Path
}
