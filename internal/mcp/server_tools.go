package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go.klarlabs.de/mcp"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
	"github.com/relicta-tech/relicta/v4/internal/config"
	cgpsdk "github.com/relicta-tech/relicta/v4/pkg/cgp"
)

// Tool handlers

func (s *Server) handleInit(ctx context.Context, input InitToolInput) (string, error) {
	// Get current working directory for config placement
	repoPath := s.ensureRepoPath(ctx)

	// Check for existing config
	existingConfig, _ := config.FindConfigFile(repoPath)
	if existingConfig != "" && !input.Force {
		result := map[string]any{
			"status":      "exists",
			"config_file": existingConfig,
			"message":     "Configuration file already exists. Use force=true to overwrite.",
		}
		jsonBytes, _ := json.Marshal(result)
		return string(jsonBytes), nil
	}

	// Determine config file name based on format
	configFile := ".relicta.yaml"
	format := input.Format
	if format == "" {
		format = "yaml"
	}
	if format == "json" {
		configFile = ".relicta.json"
	}

	// Start with default configuration
	cfg := config.DefaultConfig()

	// Try to detect repository settings from git
	if s.gitService != nil {
		// Get repository info
		info, err := s.gitService.GetRepositoryInfo(ctx)
		if err == nil && info.DefaultBranch != "" {
			cfg.Workflow.AllowedBranches = []string{info.DefaultBranch}
		}

		// Try to detect remote URL and extract owner/repo
		remoteURL, err := s.gitService.GetRemoteURL(ctx, "origin")
		if err == nil && remoteURL != "" {
			// Parse GitHub/GitLab URL
			repoURL := parseRemoteURL(remoteURL)
			if repoURL != "" {
				cfg.Changelog.RepositoryURL = repoURL
			}
		}
	}

	// Write config file
	if err := config.WriteConfig(cfg, configFile); err != nil {
		return "", fmt.Errorf("failed to write config file: %w", err)
	}

	result := map[string]any{
		"status":      "created",
		"config_file": configFile,
		"format":      format,
		"message":     fmt.Sprintf("Created %s with default configuration", configFile),
		"next_steps": []string{
			"Review and customize your config file",
			"Set up required environment variables (OPENAI_API_KEY, GITHUB_TOKEN)",
			"Run 'relicta plan' to analyze your commits",
		},
	}

	// Add detected settings info
	if cfg.Changelog.RepositoryURL != "" {
		result["detected_repository"] = cfg.Changelog.RepositoryURL
	}
	if len(cfg.Workflow.AllowedBranches) > 0 {
		result["detected_branch"] = cfg.Workflow.AllowedBranches[0]
	}

	// Hot-reload: reinitialize config and adapter so subsequent commands work
	// without restarting the MCP server (fixes #83).
	if s.configReloader != nil {
		reloaded, err := s.configReloader(ctx)
		if err != nil {
			s.logger.Warn("config reload after init failed, tools may require server restart", "error", err)
			result["reload_warning"] = "Config created but live reload failed. Restart MCP server if tools return not_configured."
		} else {
			s.mu.Lock()
			s.config = reloaded.Config
			if reloaded.Adapter != nil {
				s.adapter = reloaded.Adapter
			}
			// The evaluator is what the cgp_* tools decide with, and it was not
			// refreshed here — so after init they kept failing while this handler
			// reported that tools were available.
			if reloaded.Evaluator != nil {
				s.evaluator = reloaded.Evaluator
				// ensureCGPService caches the service it builds. Without clearing it,
				// a service built from the previous evaluator would survive the reload
				// and keep deciding by the old rules.
				s.cgpService = nil
			}
			s.mu.Unlock()
			if s.cache != nil {
				s.cache.InvalidateAll()
			}
			s.logger.Info("config reloaded after init, tools are now available")
			result["reloaded"] = true
		}
	}

	jsonBytes, _ := json.Marshal(result)
	return string(jsonBytes), nil
}

