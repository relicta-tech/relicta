// Package main implements tests for the Discord plugin.
package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/felixgeelhaar/release-pilot/pkg/plugin"
)

func TestURLValidator_DiscordWebhook(t *testing.T) {
	// Create validator matching Discord webhook requirements
	validator := plugin.NewURLValidator("https").
		WithHosts("discord.com", "discordapp.com").
		WithPathPrefix("/api/webhooks/")

	tests := []struct {
		name    string
		webhook string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid discord.com webhook URL",
			webhook: "https://discord.com/api/webhooks/1234567890/abcdefghijklmnopqrstuvwxyz",
			wantErr: false,
		},
		{
			name:    "valid discordapp.com webhook URL (legacy)",
			webhook: "https://discordapp.com/api/webhooks/1234567890/abcdefghijklmnopqrstuvwxyz",
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
			webhook: "http://discord.com/api/webhooks/1234567890/abcdefghijklmnopqrstuvwxyz",
			wantErr: true,
			errMsg:  "https",
		},
		{
			name:    "wrong host - subdomain attack",
			webhook: "https://discord.com.malicious.com/api/webhooks/1234567890/abcdefghijklmnopqrstuvwxyz",
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "wrong host - different domain",
			webhook: "https://evil.com/api/webhooks/1234567890/abcdefghijklmnopqrstuvwxyz",
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "wrong host - canary subdomain (not supported)",
			webhook: "https://canary.discord.com/api/webhooks/1234567890/abcdefghijklmnopqrstuvwxyz",
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "missing /api/webhooks/ path",
			webhook: "https://discord.com/api/channels/1234567890",
			wantErr: true,
			errMsg:  "must start with",
		},
		{
			name:    "path traversal attempt",
			webhook: "https://discord.com/../../../etc/passwd",
			wantErr: true,
			errMsg:  "must start with",
		},
		{
			name:    "FTP scheme",
			webhook: "ftp://discord.com/api/webhooks/1234567890/abcdefghijklmnopqrstuvwxyz",
			wantErr: true,
			errMsg:  "https",
		},
		{
			name:    "URL with port",
			webhook: "https://discord.com:443/api/webhooks/1234567890/abcdefghijklmnopqrstuvwxyz",
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

func TestBuildMentionText_Discord(t *testing.T) {
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
			mentions: []string{"123456789012345678"},
			want:     "<@123456789012345678>",
		},
		{
			name:     "multiple user IDs",
			mentions: []string{"123456789012345678", "876543210987654321"},
			want:     "<@123456789012345678> <@876543210987654321>",
		},
		{
			name:     "user mention already formatted",
			mentions: []string{"<@123456789012345678>"},
			want:     "<@123456789012345678>",
		},
		{
			name:     "role mention already formatted",
			mentions: []string{"<@&123456789012345678>"},
			want:     "<@&123456789012345678>",
		},
		{
			name:     "channel mention already formatted",
			mentions: []string{"<#123456789012345678>"},
			want:     "<#123456789012345678>",
		},
		{
			name:     "@everyone mention",
			mentions: []string{"@everyone"},
			want:     "@everyone",
		},
		{
			name:     "@here mention",
			mentions: []string{"@here"},
			want:     "@here",
		},
		{
			name:     "mixed formats",
			mentions: []string{"123456789012345678", "<@876543210987654321>", "<@&111222333444555666>", "@here"},
			want:     "<@123456789012345678> <@876543210987654321> <@&111222333444555666> @here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := plugin.BuildMentionText(tt.mentions, plugin.MentionFormatDiscord)
			if got != tt.want {
				t.Errorf("BuildMentionText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDiscordPlugin_Validate(t *testing.T) {
	p := &DiscordPlugin{}
	ctx := context.Background()

	tests := []struct {
		name      string
		config    map[string]any
		wantValid bool
		wantField string
	}{
		{
			name: "valid config with discord.com webhook",
			config: map[string]any{
				"webhook": "https://discord.com/api/webhooks/1234567890/abcdefghijklmnopqrstuvwxyz",
			},
			wantValid: true,
		},
		{
			name: "valid config with discordapp.com webhook",
			config: map[string]any{
				"webhook": "https://discordapp.com/api/webhooks/1234567890/abcdefghijklmnopqrstuvwxyz",
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
				"webhook": "http://discord.com/api/webhooks/1234567890/abcdefghijklmnopqrstuvwxyz",
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

func TestDiscordPlugin_GetInfo(t *testing.T) {
	p := &DiscordPlugin{}
	info := p.GetInfo()

	if info.Name != "discord" {
		t.Errorf("GetInfo().Name = %q, want %q", info.Name, "discord")
	}
	if info.Version == "" {
		t.Error("GetInfo().Version should not be empty")
	}
	if len(info.Hooks) == 0 {
		t.Error("GetInfo().Hooks should not be empty")
	}
	if info.Description == "" {
		t.Error("GetInfo().Description should not be empty")
	}

	// Verify expected hooks
	expectedHooks := map[plugin.Hook]bool{
		plugin.HookPostPublish: true,
		plugin.HookOnSuccess:   true,
		plugin.HookOnError:     true,
	}
	for _, h := range info.Hooks {
		if !expectedHooks[h] {
			t.Errorf("unexpected hook %q in GetInfo().Hooks", h)
		}
	}
}

func TestDiscordPlugin_ParseConfig(t *testing.T) {
	p := &DiscordPlugin{}

	tests := []struct {
		name              string
		config            map[string]any
		wantUsername      string
		wantNotifySuccess bool
		wantNotifyError   bool
		wantThreadID      string
		wantColor         int
	}{
		{
			name:              "defaults",
			config:            map[string]any{},
			wantUsername:      "ReleasePilot",
			wantNotifySuccess: true,
			wantNotifyError:   true,
			wantThreadID:      "",
			wantColor:         0,
		},
		{
			name: "custom username",
			config: map[string]any{
				"username": "MyBot",
			},
			wantUsername:      "MyBot",
			wantNotifySuccess: true,
			wantNotifyError:   true,
		},
		{
			name: "disable success notification",
			config: map[string]any{
				"notify_on_success": false,
			},
			wantUsername:      "ReleasePilot",
			wantNotifySuccess: false,
			wantNotifyError:   true,
		},
		{
			name: "disable error notification",
			config: map[string]any{
				"notify_on_error": false,
			},
			wantUsername:      "ReleasePilot",
			wantNotifySuccess: true,
			wantNotifyError:   false,
		},
		{
			name: "with thread ID",
			config: map[string]any{
				"thread_id": "123456789012345678",
			},
			wantUsername:      "ReleasePilot",
			wantNotifySuccess: true,
			wantNotifyError:   true,
			wantThreadID:      "123456789012345678",
		},
		{
			name: "with custom color",
			config: map[string]any{
				"color": float64(16711680), // Red in decimal
			},
			wantUsername:      "ReleasePilot",
			wantNotifySuccess: true,
			wantNotifyError:   true,
			wantColor:         16711680,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := p.parseConfig(tt.config)
			if cfg.Username != tt.wantUsername {
				t.Errorf("parseConfig().Username = %q, want %q", cfg.Username, tt.wantUsername)
			}
			if cfg.NotifyOnSuccess != tt.wantNotifySuccess {
				t.Errorf("parseConfig().NotifyOnSuccess = %v, want %v", cfg.NotifyOnSuccess, tt.wantNotifySuccess)
			}
			if cfg.NotifyOnError != tt.wantNotifyError {
				t.Errorf("parseConfig().NotifyOnError = %v, want %v", cfg.NotifyOnError, tt.wantNotifyError)
			}
			if cfg.ThreadID != tt.wantThreadID {
				t.Errorf("parseConfig().ThreadID = %q, want %q", cfg.ThreadID, tt.wantThreadID)
			}
			if cfg.Color != tt.wantColor {
				t.Errorf("parseConfig().Color = %d, want %d", cfg.Color, tt.wantColor)
			}
		})
	}
}

func TestDiscordPlugin_Execute_DisabledNotifications(t *testing.T) {
	p := &DiscordPlugin{}
	ctx := context.Background()

	tests := []struct {
		name    string
		hook    plugin.Hook
		config  map[string]any
		wantMsg string
	}{
		{
			name: "success notification disabled on post-publish",
			hook: plugin.HookPostPublish,
			config: map[string]any{
				"webhook":           "https://discord.com/api/webhooks/1234567890/abc",
				"notify_on_success": false,
			},
			wantMsg: "Success notification disabled",
		},
		{
			name: "success notification disabled on on-success",
			hook: plugin.HookOnSuccess,
			config: map[string]any{
				"webhook":           "https://discord.com/api/webhooks/1234567890/abc",
				"notify_on_success": false,
			},
			wantMsg: "Success notification disabled",
		},
		{
			name: "error notification disabled",
			hook: plugin.HookOnError,
			config: map[string]any{
				"webhook":         "https://discord.com/api/webhooks/1234567890/abc",
				"notify_on_error": false,
			},
			wantMsg: "Error notification disabled",
		},
		{
			name: "unhandled hook",
			hook: plugin.HookPrePlan,
			config: map[string]any{
				"webhook": "https://discord.com/api/webhooks/1234567890/abc",
			},
			wantMsg: "Hook pre-plan not handled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := plugin.ExecuteRequest{
				Hook:   tt.hook,
				Config: tt.config,
				Context: plugin.ReleaseContext{
					Version:     "1.0.0",
					TagName:     "v1.0.0",
					Branch:      "main",
					ReleaseType: "minor",
				},
				DryRun: false,
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				t.Fatalf("Execute() unexpected error: %v", err)
			}
			if !resp.Success {
				t.Errorf("Execute() success = %v, want true", resp.Success)
			}
			if resp.Message != tt.wantMsg {
				t.Errorf("Execute() message = %q, want %q", resp.Message, tt.wantMsg)
			}
		})
	}
}

func TestDiscordPlugin_Execute_DryRun(t *testing.T) {
	p := &DiscordPlugin{}
	ctx := context.Background()

	tests := []struct {
		name    string
		hook    plugin.Hook
		wantMsg string
	}{
		{
			name:    "dry run post-publish",
			hook:    plugin.HookPostPublish,
			wantMsg: "Would send Discord success notification",
		},
		{
			name:    "dry run on-success",
			hook:    plugin.HookOnSuccess,
			wantMsg: "Would send Discord success notification",
		},
		{
			name:    "dry run on-error",
			hook:    plugin.HookOnError,
			wantMsg: "Would send Discord error notification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := plugin.ExecuteRequest{
				Hook: tt.hook,
				Config: map[string]any{
					"webhook": "https://discord.com/api/webhooks/1234567890/abc",
				},
				Context: plugin.ReleaseContext{
					Version:     "1.0.0",
					TagName:     "v1.0.0",
					Branch:      "main",
					ReleaseType: "minor",
				},
				DryRun: true,
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				t.Fatalf("Execute() unexpected error: %v", err)
			}
			if !resp.Success {
				t.Errorf("Execute() success = %v, want true", resp.Success)
			}
			if resp.Message != tt.wantMsg {
				t.Errorf("Execute() message = %q, want %q", resp.Message, tt.wantMsg)
			}
		})
	}
}

func TestDiscordColors(t *testing.T) {
	// Verify color constants are sensible values
	if ColorSuccess <= 0 {
		t.Error("ColorSuccess should be a positive value")
	}
	if ColorError <= 0 {
		t.Error("ColorError should be a positive value")
	}
	if ColorWarning <= 0 {
		t.Error("ColorWarning should be a positive value")
	}
	if ColorBlurple <= 0 {
		t.Error("ColorBlurple should be a positive value")
	}

	// Verify they're different from each other
	if ColorSuccess == ColorError {
		t.Error("ColorSuccess and ColorError should be different")
	}
}

func TestDiscordPlugin_SendSuccessNotification(t *testing.T) {
	// Mock HTTP server
	var receivedPayload DiscordMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	p := &DiscordPlugin{}
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
				Username:         "TestBot",
				NotifyOnSuccess:  true,
				IncludeChangelog: true,
				Mentions:         []string{"@user1", "@user2"},
			},
			releaseCtx: plugin.ReleaseContext{
				Version:       "1.0.0",
				ReleaseType:   "major",
				Branch:        "main",
				TagName:       "v1.0.0",
				RepositoryURL: "https://github.com/test/repo",
				ReleaseNotes:  "Test release notes",
				Changes: &plugin.CategorizedChanges{
					Features: []plugin.ConventionalCommit{{Description: "feat: new feature"}},
					Fixes:    []plugin.ConventionalCommit{{Description: "fix: bug fix"}},
					Breaking: []plugin.ConventionalCommit{{Description: "BREAKING: breaking change"}},
				},
			},
			dryRun:      false,
			wantSuccess: true,
			wantMessage: "Sent Discord success notification",
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
			wantMessage: "Would send Discord success notification",
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
			wantMessage: "Sent Discord success notification",
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
				ReleaseNotes: string(make([]byte, 2500)),
			},
			dryRun:      false,
			wantSuccess: true,
			wantMessage: "Sent Discord success notification",
		},
		{
			name: "with custom color",
			config: &Config{
				WebhookURL:      server.URL,
				NotifyOnSuccess: true,
				Color:           16711680, // Red
			},
			releaseCtx: plugin.ReleaseContext{
				Version:     "1.0.0",
				ReleaseType: "major",
				Branch:      "main",
				TagName:     "v1.0.0",
			},
			dryRun:      false,
			wantSuccess: true,
			wantMessage: "Sent Discord success notification",
		},
		{
			name: "with thread ID",
			config: &Config{
				WebhookURL:      server.URL,
				NotifyOnSuccess: true,
				ThreadID:        "123456789012345678",
			},
			releaseCtx: plugin.ReleaseContext{
				Version:     "1.0.0",
				ReleaseType: "major",
				Branch:      "main",
				TagName:     "v1.0.0",
			},
			dryRun:      false,
			wantSuccess: true,
			wantMessage: "Sent Discord success notification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receivedPayload = DiscordMessage{}

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
				if receivedPayload.Username != tt.config.Username {
					t.Errorf("received username = %q, want %q", receivedPayload.Username, tt.config.Username)
				}
				if len(receivedPayload.Embeds) == 0 {
					t.Error("expected at least one embed")
				}
				if tt.config.Color != 0 && receivedPayload.Embeds[0].Color != tt.config.Color {
					t.Errorf("embed color = %d, want %d", receivedPayload.Embeds[0].Color, tt.config.Color)
				}
			}
		})
	}
}

