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
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp/audit"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/integration"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/ai"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
	"github.com/relicta-tech/relicta/v4/internal/security/attestation"
)

// NotesGeneratorAdapter adapts the AI service to the ports.NotesGenerator interface.
type NotesGeneratorAdapter struct {
	aiService  ai.Service
	gitAdapter *git.Adapter
}

// NewNotesGeneratorAdapter creates a new NotesGeneratorAdapter.
func NewNotesGeneratorAdapter(aiService ai.Service, gitAdapter *git.Adapter) *NotesGeneratorAdapter {
	return &NotesGeneratorAdapter{
		aiService:  aiService,
		gitAdapter: gitAdapter,
	}
}

// Generate creates release notes for the given run.
func (a *NotesGeneratorAdapter) Generate(ctx context.Context, run *domain.ReleaseRun, options ports.NotesOptions) (*domain.ReleaseNotes, error) {
	// UseAI=false must mean zero AI calls: a release pipeline must never be
	// blocked by a provider's billing or rate-limit state the user never
	// opted into (issue #127).
	if !options.UseAI || a.aiService == nil || !a.aiService.IsAvailable() {
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
		Version:     ptrTo(run.VersionNext()),
		ProductName: "",
		Tone:        a.mapTone(options.TonePreset),
		Audience:    a.mapAudience(options.AudiencePreset),
	}

	// Generate changelog using AI
	changelog, err := a.aiService.GenerateChangelog(ctx, categorized, genOpts)
	if err != nil {
		return nil, fmt.Errorf("AI changelog generation failed: %w", err)
	}

	// Generate release notes. Prefer provider-native structured output when
	// supported (OpenAI / Anthropic / Gemini); fall back to free-form prose.
	releaseNotes, structuredUsed := a.generateReleaseNotesStructuredOrFreeForm(ctx, changelog, genOpts)

	// Combine changelog and release notes into Text field
	combinedText := changelog
	if releaseNotes != "" && releaseNotes != changelog {
		combinedText = releaseNotes + "\n\n## Changelog\n\n" + changelog
	}

	provider := options.Provider
	if structuredUsed {
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
// Returns (notes, structuredUsed). On structured-path failure (parse error,
// API error, missing fields) it transparently falls back to the free-form
// GenerateReleaseNotes call so callers always get something — release notes
// are cosmetic prose, not load-bearing for governance decisions.
func (a *NotesGeneratorAdapter) generateReleaseNotesStructuredOrFreeForm(
	ctx context.Context,
	changelog string,
	genOpts ai.GenerateOptions,
) (string, bool) {
	if structured, ok := a.aiService.(ai.StructuredOutputService); ok {
		systemPrompt := "You generate categorized release notes for a software release. " +
			"Return a structured ReleaseNotes payload with summary, sections by category, " +
			"and any breaking changes / upgrade notes."
		userPrompt := "Generate structured release notes from this changelog:\n\n" + changelog

		bytes, err := structured.CompleteStructured(ctx, systemPrompt, userPrompt, releaseNotesSchemaShim{})
		if err == nil && len(bytes) > 0 {
			if rendered, parseErr := renderStructuredReleaseNotes(bytes); parseErr == nil && rendered != "" {
				return rendered, true
			}
		}
	}

	notes, err := a.aiService.GenerateReleaseNotes(ctx, changelog, genOpts)
	if err != nil || notes == "" {
		return changelog, false
	}
	return notes, false
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

	// Build basic changelog from commits
	var changelog string
	for _, commit := range changeSet.Commits() {
		changelog += fmt.Sprintf("- %s\n", commit.Subject())
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
	case "public", "all":
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
	pushTags          bool
	attestationConfig *config.AttestationConfig
	auditChain        *audit.Chain
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

// WithAttestationConfig configures attestation generation for the PublisherAdapter.
func WithAttestationConfig(cfg *config.AttestationConfig) PublisherAdapterOption {
	return func(a *PublisherAdapter) {
		a.attestationConfig = cfg
	}
}

// WithAuditChain provides an audit chain for attestation generation.
func WithAuditChain(chain *audit.Chain) PublisherAdapterOption {
	return func(a *PublisherAdapter) {
		a.auditChain = chain
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

	// Check if tag already exists (idempotency)
	exists, err := a.tagCreator.TagExists(ctx, tagName)
	if err != nil {
		return &ports.StepResult{
			Success: false,
			Error:   fmt.Errorf("failed to check tag existence: %w", err),
		}, err
	}
	if exists {
		return &ports.StepResult{
			Success: true,
			Output:  fmt.Sprintf("Tag %s already exists (idempotent)", tagName),
		}, nil
	}

	// Create the annotated tag
	message := fmt.Sprintf("Release %s", run.VersionNext().String())
	if run.Notes() != nil && run.Notes().Text != "" {
		// Include a summary in the tag message
		message = fmt.Sprintf("Release %s\n\n%s", run.VersionNext().String(), run.Notes().Text)
	}

	if err := a.tagCreator.CreateTag(ctx, tagName, message); err != nil {
		return &ports.StepResult{
			Success: false,
			Error:   err,
		}, err
	}

	output := fmt.Sprintf("Created tag %s", tagName)

	// Push only when explicitly enabled.
	if a.pushTags {
		if err := a.tagCreator.PushTag(ctx, tagName, "origin"); err != nil {
			return &ports.StepResult{
				Success: false,
				Output:  output,
				Error:   fmt.Errorf("tag created but push failed: %w", err),
			}, err
		}
		output = fmt.Sprintf("Created and pushed tag %s", tagName)
	}

	return &ports.StepResult{
		Success: true,
		Output:  output,
	}, nil
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

	// Build the attestation generator
	repoID := run.RepoRoot()
	gen := attestation.NewGenerator(repoID, a.auditChain)

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

	// Generate the attestation statement
	stmt, err := gen.Generate(ctx, run)
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
		// Check if tag already exists
		if a.gitAdapter != nil {
			tagName := run.TagName()
			if tagName == "" {
				tagName = "v" + run.VersionNext().String()
			}
			// GetTag returns nil and error if tag doesn't exist
			tag, err := a.gitAdapter.GetTag(ctx, tagName)
			if err != nil {
				// Tag not found is expected, not an error
				return false, nil
			}
			// Tag exists if we got a non-nil tag
			return tag != nil, nil
		}
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
}

// NewVersionWriterAdapter creates a new VersionWriterAdapter.
func NewVersionWriterAdapter(gitAdapter *git.Adapter, repoRoot string) *VersionWriterAdapter {
	return &VersionWriterAdapter{
		gitAdapter: gitAdapter,
		repoRoot:   repoRoot,
	}
}

// WriteVersion writes the version to configured files.
func (a *VersionWriterAdapter) WriteVersion(ctx context.Context, ver version.SemanticVersion) error {
	if a.gitAdapter == nil {
		return fmt.Errorf("git adapter not configured")
	}

	// Write version to VERSION file if it exists
	// The actual file writing is typically handled by the version service
	// This adapter just ensures the git adapter can be used for commits if needed
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
}

// NewTagCreatorAdapter creates a new TagCreatorAdapter.
func NewTagCreatorAdapter(gitAdapter *git.Adapter) *TagCreatorAdapter {
	return &TagCreatorAdapter{
		gitAdapter: gitAdapter,
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
	_, err = a.gitAdapter.CreateTag(ctx, name, headCommit.Hash(), message)
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