// parseRemoteURL extracts repository URL from a git remote URL.
func parseRemoteURL(remoteURL string) string {
	// Handle SSH format: git@github.com:owner/repo.git
	if len(remoteURL) > 15 && remoteURL[:15] == "git@github.com:" {
		path := remoteURL[15:]
		if len(path) > 4 && path[len(path)-4:] == ".git" {
			path = path[:len(path)-4]
		}
		return "https://github.com/" + path
	}

	// Handle SSH format: git@gitlab.com:owner/repo.git
	if len(remoteURL) > 15 && remoteURL[:15] == "git@gitlab.com:" {
		path := remoteURL[15:]
		if len(path) > 4 && path[len(path)-4:] == ".git" {
			path = path[:len(path)-4]
		}
		return "https://gitlab.com/" + path
	}

	// Handle HTTPS format
	if len(remoteURL) > 8 && (remoteURL[:8] == "https://" || remoteURL[:7] == "http://") {
		url := remoteURL
		if len(url) > 4 && url[len(url)-4:] == ".git" {
			url = url[:len(url)-4]
		}
		return url
	}

	return ""
}

func (s *Server) handleStatus(ctx context.Context, input StatusInput) (any, error) {
	// Ensure consistent repository path (fixes issue #35)
	s.ensureRepoPath(ctx)

	// Use adapter if available (GetStatus uses releaseServices, not releaseRepo)
	if s.adapter != nil && s.adapter.HasReleaseServices() {
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return toJSONString(map[string]any{
				"status":  "no_active_release",
				"message": "No active release found. Run 'relicta plan' to start a new release.",
			}), nil
		}

		out := StatusToolOutput{
			ReleaseID:  status.ReleaseID,
			State:      status.State,
			Version:    status.Version,
			Created:    status.CreatedAt,
			Updated:    status.UpdatedAt,
			CanApprove: status.CanApprove,
			NextAction: status.NextAction,
		}

		if status.ApprovalMsg != "" {
			out.ApprovalMessage = status.ApprovalMsg
		}

		if status.Stale {
			out.Stale = true
			out.Warning = status.Warning
		}

		return out, nil
	}

	// Fallback to direct repository access
	if s.releaseRepo == nil {
		return toJSONString(map[string]any{
			"status":  "not_configured",
			"message": "No release repository configured. Run 'relicta plan' first.",
		}), nil
	}

	releases, err := s.releaseRepo.FindActive(ctx)
	if err != nil || len(releases) == 0 {
		return toJSONString(map[string]any{
			"status":  "no_active_release",
			"message": "No active release found. Run 'relicta plan' to start a new release.",
		}), nil
	}

	rel := releases[0]
	result := map[string]any{
		"state":   rel.State().String(),
		"version": "",
		"created": rel.CreatedAt().Format("2006-01-02T15:04:05Z07:00"),
		"updated": rel.UpdatedAt().Format("2006-01-02T15:04:05Z07:00"),
	}

	if !rel.VersionNext().IsZero() {
		result["version"] = rel.VersionNext().String()
	}

	return toJSONString(result), nil
}

