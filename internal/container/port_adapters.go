// Package container provides dependency injection for the Relicta application.
package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	monorepotags "github.com/relicta-tech/relicta/v5/internal/application/monorepo"
	appversioning "github.com/relicta-tech/relicta/v5/internal/application/versioning"
	"github.com/relicta-tech/relicta/v5/internal/cgp/audit"
	"github.com/relicta-tech/relicta/v5/internal/config"
	"github.com/relicta-tech/relicta/v5/internal/domain/changes"
	"github.com/relicta-tech/relicta/v5/internal/domain/communication"
	"github.com/relicta-tech/relicta/v5/internal/domain/integration"
	"github.com/relicta-tech/relicta/v5/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v5/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v5/internal/domain/version"
	"github.com/relicta-tech/relicta/v5/internal/infrastructure/ai"
	"github.com/relicta-tech/relicta/v5/internal/infrastructure/git"
	"github.com/relicta-tech/relicta/v5/internal/security/attestation"
)

// NotesGeneratorAdapter adapts the AI service to the ports.NotesGenerator interface.
type NotesGeneratorAdapter struct {
	aiService  ai.Service
	gitAdapter *git.Adapter

	// changelog is how the non-AI path renders notes. Passed as a parameter rather than
	// set afterwards because the whole changelog.* configuration was unread until this
	// existed, and an option nobody calls is how that happens.
	changelog communication.RenderOptions
}

// NewNotesGeneratorAdapter creates a new NotesGeneratorAdapter.
func NewNotesGeneratorAdapter(
	aiService ai.Service,
	gitAdapter *git.Adapter,
	changelog communication.RenderOptions,
) *NotesGeneratorAdapter {
	return &NotesGeneratorAdapter{
		aiService:  aiService,
		gitAdapter: gitAdapter,
		changelog:  changelog,
	}
}

// Generate creates release notes for the given run.
func (a *NotesGeneratorAdapter) Generate(ctx context.Context, run *domain.ReleaseRun, options ports.NotesOptions) (*domain.ReleaseNotes, error) {
	// UseAI=false must mean zero AI calls: a release pipeline must never be
	// blocked by a provider's billing or rate-limit state the user never
	// opted into (issue #127).
	if a.changelog.Format == communication.FormatCustom || !options.UseAI || a.aiService == nil || !a.aiService.IsAvailable() {
		// Fallback to basic changelog without AI enhancement
		return a.generateBasicNotes(ctx, run, options)
	}

	// Get the changeset from the run
	changeSet := run.ChangeSet()
	if changeSet == nil {
		return nil, fmt.Errorf("no changeset available in run")
	}

	// Convert changeset to git.CategorizedChanges for AI service
	categorized := a.convertToCategorizedChanges(changeSet)

	// Configure generation options
	genOpts := ai.GenerateOptions{
		Version:      ptrTo(run.VersionNext()),
		ProductName:  "",
		Tone:         a.mapTone(options.TonePreset),
		Audience:     a.mapAudience(options.AudiencePreset),
		IncludeEmoji: options.IncludeEmoji,
		Language:     options.Language,
	}

	// Generate changelog using AI
	changelog, err := a.aiService.GenerateChangelog(ctx, categorized, genOpts)
	if err != nil {
		return nil, fmt.Errorf("AI changelog generation failed: %w", err)
	}

	// Generate release notes. Prefer provider-native structured output when
	// supported (OpenAI / Anthropic / Gemini); fall back to free-form prose.
	releaseNotes, structuredUsed, narrativeGenerated := a.generateReleaseNotesStructuredOrFreeForm(ctx, changelog, genOpts)

	// Combine changelog and release notes into Text field
	combinedText := changelog
	if releaseNotes != "" && releaseNotes != changelog {
		combinedText = releaseNotes + "\n\n## Changelog\n\n" + changelog
	}

	provider := options.Provider
	if !narrativeGenerated {
		// The changelog was generated, but customer-facing prose was not. Mark
		// the persisted source distinctly so callers do not present this as a
		// completed AI release-note draft.
		provider = "changelog-fallback"
	} else if structuredUsed {
		// Tag the provider so downstream callers / audit logs can tell which
		// path produced the notes. Useful for eval harness regression checks.
		provider = provider + "+structured"
	}

	return &domain.ReleaseNotes{
		Text:           combinedText,
		AudiencePreset: options.AudiencePreset,
		TonePreset:     options.TonePreset,
		Provider:       provider,
		Model:          options.Model,
		GeneratedAt:    time.Now(),
	}, nil
}

