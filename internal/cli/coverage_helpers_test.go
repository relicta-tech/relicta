package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/relicta-tech/relicta/v4/internal/analysis"
	"github.com/relicta-tech/relicta/v4/internal/application/governance"
	"github.com/relicta-tech/relicta/v4/internal/application/versioning"
	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	releaseapp "github.com/relicta-tech/relicta/v4/internal/domain/release/app"
	releasedomain "github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// captureStdoutCov captures stdout output for test assertions.
func captureStdoutCov(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	os.Stdout = old
	return buf.String()
}

// --- buildPlanAnalysisConfig tests ---

func TestBuildPlanAnalysisConfig_NoFlags(t *testing.T) {
	origAnalyze, origReview, origDisableAI, origMinConf := planAnalyze, planReview, planDisableAI, planMinConfidence
	defer func() {
		planAnalyze, planReview, planDisableAI, planMinConfidence = origAnalyze, origReview, origDisableAI, origMinConf
	}()

	planAnalyze, planReview, planDisableAI, planMinConfidence = false, false, false, 0

	_, updated := buildPlanAnalysisConfig(false)
	assert.False(t, updated)
}

func TestBuildPlanAnalysisConfig_AnalyzeFlag(t *testing.T) {
	origAnalyze, origReview, origDisableAI := planAnalyze, planReview, planDisableAI
	defer func() { planAnalyze, planReview, planDisableAI = origAnalyze, origReview, origDisableAI }()

	planAnalyze, planReview, planDisableAI = true, false, false

	acfg, updated := buildPlanAnalysisConfig(false)
	assert.True(t, updated)
	assert.True(t, acfg.EnableAI)
}

func TestBuildPlanAnalysisConfig_ReviewFlag(t *testing.T) {
	origAnalyze, origReview, origDisableAI := planAnalyze, planReview, planDisableAI
	defer func() { planAnalyze, planReview, planDisableAI = origAnalyze, origReview, origDisableAI }()

	planAnalyze, planReview, planDisableAI = false, true, false

	_, updated := buildPlanAnalysisConfig(false)
	assert.True(t, updated)
}

func TestBuildPlanAnalysisConfig_DisableAI(t *testing.T) {
	origAnalyze, origReview, origDisableAI := planAnalyze, planReview, planDisableAI
	defer func() { planAnalyze, planReview, planDisableAI = origAnalyze, origReview, origDisableAI }()

	planAnalyze, planReview, planDisableAI = false, false, true

	acfg, updated := buildPlanAnalysisConfig(false)
	assert.True(t, updated)
	assert.False(t, acfg.EnableAI)
}

func TestBuildPlanAnalysisConfig_MinConfidenceSet(t *testing.T) {
	origAnalyze, origReview, origDisableAI, origMinConf := planAnalyze, planReview, planDisableAI, planMinConfidence
	defer func() {
		planAnalyze, planReview, planDisableAI, planMinConfidence = origAnalyze, origReview, origDisableAI, origMinConf
	}()

	planAnalyze, planReview, planDisableAI, planMinConfidence = false, false, false, 0.9

	acfg, updated := buildPlanAnalysisConfig(true)
	assert.True(t, updated)
	assert.Equal(t, 0.9, acfg.MinConfidence)
}

func TestBuildPlanAnalysisConfig_AllFlags(t *testing.T) {
	origAnalyze, origReview, origDisableAI, origMinConf := planAnalyze, planReview, planDisableAI, planMinConfidence
	defer func() {
		planAnalyze, planReview, planDisableAI, planMinConfidence = origAnalyze, origReview, origDisableAI, origMinConf
	}()

	planAnalyze, planReview, planDisableAI, planMinConfidence = true, false, true, 0.5

	acfg, updated := buildPlanAnalysisConfig(true)
	assert.True(t, updated)
	assert.False(t, acfg.EnableAI)
	assert.Equal(t, 0.5, acfg.MinConfidence)
}

// --- shouldCreateTag / shouldPushTag / shouldRunPlugins tests ---