func (s *Server) handlePlan(ctx context.Context, input PlanToolInput) (any, error) {
	// Ensure consistent repository path (fixes issue #35)
	repoPath := s.ensureRepoPath(ctx)

	// Use adapter if available
	if s.adapter != nil && s.adapter.HasReleaseAnalyzer() {
		fromRef := ""
		if input.From != "" && input.From != "auto" {
			fromRef = input.From
		}

		planInput := PlanInput{
			RepositoryPath: repoPath,
			FromRef:        fromRef,
			Analyze:        input.Analyze,
		}

		// Report progress
		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 3.0
			_ = progress.Report(1, &total)
		}

		output, err := s.adapter.Plan(ctx, planInput)
		if err != nil {
			return "", userError(err)
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 3.0
			_ = progress.Report(2, &total)
		}

		out := PlanToolOutput{
			ReleaseID:      output.ReleaseID,
			CurrentVersion: output.CurrentVersion,
			NextVersion:    output.NextVersion,
			ReleaseType:    output.ReleaseType,
			CommitCount:    output.CommitCount,
			HasBreaking:    output.HasBreaking,
			HasFeatures:    output.HasFeatures,
			HasFixes:       output.HasFixes,
			Recommendation: output.Recommendation,
		}

		// Include commit details when analyze=true
		if input.Analyze && len(output.Commits) > 0 {
			out.Commits = make([]PlanCommitInfo, 0, len(output.Commits))
			for _, c := range output.Commits {
				out.Commits = append(out.Commits, PlanCommitInfo{
					SHA:     c.SHA,
					Type:    c.Type,
					Message: c.Message,
					Author:  c.Author,
					Scope:   c.Scope,
				})
			}
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 3.0
			_ = progress.Report(3, &total)
		}

		s.invalidateCache()
		return out, nil
	}

	return toJSONString(map[string]any{
		"status":  "not_configured",
		"message": "Run 'relicta mcp serve' with configured dependencies",
	}), nil
}

func (s *Server) handleBump(ctx context.Context, input BumpToolInput) (string, error) {
	// Ensure consistent repository path (fixes issue #35)
	repoPath := s.ensureRepoPath(ctx)

	bumpType := input.Level
	if bumpType == "" {
		bumpType = "auto"
	}

	// Use adapter if available
	if s.adapter != nil && s.adapter.HasReleaseServices() {
		bumpInput := BumpInput{
			RepositoryPath: repoPath,
			BumpType:       bumpType,
			Version:        input.Version,
		}

		output, err := s.adapter.Bump(ctx, bumpInput)
		if err != nil {
			return "", userError(err)
		}

		result := map[string]any{
			"current_version": output.CurrentVersion,
			"next_version":    output.NextVersion,
			"bump_type":       output.BumpType,
			"auto_detected":   output.AutoDetected,
		}

		if output.TagName != "" {
			result["tag_name"] = output.TagName
			result["tag_created"] = output.TagCreated
		}

		s.invalidateCache()
		return toJSONString(result), nil
	}

	return "", errNotConfigured("relicta_bump")
}

func (s *Server) handleNotes(ctx context.Context, input NotesToolInput) (string, error) {
	// Ensure consistent repository path (fixes issue #35)
	s.ensureRepoPath(ctx)

	// Use adapter if available (GetStatus and Notes both use releaseServices)
	if s.adapter != nil && s.adapter.HasReleaseServices() {
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return "", userError(errors.New("no active release to generate notes for — run relicta_plan first"))
		}

		// Report progress
		totalSteps := 3.0
		if input.AI {
			totalSteps = 5.0
		}
		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			_ = progress.Report(1, &totalSteps)
		}

		notesInput := NotesInput{
			ReleaseID:        status.ReleaseID,
			UseAI:            input.AI,
			IncludeChangelog: true,
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			_ = progress.Report(2, &totalSteps)
		}

		output, err := s.adapter.Notes(ctx, notesInput)
		if err != nil {
			return "", userError(err)
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			_ = progress.Report(totalSteps, &totalSteps)
		}

		result := map[string]any{
			"summary":      output.Summary,
			"ai_generated": output.AIGenerated,
		}

		if output.Changelog != "" {
			result["changelog"] = output.Changelog
		}

		s.invalidateCache()
		return toJSONString(result), nil
	}

	return "", errNotConfigured("relicta_notes")
}

