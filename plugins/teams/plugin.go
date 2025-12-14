// Package main implements the Microsoft Teams plugin for Relicta.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

// Shared HTTP client for connection reuse across requests.
// Includes security hardening: TLS 1.3+, redirect protection, SSRF prevention.
var defaultHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		// Limit redirect chain length
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		// Prevent redirect to non-HTTPS
		if req.URL.Scheme != "https" {
			return fmt.Errorf("redirect to non-HTTPS URL not allowed")
		}
		// Prevent redirect away from Teams webhook hosts (SSRF protection)
		host := req.URL.Host
		if !isValidTeamsHost(host) {
			return fmt.Errorf("redirect away from Teams webhook host not allowed")
		}
		return nil
	},
	Transport: &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
		},
	},
}

// isValidTeamsHost checks if a host is a valid Teams webhook host.
func isValidTeamsHost(host string) bool {
	validHosts := []string{
		"outlook.office.com",
		"outlook.office365.com",
	}

	for _, valid := range validHosts {
		if host == valid || strings.HasSuffix(host, "."+valid) {
			return true
		}
	}

	// Also allow *.webhook.office.com pattern
	if strings.HasSuffix(host, ".webhook.office.com") {
		return true
	}

	return false
}

// TeamsPlugin implements the Microsoft Teams notification plugin.
type TeamsPlugin struct{}

// Config represents the Teams plugin configuration.
type Config struct {
	// WebhookURL is the Teams webhook URL.
	WebhookURL string `json:"webhook_url,omitempty"`
	// NotifyOnSuccess sends notification on successful release.
	NotifyOnSuccess bool `json:"notify_on_success"`
	// NotifyOnError sends notification on failed release.
	NotifyOnError bool `json:"notify_on_error"`
	// IncludeChangelog includes changelog in the notification.
	IncludeChangelog bool `json:"include_changelog"`
	// ThemeColor is the color for the card accent (hex color).
	ThemeColor string `json:"theme_color,omitempty"`
	// Mentions is a list of users/groups to mention.
	Mentions []string `json:"mentions,omitempty"`
}

// AdaptiveCard represents a Microsoft Adaptive Card.
// Reference: https://adaptivecards.io/explorer/AdaptiveCard.html
type AdaptiveCard struct {
	Type    string        `json:"type"`
	Version string        `json:"version"`
	Schema  string        `json:"$schema"`
	Body    []interface{} `json:"body"`
}

// TextBlock represents a text block in an Adaptive Card.
type TextBlock struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	Size   string `json:"size,omitempty"`
	Weight string `json:"weight,omitempty"`
	Wrap   bool   `json:"wrap,omitempty"`
	Color  string `json:"color,omitempty"`
}

// FactSet represents a set of facts (key-value pairs) in an Adaptive Card.
type FactSet struct {
	Type  string `json:"type"`
	Facts []Fact `json:"facts"`
}

// Fact represents a single fact (key-value pair).
type Fact struct {
	Title string `json:"title"`
	Value string `json:"value"`
}

// Container represents a container in an Adaptive Card.
type Container struct {
	Type  string        `json:"type"`
	Items []interface{} `json:"items"`
	Style string        `json:"style,omitempty"`
	Bleed bool          `json:"bleed,omitempty"`
}