// generateReleaseNotesStructuredOrFreeForm produces release-notes prose,
// preferring the structured path when the AI service supports it.
//
// Returns (notes, structuredUsed, narrativeGenerated). On structured-path
// failure (parse error, API error, missing fields) it tries the free-form
// GenerateReleaseNotes call. If that also fails, the changelog remains usable
// as source material, but the caller must be told that customer-facing prose
// was not produced.
func (a *NotesGeneratorAdapter) generateReleaseNotesStructuredOrFreeForm(
	ctx context.Context,
	changelog string,
	genOpts ai.GenerateOptions,
) (string, bool, bool) {
	if structured, ok := a.aiService.(ai.StructuredOutputService); ok {
		systemPrompt := "You generate categorized release notes for a software release. " +
			"Return a structured ReleaseNotes payload with summary, sections by category, " +
			"and any breaking changes / upgrade notes."
		userPrompt := "Generate structured release notes from this changelog:\n\n" + changelog

		bytes, err := structured.CompleteStructured(ctx, systemPrompt, userPrompt, releaseNotesSchemaShim{})
		if err == nil && len(bytes) > 0 {
			if rendered, parseErr := renderStructuredReleaseNotes(bytes); parseErr == nil && rendered != "" {
				return rendered, true, true
			}
		}
	}

	notes, err := a.aiService.GenerateReleaseNotes(ctx, changelog, genOpts)
	if err != nil || notes == "" {
		return changelog, false, false
	}
	return notes, false, true
}

// releaseNotesSchemaShim mirrors the public ReleaseNotesSchema shape without
// importing the schemas package (avoids container → infrastructure → schemas
// import-cycle risk; both sides are infrastructure-tier already but keeping
// the shim explicit clarifies the contract).
type releaseNotesSchemaShim struct{}

func (releaseNotesSchemaShim) Name() string { return "ReleaseNotes" }
func (releaseNotesSchemaShim) Description() string {
	return "Structured release notes with categorized sections."
}
func (releaseNotesSchemaShim) Strict() bool { return false }
func (releaseNotesSchemaShim) MarshalJSON() ([]byte, error) {
	return []byte(`{
		"type": "object",
		"additionalProperties": false,
		"required": ["summary", "sections"],
		"properties": {
			"summary":  {"type": "string", "description": "Brief release headline (1–3 sentences)."},
			"sections": {
				"type":  "array",
				"items": {
					"type": "object",
					"additionalProperties": false,
					"required": ["category", "items"],
					"properties": {
						"category": {"type": "string", "enum": ["features", "fixes", "performance", "security", "deprecations", "breaking", "internal"]},
						"items":    {"type": "array", "items": {"type": "string"}}
					}
				}
			},
			"breaking_changes": {"type": "array", "items": {"type": "string"}},
			"upgrade_notes":   {"type": "string"}
		}
	}`), nil
}