func (s *Server) handleEvaluate(ctx context.Context, input EvaluateToolInput) (string, error) {
	// Ensure consistent repository path (fixes issue #35). The return value is
	// the resolved root, and it is needed below: governance validates that a
	// proposal names a repository, and EvaluateInput.Repository was left empty,
	// so every relicta_evaluate call failed with "repository is required". The
	// CLI passes repoInfo.Path for the same field.
	repoPath := s.ensureRepoPath(ctx)

	// Use adapter for full governance evaluation if available (GetStatus uses releaseServices)
	if s.adapter != nil && s.adapter.HasGovernanceService() && s.adapter.HasReleaseServices() {
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return "", userError(errors.New("no active release to evaluate — run relicta_plan first"))
		}

		// Report progress
		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 4.0
			_ = progress.Report(1, &total)
		}

		evalInput := EvaluateInput{
			ReleaseID:      status.ReleaseID,
			Repository:     repoPath,
			IncludeHistory: true,
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 4.0
			_ = progress.Report(2, &total)
		}

		output, err := s.adapter.Evaluate(ctx, evalInput)
		if err != nil {
			return "", userError(err)
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 4.0
			_ = progress.Report(4, &total)
		}

		return toJSONString(map[string]any{
			"decision":         output.Decision,
			"risk_score":       output.RiskScore,
			"severity":         output.Severity,
			"can_auto_approve": output.CanAutoApprove,
			"required_actions": output.RequiredActions,
			"risk_factors":     output.RiskFactors,
			"rationale":        output.Rationale,
		}), nil
	}

	// Fallback to basic risk calculation
	if s.riskCalc == nil {
		return "", fmt.Errorf("risk calculator not configured")
	}

	proposal := cgp.NewProposal(
		cgp.Actor{
			Kind: cgp.ActorKindAgent,
			ID:   "mcp-client",
			Name: "MCP Agent",
		},
		cgp.ProposalScope{
			Repository:  "unknown",
			CommitRange: "HEAD~5..HEAD",
		},
		cgp.ProposalIntent{
			Summary:    "Release evaluation via MCP",
			Confidence: 0.8,
		},
	)

	assessment, err := s.riskCalc.Calculate(ctx, proposal, nil)
	if err != nil {
		return "", userError(err)
	}

	return toJSONString(map[string]any{
		"score":    assessment.Score,
		"severity": string(assessment.Severity),
		"summary":  assessment.Summary,
		"factors":  assessment.Factors,
	}), nil
}

func (s *Server) handleApprove(ctx context.Context, input ApproveToolInput) (string, error) {
	// Ensure consistent repository path (fixes issue #35)
	s.ensureRepoPath(ctx)

	// Use adapter if available (GetStatus and Approve both use releaseServices)
	if s.adapter != nil && s.adapter.HasReleaseServices() {
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return "", userError(errors.New("no active release to approve — run relicta_plan first"))
		}

		// Enforce actor autonomy budget before privileged operation.
		if err := s.checkBudget(ctx, policy.Operation{
			Tool:        "relicta_approve",
			BlastRadius: blastRadiusForRiskScore(status.RiskScore),
			RiskScore:   status.RiskScore,
		}); err != nil {
			return "", err
		}

		// Elicitation: for major version releases or releases with breaking changes,
		// ask the agent for explicit confirmation before proceeding. This leverages
		// MCP elicitation to ensure the human operator acknowledges the release.
		if elicitor := mcp.ElicitFromContext(ctx); elicitor != nil {
			result, elicitErr := elicitor.Elicit(ctx, &mcp.ElicitRequest{
				Message: fmt.Sprintf(
					"Approve release %s (version %s) for publishing?",
					status.ReleaseID, status.Version,
				),
			})
			if elicitErr != nil {
				s.logger.Debug("elicitation unavailable, proceeding with approval", "error", elicitErr)
			} else if result.Action != "accept" {
				return toJSONString(map[string]any{
					"approved": false,
					"reason":   "release approval declined",
				}), nil
			}
		}

		approveInput := ApproveInput{
			ReleaseID:   status.ReleaseID,
			ApprovedBy:  "mcp-agent",
			AutoApprove: true,
			EditedNotes: input.Notes,
		}

		output, err := s.adapter.Approve(ctx, approveInput)
		if err != nil {
			return "", userError(err)
		}

		s.invalidateCache()
		return toJSONString(map[string]any{
			"approved":    output.Approved,
			"approved_by": output.ApprovedBy,
			"version":     output.Version,
		}), nil
	}

	return "", errNotConfigured("relicta_approve")
}