// TeamsMessage represents a Teams message payload with MessageCard format.
type TeamsMessage struct {
	Type        string       `json:"@type"`
	Context     string       `json:"@context"`
	ThemeColor  string       `json:"themeColor,omitempty"`
	Summary     string       `json:"summary"`
	Sections    []Section    `json:"sections,omitempty"`
	Text        string       `json:"text,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Section represents a section in a MessageCard.
type Section struct {
	ActivityTitle    string `json:"activityTitle,omitempty"`
	ActivitySubtitle string `json:"activitySubtitle,omitempty"`
	ActivityImage    string `json:"activityImage,omitempty"`
	Facts            []Fact `json:"facts,omitempty"`
	Text             string `json:"text,omitempty"`
	Markdown         bool   `json:"markdown,omitempty"`
}

// Attachment represents an attachment with Adaptive Card.
type Attachment struct {
	ContentType string       `json:"contentType"`
	Content     AdaptiveCard `json:"content"`
}

// GetInfo returns plugin metadata.
func (p *TeamsPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "teams",
		Version:     "1.0.0",
		Description: "Send Microsoft Teams notifications for releases",
		Author:      "Relicta Team",
		Hooks: []plugin.Hook{
			plugin.HookPostPublish,
			plugin.HookOnSuccess,
			plugin.HookOnError,
		},
		ConfigSchema: `{
			"type": "object",
			"properties": {
				"webhook_url": {"type": "string", "description": "Teams webhook URL (or use TEAMS_WEBHOOK_URL env)"},
				"notify_on_success": {"type": "boolean", "description": "Notify on success", "default": true},
				"notify_on_error": {"type": "boolean", "description": "Notify on error", "default": true},
				"include_changelog": {"type": "boolean", "description": "Include changelog", "default": false},
				"theme_color": {"type": "string", "description": "Theme color (hex)", "default": "28a745"},
				"mentions": {"type": "array", "items": {"type": "string"}, "description": "Users/groups to mention"}
			},
			"required": ["webhook_url"]
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *TeamsPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	cfg := p.parseConfig(req.Config)

	switch req.Hook {
	case plugin.HookPostPublish, plugin.HookOnSuccess:
		if !cfg.NotifyOnSuccess {
			return &plugin.ExecuteResponse{
				Success: true,
				Message: "Success notification disabled",
			}, nil
		}
		return p.sendSuccessNotification(ctx, cfg, req.Context, req.DryRun)

	case plugin.HookOnError:
		if !cfg.NotifyOnError {
			return &plugin.ExecuteResponse{
				Success: true,
				Message: "Error notification disabled",
			}, nil
		}
		return p.sendErrorNotification(ctx, cfg, req.Context, req.DryRun)

	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

// sendSuccessNotification sends a success notification.
func (p *TeamsPlugin) sendSuccessNotification(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	// Build Adaptive Card
	title := fmt.Sprintf("🚀 Release %s Published!", releaseCtx.Version)

	facts := []Fact{
		{Title: "Version", Value: releaseCtx.Version},
		{Title: "Release Type", Value: cases.Title(language.English).String(releaseCtx.ReleaseType)},
		{Title: "Branch", Value: releaseCtx.Branch},
		{Title: "Tag", Value: releaseCtx.TagName},
	}

	if releaseCtx.Changes != nil {
		features := len(releaseCtx.Changes.Features)
		fixes := len(releaseCtx.Changes.Fixes)
		breaking := len(releaseCtx.Changes.Breaking)

		summary := fmt.Sprintf("%d features, %d fixes", features, fixes)
		if breaking > 0 {
			summary += fmt.Sprintf(", %d breaking changes", breaking)
		}
		facts = append(facts, Fact{Title: "Changes", Value: summary})
	}

	// Build card body
	body := []interface{}{
		TextBlock{
			Type:   "TextBlock",
			Text:   title,
			Size:   "large",
			Weight: "bolder",
			Wrap:   true,
		},
		FactSet{
			Type:  "FactSet",
			Facts: facts,
		},
	}

	// Add changelog if enabled
	if cfg.IncludeChangelog && releaseCtx.ReleaseNotes != "" {
		notes := releaseCtx.ReleaseNotes
		// Truncate if too long (Teams has a 28KB limit)
		if len(notes) > 2000 {
			notes = notes[:2000] + "..."
		}
		// Escape HTML to prevent XSS attacks in release notes
		notes = html.EscapeString(notes)
		body = append(body, TextBlock{
			Type: "TextBlock",
			Text: notes,
			Wrap: true,
		})
	}

	card := AdaptiveCard{
		Type:    "AdaptiveCard",
		Version: "1.4",
		Schema:  "http://adaptivecards.io/schemas/adaptive-card.json",
		Body:    body,
	}

	// Build message with mentions
	text := ""
	if len(cfg.Mentions) > 0 {
		// Teams uses @mention format similar to plain text
		text = plugin.BuildMentionText(cfg.Mentions, plugin.MentionFormatPlain)
	}

	msg := TeamsMessage{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: getThemeColor(cfg.ThemeColor, "28a745"), // Green
		Summary:    title,
		Text:       text,
		Attachments: []Attachment{
			{
				ContentType: "application/vnd.microsoft.card.adaptive",
				Content:     card,
			},
		},
	}

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Would send Teams success notification",
			Outputs: map[string]any{
				"version": releaseCtx.Version,
			},
		}, nil
	}

	if err := p.sendMessage(ctx, cfg.WebhookURL, msg); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to send Teams message: %v", err),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: "Sent Teams success notification",
	}, nil
}