func TestDiscordPlugin_SendErrorNotification(t *testing.T) {
	// Mock HTTP server
	var receivedPayload DiscordMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	p := &DiscordPlugin{}
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
				Username:      "AlertBot",
				NotifyOnError: true,
				Mentions:      []string{"@oncall"},
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "1.0.0",
				Branch:  "main",
			},
			dryRun:      false,
			wantSuccess: true,
			wantMessage: "Sent Discord error notification",
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
			wantMessage: "Would send Discord error notification",
		},
		{
			name: "with custom color",
			config: &Config{
				WebhookURL:    server.URL,
				NotifyOnError: true,
				Color:         16711680,
			},
			releaseCtx: plugin.ReleaseContext{
				Version: "1.0.0",
				Branch:  "main",
			},
			dryRun:      false,
			wantSuccess: true,
			wantMessage: "Sent Discord error notification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receivedPayload = DiscordMessage{}

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
				if receivedPayload.Username != tt.config.Username {
					t.Errorf("received username = %q, want %q", receivedPayload.Username, tt.config.Username)
				}
				if len(receivedPayload.Embeds) == 0 {
					t.Error("expected at least one embed")
				}
				if tt.config.Color != 0 && receivedPayload.Embeds[0].Color != tt.config.Color {
					t.Errorf("embed color = %d, want %d", receivedPayload.Embeds[0].Color, tt.config.Color)
				}
			}
		})
	}
}