func (s *Server) handlePublish(ctx context.Context, input PublishToolInput) (string, error) {
	// Ensure consistent repository path (fixes issue #35)
	s.ensureRepoPath(ctx)

	// Use adapter if available (GetStatus and Publish both use releaseServices)
	if s.adapter != nil && s.adapter.HasReleaseServices() {
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return "", userError(errors.New("no active release to publish — run relicta_plan first"))
		}

		// Enforce actor autonomy budget before publish.
		if err := s.checkBudget(ctx, policy.Operation{
			Tool:        "relicta_publish",
			BlastRadius: blastRadiusForRiskScore(status.RiskScore),
			RiskScore:   status.RiskScore,
		}); err != nil {
			return "", err
		}

		// Report progress
		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 5.0
			_ = progress.Report(1, &total)
		}

		publishInput := PublishInput{
			ReleaseID: status.ReleaseID,
			DryRun:    input.DryRun,
			CreateTag: true,
			PushTag:   !input.DryRun,
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 5.0
			_ = progress.Report(2, &total)
		}

		output, err := s.adapter.Publish(ctx, publishInput)
		if err != nil {
			return "", userError(err)
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 5.0
			_ = progress.Report(4, &total)
		}

		result := map[string]any{
			"tag_name":    output.TagName,
			"release_url": output.ReleaseURL,
			"dry_run":     input.DryRun,
		}

		if len(output.PluginResults) > 0 {
			plugins := make([]map[string]any, 0, len(output.PluginResults))
			for _, pr := range output.PluginResults {
				plugins = append(plugins, map[string]any{
					"plugin":  pr.PluginName,
					"hook":    pr.Hook,
					"success": pr.Success,
					"message": pr.Message,
				})
			}
			result["plugin_results"] = plugins
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 5.0
			_ = progress.Report(5, &total)
		}

		s.invalidateCache()
		return toJSONString(result), nil
	}

	return "", errNotConfigured("relicta_publish")
}

func (s *Server) handleCancel(ctx context.Context, input CancelToolInput) (string, error) {
	// Ensure consistent repository path (fixes issue #35)
	s.ensureRepoPath(ctx)

	// Use adapter if available (GetStatus uses releaseServices, Cancel uses releaseRepo)
	if s.adapter != nil && s.adapter.HasReleaseServices() && s.adapter.HasReleaseRepository() {
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return "", fmt.Errorf("no active release to cancel: %w", err)
		}

		// Check if release can be canceled
		if status.State == "published" {
			return "", fmt.Errorf("cannot cancel a published release")
		}
		if status.State == "publishing" && !input.Force {
			return "", fmt.Errorf("cannot cancel during publishing - wait for completion or use force=true")
		}
		if status.State == "failed" || status.State == "canceled" {
			return toJSONString(map[string]any{
				"release_id": status.ReleaseID,
				"state":      status.State,
				"message":    "release is already in terminal state - use reset to start fresh",
			}), nil
		}

		// Enforce actor autonomy budget on cancel.
		if err := s.checkBudget(ctx, policy.Operation{
			Tool:        "relicta_cancel",
			BlastRadius: blastRadiusForRiskScore(status.RiskScore),
			RiskScore:   status.RiskScore,
		}); err != nil {
			return "", err
		}

		reason := input.Reason
		if reason == "" {
			reason = "canceled via MCP"
		}

		// Cancel the release
		cancelInput := CancelInput{
			ReleaseID: status.ReleaseID,
			Reason:    reason,
		}

		output, err := s.adapter.Cancel(ctx, cancelInput)
		if err != nil {
			return "", userError(err)
		}

		s.invalidateCache()
		return toJSONString(map[string]any{
			"release_id":     output.ReleaseID,
			"previous_state": output.PreviousState,
			"new_state":      output.NewState,
			"reason":         reason,
			"message":        "release canceled successfully",
		}), nil
	}

	return "", errNotConfigured("relicta_cancel")
}

