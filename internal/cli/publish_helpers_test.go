package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	apprelease "github.com/relicta-tech/relicta/internal/application/release"
	"github.com/relicta-tech/relicta/internal/config"
	domainrelease "github.com/relicta-tech/relicta/internal/domain/release"
)

func captureCLIStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	os.Stdout = old

	return buf.String()
}

func TestValidateReleaseForPublish(t *testing.T) {
	cfg = config.DefaultConfig()
	t.Cleanup(func() {
		publishSkipApproval = false
	})

	rel := newTestRelease(t, "validate-1")

	if err := validateReleaseForPublish(rel); err == nil {
		t.Fatal("expected validation to fail when release is not approved")
	}

	publishSkipApproval = true
	if err := validateReleaseForPublish(rel); err != nil {
		t.Fatalf("unexpected validation error with skip flag: %v", err)
	}

	publishSkipApproval = false
	unplanned := domainrelease.NewRelease(domainrelease.ReleaseID("no-plan"), "main", ".")
	if err := validateReleaseForPublish(unplanned); err == nil {
		t.Fatal("expected validation to fail when no plan is set")
	}
}

func TestShouldCreatePushAndPluginsFlags(t *testing.T) {
	cfg = config.DefaultConfig()
	t.Cleanup(func() {
		publishSkipTag = false
		publishSkipPush = false
		publishSkipPlugins = false
	})

	cfg.Versioning.GitTag = true
	cfg.Versioning.GitPush = true
	enabled := true
	cfg.Plugins = []config.PluginConfig{{Name: "github", Enabled: &enabled}}

	if !shouldCreateTag() {
		t.Fatal("expected create tag when git tag enabled and flag false")
	}
	publishSkipTag = true
	if shouldCreateTag() {
		t.Fatal("expected create tag to be skipped when flag true")
	}

	if !shouldPushTag() {
		t.Fatal("expected push tag when git push enabled and flag false")
	}
	publishSkipPush = true
	if shouldPushTag() {
		t.Fatal("expected push tag to be skipped when flag true")
	}

	if !shouldRunPlugins() {
		t.Fatal("expected plugins to run when enabled and flag false")
	}
	publishSkipPlugins = true
	if shouldRunPlugins() {
		t.Fatal("expected plugins to skip when flag true")
	}
}

func TestBuildPublishInputIncludesFlags(t *testing.T) {
	cfg = config.DefaultConfig()
	t.Cleanup(func() {
		dryRun = false
	})

	dryRun = true
	release := newTestRelease(t, "input-1")
	input := buildPublishInput(release)
	if !input.DryRun {
		t.Fatal("expected input to reflect dry run flag")
	}
	if input.TagPrefix != cfg.Versioning.TagPrefix {
		t.Fatalf("unexpected tag prefix: %s", input.TagPrefix)
	}
	if !input.CreateTag {
		t.Fatal("expected create tag when git tag enabled")
	}
}

func TestOutputPublishAndPluginResults(t *testing.T) {
	cfg = config.DefaultConfig()

	output := apprelease.PublishReleaseOutput{
		TagName:    "v0.1.0",
		ReleaseURL: "https://example.com/release",
	}
	result := apprelease.PluginResult{
		PluginName: "github",
		Message:    "done",
		Success:    true,
	}

	pluginResults := []apprelease.PluginResult{
		result,
		{PluginName: "npm", Message: "failed", Success: false},
	}

	stdout := captureCLIStdout(func() {
		outputPublishResults(&output)
		outputPluginResults(pluginResults)
	})

	if !strings.Contains(stdout, "Created tag v0.1.0") {
		t.Fatal("expected tag creation message")
	}
	if !strings.Contains(stdout, "Release URL: https://example.com/release") {
		t.Fatal("expected release URL message")
	}
	if !strings.Contains(stdout, "github: done") || !strings.Contains(stdout, "npm: failed") {
		t.Fatalf("expected plugin summaries, got: %s", stdout)
	}
}