func TestShouldCreateTag(t *testing.T) {
	origSkipTag, origCfg := publishSkipTag, cfg
	defer func() { publishSkipTag, cfg = origSkipTag, origCfg }()

	tests := []struct {
		name     string
		skipTag  bool
		gitTag   bool
		expected bool
	}{
		{"both enabled", false, true, true},
		{"skip tag", true, true, false},
		{"config disabled", false, false, false},
		{"both disabled", true, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publishSkipTag = tt.skipTag
			cfg = &config.Config{Versioning: config.VersioningConfig{GitTag: tt.gitTag}}
			assert.Equal(t, tt.expected, shouldCreateTag())
		})
	}
}

func TestShouldPushTag(t *testing.T) {
	origSkipPush, origCfg := publishSkipPush, cfg
	defer func() { publishSkipPush, cfg = origSkipPush, origCfg }()

	tests := []struct {
		name     string
		skipPush bool
		gitPush  bool
		expected bool
	}{
		{"both enabled", false, true, true},
		{"skip push", true, true, false},
		{"config disabled", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publishSkipPush = tt.skipPush
			cfg = &config.Config{Versioning: config.VersioningConfig{GitPush: tt.gitPush}}
			assert.Equal(t, tt.expected, shouldPushTag())
		})
	}
}

func TestShouldRunPlugins(t *testing.T) {
	origSkipPlugins, origCfg := publishSkipPlugins, cfg
	defer func() { publishSkipPlugins, cfg = origSkipPlugins, origCfg }()

	tests := []struct {
		name        string
		skipPlugins bool
		plugins     []config.PluginConfig
		expected    bool
	}{
		{"plugins available", false, []config.PluginConfig{{Name: "github"}}, true},
		{"skip plugins", true, []config.PluginConfig{{Name: "github"}}, false},
		{"no plugins", false, nil, false},
		{"empty plugins", false, []config.PluginConfig{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publishSkipPlugins = tt.skipPlugins
			cfg = &config.Config{Plugins: tt.plugins}
			assert.Equal(t, tt.expected, shouldRunPlugins())
		})
	}
}

// --- publish helper output tests ---

func TestOutputStepResults(t *testing.T) {
	t.Run("empty results", func(t *testing.T) {
		out := captureStdoutCov(func() {
			outputStepResults(nil)
		})
		assert.Empty(t, out)
	})

	t.Run("mixed results", func(t *testing.T) {
		results := []releaseapp.StepResult{
			{StepName: "tag", Success: true, Output: "created v1.0.0"},
			{StepName: "push", Success: false, Error: "auth failed"},
			{StepName: "notify", Skipped: true},
		}
		out := captureStdoutCov(func() {
			outputStepResults(results)
		})
		assert.Contains(t, out, "Step Results")
		assert.Contains(t, out, "tag")
		assert.Contains(t, out, "push")
		assert.Contains(t, out, "notify")
	})
}

func TestDisplayPublishActions(t *testing.T) {
	origCfg, origSkipTag, origSkipPush, origSkipPlugins := cfg, publishSkipTag, publishSkipPush, publishSkipPlugins
	defer func() {
		cfg, publishSkipTag, publishSkipPush, publishSkipPlugins = origCfg, origSkipTag, origSkipPush, origSkipPlugins
	}()

	cfg = &config.Config{
		Versioning: config.VersioningConfig{TagPrefix: "v", GitTag: true, GitPush: true},
		Plugins:    []config.PluginConfig{{Name: "github"}},
	}
	publishSkipTag, publishSkipPush, publishSkipPlugins = false, false, false

	out := captureStdoutCov(func() {
		displayPublishActions("1.2.3")
	})
	assert.Contains(t, out, "v1.2.3")
	assert.Contains(t, out, "Release Actions")
}