func (s *Server) handleReset(ctx context.Context, input ResetToolInput) (string, error) {
	// Ensure consistent repository path (fixes issue #35)
	s.ensureRepoPath(ctx)

	// Use adapter if available (GetStatus uses releaseServices, Reset uses releaseRepo)
	if s.adapter != nil && s.adapter.HasReleaseServices() && s.adapter.HasReleaseRepository() {
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return toJSONString(map[string]any{
				"message": "no active release found - nothing to reset",
			}), nil
		}

		// Check if release can be reset
		if status.State == "published" {
			return "", fmt.Errorf("published releases cannot be reset - run 'relicta plan' to start a new release")
		}
		if status.State == "publishing" && !input.Force {
			return "", fmt.Errorf("cannot reset during publishing - wait for completion or use force=true")
		}

		// For in-progress releases that aren't failed/canceled, suggest cancel first
		if status.State != "failed" && status.State != "canceled" && !input.Force {
			return toJSONString(map[string]any{
				"release_id": status.ReleaseID,
				"state":      status.State,
				"message":    "release is in progress - use cancel first, or force=true to delete",
			}), nil
		}

		// Enforce actor autonomy budget on reset (destructive).
		if err := s.checkBudget(ctx, policy.Operation{
			Tool:        "relicta_reset",
			BlastRadius: blastRadiusForRiskScore(status.RiskScore),
			RiskScore:   status.RiskScore,
		}); err != nil {
			return "", err
		}

		// Reset (delete) the release
		resetInput := ResetInput{
			ReleaseID: status.ReleaseID,
		}

		output, err := s.adapter.Reset(ctx, resetInput)
		if err != nil {
			return "", userError(err)
		}

		s.invalidateCache()
		return toJSONString(map[string]any{
			"release_id":     output.ReleaseID,
			"previous_state": output.PreviousState,
			"deleted":        output.Deleted,
			"message":        "release reset successfully - run 'relicta plan' to start fresh",
		}), nil
	}

	return "", errNotConfigured("relicta_reset")
}

// --- Specialized AI Agent Tool Handlers ---

func (s *Server) handleBlastRadius(ctx context.Context, input BlastRadiusToolInput) (any, error) {
	if s.adapter == nil || !s.adapter.HasBlastService() {
		return toJSONString(map[string]any{
			"status":  "not_configured",
			"message": "Blast radius service not configured. This tool requires monorepo analysis to be enabled.",
		}), nil
	}

	// Report progress
	if progress := mcp.ProgressFromContext(ctx); progress != nil {
		total := 4.0
		_ = progress.Report(1, &total)
	}

	blastInput := BlastRadiusInput{
		FromRef:           input.From,
		ToRef:             input.To,
		IncludeTransitive: input.Transitive,
		GenerateGraph:     input.Graph,
		PackagePaths:      input.PackagePaths,
	}

	if progress := mcp.ProgressFromContext(ctx); progress != nil {
		total := 4.0
		_ = progress.Report(2, &total)
	}

	output, err := s.adapter.BlastRadius(ctx, blastInput)
	if err != nil {
		return "", userError(err)
	}

	if progress := mcp.ProgressFromContext(ctx); progress != nil {
		total := 4.0
		_ = progress.Report(4, &total)
	}

	// BlastRadiusOutput already carries the exact snake_case JSON tags and
	// omitempty semantics the response requires, so it doubles as the tool's
	// structured output type (advertised via OutputSchema at registration).
	return output, nil
}

