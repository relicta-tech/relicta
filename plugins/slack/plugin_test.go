// Package main implements tests for the Slack plugin.
package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felixgeelhaar/release-pilot/pkg/plugin"
)

func TestURLValidator_SlackWebhook(t *testing.T) {
	// Create validator matching Slack webhook requirements
	validator := plugin.NewURLValidator("https").
		WithHosts("hooks.slack.com").
		WithPathPrefix("/services/")

	tests := []struct {
		name    string
		webhook string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid webhook URL",
			webhook: "https://hooks.slack.com/services/TTEST/BTEST/testtoken",
			wantErr: false,
		},
		{
			name:    "valid webhook URL with longer path",
			webhook: "https://hooks.slack.com/services/TTEST123/BTEST456/testtoken789",
			wantErr: false,
		},
		{
			name:    "empty URL",
			webhook: "",
			wantErr: true,
			errMsg:  "required",
		},
		{
			name:    "HTTP instead of HTTPS",
			webhook: "http://hooks.slack.com/services/TTEST/BTEST/testtoken",
			wantErr: true,
			errMsg:  "https",
		},
		{
			name:    "wrong host - subdomain attack",
			webhook: "https://hooks.slack.com.malicious.com/services/TTEST/BTEST/testtoken",
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "wrong host - different domain",
			webhook: "https://evil.com/services/TTEST/BTEST/testtoken",
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "wrong host - slack.com without hooks subdomain",
			webhook: "https://slack.com/services/TTEST/BTEST/testtoken",
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "missing /services/ path",
			webhook: "https://hooks.slack.com/api/TTEST/BTEST/testtoken",
			wantErr: true,
			errMsg:  "must start with",
		},
		{
			name:    "path traversal attempt",
			webhook: "https://hooks.slack.com/../../../etc/passwd",
			wantErr: true,
			errMsg:  "must start with",
		},
		{
			name:    "FTP scheme",
			webhook: "ftp://hooks.slack.com/services/TTEST/BTEST/testtoken",
			wantErr: true,
			errMsg:  "https",
		},
		{
			name:    "URL with credentials (user info attack)",
			webhook: "https://user:pass@hooks.slack.com/services/TTEST/BTEST/testtoken",
			wantErr: false, // URL is technically valid, hooks.slack.com will reject the auth
		},
		{
			name:    "URL with port",
			webhook: "https://hooks.slack.com:443/services/TTEST/BTEST/testtoken",
			wantErr: true, // Host includes port, so doesn't match exactly
			errMsg:  "not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.webhook)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil {
					t.Errorf("expected error containing %q but got nil", tt.errMsg)
				} else if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestBuildMentionText_Slack(t *testing.T) {
	tests := []struct {
		name     string
		mentions []string
		want     string
	}{
		{
			name:     "empty mentions",
			mentions: []string{},
			want:     "",
		},
		{
			name:     "nil mentions",
			mentions: nil,
			want:     "",
		},
		{
			name:     "single user ID",
			mentions: []string{"U12345678"},
			want:     "<@U12345678>",
		},
		{
			name:     "multiple user IDs",
			mentions: []string{"U12345678", "U87654321"},
			want:     "<@U12345678> <@U87654321>",
		},
		{
			name:     "user with @ prefix already",
			mentions: []string{"@U12345678"},
			want:     "<@U12345678>",
		},
		{
			name:     "special mention format",
			mentions: []string{"<!here>"},
			want:     "<!here>",
		},
		{
			name:     "mixed formats",
			mentions: []string{"U12345678", "@U87654321", "<!channel>"},
			want:     "<@U12345678> <@U87654321> <!channel>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := plugin.BuildMentionText(tt.mentions, plugin.MentionFormatSlack)
			if got != tt.want {
				t.Errorf("BuildMentionText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSlackPlugin_Validate(t *testing.T) {
	p := &SlackPlugin{}
	ctx := context.Background()

	tests := []struct {
		name      string
		config    map[string]any
		wantValid bool
		wantField string
	}{
		{
			name: "valid config with webhook",
			config: map[string]any{
				"webhook": "https://hooks.slack.com/services/TTEST/BTEST/test",
			},
			wantValid: true,
		},
		{
			name:      "missing webhook",
			config:    map[string]any{},
			wantValid: false,
			wantField: "webhook",
		},
		{
			name: "invalid webhook URL format",
			config: map[string]any{
				"webhook": "https://evil.com/steal",
			},
			wantValid: false,
			wantField: "webhook",
		},
		{
			name: "HTTP webhook (insecure)",
			config: map[string]any{
				"webhook": "http://hooks.slack.com/services/TTEST/BTEST/test",
			},
			wantValid: false,
			wantField: "webhook",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := p.Validate(ctx, tt.config)
			if err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}
			if resp.Valid != tt.wantValid {
				t.Errorf("Validate() valid = %v, want %v", resp.Valid, tt.wantValid)
			}
			if !tt.wantValid && tt.wantField != "" {
				found := false
				for _, e := range resp.Errors {
					if e.Field == tt.wantField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected validation error for field %q", tt.wantField)
				}
			}
		})
	}
}

func TestSlackPlugin_GetInfo(t *testing.T) {
	p := &SlackPlugin{}
	info := p.GetInfo()

	if info.Name != "slack" {
		t.Errorf("GetInfo().Name = %q, want %q", info.Name, "slack")
	}
	if info.Version == "" {
		t.Error("GetInfo().Version should not be empty")
	}
	if len(info.Hooks) == 0 {
		t.Error("GetInfo().Hooks should not be empty")
	}
}

func TestSlackPlugin_ParseConfig(t *testing.T) {
	p := &SlackPlugin{}

	t.Run("defaults are applied", func(t *testing.T) {
		cfg := p.parseConfig(map[string]any{})
		if cfg.Username != "ReleasePilot" {
			t.Errorf("Username = %q, want %q", cfg.Username, "ReleasePilot")
		}
		if cfg.IconEmoji != ":rocket:" {
			t.Errorf("IconEmoji = %q, want %q", cfg.IconEmoji, ":rocket:")
		}
		if !cfg.NotifyOnSuccess {
			t.Error("NotifyOnSuccess should be true by default")
		}
		if !cfg.NotifyOnError {
			t.Error("NotifyOnError should be true by default")
		}
	})

	t.Run("values are parsed correctly", func(t *testing.T) {
		cfg := p.parseConfig(map[string]any{
			"webhook":           "https://hooks.slack.com/services/xxx",
			"channel":           "#releases",
			"username":          "CustomBot",
			"notify_on_success": false,
			"mentions":          []any{"U123", "U456"},
		})
		if cfg.WebhookURL != "https://hooks.slack.com/services/xxx" {
			t.Errorf("WebhookURL = %q", cfg.WebhookURL)
		}
		if cfg.Channel != "#releases" {
			t.Errorf("Channel = %q", cfg.Channel)
		}
		if cfg.Username != "CustomBot" {
			t.Errorf("Username = %q", cfg.Username)
		}
		if cfg.NotifyOnSuccess {
			t.Error("NotifyOnSuccess should be false")
		}
		if len(cfg.Mentions) != 2 {
			t.Errorf("Mentions len = %d, want 2", len(cfg.Mentions))
		}
	})
}

func TestSlackPlugin_Execute(t *testing.T) {
	p := &SlackPlugin{}
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
				"webhook":           "https://hooks.slack.com/services/test",
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
			wantMessage: "Would send Slack success notification",
		},
		{
			name: "success notification disabled",
			hook: plugin.HookOnSuccess,
			config: map[string]any{
				"webhook":           "https://hooks.slack.com/services/test",
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
				"webhook":         "https://hooks.slack.com/services/test",
				"notify_on_error": true,
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "1.0.0",
				Branch:  "main",
			},
			dryRun:      true,
			wantSuccess: true,
			wantMessage: "Would send Slack error notification",
		},
		{
			name: "error notification disabled",
			hook: plugin.HookOnError,
			config: map[string]any{
				"webhook":         "https://hooks.slack.com/services/test",
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
				"webhook": "https://hooks.slack.com/services/test",
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
				"webhook":           "https://hooks.slack.com/services/test",
				"notify_on_success": true,
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "1.0.0",
			},
			dryRun:      true,
			wantSuccess: true,
			wantMessage: "Would send Slack success notification",
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

func TestSlackPlugin_SendSuccessNotification(t *testing.T) {
	// Mock HTTP server
	var receivedPayload SlackMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := &SlackPlugin{}
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
				Channel:          "#releases",
				Username:         "TestBot",
				IconEmoji:        ":test:",
				NotifyOnSuccess:  true,
				IncludeChangelog: true,
				Mentions:         []string{"@user1", "@user2"},
			},
			releaseCtx: plugin.ReleaseContext{
				Version:      "1.0.0",
				ReleaseType:  "major",
				Branch:       "main",
				TagName:      "v1.0.0",
				ReleaseNotes: "Test release notes",
				Changes: &plugin.CategorizedChanges{
					Features: []plugin.ConventionalCommit{{Description: "feat: new feature"}},
					Fixes:    []plugin.ConventionalCommit{{Description: "fix: bug fix"}},
					Breaking: []plugin.ConventionalCommit{{Description: "BREAKING: breaking change"}},
				},
			},
			dryRun:      false,
			wantSuccess: true,
			wantMessage: "Sent Slack success notification",
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
			wantMessage: "Would send Slack success notification",
		},
		{
			name: "successful notification without changelog",
			config: &Config{
				WebhookURL:       server.URL,
				NotifyOnSuccess:  true,
				IncludeChangelog: false,
			},
			releaseCtx: plugin.ReleaseContext{
				Version:      "2.0.0",
				ReleaseType:  "minor",
				Branch:       "develop",
				TagName:      "v2.0.0",
				ReleaseNotes: "Should not be included",
			},
			dryRun:      false,
			wantSuccess: true,
			wantMessage: "Sent Slack success notification",
		},
		{
			name: "truncate long changelog",
			config: &Config{
				WebhookURL:       server.URL,
				NotifyOnSuccess:  true,
				IncludeChangelog: true,
			},
			releaseCtx: plugin.ReleaseContext{
				Version:      "1.0.0",
				ReleaseType:  "patch",
				Branch:       "main",
				TagName:      "v1.0.0",
				ReleaseNotes: string(make([]byte, 2500)), // Very long notes
			},
			dryRun:      false,
			wantSuccess: true,
			wantMessage: "Sent Slack success notification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receivedPayload = SlackMessage{} // Reset

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

			// Verify message structure (only for non-dry-run)
			if !tt.dryRun {
				if receivedPayload.Channel != tt.config.Channel {
					t.Errorf("received channel = %q, want %q", receivedPayload.Channel, tt.config.Channel)
				}
				if len(receivedPayload.Attachments) == 0 {
					t.Error("expected at least one attachment")
				}
				if receivedPayload.Attachments[0].Color != "good" {
					t.Errorf("attachment color = %q, want %q", receivedPayload.Attachments[0].Color, "good")
				}
			}
		})
	}
}

