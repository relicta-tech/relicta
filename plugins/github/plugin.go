// Package main implements the GitHub plugin for ReleasePilot.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"

	"github.com/felixgeelhaar/release-pilot/pkg/plugin"
)

// GitHubPlugin implements the GitHub release plugin.
type GitHubPlugin struct{}

// Config represents the GitHub plugin configuration.
type Config struct {
	// Owner is the repository owner.
	Owner string `json:"owner,omitempty"`
	// Repo is the repository name.
	Repo string `json:"repo,omitempty"`
	// Token is the GitHub token.
	Token string `json:"token,omitempty"`
	// Draft creates the release as a draft.
	Draft bool `json:"draft"`
	// Prerelease marks the release as a prerelease.
	Prerelease bool `json:"prerelease"`
	// GenerateReleaseNotes uses GitHub's auto-generated release notes.
	GenerateReleaseNotes bool `json:"generate_release_notes"`
	// Assets is a list of files to upload as release assets.
	Assets []string `json:"assets,omitempty"`
	// DiscussionCategory creates a discussion for the release.
	DiscussionCategory string `json:"discussion_category,omitempty"`
}

// GetInfo returns plugin metadata.
func (p *GitHubPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "github",
		Version:     "1.0.0",
		Description: "Create GitHub releases and upload assets",
		Author:      "ReleasePilot Team",
		Hooks: []plugin.Hook{
			plugin.HookPostPublish,
			plugin.HookOnSuccess,
			plugin.HookOnError,
		},
		ConfigSchema: `{
			"type": "object",
			"properties": {
				"owner": {"type": "string", "description": "Repository owner"},
				"repo": {"type": "string", "description": "Repository name"},
				"token": {"type": "string", "description": "GitHub token (or use GITHUB_TOKEN env)"},
				"draft": {"type": "boolean", "description": "Create as draft", "default": false},
				"prerelease": {"type": "boolean", "description": "Mark as prerelease", "default": false},
				"generate_release_notes": {"type": "boolean", "description": "Use GitHub's auto-generated notes", "default": false},
				"assets": {"type": "array", "items": {"type": "string"}, "description": "Files to upload"},
				"discussion_category": {"type": "string", "description": "Discussion category name"}
			}
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *GitHubPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	cfg := p.parseConfig(req.Config)

	switch req.Hook {
	case plugin.HookPostPublish:
		return p.createRelease(ctx, cfg, req.Context, req.DryRun)
	case plugin.HookOnSuccess:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Release successful",
		}, nil
	case plugin.HookOnError:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Release failed notification acknowledged",
		}, nil
	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

// createRelease creates a GitHub release.
func (p *GitHubPlugin) createRelease(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	// Get GitHub client
	client, err := p.getClient(ctx, cfg)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to create GitHub client: %v", err),
		}, nil
	}

	// Get owner/repo
	owner := cfg.Owner
	repo := cfg.Repo

	if owner == "" {
		owner = releaseCtx.RepositoryOwner
	}
	if repo == "" {
		repo = releaseCtx.RepositoryName
	}

	if owner == "" || repo == "" {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   "repository owner and name are required",
		}, nil
	}

	// Prepare release
	tagName := releaseCtx.TagName
	name := fmt.Sprintf("Release %s", releaseCtx.Version)
	body := releaseCtx.ReleaseNotes
	if body == "" {
		body = releaseCtx.Changelog
	}

	release := &github.RepositoryRelease{
		TagName:              &tagName,
		Name:                 &name,
		Body:                 &body,
		Draft:                &cfg.Draft,
		Prerelease:           &cfg.Prerelease,
		GenerateReleaseNotes: &cfg.GenerateReleaseNotes,
	}

	if cfg.DiscussionCategory != "" {
		release.DiscussionCategoryName = &cfg.DiscussionCategory
	}

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Would create GitHub release for %s/%s: %s", owner, repo, tagName),
			Outputs: map[string]any{
				"tag_name":   tagName,
				"owner":      owner,
				"repo":       repo,
				"draft":      cfg.Draft,
				"prerelease": cfg.Prerelease,
			},
		}, nil
	}

	// Create release
	createdRelease, _, err := client.Repositories.CreateRelease(ctx, owner, repo, release)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to create release: %v", err),
		}, nil
	}

	releaseID := createdRelease.GetID()
	htmlURL := createdRelease.GetHTMLURL()

	// Upload assets
	var artifacts []plugin.Artifact
	for _, assetPath := range cfg.Assets {
		artifact, err := p.uploadAsset(ctx, client, owner, repo, releaseID, assetPath)
		if err != nil {
			// Log but don't fail
			continue
		}
		artifacts = append(artifacts, *artifact)
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Created GitHub release: %s", htmlURL),
		Outputs: map[string]any{
			"release_id":  releaseID,
			"release_url": htmlURL,
			"tag_name":    tagName,
		},
		Artifacts: artifacts,
	}, nil
}

// uploadAsset uploads a release asset.
func (p *GitHubPlugin) uploadAsset(ctx context.Context, client *github.Client, owner, repo string, releaseID int64, assetPath string) (*plugin.Artifact, error) {
	// Validate and sanitize the asset path to prevent path traversal
	validatedPath, err := plugin.ValidateAssetPath(assetPath)
	if err != nil {
		return nil, fmt.Errorf("invalid asset path %s: %w", assetPath, err)
	}

	// Verify file exists and is a regular file (not a directory)
	info, err := os.Lstat(validatedPath)
	if err != nil {
		return nil, fmt.Errorf("asset file not accessible %s: %w", assetPath, err)
	}

	// Reject directories
	if info.IsDir() {
		return nil, fmt.Errorf("asset path is a directory, not a file: %s", assetPath)
	}

	// Note: We already resolved symlinks in validateAssetPath via EvalSymlinks,
	// so if we reach here with a symlink, EvalSymlinks followed it safely within
	// the working directory boundary. However, we still reject symlinks as a
	// defense-in-depth measure to prevent any edge cases.
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("symlinks not allowed for asset paths: %s", assetPath)
	}

	file, err := os.Open(validatedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open asset %s: %w", assetPath, err)
	}
	defer file.Close()

	// Get file info for size
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat asset %s: %w", assetPath, err)
	}

	// Upload
	name := fileInfo.Name()
	opts := &github.UploadOptions{Name: name}

	asset, _, err := client.Repositories.UploadReleaseAsset(ctx, owner, repo, releaseID, opts, file)
	if err != nil {
		return nil, fmt.Errorf("failed to upload asset: %w", err)
	}

	return &plugin.Artifact{
		Name: name,
		Path: asset.GetBrowserDownloadURL(),
		Type: "url",
		Size: fileInfo.Size(),
	}, nil
}

// getClient creates a GitHub client.
func (p *GitHubPlugin) getClient(ctx context.Context, cfg *Config) (*github.Client, error) {
	token := cfg.Token
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}

	if token == "" {
		return nil, fmt.Errorf("GitHub token is required (set GITHUB_TOKEN or configure token)")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)

	return github.NewClient(tc), nil
}

// parseConfig parses the plugin configuration using the shared ConfigParser.
func (p *GitHubPlugin) parseConfig(raw map[string]any) *Config {
	parser := plugin.NewConfigParser(raw)

	return &Config{
		Owner:                parser.GetString("owner"),
		Repo:                 parser.GetString("repo"),
		Token:                parser.GetString("token", "GITHUB_TOKEN", "GH_TOKEN"),
		Draft:                parser.GetBool("draft"),
		Prerelease:           parser.GetBool("prerelease"),
		GenerateReleaseNotes: parser.GetBool("generate_release_notes"),
		Assets:               parser.GetStringSlice("assets"),
		DiscussionCategory:   parser.GetString("discussion_category"),
	}
}

// Validate validates the plugin configuration using the shared ValidationBuilder.
func (p *GitHubPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := plugin.NewValidationBuilder()

	// Token is required (either from config or environment)
	parser := plugin.NewConfigParser(config)
	token := parser.GetString("token", "GITHUB_TOKEN", "GH_TOKEN")

	if token == "" {
		vb.AddError("token",
			"GitHub token is required (set GITHUB_TOKEN env var or configure token)",
			"required")
	}

	// Validate assets if provided
	vb.ValidateStringSlice(config, "assets")

	return vb.Build(), nil
}
