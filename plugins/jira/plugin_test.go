// Package main implements tests for the Jira plugin.
package main

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/felixgeelhaar/release-pilot/pkg/plugin"
)

func TestGetInfo(t *testing.T) {
	p := &JiraPlugin{}
	info := p.GetInfo()

	if info.Name != "jira" {
		t.Errorf("Name = %v, want jira", info.Name)
	}
	if info.Version != "1.0.0" {
		t.Errorf("Version = %v, want 1.0.0", info.Version)
	}
	if info.Description == "" {
		t.Error("Description should not be empty")
	}
	if len(info.Hooks) != 4 {
		t.Errorf("Expected 4 hooks, got %d", len(info.Hooks))
	}

	// Check hooks
	expectedHooks := map[plugin.Hook]bool{
		plugin.HookPostPlan:    true,
		plugin.HookPostPublish: true,
		plugin.HookOnSuccess:   true,
		plugin.HookOnError:     true,
	}
	for _, hook := range info.Hooks {
		if !expectedHooks[hook] {
			t.Errorf("Unexpected hook: %v", hook)
		}
	}
}

func TestParseConfig(t *testing.T) {
	p := &JiraPlugin{}

	tests := []struct {
		name     string
		raw      map[string]any
		expected *Config
	}{
		{
			name: "empty config - defaults",
			raw:  map[string]any{},
			expected: &Config{
				CreateVersion:   true,
				ReleaseVersion:  true,
				AssociateIssues: true,
			},
		},
		{
			name: "full config",
			raw: map[string]any{
				"base_url":            "https://company.atlassian.net",
				"username":            "user@example.com",
				"token":               "test-token",
				"project_key":         "PROJ",
				"version_name":        "v1.0.0",
				"version_description": "Release notes here",
				"create_version":      true,
				"release_version":     true,
				"transition_issues":   true,
				"transition_name":     "Done",
				"add_comment":         true,
				"comment_template":    "Released in {version}",
				"issue_pattern":       "PROJ-\\d+",
				"associate_issues":    false,
			},
			expected: &Config{
				BaseURL:            "https://company.atlassian.net",
				Username:           "user@example.com",
				Token:              "test-token",
				ProjectKey:         "PROJ",
				VersionName:        "v1.0.0",
				VersionDescription: "Release notes here",
				CreateVersion:      true,
				ReleaseVersion:     true,
				TransitionIssues:   true,
				TransitionName:     "Done",
				AddComment:         true,
				CommentTemplate:    "Released in {version}",
				IssuePattern:       "PROJ-\\d+",
				AssociateIssues:    false,
			},
		},
		{
			name: "override defaults",
			raw: map[string]any{
				"create_version":   false,
				"release_version":  false,
				"associate_issues": false,
			},
			expected: &Config{
				CreateVersion:   false,
				ReleaseVersion:  false,
				AssociateIssues: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := p.parseConfig(tt.raw)

			if cfg.BaseURL != tt.expected.BaseURL {
				t.Errorf("BaseURL = %v, want %v", cfg.BaseURL, tt.expected.BaseURL)
			}
			if cfg.Username != tt.expected.Username {
				t.Errorf("Username = %v, want %v", cfg.Username, tt.expected.Username)
			}
			if cfg.Token != tt.expected.Token {
				t.Errorf("Token = %v, want %v", cfg.Token, tt.expected.Token)
			}
			if cfg.ProjectKey != tt.expected.ProjectKey {
				t.Errorf("ProjectKey = %v, want %v", cfg.ProjectKey, tt.expected.ProjectKey)
			}
			if cfg.CreateVersion != tt.expected.CreateVersion {
				t.Errorf("CreateVersion = %v, want %v", cfg.CreateVersion, tt.expected.CreateVersion)
			}
			if cfg.ReleaseVersion != tt.expected.ReleaseVersion {
				t.Errorf("ReleaseVersion = %v, want %v", cfg.ReleaseVersion, tt.expected.ReleaseVersion)
			}
			if cfg.TransitionIssues != tt.expected.TransitionIssues {
				t.Errorf("TransitionIssues = %v, want %v", cfg.TransitionIssues, tt.expected.TransitionIssues)
			}
			if cfg.TransitionName != tt.expected.TransitionName {
				t.Errorf("TransitionName = %v, want %v", cfg.TransitionName, tt.expected.TransitionName)
			}
			if cfg.AssociateIssues != tt.expected.AssociateIssues {
				t.Errorf("AssociateIssues = %v, want %v", cfg.AssociateIssues, tt.expected.AssociateIssues)
			}
		})
	}
}

