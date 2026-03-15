// Package communication provides audience-aware release communication.
// It transforms approved governance decisions and commit changes into
// audience-specific narratives for different stakeholders.
package communication

import (
	"fmt"
	"strings"
)

// AudienceType represents the type of audience for release communication.
type AudienceType string

const (
	// AudienceEngineering targets software engineers and developers.
	AudienceEngineering AudienceType = "engineering"
	// AudienceProduct targets product managers and designers.
	AudienceProduct AudienceType = "product"
	// AudienceExecutive targets C-level executives and leadership.
	AudienceExecutive AudienceType = "executive"
	// AudienceExternal targets end users and the public.
	AudienceExternal AudienceType = "external"
)

// validAudienceTypes contains all valid audience types for validation.
var validAudienceTypes = map[AudienceType]bool{
	AudienceEngineering: true,
	AudienceProduct:     true,
	AudienceExecutive:   true,
	AudienceExternal:    true,
}

// IsValidAudienceType checks whether an audience type string is valid.
func IsValidAudienceType(s string) bool {
	return validAudienceTypes[AudienceType(s)]
}

// AllAudienceTypes returns all valid audience types.
func AllAudienceTypes() []AudienceType {
	return []AudienceType{
		AudienceEngineering,
		AudienceProduct,
		AudienceExecutive,
		AudienceExternal,
	}
}

// CommTone represents the communication tone for an audience narrative.
// This is distinct from NoteTone which is used for legacy release notes.
type CommTone string

const (
	CommToneTechnical CommTone = "technical"
	CommToneBusiness  CommTone = "business"
	CommToneExecutive CommTone = "executive"
	CommTonePublic    CommTone = "public"
)

// DetailLevel represents how much detail to include for an audience.
type DetailLevel string

const (
	DetailFull       DetailLevel = "full"
	DetailSummary    DetailLevel = "summary"
	DetailHighlights DetailLevel = "highlights"
)

// Section represents a section that can be included in a narrative.
type Section string

const (
	SectionBreakingChanges Section = "breaking_changes"
	SectionFeatures        Section = "features"
	SectionFixes           Section = "fixes"
	SectionPerformance     Section = "performance"
	SectionSecurity        Section = "security"
	SectionMigration       Section = "migration"
	SectionMetrics         Section = "metrics"
	SectionRiskAssessment  Section = "risk_assessment"
	SectionBusinessValue   Section = "business_value"
	SectionUserImpact      Section = "user_impact"
	SectionUpgradeGuide    Section = "upgrade_guide"
	SectionContributors    Section = "contributors"
	SectionDocumentation   Section = "documentation"
	SectionStrategicAlign  Section = "strategic_alignment"
)

// Audience defines the configuration for a specific audience.
type Audience struct {
	// Type is the audience type identifier.
	Type AudienceType
	// Name is the human-readable name for this audience.
	Name string
	// Tone determines the writing style.
	Tone CommTone
	// DetailLevel controls the amount of detail included.
	DetailLevel DetailLevel
	// Sections lists which sections to include in the narrative.
	Sections []Section
	// CustomPrompt allows overriding the AI system prompt for this audience.
	CustomPrompt string
}

// Validate checks that the audience configuration is valid.
func (a Audience) Validate() error {
	if a.Type == "" {
		return fmt.Errorf("audience type is required")
	}
	if !validAudienceTypes[a.Type] {
		return fmt.Errorf("invalid audience type %q: valid types are %s", a.Type, validAudienceTypesList())
	}
	if a.Tone == "" {
		return fmt.Errorf("tone is required for audience %q", a.Type)
	}
	if a.DetailLevel == "" {
		return fmt.Errorf("detail level is required for audience %q", a.Type)
	}
	if len(a.Sections) == 0 {
		return fmt.Errorf("at least one section is required for audience %q", a.Type)
	}
	return nil
}

func validAudienceTypesList() string {
	types := AllAudienceTypes()
	strs := make([]string, len(types))
	for i, t := range types {
		strs[i] = string(t)
	}
	return strings.Join(strs, ", ")
}