func TestPrintPublishSummary_GitHub(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = &config.Config{Versioning: config.VersioningConfig{TagPrefix: "v"}}

	out := captureStdoutCov(func() {
		printPublishSummary("1.0.0", "v1.0.0", "https://github.com/org/repo")
	})
	assert.Contains(t, out, "Release Summary")
	assert.Contains(t, out, "v1.0.0")
	assert.Contains(t, out, "published")
}

func TestPrintPublishSummary_GitLab(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = &config.Config{Versioning: config.VersioningConfig{TagPrefix: "v"}}

	out := captureStdoutCov(func() {
		printPublishSummary("1.0.0", "v1.0.0", "https://gitlab.com/org/repo")
	})
	assert.Contains(t, out, "GitLab Release")
}

// --- releaseTypeToBumpKind (release.go) tests ---

func TestReleaseTypeToBumpKind_Release(t *testing.T) {
	tests := []struct {
		input changes.ReleaseType
		want  releasedomain.BumpKind
	}{
		{changes.ReleaseTypeMajor, releasedomain.BumpMajor},
		{changes.ReleaseTypeMinor, releasedomain.BumpMinor},
		{changes.ReleaseTypePatch, releasedomain.BumpPatch},
		{changes.ReleaseType(""), releasedomain.BumpPatch},
	}
	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			assert.Equal(t, tt.want, releaseTypeToBumpKind(tt.input))
		})
	}
}

// --- displayAutoApprovalBlocked tests ---

func TestDisplayAutoApprovalBlocked(t *testing.T) {
	t.Run("high risk score", func(t *testing.T) {
		result := &governance.EvaluateReleaseOutput{
			RiskScore: 0.5,
			Decision:  cgp.DecisionApprovalRequired,
		}
		out := captureStdoutCov(func() { displayAutoApprovalBlocked(result) })
		assert.Contains(t, out, "Auto-approval not available")
		assert.Contains(t, out, "exceeds auto-approve threshold")
		assert.Contains(t, out, "manual review")
	})

	t.Run("security risk factors", func(t *testing.T) {
		result := &governance.EvaluateReleaseOutput{
			RiskScore:   0.1,
			Decision:    cgp.DecisionApproved,
			RiskFactors: []cgp.RiskFactor{{Category: "security", Description: "security changes"}},
		}
		out := captureStdoutCov(func() { displayAutoApprovalBlocked(result) })
		assert.Contains(t, out, "Security-related changes")
	})

	t.Run("breaking changes", func(t *testing.T) {
		result := &governance.EvaluateReleaseOutput{
			RiskScore:   0.1,
			Decision:    cgp.DecisionApproved,
			RiskFactors: []cgp.RiskFactor{{Category: "breaking", Description: "breaking API"}},
		}
		out := captureStdoutCov(func() { displayAutoApprovalBlocked(result) })
		assert.Contains(t, out, "Breaking changes")
	})

	t.Run("required actions", func(t *testing.T) {
		result := &governance.EvaluateReleaseOutput{
			RiskScore:       0.1,
			Decision:        cgp.DecisionApproved,
			RequiredActions: []cgp.RequiredAction{{Description: "run security scan"}},
		}
		out := captureStdoutCov(func() { displayAutoApprovalBlocked(result) })
		assert.Contains(t, out, "run security scan")
	})

	t.Run("no specific reasons fallback", func(t *testing.T) {
		result := &governance.EvaluateReleaseOutput{
			RiskScore: 0.1,
			Decision:  cgp.DecisionApproved,
		}
		out := captureStdoutCov(func() { displayAutoApprovalBlocked(result) })
		assert.Contains(t, out, "Governance policy requires manual approval")
	})
}

// --- printBumpNextSteps tests ---

func TestPrintBumpNextSteps(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = &config.Config{Versioning: config.VersioningConfig{TagPrefix: "v"}}

	out := captureStdoutCov(func() {
		printBumpNextSteps(version.MustParse("1.2.0"))
	})
	assert.Contains(t, out, "v1.2.0")
	assert.Contains(t, out, "Next Steps")
	assert.Contains(t, out, "relicta notes")
	assert.Contains(t, out, "relicta publish")
}