func TestExtractIssueKeys(t *testing.T) {
	p := &JiraPlugin{}

	tests := []struct {
		name     string
		cfg      *Config
		changes  *plugin.CategorizedChanges
		expected []string
	}{
		{
			name: "default pattern - multiple projects",
			cfg:  &Config{},
			changes: &plugin.CategorizedChanges{
				Features: []plugin.ConventionalCommit{
					{Description: "implement login PROJ-123"},
				},
				Fixes: []plugin.ConventionalCommit{
					{Description: "resolve issue ABC-456"},
				},
				Other: []plugin.ConventionalCommit{
					{Description: "update deps TEST-789"},
					{Description: "update readme (no issue)"},
				},
			},
			expected: []string{"PROJ-123", "ABC-456", "TEST-789"},
		},
		{
			name: "custom pattern - single project",
			cfg:  &Config{IssuePattern: `MYPROJ-\d+`},
			changes: &plugin.CategorizedChanges{
				Features: []plugin.ConventionalCommit{
					{Description: "implement feature MYPROJ-100"},
				},
				Fixes: []plugin.ConventionalCommit{
					{Description: "bug fix MYPROJ-101 and MYPROJ-102"},
				},
				Other: []plugin.ConventionalCommit{
					{Description: "OTHER-123 should not match"},
				},
			},
			expected: []string{"MYPROJ-100", "MYPROJ-101", "MYPROJ-102"},
		},
		{
			name: "deduplicate issues",
			cfg:  &Config{},
			changes: &plugin.CategorizedChanges{
				Features: []plugin.ConventionalCommit{
					{Description: "start PROJ-123"},
				},
				Fixes: []plugin.ConventionalCommit{
					{Description: "update PROJ-123"},
				},
				Other: []plugin.ConventionalCommit{
					{Description: "test PROJ-123"},
				},
			},
			expected: []string{"PROJ-123"},
		},
		{
			name: "no matches",
			cfg:  &Config{},
			changes: &plugin.CategorizedChanges{
				Features: []plugin.ConventionalCommit{
					{Description: "no issue reference"},
				},
				Fixes: []plugin.ConventionalCommit{
					{Description: "another commit without issue"},
				},
			},
			expected: nil,
		},
		{
			name: "case insensitive match via custom pattern",
			cfg:  &Config{IssuePattern: `(?i)[A-Z][A-Z0-9]*-\d+`},
			changes: &plugin.CategorizedChanges{
				Features: []plugin.ConventionalCommit{
					{Description: "proj-123 lowercase"},
				},
			},
			expected: []string{"PROJ-123"},
		},
		{
			name: "multiple issues in one commit",
			cfg:  &Config{},
			changes: &plugin.CategorizedChanges{
				Features: []plugin.ConventionalCommit{
					{Description: "implement PROJ-1, PROJ-2, and PROJ-3"},
				},
			},
			expected: []string{"PROJ-1", "PROJ-2", "PROJ-3"},
		},
		{
			name:     "nil changes",
			cfg:      &Config{},
			changes:  nil,
			expected: nil,
		},
		{
			name: "extract from body",
			cfg:  &Config{},
			changes: &plugin.CategorizedChanges{
				Features: []plugin.ConventionalCommit{
					{Description: "implement feature", Body: "This fixes PROJ-999"},
				},
			},
			expected: []string{"PROJ-999"},
		},
		{
			name: "extract from issues field",
			cfg:  &Config{},
			changes: &plugin.CategorizedChanges{
				Fixes: []plugin.ConventionalCommit{
					{Description: "fix bug", Issues: []string{"PROJ-555"}},
				},
			},
			expected: []string{"PROJ-555"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.extractIssueKeys(tt.cfg, tt.changes)

			if len(result) != len(tt.expected) {
				t.Errorf("extractIssueKeys() returned %d keys, want %d", len(result), len(tt.expected))
				t.Errorf("got: %v, want: %v", result, tt.expected)
				return
			}

			for i, key := range result {
				if key != tt.expected[i] {
					t.Errorf("extractIssueKeys()[%d] = %v, want %v", i, key, tt.expected[i])
				}
			}
		})
	}
}

