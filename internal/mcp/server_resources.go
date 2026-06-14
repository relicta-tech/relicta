package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"go.klarlabs.de/mcp"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/domain/release"
)

func (s *Server) handleResourceState(ctx context.Context, uri string, params map[string]string) (*mcp.ResourceContent, error) {
	// Check cache first
	if s.cache != nil {
		if cached := s.cache.Get(uri); cached != nil {
			s.logger.Debug("cache hit", "uri", uri)
			if len(cached.Contents) > 0 {
				return &mcp.ResourceContent{
					URI:      cached.Contents[0].URI,
					MimeType: cached.Contents[0].MIMEType,
					Text:     cached.Contents[0].Text,
				}, nil
			}
		}
	}

	if s.releaseRepo == nil {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     `{"status": "no release repository configured"}`,
		}, nil
	}

	releases, err := s.releaseRepo.FindActive(ctx)
	if err != nil || len(releases) == 0 {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     `{"status": "no active release"}`,
		}, nil
	}

	rel := releases[0]
	version := ""
	if !rel.VersionNext().IsZero() {
		version = rel.VersionNext().String()
	}

	content := fmt.Sprintf(`{
  "state": %q,
  "version": %q,
  "created_at": %q,
  "updated_at": %q
}`, rel.State().String(), version, rel.CreatedAt().Format("2006-01-02T15:04:05Z07:00"), rel.UpdatedAt().Format("2006-01-02T15:04:05Z07:00"))

	result := &mcp.ResourceContent{
		URI:      uri,
		MimeType: "application/json",
		Text:     content,
	}

	// Cache the result
	if s.cache != nil {
		s.cache.Set(uri, &ReadResourceResult{
			Contents: []ResourceContent{{URI: uri, MIMEType: "application/json", Text: content}},
		})
	}

	return result, nil
}

func (s *Server) handleResourceConfig(ctx context.Context, uri string, params map[string]string) (*mcp.ResourceContent, error) {
	if s.config == nil {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     `{"status": "no configuration loaded"}`,
		}, nil
	}

	productName := s.config.Changelog.ProductName
	if productName == "" {
		productName = "Relicta"
	}

	content := fmt.Sprintf(`{
  "product_name": %q,
  "ai_enabled": %t,
  "ai_provider": %q,
  "versioning_strategy": %q
}`, productName, s.config.AI.Enabled, s.config.AI.Provider, s.config.Versioning.Strategy)

	return &mcp.ResourceContent{
		URI:      uri,
		MimeType: "application/json",
		Text:     content,
	}, nil
}

func (s *Server) handleResourceCommits(ctx context.Context, uri string, params map[string]string) (*mcp.ResourceContent, error) {
	if s.releaseRepo == nil {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     `{"status": "no release repository configured"}`,
		}, nil
	}

	releases, err := s.releaseRepo.FindActive(ctx)
	if err != nil || len(releases) == 0 {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     `{"status": "no active release", "commits": []}`,
		}, nil
	}

	rel := releases[0]
	plan := release.GetPlan(rel)
	if plan == nil {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     `{"status": "no plan available", "commits": []}`,
		}, nil
	}

	// Check if changeset is loaded
	if !plan.HasChangeSet() {
		content := fmt.Sprintf(`{
  "status": "changeset not loaded",
  "changeset_id": %q,
  "release_type": %q,
  "current_version": %q,
  "next_version": %q,
  "commits": []
}`, plan.ChangeSetID, plan.ReleaseType, plan.CurrentVersion.String(), plan.NextVersion.String())
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     content,
		}, nil
	}

	// Get commits from changeset
	changeSet := plan.GetChangeSet()
	commits := changeSet.Commits()

	// Build commits array
	commitList := make([]map[string]any, 0, len(commits))
	for _, c := range commits {
		commit := map[string]any{
			"sha":      c.ShortHash(),
			"full_sha": c.Hash(),
			"type":     string(c.Type()),
			"subject":  c.Subject(),
			"author":   c.Author(),
			"date":     c.Date().Format("2006-01-02T15:04:05Z07:00"),
			"breaking": c.IsBreaking(),
		}
		if c.Scope() != "" {
			commit["scope"] = c.Scope()
		}
		commitList = append(commitList, commit)
	}

	result := map[string]any{
		"status":          "ok",
		"changeset_id":    string(plan.ChangeSetID),
		"release_type":    string(plan.ReleaseType),
		"current_version": plan.CurrentVersion.String(),
		"next_version":    plan.NextVersion.String(),
		"commit_count":    len(commits),
		"commits":         commitList,
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     fmt.Sprintf(`{"status": "error", "error": %q}`, err.Error()),
		}, nil
	}

	return &mcp.ResourceContent{
		URI:      uri,
		MimeType: "application/json",
		Text:     string(jsonBytes),
	}, nil
}