// DefaultAudiences returns the built-in audience configurations.
func DefaultAudiences() map[AudienceType]Audience {
	return map[AudienceType]Audience{
		AudienceEngineering: {
			Type:        AudienceEngineering,
			Name:        "Engineering",
			Tone:        CommToneTechnical,
			DetailLevel: DetailFull,
			Sections: []Section{
				SectionBreakingChanges,
				SectionFeatures,
				SectionFixes,
				SectionPerformance,
				SectionSecurity,
				SectionMigration,
				SectionDocumentation,
				SectionContributors,
			},
		},
		AudienceProduct: {
			Type:        AudienceProduct,
			Name:        "Product",
			Tone:        CommToneBusiness,
			DetailLevel: DetailSummary,
			Sections: []Section{
				SectionFeatures,
				SectionUserImpact,
				SectionBusinessValue,
				SectionBreakingChanges,
				SectionFixes,
			},
		},
		AudienceExecutive: {
			Type:        AudienceExecutive,
			Name:        "Executive",
			Tone:        CommToneExecutive,
			DetailLevel: DetailHighlights,
			Sections: []Section{
				SectionMetrics,
				SectionRiskAssessment,
				SectionStrategicAlign,
				SectionBreakingChanges,
			},
		},
		AudienceExternal: {
			Type:        AudienceExternal,
			Name:        "External / Public",
			Tone:        CommTonePublic,
			DetailLevel: DetailSummary,
			Sections: []Section{
				SectionFeatures,
				SectionFixes,
				SectionBreakingChanges,
				SectionUpgradeGuide,
			},
		},
	}
}

// CommunicationConfig holds the configuration for audience-aware communication.
type CommunicationConfig struct {
	// DefaultAudience is the audience used when none is specified.
	DefaultAudience AudienceType `mapstructure:"default_audience" json:"default_audience"`
	// Audiences maps audience types to their configurations.
	// If empty, DefaultAudiences() is used.
	Audiences map[AudienceType]AudienceConfig `mapstructure:"audiences" json:"audiences,omitempty"`
}

// AudienceConfig is the YAML-serializable form of an audience definition.
type AudienceConfig struct {
	Name         string   `mapstructure:"name" json:"name"`
	Tone         string   `mapstructure:"tone" json:"tone"`
	DetailLevel  string   `mapstructure:"detail_level" json:"detail_level"`
	Sections     []string `mapstructure:"sections" json:"sections"`
	CustomPrompt string   `mapstructure:"custom_prompt" json:"custom_prompt,omitempty"`
}

// DefaultCommunicationConfig returns the default communication configuration.
func DefaultCommunicationConfig() CommunicationConfig {
	return CommunicationConfig{
		DefaultAudience: AudienceEngineering,
	}
}

// ResolveAudience resolves an audience from the configuration.
// It merges user-configured overrides with defaults.
func (c CommunicationConfig) ResolveAudience(audienceType AudienceType) (Audience, error) {
	// Check user-configured audiences first
	if userCfg, ok := c.Audiences[audienceType]; ok {
		return audienceFromConfig(audienceType, userCfg)
	}

	// Fall back to defaults
	defaults := DefaultAudiences()
	if aud, ok := defaults[audienceType]; ok {
		return aud, nil
	}

	return Audience{}, fmt.Errorf("unknown audience type %q", audienceType)
}

// ResolveAllAudiences resolves all configured audiences, merging with defaults.
func (c CommunicationConfig) ResolveAllAudiences() ([]Audience, error) {
	defaults := DefaultAudiences()
	result := make([]Audience, 0, len(defaults))

	for _, at := range AllAudienceTypes() {
		if userCfg, ok := c.Audiences[at]; ok {
			aud, err := audienceFromConfig(at, userCfg)
			if err != nil {
				return nil, fmt.Errorf("invalid audience config for %q: %w", at, err)
			}
			result = append(result, aud)
		} else if def, ok := defaults[at]; ok {
			result = append(result, def)
		}
	}

	return result, nil
}

// audienceFromConfig converts YAML config into the domain Audience type.
func audienceFromConfig(audienceType AudienceType, cfg AudienceConfig) (Audience, error) {
	sections := make([]Section, len(cfg.Sections))
	for i, s := range cfg.Sections {
		sections[i] = Section(s)
	}

	aud := Audience{
		Type:         audienceType,
		Name:         cfg.Name,
		Tone:         CommTone(cfg.Tone),
		DetailLevel:  DetailLevel(cfg.DetailLevel),
		Sections:     sections,
		CustomPrompt: cfg.CustomPrompt,
	}

	if aud.Name == "" {
		aud.Name = string(audienceType)
	}

	if err := aud.Validate(); err != nil {
		return Audience{}, err
	}
	return aud, nil
}

// OutputFormat represents the output format for narratives.
type OutputFormat string

const (
	OutputMarkdown  OutputFormat = "markdown"
	OutputPlainText OutputFormat = "plaintext"
	OutputHTML      OutputFormat = "html"
)

// IsValidOutputFormat checks whether the given format string is valid.
func IsValidOutputFormat(s string) bool {
	switch OutputFormat(s) {
	case OutputMarkdown, OutputPlainText, OutputHTML:
		return true
	}
	return false
}