func TestBuildComment(t *testing.T) {
	p := &JiraPlugin{}

	tests := []struct {
		name       string
		template   string
		releaseCtx plugin.ReleaseContext
		expected   string
	}{
		{
			name:     "simple version placeholder",
			template: "Released in {version}",
			releaseCtx: plugin.ReleaseContext{
				Version: "1.0.0",
			},
			expected: "Released in 1.0.0",
		},
		{
			name:     "multiple placeholders",
			template: "This issue was released in {version} ({tag}). See {release_url} for details.",
			releaseCtx: plugin.ReleaseContext{
				Version:       "1.0.0",
				TagName:       "v1.0.0",
				RepositoryURL: "https://github.com/org/repo/releases/tag/v1.0.0",
			},
			expected: "This issue was released in 1.0.0 (v1.0.0). See https://github.com/org/repo/releases/tag/v1.0.0 for details.",
		},
		{
			name:     "repository placeholder",
			template: "Released in {repository} {version}",
			releaseCtx: plugin.ReleaseContext{
				Version:        "2.0.0",
				RepositoryName: "my-app",
			},
			expected: "Released in my-app 2.0.0",
		},
		{
			name:     "no placeholders",
			template: "This issue has been resolved.",
			releaseCtx: plugin.ReleaseContext{
				Version: "1.0.0",
			},
			expected: "This issue has been resolved.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.buildComment(tt.template, tt.releaseCtx)
			if result != tt.expected {
				t.Errorf("buildComment() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	p := &JiraPlugin{}
	ctx := context.Background()

	// Clear environment variables for testing
	os.Unsetenv("JIRA_TOKEN")
	os.Unsetenv("JIRA_API_TOKEN")
	os.Unsetenv("JIRA_USERNAME")

	tests := []struct {
		name        string
		config      map[string]any
		envVars     map[string]string
		expectValid bool
	}{
		{
			name:        "missing all required fields",
			config:      map[string]any{},
			expectValid: false,
		},
		{
			name: "missing token and username",
			config: map[string]any{
				"base_url":    "https://company.atlassian.net",
				"project_key": "PROJ",
			},
			expectValid: false,
		},
		{
			name: "all required fields in config",
			config: map[string]any{
				"base_url":    "https://company.atlassian.net",
				"project_key": "PROJ",
				"username":    "user@example.com",
				"token":       "test-token",
			},
			expectValid: true,
		},
		{
			name: "credentials from env vars",
			config: map[string]any{
				"base_url":    "https://company.atlassian.net",
				"project_key": "PROJ",
			},
			envVars: map[string]string{
				"JIRA_USERNAME": "user@example.com",
				"JIRA_TOKEN":    "env-token",
			},
			expectValid: true,
		},
		{
			name: "invalid base_url format",
			config: map[string]any{
				"base_url":    "company.atlassian.net",
				"project_key": "PROJ",
				"username":    "user@example.com",
				"token":       "test-token",
			},
			expectValid: false,
		},
		{
			name: "invalid issue_pattern regex",
			config: map[string]any{
				"base_url":      "https://company.atlassian.net",
				"project_key":   "PROJ",
				"username":      "user@example.com",
				"token":         "test-token",
				"issue_pattern": "[invalid(regex",
			},
			expectValid: false,
		},
		{
			name: "valid issue_pattern",
			config: map[string]any{
				"base_url":      "https://company.atlassian.net",
				"project_key":   "PROJ",
				"username":      "user@example.com",
				"token":         "test-token",
				"issue_pattern": "PROJ-\\d+",
			},
			expectValid: true,
		},
		{
			name: "transition_issues without transition_name",
			config: map[string]any{
				"base_url":          "https://company.atlassian.net",
				"project_key":       "PROJ",
				"username":          "user@example.com",
				"token":             "test-token",
				"transition_issues": true,
			},
			expectValid: false,
		},
		{
			name: "transition_issues with transition_name",
			config: map[string]any{
				"base_url":          "https://company.atlassian.net",
				"project_key":       "PROJ",
				"username":          "user@example.com",
				"token":             "test-token",
				"transition_issues": true,
				"transition_name":   "Done",
			},
			expectValid: true,
		},
		{
			name: "add_comment without comment_template",
			config: map[string]any{
				"base_url":    "https://company.atlassian.net",
				"project_key": "PROJ",
				"username":    "user@example.com",
				"token":       "test-token",
				"add_comment": true,
			},
			expectValid: false,
		},
		{
			name: "add_comment with comment_template",
			config: map[string]any{
				"base_url":         "https://company.atlassian.net",
				"project_key":      "PROJ",
				"username":         "user@example.com",
				"token":            "test-token",
				"add_comment":      true,
				"comment_template": "Released in {version}",
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars if specified
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			defer func() {
				for k := range tt.envVars {
					os.Unsetenv(k)
				}
			}()

			resp, err := p.Validate(ctx, tt.config)
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}

			if resp.Valid != tt.expectValid {
				t.Errorf("Valid = %v, want %v, errors: %v", resp.Valid, tt.expectValid, resp.Errors)
			}
		})
	}
}

func TestExecute_PostPlan(t *testing.T) {
	p := &JiraPlugin{}
	ctx := context.Background()

	tests := []struct {
		name           string
		changes        *plugin.CategorizedChanges
		expectIssues   int
		expectContains string
	}{
		{
			name: "finds issues",
			changes: &plugin.CategorizedChanges{
				Features: []plugin.ConventionalCommit{
					{Description: "implement PROJ-123"},
				},
				Fixes: []plugin.ConventionalCommit{
					{Description: "resolve PROJ-456"},
				},
			},
			expectIssues:   2,
			expectContains: "PROJ-123",
		},
		{
			name: "no issues found",
			changes: &plugin.CategorizedChanges{
				Features: []plugin.ConventionalCommit{
					{Description: "implement feature"},
				},
			},
			expectIssues:   0,
			expectContains: "No Jira issues found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := plugin.ExecuteRequest{
				Hook:   plugin.HookPostPlan,
				Config: map[string]any{},
				Context: plugin.ReleaseContext{
					Version: "1.0.0",
					Changes: tt.changes,
				},
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if !resp.Success {
				t.Error("Expected success")
			}

			issuesFound, _ := resp.Outputs["issues_found"].(int)
			if issuesFound != tt.expectIssues {
				t.Errorf("issues_found = %d, want %d", issuesFound, tt.expectIssues)
			}

			if !containsString(resp.Message, tt.expectContains) {
				t.Errorf("Message = %v, expected to contain %v", resp.Message, tt.expectContains)
			}
		})
	}
}

func TestExecute_PostPublish_DryRun(t *testing.T) {
	p := &JiraPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook:   plugin.HookPostPublish,
		DryRun: true,
		Config: map[string]any{
			"base_url":          "https://company.atlassian.net",
			"project_key":       "PROJ",
			"username":          "user@example.com",
			"token":             "test-token",
			"create_version":    true,
			"release_version":   true,
			"transition_issues": true,
			"transition_name":   "Done",
		},
		Context: plugin.ReleaseContext{
			Version: "1.0.0",
			Changes: &plugin.CategorizedChanges{
				Features: []plugin.ConventionalCommit{
					{Description: "implement PROJ-123"},
				},
			},
		},
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected success, got error: %s", resp.Error)
	}

	if !containsString(resp.Message, "Would perform") {
		t.Errorf("Message = %v, expected to contain 'Would perform'", resp.Message)
	}

	actions, ok := resp.Outputs["actions"].([]string)
	if !ok {
		t.Fatal("Expected actions in outputs")
	}

	if len(actions) == 0 {
		t.Error("Expected actions to be listed")
	}
}

func TestExecute_OnSuccess(t *testing.T) {
	p := &JiraPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook:   plugin.HookOnSuccess,
		Config: map[string]any{},
		Context: plugin.ReleaseContext{
			Version: "1.0.0",
		},
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !resp.Success {
		t.Error("Expected success")
	}
}

func TestExecute_OnError(t *testing.T) {
	p := &JiraPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook:   plugin.HookOnError,
		Config: map[string]any{},
		Context: plugin.ReleaseContext{
			Version: "1.0.0",
		},
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !resp.Success {
		t.Error("Expected success")
	}
}

func TestExecute_UnhandledHook(t *testing.T) {
	p := &JiraPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook:   plugin.HookPrePlan,
		Config: map[string]any{},
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !resp.Success {
		t.Error("Expected success for unhandled hook")
	}
}

func TestGetClient_MissingCredentials(t *testing.T) {
	p := &JiraPlugin{}

	// Clear env vars
	os.Unsetenv("JIRA_USERNAME")
	os.Unsetenv("JIRA_TOKEN")
	os.Unsetenv("JIRA_API_TOKEN")

	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "missing all",
			cfg:  &Config{BaseURL: "https://company.atlassian.net"},
		},
		{
			name: "missing username",
			cfg: &Config{
				BaseURL: "https://company.atlassian.net",
				Token:   "test-token",
			},
		},
		{
			name: "missing token",
			cfg: &Config{
				BaseURL:  "https://company.atlassian.net",
				Username: "user@example.com",
			},
		},
		{
			name: "missing base URL",
			cfg: &Config{
				Username: "user@example.com",
				Token:    "test-token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.getClient(tt.cfg)
			if err == nil {
				t.Error("Expected error for missing credentials")
			}
		})
	}
}

