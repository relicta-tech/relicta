package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"
)

// worktreeConfigStorer removes the worktreeConfig extension only from the
// in-memory config go-git validates. go-git does not implement this Git
// extension, but the extension itself does not alter refs or object storage.
// Preserve it whenever go-git writes the shared config back to disk.
type worktreeConfigStorer struct {
	storage.Storer
	worktreeConfig bool
}

func (s worktreeConfigStorer) Config() (*config.Config, error) {
	cfg, err := s.Storer.Config()
	if err != nil {
		return nil, err
	}
	if section := cfg.Raw.Section("extensions"); section != nil {
		section.RemoveOption("worktreeConfig")
	}
	return cfg, nil
}

func (s worktreeConfigStorer) SetConfig(cfg *config.Config) error {
	if s.worktreeConfig {
		cfg.Raw.Section("extensions").SetOption("worktreeConfig", "true")
	}
	return s.Storer.SetConfig(cfg)
}

// openWorktreeConfigRepository uses Git to locate the private and common
// gitdirs, then lets go-git operate on their standard layout. It handles both
// linked worktrees and ordinary repositories whose config enables
// extensions.worktreeConfig, which go-git's extension validator rejects.
// This fallback is only reached when go-git cannot open the repository.
func openWorktreeConfigRepository(path string) (*git.Repository, error) {
	root, err := gitRevParse(path, "--show-toplevel")
	if err != nil {
		return nil, err
	}
	root = filepath.Clean(root)
	gitEntry := filepath.Join(root, ".git")
	info, err := os.Stat(gitEntry)
	if err != nil {
		return nil, err
	}
	gitDir, err := gitRevParse(root, "--absolute-git-dir")
	if err != nil {
		return nil, err
	}
	commonDir, err := gitRevParse(root, "--git-common-dir")
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(root, commonDir)
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(root, gitDir)
	}
	if info.IsDir() && filepath.Clean(gitDir) != filepath.Clean(commonDir) {
		return nil, fmt.Errorf("repository .git directory differs from the resolved git directories")
	}
	if !info.IsDir() && filepath.Clean(gitDir) == filepath.Clean(commonDir) {
		return nil, fmt.Errorf("repository .git file does not resolve to a linked worktree")
	}
	commonDir = filepath.Clean(commonDir)

	privateFS := osfs.New(filepath.Clean(gitDir))
	commonFS := osfs.New(commonDir)
	repoFS := dotgit.NewRepositoryFilesystem(privateFS, commonFS)
	base := filesystem.NewStorage(repoFS, cache.NewObjectLRUDefault())
	cfg, err := base.Config()
	if err != nil {
		return nil, err
	}
	worktreeConfig := cfg.Raw.Section("extensions").Option("worktreeConfig") != ""
	if info.IsDir() && !worktreeConfig {
		return nil, fmt.Errorf("repository does not enable extensions.worktreeConfig")
	}
	store := worktreeConfigStorer{Storer: base, worktreeConfig: worktreeConfig}
	return git.Open(store, osfs.New(root))
}

func gitRevParse(path, arg string) (string, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", arg)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w", arg, err)
	}
	return strings.TrimSpace(string(output)), nil
}

var _ storage.Storer = worktreeConfigStorer{}