func TestDiscordPlugin_SendMessage(t *testing.T) {
	p := &DiscordPlugin{}
	ctx := context.Background()

	tests := []struct {
		name        string
		setupMock   func() *httptest.Server
		msg         DiscordMessage
		wantErr     bool
		errContains string
	}{
		{
			name: "successful message send - 200 OK",
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
			msg: DiscordMessage{
				Content: "test message",
			},
			wantErr: false,
		},
		{
			name: "successful message send - 204 No Content",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}))
			},
			msg: DiscordMessage{
				Content: "test message",
			},
			wantErr: false,
		},
		{
			name: "with thread ID in message",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Verify thread_id is in query params
					threadID := r.URL.Query().Get("thread_id")
					if threadID != "123456789012345678" {
						t.Errorf("expected thread_id=123456789012345678, got %s", threadID)
					}
					w.WriteHeader(http.StatusNoContent)
				}))
			},
			msg: DiscordMessage{
				Content:  "test message",
				ThreadID: "123456789012345678",
			},
			wantErr: false,
		},
		{
			name: "Discord returns error status",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
				}))
			},
			msg: DiscordMessage{
				Content: "test message",
			},
			wantErr:     true,
			errContains: "discord returned status 400",
		},
		{
			name: "network error",
			setupMock: func() *httptest.Server {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
				server.Close()
				return server
			},
			msg: DiscordMessage{
				Content: "test message",
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
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("sendMessage() error = %v, should contain %q", err, tt.errContains)
				}
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