func TestGetClient_ValidConfig(t *testing.T) {
	p := &JiraPlugin{}

	cfg := &Config{
		BaseURL:  "https://company.atlassian.net",
		Username: "user@example.com",
		Token:    "test-token",
	}

	client, err := p.getClient(cfg)
	if err != nil {
		t.Errorf("getClient() error = %v", err)
	}
	if client == nil {
		t.Error("client should not be nil")
	}
}

func TestGetClient_TokenFromEnv(t *testing.T) {
	p := &JiraPlugin{}

	os.Setenv("JIRA_USERNAME", "env-user@example.com")
	os.Setenv("JIRA_TOKEN", "env-token")
	defer func() {
		os.Unsetenv("JIRA_USERNAME")
		os.Unsetenv("JIRA_TOKEN")
	}()

	cfg := &Config{
		BaseURL: "https://company.atlassian.net",
	}

	client, err := p.getClient(cfg)
	if err != nil {
		t.Errorf("getClient() error = %v", err)
	}
	if client == nil {
		t.Error("client should not be nil")
	}
}

// Helper function to check if string contains substring.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestIsPrivateIP tests the SSRF protection function.
func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		// Private IPv4 addresses (RFC 1918)
		{name: "10.0.0.1", ip: "10.0.0.1", expected: true},
		{name: "10.255.255.255", ip: "10.255.255.255", expected: true},
		{name: "172.16.0.1", ip: "172.16.0.1", expected: true},
		{name: "172.31.255.255", ip: "172.31.255.255", expected: true},
		{name: "192.168.0.1", ip: "192.168.0.1", expected: true},
		{name: "192.168.255.255", ip: "192.168.255.255", expected: true},

		// Loopback
		{name: "127.0.0.1", ip: "127.0.0.1", expected: true},
		{name: "127.255.255.255", ip: "127.255.255.255", expected: true},

		// Link-local
		{name: "169.254.0.1", ip: "169.254.0.1", expected: true},
		{name: "169.254.255.255", ip: "169.254.255.255", expected: true},

		// Carrier-grade NAT
		{name: "100.64.0.1 CGNAT", ip: "100.64.0.1", expected: true},
		{name: "100.127.255.255 CGNAT", ip: "100.127.255.255", expected: true},

		// TEST-NET ranges
		{name: "192.0.2.1 TEST-NET-1", ip: "192.0.2.1", expected: true},
		{name: "198.51.100.1 TEST-NET-2", ip: "198.51.100.1", expected: true},
		{name: "203.0.113.1 TEST-NET-3", ip: "203.0.113.1", expected: true},

		// Reserved range
		{name: "240.0.0.1 reserved", ip: "240.0.0.1", expected: true},

		// Public IPv4 addresses
		{name: "8.8.8.8 Google DNS", ip: "8.8.8.8", expected: false},
		{name: "1.1.1.1 Cloudflare", ip: "1.1.1.1", expected: false},
		{name: "52.45.60.1 AWS", ip: "52.45.60.1", expected: false},
		{name: "104.16.0.1 Cloudflare", ip: "104.16.0.1", expected: false},

		// IPv6 Loopback
		{name: "::1 IPv6 loopback", ip: "::1", expected: true},

		// IPv6 Link-local
		{name: "fe80::1 IPv6 link-local", ip: "fe80::1", expected: true},

		// IPv6 Unique Local Address (ULA)
		{name: "fc00::1 IPv6 ULA", ip: "fc00::1", expected: true},
		{name: "fd00::1 IPv6 ULA", ip: "fd00::1", expected: true},

		// Public IPv6 addresses
		{name: "2001:4860:4860::8888 Google IPv6", ip: "2001:4860:4860::8888", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := parseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}

			result := isPrivateIP(ip)
			if result != tt.expected {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

// TestIsPrivateIP_BoundaryConditions tests edge cases for IP range boundaries.
func TestIsPrivateIP_BoundaryConditions(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		// Boundary tests for 10.0.0.0/8
		{name: "9.255.255.255 before 10.x", ip: "9.255.255.255", expected: false},
		{name: "10.0.0.0 start", ip: "10.0.0.0", expected: true},
		{name: "10.255.255.255 end", ip: "10.255.255.255", expected: true},
		{name: "11.0.0.0 after 10.x", ip: "11.0.0.0", expected: false},

		// Boundary tests for 172.16.0.0/12
		{name: "172.15.255.255 before", ip: "172.15.255.255", expected: false},
		{name: "172.16.0.0 start", ip: "172.16.0.0", expected: true},
		{name: "172.31.255.255 end", ip: "172.31.255.255", expected: true},
		{name: "172.32.0.0 after", ip: "172.32.0.0", expected: false},

		// Boundary tests for 192.168.0.0/16
		{name: "192.167.255.255 before", ip: "192.167.255.255", expected: false},
		{name: "192.168.0.0 start", ip: "192.168.0.0", expected: true},
		{name: "192.168.255.255 end", ip: "192.168.255.255", expected: true},
		{name: "192.169.0.0 after", ip: "192.169.0.0", expected: false},

		// Boundary tests for CGNAT 100.64.0.0/10
		{name: "100.63.255.255 before CGNAT", ip: "100.63.255.255", expected: false},
		{name: "100.64.0.0 start CGNAT", ip: "100.64.0.0", expected: true},
		{name: "100.127.255.255 end CGNAT", ip: "100.127.255.255", expected: true},
		{name: "100.128.0.0 after CGNAT", ip: "100.128.0.0", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := parseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}

			result := isPrivateIP(ip)
			if result != tt.expected {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

// TestValidateBaseURL tests URL validation for SSRF protection.
func TestValidateBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		// Valid HTTPS URLs
		{name: "valid https atlassian", url: "https://company.atlassian.net", wantErr: false},
		{name: "valid https with port", url: "https://jira.example.com:8443", wantErr: false},
		{name: "valid https with path", url: "https://jira.example.com/jira", wantErr: false},

		// Note: localhost HTTP is rejected because it resolves to private IP (127.0.0.1/::1)
		// This is intentional security behavior - the scheme check allows it but private IP check catches it
		{name: "localhost http rejected", url: "http://localhost:8080", wantErr: true},
		{name: "127.0.0.1 http rejected", url: "http://127.0.0.1:8080", wantErr: true},

		// Invalid - missing URL
		{name: "empty string", url: "", wantErr: true},

		// Invalid - non-HTTPS for non-localhost
		{name: "http non-localhost", url: "http://jira.example.com", wantErr: true},

		// Invalid - wrong scheme
		{name: "ftp scheme", url: "ftp://jira.example.com", wantErr: true},
		{name: "file scheme", url: "file:///etc/passwd", wantErr: true},

		// Invalid - control characters (injection)
		{name: "newline injection", url: "https://jira.example.com\nHost: evil.com", wantErr: true},
		{name: "carriage return", url: "https://jira.example.com\rHost: evil.com", wantErr: true},
		{name: "tab injection", url: "https://jira.example.com\tevil", wantErr: true},

		// Invalid - localhost on HTTPS (block SSRF to localhost via HTTPS)
		{name: "https localhost", url: "https://localhost:8080", wantErr: true},
		{name: "https 127.0.0.1", url: "https://127.0.0.1:8080", wantErr: true},
		{name: "https ::1", url: "https://[::1]:8080", wantErr: true},

		// Invalid - cloud metadata endpoints
		{name: "AWS metadata", url: "https://169.254.169.254", wantErr: true},
		{name: "GCP metadata", url: "https://metadata.google.internal", wantErr: true},
		{name: "GCP metadata alt", url: "https://metadata.goog", wantErr: true},
		{name: "Alibaba metadata", url: "https://100.100.100.200", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBaseURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBaseURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

// TestValidateBaseURL_SSRFAttempts tests common SSRF attack vectors.
func TestValidateBaseURL_SSRFAttempts(t *testing.T) {
	ssrfAttempts := []struct {
		name string
		url  string
	}{
		{name: "AWS metadata endpoint", url: "https://169.254.169.254/latest/meta-data/"},
		{name: "GCP metadata endpoint", url: "https://metadata.google.internal/computeMetadata/v1/"},
		{name: "GCP metadata alt", url: "https://metadata.goog/computeMetadata/v1/"},
		{name: "Alibaba Cloud metadata", url: "https://100.100.100.200/latest/meta-data/"},
		{name: "Private 10.x network", url: "https://10.0.0.1/api"},
		{name: "Private 172.x network", url: "https://172.16.0.1/api"},
		{name: "Private 192.168.x network", url: "https://192.168.1.1/api"},
		{name: "Loopback via IP", url: "https://127.0.0.1/api"},
	}

	for _, tt := range ssrfAttempts {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBaseURL(tt.url)
			if err == nil {
				t.Errorf("validateBaseURL(%q) should reject SSRF attempt", tt.url)
			}
		})
	}
}

// parseIP is a helper to parse IP strings for testing.
func parseIP(ipStr string) net.IP {
	return net.ParseIP(ipStr)
}