func (s *Server) handleResourceChangelog(ctx context.Context, uri string, params map[string]string) (*mcp.ResourceContent, error) {
	if s.releaseRepo == nil {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "text/markdown",
			Text:     "# Changelog\n\nNo release repository configured.",
		}, nil
	}

	releases, err := s.releaseRepo.FindActive(ctx)
	if err != nil || len(releases) == 0 {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "text/markdown",
			Text:     "# Changelog\n\nNo active release found. Run `relicta plan` to start a new release.",
		}, nil
	}

	rel := releases[0]
	notes := rel.Notes()

	if notes == nil {
		version := ""
		if !rel.VersionNext().IsZero() {
			version = rel.VersionNext().String()
		}

		content := fmt.Sprintf("# Changelog\n\nNo changelog generated yet for version %s.\n\nRun `relicta notes` to generate release notes.", version)
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "text/markdown",
			Text:     content,
		}, nil
	}

	changelog := notes.Text
	if changelog == "" {
		changelog = "# Release Notes\n\nNo content available."
	}

	return &mcp.ResourceContent{
		URI:      uri,
		MimeType: "text/markdown",
		Text:     changelog,
	}, nil
}

func (s *Server) handleResourceRiskReport(ctx context.Context, uri string, params map[string]string) (*mcp.ResourceContent, error) {
	if s.releaseRepo == nil {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     `{"status": "no release repository configured"}`,
		}, nil
	}

	releases, err := s.releaseRepo.FindActive(ctx)
	if err != nil || len(releases) == 0 {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     `{"status": "no active release"}`,
		}, nil
	}

	rel := releases[0]

	// Try to get risk assessment from adapter if available
	if s.adapter != nil && s.adapter.HasGovernanceService() {
		evalInput := EvaluateInput{
			ReleaseID:      string(rel.ID()),
			IncludeHistory: false,
		}

		output, err := s.adapter.Evaluate(ctx, evalInput)
		if err == nil {
			result := map[string]any{
				"status":           "ok",
				"decision":         output.Decision,
				"risk_score":       output.RiskScore,
				"severity":         output.Severity,
				"can_auto_approve": output.CanAutoApprove,
				"required_actions": output.RequiredActions,
				"risk_factors":     output.RiskFactors,
				"rationale":        output.Rationale,
			}

			jsonBytes, err := json.MarshalIndent(result, "", "  ")
			if err == nil {
				return &mcp.ResourceContent{
					URI:      uri,
					MimeType: "application/json",
					Text:     string(jsonBytes),
				}, nil
			}
		}
	}

	// Fallback: Use basic risk calculation if available
	if s.riskCalc != nil {
		proposal := cgp.NewProposal(
			cgp.Actor{
				Kind: cgp.ActorKindAgent,
				ID:   "mcp-resource-reader",
				Name: "MCP Resource Reader",
			},
			cgp.ProposalScope{
				Repository:  rel.RepoID(),
				CommitRange: "HEAD~5..HEAD",
			},
			cgp.ProposalIntent{
				Summary:    "Risk assessment for active release",
				Confidence: 0.8,
			},
		)

		assessment, err := s.riskCalc.Calculate(ctx, proposal, nil)
		if err == nil {
			result := map[string]any{
				"status":   "ok",
				"score":    assessment.Score,
				"severity": string(assessment.Severity),
				"summary":  assessment.Summary,
				"factors":  assessment.Factors,
			}

			jsonBytes, err := json.MarshalIndent(result, "", "  ")
			if err == nil {
				return &mcp.ResourceContent{
					URI:      uri,
					MimeType: "application/json",
					Text:     string(jsonBytes),
				}, nil
			}
		}
	}

	return &mcp.ResourceContent{
		URI:      uri,
		MimeType: "application/json",
		Text:     `{"status": "no risk assessment available", "hint": "Run 'relicta evaluate' to perform risk assessment"}`,
	}, nil
}

// Prompt handlers
