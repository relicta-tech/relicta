// Package main implements tests for the Microsoft Teams plugin.
package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

func TestIsValidTeamsHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want bool
	}{
		{
			name: "valid outlook.office.com",
			host: "outlook.office.com",
			want: true,
		},
		{
			name: "valid outlook.office365.com",
			host: "outlook.office365.com",
			want: true,
		},
		{
			name: "valid webhook.office.com pattern",
			host: "emea.webhook.office.com",
			want: true,
		},
		{
			name: "invalid host - malicious subdomain",
			host: "outlook.office.com.evil.com",
			want: false,
		},
		{
			name: "invalid host - wrong domain",
			host: "evil.com",
			want: false,
		},
		{
			name: "invalid host - partial match",
			host: "office.com",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidTeamsHost(tt.host)
			if got != tt.want {
				t.Errorf("isValidTeamsHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

func TestTeamsPlugin_Validate(t *testing.T) {
	p := &TeamsPlugin{}
	ctx := context.Background()

	tests := []struct {
		name      string
		config    map[string]any
		wantValid bool
		wantField string
		wantError string
	}{
		{
			name: "valid config with webhook_url",
			config: map[string]any{
				"webhook_url": "https://outlook.office.com/webhook/xxx",
			},
			wantValid: true,
		},
		{
			name: "valid config with outlook.office365.com",
			config: map[string]any{
				"webhook_url": "https://outlook.office365.com/webhook/test",
			},
			wantValid: true,
		},
		{
			name: "valid config with webhook.office.com",
			config: map[string]any{
				"webhook_url": "https://emea.webhook.office.com/webhookb2/test",
			},
			wantValid: true,
		},
		{
			name:      "missing webhook_url",
			config:    map[string]any{},
			wantValid: false,
			wantField: "webhook_url",
		},
		{
			name: "invalid webhook URL - HTTP instead of HTTPS",
			config: map[string]any{
				"webhook_url": "http://outlook.office.com/webhook/test",
			},
			wantValid: false,
			wantField: "webhook_url",
			wantError: "HTTPS",
		},
		{
			name: "invalid webhook URL - wrong host",
			config: map[string]any{
				"webhook_url": "https://evil.com/webhook/steal",
			},
			wantValid: false,
			wantField: "webhook_url",
			wantError: "valid Teams webhook host",
		},
		{
			name: "invalid webhook URL - subdomain attack",
			config: map[string]any{
				"webhook_url": "https://outlook.office.com.malicious.com/webhook/test",
			},
			wantValid: false,
			wantField: "webhook_url",
			wantError: "valid Teams webhook host",
		},
		{
			name: "valid theme color with # prefix",
			config: map[string]any{
				"webhook_url": "https://outlook.office.com/webhook/xxx",
				"theme_color": "#28a745",
			},
			wantValid: true,
		},
		{
			name: "valid theme color without # prefix",
			config: map[string]any{
				"webhook_url": "https://outlook.office.com/webhook/xxx",
				"theme_color": "28a745",
			},
			wantValid: true,
		},
		{
			name: "invalid theme color - too short",
			config: map[string]any{
				"webhook_url": "https://outlook.office.com/webhook/xxx",
				"theme_color": "fff",
			},
			wantValid: false,
			wantField: "theme_color",
			wantError: "6-digit hex",
		},
		{
			name: "invalid theme color - too long",
			config: map[string]any{
				"webhook_url": "https://outlook.office.com/webhook/xxx",
				"theme_color": "#28a74512",
			},
			wantValid: false,
			wantField: "theme_color",
			wantError: "6-digit hex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := p.Validate(ctx, tt.config)
			if err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}
			if resp.Valid != tt.wantValid {
				t.Errorf("Validate() valid = %v, want %v (errors: %v)", resp.Valid, tt.wantValid, resp.Errors)
			}
			if !tt.wantValid && tt.wantField != "" {
				found := false
				var foundError string
				for _, e := range resp.Errors {
					if e.Field == tt.wantField {
						found = true
						foundError = e.Message
						break
					}
				}
				if !found {
					t.Errorf("expected validation error for field %q", tt.wantField)
				}
				if tt.wantError != "" && foundError != "" {
					if !strings.Contains(foundError, tt.wantError) {
						t.Errorf("error %q should contain %q", foundError, tt.wantError)
					}
				}
			}
		})
	}
}

func TestTeamsPlugin_GetInfo(t *testing.T) {
	p := &TeamsPlugin{}
	info := p.GetInfo()

	if info.Name != "teams" {
		t.Errorf("GetInfo().Name = %q, want %q", info.Name, "teams")
	}
	if info.Version == "" {
		t.Error("GetInfo().Version should not be empty")
	}
	if len(info.Hooks) == 0 {
		t.Error("GetInfo().Hooks should not be empty")
	}
	// Verify expected hooks
	expectedHooks := []plugin.Hook{
		plugin.HookPostPublish,
		plugin.HookOnSuccess,
		plugin.HookOnError,
	}
	if len(info.Hooks) != len(expectedHooks) {
		t.Errorf("GetInfo().Hooks count = %d, want %d", len(info.Hooks), len(expectedHooks))
	}
}

func TestTeamsPlugin_ParseConfig(t *testing.T) {
	p := &TeamsPlugin{}

	t.Run("defaults are applied", func(t *testing.T) {
		cfg := p.parseConfig(map[string]any{})
		if !cfg.NotifyOnSuccess {
			t.Error("NotifyOnSuccess should be true by default")
		}
		if !cfg.NotifyOnError {
			t.Error("NotifyOnError should be true by default")
		}
		if cfg.IncludeChangelog {
			t.Error("IncludeChangelog should be false by default")
		}
	})

	t.Run("values are parsed correctly", func(t *testing.T) {
		cfg := p.parseConfig(map[string]any{
			"webhook_url":       "https://outlook.office.com/webhook/xxx",
			"notify_on_success": false,
			"theme_color":       "dc3545",
			"include_changelog": true,
			"mentions":          []any{"@user1", "@user2"},
		})
		if cfg.WebhookURL != "https://outlook.office.com/webhook/xxx" {
			t.Errorf("WebhookURL = %q", cfg.WebhookURL)
		}
		if cfg.NotifyOnSuccess {
			t.Error("NotifyOnSuccess should be false")
		}
		if cfg.ThemeColor != "dc3545" {
			t.Errorf("ThemeColor = %q", cfg.ThemeColor)
		}
		if !cfg.IncludeChangelog {
			t.Error("IncludeChangelog should be true")
		}
		if len(cfg.Mentions) != 2 {
			t.Errorf("Mentions len = %d, want 2", len(cfg.Mentions))
		}
	})
}

func TestGetThemeColor(t *testing.T) {
	tests := []struct {
		name         string
		color        string
		defaultColor string
		want         string
	}{
		{
			name:         "uses provided color",
			color:        "28a745",
			defaultColor: "dc3545",
			want:         "28a745",
		},
		{
			name:         "uses default when empty",
			color:        "",
			defaultColor: "dc3545",
			want:         "dc3545",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getThemeColor(tt.color, tt.defaultColor)
			if got != tt.want {
				t.Errorf("getThemeColor(%q, %q) = %q, want %q", tt.color, tt.defaultColor, got, tt.want)
			}
		})
	}
}

func TestTeamsPlugin_Execute(t *testing.T) {
	p := &TeamsPlugin{}
	ctx := context.Background()

	tests := []struct {
		name        string
		hook        plugin.Hook
		config      map[string]any
		releaseCtx  plugin.ReleaseContext
		dryRun      bool
		wantSuccess bool
		wantMessage string
	}{
		{
			name: "success notification enabled",
			hook: plugin.HookOnSuccess,
			config: map[string]any{
				"webhook_url":       "https://outlook.office.com/webhook/test",
				"notify_on_success": true,
			},
			releaseCtx: plugin.ReleaseContext{
				Version:     "1.0.0",
				ReleaseType: "major",
				Branch:      "main",
				TagName:     "v1.0.0",
			},
			dryRun:      true,
			wantSuccess: true,
			wantMessage: "Would send Teams success notification",
		},
		{
			name: "success notification disabled",
			hook: plugin.HookOnSuccess,
			config: map[string]any{
				"webhook_url":       "https://outlook.office.com/webhook/test",
				"notify_on_success": false,
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "1.0.0",
			},
			dryRun:      false,
			wantSuccess: true,
			wantMessage: "Success notification disabled",
		},
		{
			name: "error notification enabled",
			hook: plugin.HookOnError,
			config: map[string]any{
				"webhook_url":     "https://outlook.office.com/webhook/test",
				"notify_on_error": true,
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "1.0.0",
				Branch:  "main",
			},
			dryRun:      true,
			wantSuccess: true,
			wantMessage: "Would send Teams error notification",
		},
		{
			name: "error notification disabled",
			hook: plugin.HookOnError,
			config: map[string]any{
				"webhook_url":     "https://outlook.office.com/webhook/test",
				"notify_on_error": false,
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "1.0.0",
			},
			dryRun:      false,
			wantSuccess: true,
			wantMessage: "Error notification disabled",
		},
		{
			name: "unhandled hook",
			hook: plugin.HookPrePublish,
			config: map[string]any{
				"webhook_url": "https://outlook.office.com/webhook/test",
			},
			releaseCtx:  plugin.ReleaseContext{},
			dryRun:      false,
			wantSuccess: true,
			wantMessage: "Hook pre-publish not handled",
		},
		{
			name: "PostPublish hook with success notification",
			hook: plugin.HookPostPublish,
			config: map[string]any{
				"webhook_url":       "https://outlook.office.com/webhook/test",
				"notify_on_success": true,
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "1.0.0",
			},
			dryRun:      true,
			wantSuccess: true,
			wantMessage: "Would send Teams success notification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := plugin.ExecuteRequest{
				Hook:    tt.hook,
				Config:  tt.config,
				Context: tt.releaseCtx,
				DryRun:  tt.dryRun,
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				t.Fatalf("Execute() unexpected error: %v", err)
			}
			if resp.Success != tt.wantSuccess {
				t.Errorf("Execute() success = %v, want %v", resp.Success, tt.wantSuccess)
			}
			if resp.Message != tt.wantMessage {
				t.Errorf("Execute() message = %q, want %q", resp.Message, tt.wantMessage)
			}
		})
	}
}

