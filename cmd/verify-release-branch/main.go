// verify-release-branch ensures a tagged commit is reachable from an allowed branch.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/relicta-tech/relicta/v5/internal/config"
)

func main() {
	commit := flag.String("commit", "", "tagged commit SHA")
	remote := flag.String("remote", "origin", "trusted Git remote")
	flag.Parse()
	if *commit == "" {
		fatalf("-commit is required")
	}
	cfg, err := config.LoadFromDirectory(".")
	if err != nil {
		fatalf("load Relicta configuration: %v", err)
	}
	branches, err := allowedRemoteBranches(context.Background(), *commit, *remote)
	if err != nil {
		fatalf("inspect remote branches: %v", err)
	}
	if len(cfg.Workflow.AllowedBranches) == 0 {
		fmt.Println("workflow.allowed_branches is empty; no branch restriction applies")
		return
	}
	for _, branch := range branches {
		if branchAllowed(cfg.Workflow.AllowedBranches, branch) {
			fmt.Printf("Tagged commit %s is reachable from allowed branch %s/%s\n", *commit, *remote, branch)
			return
		}
	}
	fatalf("tagged commit %s is not reachable from any %s branch allowed by workflow.allowed_branches (%s)",
		*commit, *remote, strings.Join(cfg.Workflow.AllowedBranches, ", "))
}

func allowedRemoteBranches(ctx context.Context, commit, remote string) ([]string, error) {
	refs, err := gitOutput(ctx, "for-each-ref", "--format=%(refname:short)", "refs/remotes/"+remote)
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, ref := range strings.Fields(refs) {
		branch := strings.TrimPrefix(ref, remote+"/")
		if branch == ref || branch == "HEAD" {
			continue
		}
		if err := exec.CommandContext(ctx, "git", "merge-base", "--is-ancestor", commit, ref).Run(); err == nil {
			branches = append(branches, branch)
		} else {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				return nil, fmt.Errorf("check whether %s is reachable from %s: %w", commit, ref, err)
			}
		}
	}
	return branches, nil
}

func branchAllowed(patterns []string, branch string) bool {
	for _, pattern := range patterns {
		if matched, err := path.Match(pattern, branch); err == nil && matched {
			return true
		}
	}
	return false
}

func gitOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(output), nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "release branch verification failed: "+format+"\n", args...)
	os.Exit(1)
}