func (s *Server) handleInferVersion(ctx context.Context, input InferVersionToolInput) (any, error) {
	if s.adapter == nil || !s.adapter.HasReleaseAnalyzer() {
		return toJSONString(map[string]any{
			"status":  "not_configured",
			"message": "Release analyzer not configured. Run 'relicta mcp serve' with configured dependencies.",
		}), nil
	}

	// Report progress
	if progress := mcp.ProgressFromContext(ctx); progress != nil {
		total := 2.0
		_ = progress.Report(1, &total)
	}

	inferInput := InferVersionInput{
		FromRef:     input.From,
		ToRef:       input.To,
		IncludeRisk: input.IncludeRisk,
	}

	output, err := s.adapter.InferVersion(ctx, inferInput)
	if err != nil {
		return "", userError(err)
	}

	if progress := mcp.ProgressFromContext(ctx); progress != nil {
		total := 2.0
		_ = progress.Report(2, &total)
	}

	out := InferVersionToolOutput{
		CurrentVersion: output.CurrentVersion,
		NextVersion:    output.NextVersion,
		BumpType:       output.BumpType,
		HasBreaking:    output.HasBreaking,
		HasFeatures:    output.HasFeatures,
		HasFixes:       output.HasFixes,
		CommitCount:    output.CommitCount,
		Confidence:     output.Confidence,
	}

	if len(output.Rationale) > 0 {
		out.Rationale = output.Rationale
	}

	if input.IncludeRisk {
		riskScore := output.RiskScore
		riskSeverity := output.RiskSeverity
		out.RiskScore = &riskScore
		out.RiskSeverity = &riskSeverity
	}

	return out, nil
}

func (s *Server) handleSummarizeDiff(ctx context.Context, input SummarizeDiffToolInput) (string, error) {
	if s.adapter == nil || !s.adapter.HasReleaseAnalyzer() {
		return toJSONString(map[string]any{
			"status":  "not_configured",
			"message": "Release analyzer not configured. Run 'relicta mcp serve' with configured dependencies.",
		}), nil
	}

	summarizeInput := SummarizeDiffInput{
		FromRef:   input.From,
		ToRef:     input.To,
		Audience:  input.Audience,
		MaxLength: input.MaxLength,
	}

	output, err := s.adapter.SummarizeDiff(ctx, summarizeInput)
	if err != nil {
		return "", userError(err)
	}

	result := map[string]any{
		"summary":         output.Summary,
		"audience":        output.Audience,
		"ai_generated":    output.AIGenerated,
		"character_count": output.CharacterCount,
	}

	if len(output.Highlights) > 0 {
		result["highlights"] = output.Highlights
	}
	if len(output.Categories) > 0 {
		result["categories"] = output.Categories
	}
	if output.Signals != nil {
		result["signals"] = output.Signals
		// Flag the structured-AI path so agents and CI can branch on it.
		result["structured"] = true
	}

	return toJSONString(result), nil
}

func (s *Server) handleValidateRelease(ctx context.Context, input ValidateReleaseToolInput) (ValidateReleaseToolOutput, error) {
	// Ensure consistent repository path (fixes issue #35)
	s.ensureRepoPath(ctx)

	// Get release ID from input or active release
	releaseID := input.ReleaseID
	if releaseID == "" && s.adapter != nil && s.adapter.HasReleaseRepository() {
		status, err := s.adapter.GetStatus(ctx)
		if err == nil {
			releaseID = status.ReleaseID
		}
	}

	validateInput := ValidateReleaseInput{
		ReleaseID:       releaseID,
		CheckGit:        input.CheckGit,
		CheckPlugins:    input.CheckPlugins,
		CheckGovernance: input.CheckGovernance,
		Checks:          input.Checks,
	}

	// Use adapter if available, otherwise run minimal checks
	if s.adapter != nil {
		output, err := s.adapter.ValidateRelease(ctx, validateInput)
		if err != nil {
			return ValidateReleaseToolOutput{}, userError(err)
		}

		// ValidationCheckResult already carries the exact JSON tags the
		// response requires, so checks pass through unchanged. omitempty on
		// the slice fields preserves the prior conditional-inclusion behavior.
		return ValidateReleaseToolOutput{
			Valid:          output.Valid,
			CanProceed:     output.CanProceed,
			Recommendation: output.Recommendation,
			Checks:         output.Checks,
			BlockingIssues: output.BlockingIssues,
			Warnings:       output.Warnings,
		}, nil
	}

	// Minimal validation without adapter
	return ValidateReleaseToolOutput{
		Valid:          true,
		CanProceed:     true,
		Recommendation: "Basic validation passed. Full validation requires configured dependencies.",
		Checks: []ValidationCheckResult{
			{Name: "basic", Status: "passed", Message: "Basic checks passed"},
		},
	}, nil
}