func TestTeamsPlugin_SendSuccessNotification(t *testing.T) {
	// Mock HTTP server
	var receivedPayload TeamsMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := &TeamsPlugin{}
	ctx := context.Background()

	tests := []struct {
		name        string
		config      *Config
		releaseCtx  plugin.ReleaseContext
		dryRun      bool
		wantSuccess bool
		wantMessage string
	}{
		{
			name: "successful notification with changelog",
			config: &Config{
				WebhookURL:       server.URL,
				NotifyOnSuccess:  true,
				IncludeChangelog: true,
				ThemeColor:       "28a745",
				Mentions:         []string{"@user1", "@user2"},
			},
			releaseCtx: plugin.ReleaseContext{
				Version:      "2.0.0",
				ReleaseType:  "major",
				Branch:       "main",
				TagName:      "v2.0.0",
				ReleaseNotes: "## What's New\n\n- Feature A\n- Feature B",
				Changes: &plugin.CategorizedChanges{
					Features: []plugin.ConventionalCommit{
						{Type: "feat", Scope: "", Description: "Feature A"},
						{Type: "feat", Scope: "", Description: "Feature B"},
					},
					Fixes: []plugin.ConventionalCommit{
						{Type: "fix", Scope: "", Description: "Fix 1"},
					},
					Breaking: []plugin.ConventionalCommit{},
				},
			},
			dryRun:      false,
			wantSuccess: true,
			wantMessage: "Sent Teams success notification",
		},
		{
			name: "successful notification without changelog",
			config: &Config{
				WebhookURL:       server.URL,
				NotifyOnSuccess:  true,
				IncludeChangelog: false,
				ThemeColor:       "",
			},
			releaseCtx: plugin.ReleaseContext{
				Version:     "1.0.1",
				ReleaseType: "patch",
				Branch:      "main",
				TagName:     "v1.0.1",
			},
			dryRun:      false,
			wantSuccess: true,
			wantMessage: "Sent Teams success notification",
		},
		{
			name: "dry run mode",
			config: &Config{
				WebhookURL:      server.URL,
				NotifyOnSuccess: true,
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "1.0.0",
			},
			dryRun:      true,
			wantSuccess: true,
			wantMessage: "Would send Teams success notification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := p.sendSuccessNotification(ctx, tt.config, tt.releaseCtx, tt.dryRun)
			if err != nil {
				t.Fatalf("sendSuccessNotification() unexpected error: %v", err)
			}
			if resp.Success != tt.wantSuccess {
				t.Errorf("sendSuccessNotification() success = %v, want %v", resp.Success, tt.wantSuccess)
			}
			if resp.Message != tt.wantMessage {
				t.Errorf("sendSuccessNotification() message = %q, want %q", resp.Message, tt.wantMessage)
			}

			// Verify message structure for non-dry-run
			if !tt.dryRun {
				if receivedPayload.Type != "MessageCard" {
					t.Errorf("message type = %q, want %q", receivedPayload.Type, "MessageCard")
				}
				if receivedPayload.ThemeColor != getThemeColor(tt.config.ThemeColor, "28a745") {
					t.Errorf("theme color = %q, want %q", receivedPayload.ThemeColor, getThemeColor(tt.config.ThemeColor, "28a745"))
				}
				if len(receivedPayload.Attachments) != 1 {
					t.Errorf("attachments count = %d, want 1", len(receivedPayload.Attachments))
				}
				if receivedPayload.Attachments[0].ContentType != "application/vnd.microsoft.card.adaptive" {
					t.Errorf("attachment content type = %q", receivedPayload.Attachments[0].ContentType)
				}
			}
		})
	}
}