// sendErrorNotification sends an error notification.
func (p *TeamsPlugin) sendErrorNotification(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	title := fmt.Sprintf("❌ Release %s Failed", releaseCtx.Version)

	facts := []Fact{
		{Title: "Version", Value: releaseCtx.Version},
		{Title: "Branch", Value: releaseCtx.Branch},
	}

	// Build card body
	body := []interface{}{
		TextBlock{
			Type:   "TextBlock",
			Text:   title,
			Size:   "large",
			Weight: "bolder",
			Wrap:   true,
			Color:  "attention",
		},
		FactSet{
			Type:  "FactSet",
			Facts: facts,
		},
	}

	card := AdaptiveCard{
		Type:    "AdaptiveCard",
		Version: "1.4",
		Schema:  "http://adaptivecards.io/schemas/adaptive-card.json",
		Body:    body,
	}

	// Build message with mentions
	text := ""
	if len(cfg.Mentions) > 0 {
		// Teams uses @mention format similar to plain text
		text = plugin.BuildMentionText(cfg.Mentions, plugin.MentionFormatPlain)
	}

	msg := TeamsMessage{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: getThemeColor(cfg.ThemeColor, "dc3545"), // Red
		Summary:    title,
		Text:       text,
		Attachments: []Attachment{
			{
				ContentType: "application/vnd.microsoft.card.adaptive",
				Content:     card,
			},
		},
	}

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Would send Teams error notification",
		}, nil
	}

	if err := p.sendMessage(ctx, cfg.WebhookURL, msg); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to send Teams message: %v", err),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: "Sent Teams error notification",
	}, nil
}

// sendMessage sends a message to Teams.
func (p *TeamsPlugin) sendMessage(ctx context.Context, webhookURL string, msg TeamsMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Teams has a 28KB message size limit
	if len(payload) > 28*1024 {
		return fmt.Errorf("message size exceeds Teams 28KB limit")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read response body for better error diagnostics (limit to 1KB)
		limitedReader := io.LimitReader(resp.Body, 1024)
		bodyBytes, readErr := io.ReadAll(limitedReader)
		body := ""
		if readErr == nil && len(bodyBytes) > 0 {
			body = strings.TrimSpace(string(bodyBytes))
		}

		if body != "" {
			return fmt.Errorf("teams returned status %d: %s", resp.StatusCode, body)
		}
		return fmt.Errorf("teams returned status %d", resp.StatusCode)
	}

	return nil
}

// parseConfig parses the plugin configuration using the shared ConfigParser.
func (p *TeamsPlugin) parseConfig(raw map[string]any) *Config {
	parser := plugin.NewConfigParser(raw)

	return &Config{
		WebhookURL:       parser.GetString("webhook_url", "TEAMS_WEBHOOK_URL"),
		NotifyOnSuccess:  parser.GetBoolDefault("notify_on_success", true),
		NotifyOnError:    parser.GetBoolDefault("notify_on_error", true),
		IncludeChangelog: parser.GetBool("include_changelog"),
		ThemeColor:       parser.GetString("theme_color"),
		Mentions:         parser.GetStringSlice("mentions"),
	}
}

// getThemeColor returns the color if non-empty, otherwise the default.
func getThemeColor(color, defaultColor string) string {
	if color != "" {
		return color
	}
	return defaultColor
}

// Validate validates the plugin configuration using the shared ValidationBuilder.
func (p *TeamsPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := plugin.NewValidationBuilder()

	// Get webhook URL with env fallback
	parser := plugin.NewConfigParser(config)
	webhook := parser.GetString("webhook_url", "TEAMS_WEBHOOK_URL")

	if webhook == "" {
		vb.AddError("webhook_url",
			"Teams webhook URL is required (set TEAMS_WEBHOOK_URL env var or configure webhook_url)",
			"required")
	} else {
		// Validate HTTPS scheme
		if !strings.HasPrefix(webhook, "https://") {
			vb.AddFormatError("webhook_url", "URL must use HTTPS")
		} else {
			// Extract and validate host
			hostStart := len("https://")
			hostEnd := strings.Index(webhook[hostStart:], "/")
			if hostEnd == -1 {
				hostEnd = len(webhook)
			} else {
				hostEnd += hostStart
			}
			host := webhook[hostStart:hostEnd]

			if !isValidTeamsHost(host) {
				vb.AddFormatError("webhook_url",
					"URL must be a valid Teams webhook host (outlook.office.com, outlook.office365.com, or *.webhook.office.com)")
			}
		}
	}

	// Validate theme color if provided
	themeColor := parser.GetString("theme_color")
	if themeColor != "" {
		// Simple hex color validation (6 digits, with or without #)
		cleaned := strings.TrimPrefix(themeColor, "#")
		if len(cleaned) != 6 {
			vb.AddFormatError("theme_color", "theme color must be a 6-digit hex color (e.g., '28a745' or '#28a745')")
		}
	}

	return vb.Build(), nil
}