// --- outputAnalysisText tests ---

func TestOutputAnalysisText_WithClassifications(t *testing.T) {
	hash := sourcecontrol.CommitHash("abc1234567890")
	result := &analysis.AnalysisResult{
		Stats: analysis.AnalysisStats{
			TotalCommits:      2,
			AverageConfidence: 0.9,
			ConventionalCount: 1,
			HeuristicCount:    1,
		},
		Classifications: map[sourcecontrol.CommitHash]*analysis.CommitClassification{
			hash: {
				CommitHash: hash,
				Type:       changes.CommitTypeFeat,
				Method:     analysis.MethodConventional,
				Confidence: 1.0,
				Reasoning:  "conventional commit",
			},
		},
	}

	commitInfos := []analysis.CommitInfo{
		{Hash: hash, Subject: "feat: add feature"},
		{Hash: "def456", Subject: "unknown commit"},
	}

	out := captureStdoutCov(func() {
		err := outputAnalysisText(result, commitInfos)
		assert.NoError(t, err)
	})
	assert.Contains(t, out, "Commit Analysis")
	assert.Contains(t, out, "Analyzed 2 commits")
	assert.Contains(t, out, "feat")
	assert.Contains(t, out, "unknown")
}

func TestOutputAnalysisText_WithSkipAndBreaking(t *testing.T) {
	hash := sourcecontrol.CommitHash("abc1234567890")
	result := &analysis.AnalysisResult{
		Stats: analysis.AnalysisStats{
			TotalCommits:         1,
			AverageConfidence:    1.0,
			SkippedCount:         1,
			LowConfidenceCount:   1,
			LowConfidenceCommits: []sourcecontrol.CommitHash{hash},
		},
		Classifications: map[sourcecontrol.CommitHash]*analysis.CommitClassification{
			hash: {
				CommitHash:     hash,
				Type:           changes.CommitTypeFeat,
				ShouldSkip:     true,
				SkipReason:     "merge commit",
				IsBreaking:     true,
				BreakingReason: "api removed",
				Reasoning:      "detected via heuristic",
				Method:         analysis.MethodHeuristic,
				Confidence:     0.3,
			},
		},
	}

	commitInfos := []analysis.CommitInfo{
		{Hash: hash, Subject: "feat!: breaking change"},
	}

	out := captureStdoutCov(func() {
		err := outputAnalysisText(result, commitInfos)
		assert.NoError(t, err)
	})
	assert.Contains(t, out, "skip")
	assert.Contains(t, out, "Low confidence commits")
}

// --- outputBumpJSON verifies JSON includes tag_name ---

func TestOutputBumpJSON_IncludesTagName(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = &config.Config{Versioning: config.VersioningConfig{TagPrefix: "v"}}

	// outputBumpJSON writes to stdout; just verify no error
	out := captureStdoutCov(func() {
		err := outputBumpJSON(version.MustParse("1.0.0"), version.MustParse("2.0.0"), version.BumpMajor, false)
		assert.NoError(t, err)
	})
	assert.Contains(t, out, "v2.0.0")
	assert.Contains(t, out, "tag_name")
}

// --- outputSetVersionJSON ---

func TestOutputSetVersionJSON_Coverage(t *testing.T) {
	output := &versioning.SetVersionOutput{
		Version:    version.MustParse("3.0.0"),
		TagName:    "v3.0.0",
		TagCreated: true,
		TagPushed:  false,
	}

	out := captureStdoutCov(func() {
		err := outputSetVersionJSON(output)
		assert.NoError(t, err)
	})
	assert.Contains(t, out, "v3.0.0")
	assert.Contains(t, out, "tag_created")
}

// --- printNotesNextSteps (already tested but adding coverage variant) ---

func TestPrintNotesNextStepsCoverage(t *testing.T) {
	out := captureStdoutCov(func() {
		printNotesNextSteps()
	})
	assert.Contains(t, out, "Next Steps")
	assert.Contains(t, out, "relicta approve")
}