// Resource handlers

func (s *Server) handleCGPPropose(ctx context.Context, input CGPProposeToolInput) (string, error) {
	if err := s.ensureCGPService(ctx); err != nil {
		return "", err
	}

	proposal := &cgpsdk.ChangeProposal{
		CGPVersion: cgpsdk.ProtocolVersion,
		Type:       cgpsdk.TypeChangeProposal,
		ID:         cgp.GenerateProposalID(),
		Timestamp:  timeNowUTC(),
		Actor: cgpsdk.Actor{
			Kind: input.ActorKind,
			ID:   input.ActorID,
			Name: input.ActorName,
		},
		Scope: cgpsdk.Scope{
			Repository:  input.Repository,
			CommitRange: input.CommitRange,
		},
		Intent: cgpsdk.Intent{
			Summary:    input.Summary,
			Confidence: input.Confidence,
			Categories: input.Categories,
		},
	}

	decision, err := s.cgpService.EvaluateProposal(ctx, proposal)
	if err != nil {
		return "", userError(err)
	}

	// Marshal the decision using the CGP wire format codec.
	data, err := cgpsdk.Marshal(decision)
	if err != nil {
		return "", fmt.Errorf("failed to marshal decision: %w", err)
	}

	return string(data), nil
}

func (s *Server) handleCGPAuthorize(ctx context.Context, input CGPAuthorizeToolInput) (string, error) {
	if err := s.ensureCGPService(ctx); err != nil {
		return "", err
	}

	now := timeNowUTC()
	auth := &cgpsdk.ExecutionAuthorization{
		CGPVersion: cgpsdk.ProtocolVersion,
		Type:       cgpsdk.TypeExecutionAuthorization,
		ID:         cgp.GenerateAuthorizationID(),
		ProposalID: input.ProposalID,
		DecisionID: input.DecisionID,
		Timestamp:  now,
		ApprovedBy: cgpsdk.Actor{
			Kind: "human",
			ID:   input.ApproverID,
		},
		Version:    input.Version,
		ValidUntil: now.Add(24 * 60 * 60 * 1e9), // 24 hours
	}

	if err := s.cgpService.RecordAuthorization(ctx, auth); err != nil {
		return "", userError(err)
	}

	data, err := cgpsdk.Marshal(auth)
	if err != nil {
		return "", fmt.Errorf("failed to marshal authorization: %w", err)
	}

	return string(data), nil
}

func (s *Server) handleCGPStatus(ctx context.Context, input CGPStatusToolInput) (CGPStatusToolOutput, error) {
	if err := s.ensureCGPService(ctx); err != nil {
		return CGPStatusToolOutput{}, err
	}

	status, err := s.cgpService.GetStatus(ctx, input.ProposalID)
	if err != nil {
		return CGPStatusToolOutput{}, userError(err)
	}

	return CGPStatusToolOutput{
		ProposalID:    status.ProposalID,
		State:         status.State,
		Proposal:      status.Proposal,
		Decision:      status.Decision,
		Authorization: status.Authorization,
	}, nil
}