func TestTeamsPlugin_SendErrorNotification(t *testing.T) {
	// Mock HTTP server
	var receivedPayload TeamsMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := &TeamsPlugin{}
	ctx := context.Background()

	tests := []struct {
		name        string
		config      *Config
		releaseCtx  plugin.ReleaseContext
		dryRun      bool
		wantSuccess bool
		wantMessage string
	}{
		{
			name: "error notification",
			config: &Config{
				WebhookURL:    server.URL,
				NotifyOnError: true,
				ThemeColor:    "dc3545",
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "1.0.0",
				Branch:  "main",
			},
			dryRun:      false,
			wantSuccess: true,
			wantMessage: "Sent Teams error notification",
		},
		{
			name: "dry run mode",
			config: &Config{
				WebhookURL:    server.URL,
				NotifyOnError: true,
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "1.0.0",
			},
			dryRun:      true,
			wantSuccess: true,
			wantMessage: "Would send Teams error notification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := p.sendErrorNotification(ctx, tt.config, tt.releaseCtx, tt.dryRun)
			if err != nil {
				t.Fatalf("sendErrorNotification() unexpected error: %v", err)
			}
			if resp.Success != tt.wantSuccess {
				t.Errorf("sendErrorNotification() success = %v, want %v", resp.Success, tt.wantSuccess)
			}
			if resp.Message != tt.wantMessage {
				t.Errorf("sendErrorNotification() message = %q, want %q", resp.Message, tt.wantMessage)
			}

			// Verify message structure for non-dry-run
			if !tt.dryRun {
				if receivedPayload.Type != "MessageCard" {
					t.Errorf("message type = %q, want %q", receivedPayload.Type, "MessageCard")
				}
				if receivedPayload.ThemeColor != getThemeColor(tt.config.ThemeColor, "dc3545") {
					t.Errorf("theme color = %q, want %q", receivedPayload.ThemeColor, getThemeColor(tt.config.ThemeColor, "dc3545"))
				}
			}
		})
	}
}