func TestSlackPlugin_SendErrorNotification(t *testing.T) {
	// Mock HTTP server
	var receivedPayload SlackMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := &SlackPlugin{}
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
			name: "error notification with mentions",
			config: &Config{
				WebhookURL:    server.URL,
				Channel:       "#alerts",
				Username:      "AlertBot",
				IconEmoji:     ":warning:",
				NotifyOnError: true,
				Mentions:      []string{"@oncall"},
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "1.0.0",
				Branch:  "main",
			},
			dryRun:      false,
			wantSuccess: true,
			wantMessage: "Sent Slack error notification",
		},
		{
			name: "dry run mode",
			config: &Config{
				WebhookURL:    server.URL,
				NotifyOnError: true,
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "1.0.0",
				Branch:  "main",
			},
			dryRun:      true,
			wantSuccess: true,
			wantMessage: "Would send Slack error notification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receivedPayload = SlackMessage{} // Reset

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

			// Verify message structure (only for non-dry-run)
			if !tt.dryRun {
				if receivedPayload.Channel != tt.config.Channel {
					t.Errorf("received channel = %q, want %q", receivedPayload.Channel, tt.config.Channel)
				}
				if len(receivedPayload.Attachments) == 0 {
					t.Error("expected at least one attachment")
				}
				if receivedPayload.Attachments[0].Color != "danger" {
					t.Errorf("attachment color = %q, want %q", receivedPayload.Attachments[0].Color, "danger")
				}
			}
		})
	}
}