// renderStructuredReleaseNotes converts a ReleaseNotes JSON payload into
// markdown. Order: summary → categorized sections → breaking changes →
// upgrade notes. Empty payload yields empty string so caller can fall back.
func renderStructuredReleaseNotes(b []byte) (string, error) {
	var payload struct {
		Summary  string `json:"summary"`
		Sections []struct {
			Category string   `json:"category"`
			Items    []string `json:"items"`
		} `json:"sections"`
		BreakingChanges []string `json:"breaking_changes"`
		UpgradeNotes    string   `json:"upgrade_notes"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return "", err
	}
	if payload.Summary == "" && len(payload.Sections) == 0 {
		return "", fmt.Errorf("structured release notes payload is empty")
	}

	var sb stringBuilder
	if payload.Summary != "" {
		sb.WriteString(payload.Summary + "\n\n")
	}
	for _, sec := range payload.Sections {
		if len(sec.Items) == 0 {
			continue
		}
		sb.WriteString("## " + capitalizeCategory(sec.Category) + "\n\n")
		for _, item := range sec.Items {
			sb.WriteString("- " + item + "\n")
		}
		sb.WriteString("\n")
	}
	if len(payload.BreakingChanges) > 0 {
		sb.WriteString("## Breaking Changes\n\n")
		for _, change := range payload.BreakingChanges {
			sb.WriteString("- " + change + "\n")
		}
		sb.WriteString("\n")
	}
	if payload.UpgradeNotes != "" {
		sb.WriteString("## Upgrade Notes\n\n" + payload.UpgradeNotes + "\n")
	}
	return sb.String(), nil
}

// stringBuilder is a thin alias for strings.Builder to keep the import list
// stable when this file already pulls in many packages.
type stringBuilder struct {
	buf []byte
}

func (s *stringBuilder) WriteString(x string) { s.buf = append(s.buf, x...) }
func (s *stringBuilder) String() string       { return string(s.buf) }

// capitalizeCategory turns "features" → "Features" for prettier section titles.
func capitalizeCategory(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]&^32) + s[1:]
}

// ComputeInputsHash computes a hash of the inputs used to generate notes.
func (a *NotesGeneratorAdapter) ComputeInputsHash(run *domain.ReleaseRun, options ports.NotesOptions) string {
	h := sha256.New()

	// Include version in hash
	h.Write([]byte(run.VersionNext().String()))

	// Include HEAD SHA
	h.Write([]byte(run.HeadSHA().String()))

	// Include options
	h.Write([]byte(options.AudiencePreset))
	h.Write([]byte(options.TonePreset))
	if options.UseAI {
		h.Write([]byte("ai:true"))
		h.Write([]byte(options.Provider))
		h.Write([]byte(options.Model))
	}

	return hex.EncodeToString(h.Sum(nil))[:16]
}

// generateBasicNotes creates basic release notes without AI.
func (a *NotesGeneratorAdapter) generateBasicNotes(ctx context.Context, run *domain.ReleaseRun, options ports.NotesOptions) (*domain.ReleaseNotes, error) {
	changeSet := run.ChangeSet()
	if changeSet == nil {
		return &domain.ReleaseNotes{
			Text:           "Release " + run.VersionNext().String(),
			AudiencePreset: options.AudiencePreset,
			TonePreset:     options.TonePreset,
			Provider:       "basic",
			Model:          "",
			GeneratedAt:    time.Now(),
		}, nil
	}

	// Rendered through the changelog domain rather than as a flat list of subjects.
	//
	// This path runs whenever no AI provider is configured, which is the default, so it is
	// what most changelogs are made of. It used to emit "- subject" per commit with no
	// grouping, no version heading, no date and no hashes — while every one of those was
	// described by a `changelog.*` setting that shipped with a default and was read by
	// nothing, and while a renderer producing exactly that output already existed in
	// internal/domain/communication with no caller.
	entry := communication.BuildEntry(run.VersionNext(), changeSet, a.changelog)
	changelog, err := RenderChangelogSections(entry, a.changelog)
	if err != nil {
		return nil, fmt.Errorf("render changelog: %w", err)
	}

	return &domain.ReleaseNotes{
		Text:           changelog,
		AudiencePreset: options.AudiencePreset,
		TonePreset:     options.TonePreset,
		Provider:       "basic",
		Model:          "",
		GeneratedAt:    time.Now(),
	}, nil
}

// RenderChangelogSections applies the selected changelog format. Custom templates
// receive communication.ChangelogEntry, keeping the public template data contract
// identical to the data used by the built-in renderer.
func RenderChangelogSections(entry communication.ChangelogEntry, opts communication.RenderOptions) (string, error) {
	if opts.Format != communication.FormatCustom {
		return communication.RenderSections(entry, opts), nil
	}
	if strings.TrimSpace(opts.TemplatePath) == "" {
		return "", fmt.Errorf("changelog.format is custom but changelog.template is empty")
	}
	tmpl, err := template.New(filepath.Base(opts.TemplatePath)).Funcs(template.FuncMap{
		"join":  strings.Join,
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
	}).ParseFiles(opts.TemplatePath)
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", opts.TemplatePath, err)
	}
	var output strings.Builder
	if err := tmpl.ExecuteTemplate(&output, filepath.Base(opts.TemplatePath), entry); err != nil {
		return "", fmt.Errorf("execute template %s: %w", opts.TemplatePath, err)
	}
	return output.String(), nil
}

// convertToCategorizedChanges converts a ChangeSet to git.CategorizedChanges.
func (a *NotesGeneratorAdapter) convertToCategorizedChanges(cs *changes.ChangeSet) *git.CategorizedChanges {
	result := &git.CategorizedChanges{
		Features:      []git.ConventionalCommit{},
		Fixes:         []git.ConventionalCommit{},
		Performance:   []git.ConventionalCommit{},
		Documentation: []git.ConventionalCommit{},
		Refactoring:   []git.ConventionalCommit{},
		Breaking:      []git.ConventionalCommit{},
		Other:         []git.ConventionalCommit{},
		All:           []git.ConventionalCommit{},
	}

	for _, commit := range cs.Commits() {
		gitConventionalCommit := git.ConventionalCommit{
			Commit: git.Commit{
				Hash:    commit.Hash(),
				Message: commit.RawMessage(),
				Subject: commit.Subject(),
				Body:    commit.Body(),
			},
			Type:           git.CommitType(commit.Type()),
			Scope:          commit.Scope(),
			Description:    commit.Subject(),
			Body:           commit.Body(),
			Breaking:       commit.IsBreaking(),
			IsConventional: true,
		}

		// Add to All slice
		result.All = append(result.All, gitConventionalCommit)

		// Map commit type to category
		switch commit.Type() {
		case changes.CommitTypeFeat:
			result.Features = append(result.Features, gitConventionalCommit)
		case changes.CommitTypeFix:
			result.Fixes = append(result.Fixes, gitConventionalCommit)
		case changes.CommitTypeDocs:
			result.Documentation = append(result.Documentation, gitConventionalCommit)
		case changes.CommitTypePerf:
			result.Performance = append(result.Performance, gitConventionalCommit)
		case changes.CommitTypeRefactor:
			result.Refactoring = append(result.Refactoring, gitConventionalCommit)
		default:
			result.Other = append(result.Other, gitConventionalCommit)
		}

		// Also add to Breaking if it's a breaking change
		if commit.IsBreaking() {
			result.Breaking = append(result.Breaking, gitConventionalCommit)
		}
	}

	return result
}

// mapTone maps tone preset string to ai.Tone.
func (a *NotesGeneratorAdapter) mapTone(preset string) ai.Tone {
	switch preset {
	case "technical":
		return ai.ToneTechnical
	case "friendly", "casual":
		return ai.ToneFriendly
	case "professional", "formal":
		return ai.ToneProfessional
	case "excited", "marketing":
		return ai.ToneExcited
	default:
		return ai.ToneProfessional
	}
}

// mapAudience maps audience preset string to ai.Audience.
func (a *NotesGeneratorAdapter) mapAudience(preset string) ai.Audience {
	switch preset {
	case "developer", "developers":
		return ai.AudienceDevelopers
	case "user", "users":
		return ai.AudienceUsers
	case "public", "all", "stakeholders":
		return ai.AudiencePublic
	case "marketing":
		return ai.AudienceMarketing
	default:
		return ai.AudienceDevelopers
	}
}

// PublisherAdapter adapts the plugin executor to the ports.Publisher interface.
type PublisherAdapter struct {
	executor   integration.PluginExecutor
	gitAdapter *git.Adapter
	tagCreator ports.TagCreator
	// pushTags gates the one irreversible action in a release. Phrased
	// positively so the zero value is the safe one: a publisher built without
	// options tags locally and pushes nothing. The previous field was skipPush,
	// whose zero value meant "push", and since the option that set it was never
	// called, every publish pushed regardless of configuration.
	pushTags bool
	// createTags gates tag creation, and is phrased positively for the same reason
	// pushTags is: a publisher built without options creates nothing.
	//
	// Nothing gated this before. `versioning.git_tag` had no reader, and `publish
	// --skip-tag` reached only the printed summary and the JSON output — verified
	// against the shipped binary, which reported "Create tag: false" and then created
	// v0.1.0. The tag step is added to the execution plan at approve time, so by publish
	// there was nothing left that could decline to run it.
	createTags        bool
	attestationConfig *config.AttestationConfig

	// auditChainStore and auditChainRepo say where the governance evidence lives, so
	// the attestation can be anchored to it.
	//
	// A loaded *audit.Chain used to be passed here instead, and the container passed
	// audit.NewChain() — a fresh, empty, process-local chain that nothing ever appended
	// to. Every attestation shipped `"auditChainHash": ""` and `"auditEntryCount": 0`:
	// two fields asserting a governance audit chain and certifying an empty one.
	//
	// A store rather than a chain because the chain has to be read at the moment the
	// attestation is sealed. The entries that matter most — the publish-time evaluation
	// and decision — are appended during the same command, after this adapter was
	// built, so a chain captured at construction would be missing exactly the evidence
	// the attestation is about.
	auditChainStore audit.Store
	auditChainRepo  string

	// remote is where tags are pushed. git.default_remote had no reader, so every push went
	// to "origin" — which is the usual name and not always the name: a repository whose
	// publishing remote is called "upstream" or "release" had the setting to say so, and the
	// push went somewhere else or nowhere.
	remote string

	// packageTags lists the per-package tags a monorepo release carries, resolved when the
	// step runs rather than when the adapter is built: bump writes the manifests during the
	// same command, and the manifest is what the tag names.
	//
	// nil in a repository that is not a monorepo, which is the only shape the release path
	// knew until per-package versioning was wired (ADR-015).
	packageTags func(context.Context) ([]monorepotags.PackageTag, error)
}

// PublisherAdapterOption configures the PublisherAdapter.
type PublisherAdapterOption func(*PublisherAdapter)

// WithPushTags enables pushing tags to the remote. Off unless asked for,
// because pushing a tag starts a public release in any repository whose
// workflows trigger on it.
func WithPushTags(enabled bool) PublisherAdapterOption {
	return func(a *PublisherAdapter) {
		a.pushTags = enabled
	}
}

// WithRemote names the remote tags are pushed to.
//
// Empty means "origin", which is what every push used unconditionally before git.default_remote
// was read.
func WithRemote(remote string) PublisherAdapterOption {
	return func(a *PublisherAdapter) {
		a.remote = remote
	}
}

// pushRemote is the configured remote, or the conventional default.
func (a *PublisherAdapter) pushRemote() string {
	if a.remote == "" {
		return "origin"
	}
	return a.remote
}

// WithTagging enables git tag creation. Off unless asked for, so that a publisher built
// without options leaves the repository's tags alone.
func WithTagging(enabled bool) PublisherAdapterOption {
	return func(a *PublisherAdapter) {
		a.createTags = enabled
	}
}

// WithPackageTags supplies the per-package tags a monorepo release creates alongside the
// repository's own.
//
// Alongside, not instead: the repository tag stays the release marker the rest of the flow
// measures from — the next plan reads it, and a monorepo with no repository tags would have
// every repository-wide command counting from the start of history forever. Each package
// additionally gets its own tag, which is what makes an independent package release findable.
func WithPackageTags(resolve func(context.Context) ([]monorepotags.PackageTag, error)) PublisherAdapterOption {
	return func(a *PublisherAdapter) {
		a.packageTags = resolve
	}
}

// WithAttestationConfig configures attestation generation for the PublisherAdapter.
func WithAttestationConfig(cfg *config.AttestationConfig) PublisherAdapterOption {
	return func(a *PublisherAdapter) {
		a.attestationConfig = cfg
	}
}

// WithAuditChain points the adapter at the stored governance audit chain, so an
// attestation can name the evidence behind the release it attests to.
//
// repository is the governance identity ("owner/repo"), the same key everything else
// records against. Both must be supplied: a store with no repository has no chain to
// read, and the attestation reports an empty one rather than guessing.
func WithAuditChain(store audit.Store, repository string) PublisherAdapterOption {
	return func(a *PublisherAdapter) {
		a.auditChainStore = store
		a.auditChainRepo = repository
	}
}

// NewPublisherAdapter creates a new PublisherAdapter.
func NewPublisherAdapter(executor integration.PluginExecutor, gitAdapter *git.Adapter, tagCreator ports.TagCreator, opts ...PublisherAdapterOption) *PublisherAdapter {
	a := &PublisherAdapter{
		executor:   executor,
		gitAdapter: gitAdapter,
		tagCreator: tagCreator,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// ExecuteStep executes a single step in the publishing plan.
func (a *PublisherAdapter) ExecuteStep(ctx context.Context, run *domain.ReleaseRun, step *domain.StepPlan) (*ports.StepResult, error) {
	// Handle tag step specially - this is where tags are created during publish
	if step.Type == domain.StepTypeTag {
		return a.executeTagStep(ctx, run)
	}

	// Handle attestation step - generates signed governance attestation
	if step.Type == domain.StepTypeAttestation {
		return a.executeAttestationStep(ctx, run)
	}

	// For other steps, use the plugin executor
	if a.executor == nil {
		return nil, fmt.Errorf("no plugin executor configured")
	}

	// Build release context from run
	releaseCtx := a.buildReleaseContext(run)

	// Map step type to hook
	hook := a.mapStepTypeToHook(step.Type)

	// Execute the hook
	responses, err := a.executor.ExecuteHook(ctx, hook, releaseCtx)
	if err != nil {
		return &ports.StepResult{
			Success: false,
			Error:   err,
		}, err
	}

	// Check if any plugin failed
	for _, resp := range responses {
		if !resp.Success {
			return &ports.StepResult{
				Success: false,
				Output:  resp.Message,
				Error:   fmt.Errorf("%s", resp.Error),
			}, nil
		}
	}

	// Collect outputs
	var output string
	for _, resp := range responses {
		if resp.Message != "" {
			output += resp.Message + "\n"
		}
	}

	return &ports.StepResult{
		Success: true,
		Output:  output,
	}, nil
}

// executeTagStep creates and pushes the git tag for the release.
func (a *PublisherAdapter) executeTagStep(ctx context.Context, run *domain.ReleaseRun) (*ports.StepResult, error) {
	if a.tagCreator == nil {
		return nil, fmt.Errorf("tag creator not configured")
	}

	tagName := run.TagName()
	if tagName == "" {
		tagName = "v" + run.VersionNext().String()
	}

	// Declining here rather than dropping the step from the plan: the step stays in the
	// run's record, and its result says the tag was not created and why, which is what an
	// auditor reading the run later needs.
	if !a.createTags {
		return &ports.StepResult{
			Success: true,
			Output:  fmt.Sprintf("Tag %s not created: tagging is disabled (versioning.git_tag or --skip-tag)", tagName),
		}, nil
	}

	// Check whether the tag already exists. Existing tags are left untouched, but
	// the rest of this step still runs: a prior attempt may have created the local
	// tag and then failed to push it or finish the package tags.
	exists, err := a.tagCreator.TagExists(ctx, tagName)
	if err != nil {
		return &ports.StepResult{
			Success: false,
			Error:   fmt.Errorf("failed to check tag existence: %w", err),
		}, err
	}
	var output string
	createdTag := !exists
	if exists {
		if err := a.validateExistingTag(ctx, tagName); err != nil {
			return &ports.StepResult{Success: false, Error: err}, err
		}
		output = fmt.Sprintf("Tag %s already exists (idempotent)", tagName)
	} else {
		// Create the annotated tag.
		message := fmt.Sprintf("Release %s", run.VersionNext().String())
		if run.Notes() != nil && run.Notes().Text != "" {
			// Include a summary in the tag message.
			message = fmt.Sprintf("Release %s\n\n%s", run.VersionNext().String(), run.Notes().Text)
		}

		if err := a.tagCreator.CreateTag(ctx, tagName, message); err != nil {
			return &ports.StepResult{
				Success: false,
				Error:   err,
			}, err
		}
		output = fmt.Sprintf("Created tag %s", tagName)
	}

	// Push only when explicitly enabled.
	if a.pushTags {
		if pushErr := a.tagCreator.PushTag(ctx, tagName, a.pushRemote()); pushErr != nil {
			err := fmt.Errorf("tag %s exists locally but push failed: %w", tagName, pushErr)
			return &ports.StepResult{
				Success: false,
				Output:  output,
				Error:   err,
			}, err
		}
		if createdTag {
			output = fmt.Sprintf("Created and pushed tag %s", tagName)
		} else {
			output = fmt.Sprintf("Existing tag %s pushed", tagName)
		}
	}

	if perPackage, err := a.createPackageTags(ctx, run); err != nil {
		return &ports.StepResult{
			Success: false,
			Output:  output,
			Error:   err,
		}, err
	} else if perPackage != "" {
		output += "; " + perPackage
	}

	return &ports.StepResult{
		Success: true,
		Output:  output,
	}, nil
}

// createPackageTags tags each package of a monorepo at the version its manifest claims.
//
// Idempotent per tag, like the repository tag above it: a package whose tag already exists is
// left alone, so re-running publish after a partial failure finishes the job rather than
// refusing. Returns a description of what it did, empty when this is not a monorepo.
func (a *PublisherAdapter) createPackageTags(ctx context.Context, run *domain.ReleaseRun) (string, error) {
	if a.packageTags == nil || a.tagCreator == nil {
		return "", nil
	}

	tags, err := a.packageTags(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to resolve package tags: %w", err)
	}
	if len(tags) == 0 {
		return "", nil
	}

	created := make([]string, 0, len(tags))
	existing := 0
	for _, pkg := range tags {
		exists, existsErr := a.tagCreator.TagExists(ctx, pkg.Tag)
		if existsErr != nil {
			return "", fmt.Errorf("failed to check tag %s: %w", pkg.Tag, existsErr)
		}
		if exists {
			if err := a.validateExistingTag(ctx, pkg.Tag); err != nil {
				return "", err
			}
			existing++
		} else {
			message := fmt.Sprintf("Release %s %s", pkg.Name, pkg.Version.String())
			if createErr := a.tagCreator.CreateTag(ctx, pkg.Tag, message); createErr != nil {
				return "", fmt.Errorf("failed to tag %s: %w", pkg.Name, createErr)
			}
			created = append(created, pkg.Tag)
		}

		if a.pushTags {
			if pushErr := a.tagCreator.PushTag(ctx, pkg.Tag, a.pushRemote()); pushErr != nil {
				return "", fmt.Errorf("tag %s exists locally but push failed: %w", pkg.Tag, pushErr)
			}
		}
	}

	if len(created) == 0 {
		if a.pushTags {
			return fmt.Sprintf("%d existing package tags pushed", existing), nil
		}
		return fmt.Sprintf("%d package tags already existed", existing), nil
	}
	verb := "created"
	if a.pushTags {
		verb = "created and synchronized"
	}
	if existing > 0 {
		return fmt.Sprintf("%s package tags %s; %d existing tags synchronized", verb, strings.Join(created, ", "), existing), nil
	}
	return fmt.Sprintf("%s package tags %s", verb, strings.Join(created, ", ")), nil
}

// validateExistingTag ensures a retry never accepts a stale tag that names a
// different commit from the release currently being published.
func (a *PublisherAdapter) validateExistingTag(ctx context.Context, name string) error {
	if a.gitAdapter == nil {
		return nil
	}
	tag, err := a.gitAdapter.GetTag(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to inspect existing tag %s: %w", name, err)
	}
	if tag == nil {
		return fmt.Errorf("tag %s was reported as existing but could not be loaded", name)
	}
	head, err := a.gitAdapter.GetLatestCommit(ctx, "HEAD")
	if err != nil {
		return fmt.Errorf("failed to resolve release commit for tag %s: %w", name, err)
	}
	if head == nil || string(head.Hash()) == "" {
		return fmt.Errorf("failed to resolve release commit for tag %s: HEAD is unavailable", name)
	}
	if tag.Hash() != head.Hash() {
		return fmt.Errorf("tag %s points to %s, expected release commit %s; refusing to publish", name, tag.Hash(), head.Hash())
	}
	return nil
}

// executeAttestationStep generates a signed governance attestation for the release.
// This step is non-blocking: any failure is logged as a warning but does not prevent publishing.
func (a *PublisherAdapter) executeAttestationStep(ctx context.Context, run *domain.ReleaseRun) (*ports.StepResult, error) {
	if a.attestationConfig == nil || !a.attestationConfig.Enabled {
		return &ports.StepResult{
			Success: true,
			Output:  "Attestation generation skipped (not enabled)",
		}, nil
	}

	// attestationFailure reports an attestation error. When the operator
	// marked attestation Required, the failure blocks the publish step
	// instead of being silently swallowed as success (the previous
	// behavior shipped releases with no attestation and no signal).
	attestationFailure := func(stage string, cause error) (*ports.StepResult, error) {
		if a.attestationConfig.Required {
			wrapped := fmt.Errorf("attestation %s failed: %w", stage, cause)
			return &ports.StepResult{Success: false, Error: wrapped}, wrapped
		}
		return &ports.StepResult{
			Success: true,
			Output:  fmt.Sprintf("Attestation %s failed (non-blocking): %v", stage, cause),
		}, nil
	}

	// Read the chain now, not at construction: the publish-time evaluation and decision
	// were appended a few steps ago, in this same command, and they are precisely what
	// this attestation is evidence of.
	//
	// A chain that does not verify stops the attestation. Sealing and signing a
	// statement whose auditChainHash points into a tampered chain would launder the
	// tampering — the signature would vouch for the anchor, and the anchor would name
	// entries that no longer say what they said. Better no attestation, loudly, than a
	// signed one that certifies a broken record.
	chain, err := audit.LoadChain(ctx, a.auditChainStore, a.auditChainRepo)
	if err != nil {
		return attestationFailure("audit chain load", err)
	}

	// Build the attestation generator
	repoID := run.RepoRoot()
	gen := attestation.NewGenerator(repoID, chain)
	if a.gitAdapter == nil {
		return attestationFailure("release commit lookup", fmt.Errorf("git adapter not configured"))
	}
	releaseCommit, err := a.gitAdapter.GetLatestCommit(ctx, "HEAD")
	if err != nil {
		return attestationFailure("release commit lookup", err)
	}
	if releaseCommit == nil || releaseCommit.Hash() == "" {
		return attestationFailure("release commit lookup", fmt.Errorf("HEAD has no commit"))
	}

	// Generate the attestation statement
	stmt, err := gen.GenerateForCommit(run, string(releaseCommit.Hash()))
	if err != nil {
		return attestationFailure("generation", err)
	}

	// Parse signing mode and create signer
	mode, err := attestation.ParseSigningMode(a.attestationConfig.SigningMode)
	if err != nil {
		return attestationFailure("signing-mode parse", err)
	}

	var signerOpts []attestation.SignerOption
	if a.attestationConfig.KeyPath != "" {
		signerOpts = append(signerOpts, attestation.WithKeyPath(a.attestationConfig.KeyPath))
	}

	signer := attestation.NewSigner(mode, signerOpts...)

	// Sign the attestation
	signed, err := signer.Sign(ctx, stmt)
	if err != nil {
		return attestationFailure("signing", err)
	}

	// Write the signed attestation to the release directory
	outputPath, err := a.writeAttestation(run, signed)
	if err != nil {
		return attestationFailure("write", err)
	}

	return &ports.StepResult{
		Success: true,
		Output:  fmt.Sprintf("Governance attestation written to %s", outputPath),
	}, nil
}

// writeAttestation writes the signed attestation to the release directory.
func (a *PublisherAdapter) writeAttestation(run *domain.ReleaseRun, att *attestation.SignedAttestation) (string, error) {
	dir := filepath.Join(run.RepoRoot(), ".relicta", "releases", string(run.ID()))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create release directory: %w", err)
	}

	outputPath := filepath.Join(dir, "attestation.intoto.jsonl")

	data, err := json.Marshal(att)
	if err != nil {
		return "", fmt.Errorf("failed to marshal attestation: %w", err)
	}

	// Append newline for JSONL format
	data = append(data, '\n')

	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return "", fmt.Errorf("failed to write attestation file: %w", err)
	}

	return outputPath, nil
}

// CheckIdempotency checks if a step has already been executed.
func (a *PublisherAdapter) CheckIdempotency(ctx context.Context, run *domain.ReleaseRun, step *domain.StepPlan) (bool, error) {
	// Check specific step types for idempotency
	switch step.Type {
	case domain.StepTypeTag:
		// Tag creation, remote push and package tags form one recoverable step.
		// An existing local tag alone cannot prove that all of those effects ran;
		// executeTagStep checks for the tag, leaves it in place, and resumes the rest.
		return false, nil
	case domain.StepTypeAttestation:
		// Check if attestation file already exists
		attPath := filepath.Join(run.RepoRoot(), ".relicta", "releases", string(run.ID()), "attestation.intoto.jsonl")
		if _, err := os.Stat(attPath); err == nil {
			return true, nil
		}
	}

	// Default: not idempotent check available
	return false, nil
}

// buildReleaseContext builds an integration.ReleaseContext from a ReleaseRun.
func (a *PublisherAdapter) buildReleaseContext(run *domain.ReleaseRun) integration.ReleaseContext {
	ctx := integration.ReleaseContext{
		Version:         run.VersionNext(),
		PreviousVersion: run.VersionCurrent(),
		ReleaseType:     changes.ReleaseType(run.BumpKind()),
		RepositoryPath:  run.RepoRoot(),
		TagName:         run.TagName(),
	}

	// Add notes if available
	if run.Notes() != nil {
		ctx.Changelog = run.Notes().Text
		ctx.ReleaseNotes = run.Notes().Text
	}

	// Add changeset if available
	if run.HasChangeSet() {
		ctx.Changes = run.ChangeSet()
	}

	return ctx
}

// mapStepTypeToHook maps a step type to an integration hook.
func (a *PublisherAdapter) mapStepTypeToHook(stepType domain.StepType) integration.Hook {
	switch stepType {
	case domain.StepTypeTag:
		return integration.HookPostVersion
	case domain.StepTypeBuild:
		return integration.HookPostVersion
	case domain.StepTypeArtifact:
		return integration.HookPostPublish
	case domain.StepTypeNotify:
		return integration.HookPostPublish
	case domain.StepTypePlugin:
		return integration.HookPostPublish
	case domain.StepTypeChangelog:
		return integration.HookPostNotes
	default:
		return integration.HookPostPublish
	}
}

// VersionWriterAdapter adapts git operations to the ports.VersionWriter interface.
type VersionWriterAdapter struct {
	gitAdapter *git.Adapter
	repoRoot   string
	targets    []config.VersionTarget
}

// NewVersionWriterAdapter creates a new VersionWriterAdapter.
func NewVersionWriterAdapter(gitAdapter *git.Adapter, repoRoot string, targets ...[]config.VersionTarget) *VersionWriterAdapter {
	var configured []config.VersionTarget
	if len(targets) > 0 {
		configured = targets[0]
	}
	return &VersionWriterAdapter{
		gitAdapter: gitAdapter,
		repoRoot:   repoRoot,
		targets:    configured,
	}
}

// WriteVersion writes the version to configured files.
func (a *VersionWriterAdapter) WriteVersion(ctx context.Context, ver version.SemanticVersion) error {
	if a.gitAdapter == nil {
		return fmt.Errorf("git adapter not configured")
	}

	if _, err := appversioning.NewVersionFileWriter(a.repoRoot).Apply(a.targets, ver); err != nil {
		return fmt.Errorf("apply configured version files: %w", err)
	}
	return nil
}

// WriteChangelog writes or updates the changelog file.
func (a *VersionWriterAdapter) WriteChangelog(ctx context.Context, ver version.SemanticVersion, notes string) error {
	if a.gitAdapter == nil {
		return fmt.Errorf("git adapter not configured")
	}

	// Changelog writing is typically handled during the publish step
	// This is a placeholder for the port interface
	return nil
}

// TagCreatorAdapter adapts git operations to the ports.TagCreator interface.
// It handles creating and pushing git tags during the publish step.
type TagCreatorAdapter struct {
	gitAdapter *git.Adapter

	// requireSigning mirrors versioning.git_sign. See NewTagCreatorAdapter.
	requireSigning bool
}

// NewTagCreatorAdapter creates the adapter that tags a release.
func NewTagCreatorAdapter(gitAdapter *git.Adapter, requireSigning bool) *TagCreatorAdapter {
	return &TagCreatorAdapter{
		gitAdapter:     gitAdapter,
		requireSigning: requireSigning,
	}
}

// CreateTag creates an annotated git tag with the given name and message.
func (a *TagCreatorAdapter) CreateTag(ctx context.Context, name, message string) error {
	if a.gitAdapter == nil {
		return fmt.Errorf("git adapter not configured")
	}

	// Get the HEAD commit to tag
	headCommit, err := a.gitAdapter.GetLatestCommit(ctx, "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	// Create the tag at HEAD
	if a.requireSigning {
		_, err = a.gitAdapter.CreateSignedTag(ctx, name, headCommit.Hash(), message)
	} else {
		_, err = a.gitAdapter.CreateTag(ctx, name, headCommit.Hash(), message)
	}
	if err != nil {
		return fmt.Errorf("failed to create tag %s: %w", name, err)
	}

	return nil
}

// PushTag pushes the specified tag to the remote repository.
func (a *TagCreatorAdapter) PushTag(ctx context.Context, name, remote string) error {
	if a.gitAdapter == nil {
		return fmt.Errorf("git adapter not configured")
	}

	if err := a.gitAdapter.PushTag(ctx, name, remote); err != nil {
		return fmt.Errorf("failed to push tag %s to %s: %w", name, remote, err)
	}

	return nil
}

// TagExists checks if a tag with the given name already exists.
func (a *TagCreatorAdapter) TagExists(ctx context.Context, name string) (bool, error) {
	if a.gitAdapter == nil {
		return false, fmt.Errorf("git adapter not configured")
	}

	tag, err := a.gitAdapter.GetTag(ctx, name)
	if err != nil {
		// Tag not found is expected, not an error
		return false, nil
	}

	return tag != nil, nil
}

// Helper function to create a pointer to a value.
func ptrTo[T any](v T) *T {
	return &v
}