func TestTeamsPlugin_SendMessage(t *testing.T) {
	p := &TeamsPlugin{}
	ctx := context.Background()

	t.Run("successful HTTP request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("request method = %s, want POST", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("content type = %s, want application/json", r.Header.Get("Content-Type"))
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		msg := TeamsMessage{
			Type:    "MessageCard",
			Context: "https://schema.org/extensions",
			Summary: "Test",
		}

		err := p.sendMessage(ctx, server.URL, msg)
		if err != nil {
			t.Errorf("sendMessage() unexpected error: %v", err)
		}
	})

	t.Run("HTTP error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		msg := TeamsMessage{Summary: "Test"}
		err := p.sendMessage(ctx, server.URL, msg)
		if err == nil {
			t.Error("sendMessage() expected error for 400 status")
		}
	})

	t.Run("message size exceeds 28KB limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Create a large message that exceeds 28KB
		largeText := strings.Repeat("a", 30000)
		msg := TeamsMessage{
			Type:    "MessageCard",
			Context: "https://schema.org/extensions",
			Summary: largeText,
		}

		err := p.sendMessage(ctx, server.URL, msg)
		if err == nil {
			t.Error("sendMessage() expected error for oversized message")
		}
		if err != nil && !strings.Contains(err.Error(), "28KB") {
			t.Errorf("error should mention 28KB limit, got: %v", err)
		}
	})
}