func TestSlackPlugin_SendMessage(t *testing.T) {
	p := &SlackPlugin{}
	ctx := context.Background()

	tests := []struct {
		name        string
		setupMock   func() *httptest.Server
		msg         SlackMessage
		wantErr     bool
		errContains string
	}{
		{
			name: "successful message send",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != "POST" {
						t.Errorf("expected POST, got %s", r.Method)
					}
					if r.Header.Get("Content-Type") != "application/json" {
						t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
					}
					w.WriteHeader(http.StatusOK)
				}))
			},
			msg: SlackMessage{
				Text: "test message",
			},
			wantErr: false,
		},
		{
			name: "Slack returns error status",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
				}))
			},
			msg: SlackMessage{
				Text: "test message",
			},
			wantErr:     true,
			errContains: "slack returned status 400",
		},
		{
			name: "network error",
			setupMock: func() *httptest.Server {
				// Return a closed server to simulate network error
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
				server.Close()
				return server
			},
			msg: SlackMessage{
				Text: "test message",
			},
			wantErr:     true,
			errContains: "failed to send request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupMock()
			defer server.Close()

			err := p.sendMessage(ctx, server.URL, tt.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("sendMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !containsString(err.Error(), tt.errContains) {
					t.Errorf("sendMessage() error = %v, should contain %q", err, tt.errContains)
				}
			}
		})
	}
}

func TestGetStringOrDefault(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		defaultVal string
		want       string
	}{
		{
			name:       "non-empty value",
			value:      "custom",
			defaultVal: "default",
			want:       "custom",
		},
		{
			name:       "empty value returns default",
			value:      "",
			defaultVal: "default",
			want:       "default",
		},
		{
			name:       "empty default",
			value:      "",
			defaultVal: "",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getStringOrDefault(tt.value, tt.defaultVal)
			if got != tt.want {
				t.Errorf("getStringOrDefault() = %q, want %q", got, tt.want)
			}
		})
	}
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
