// Package schemas defines JSON schemas used for AI provider structured-output
// requests. Provider-native structured outputs eliminate runtime parse failures
// and remove regex-based markdown parsing from governance-sensitive code paths.
//
// 2026 model support:
//   - OpenAI gpt-5: response_format json_schema with strict mode
//   - Anthropic Claude 4.x: tool-use with input schema (V1 — wired in follow-up)
//   - Gemini 2.5: response schema via response_mime_type=application/json
//
// Each schema implements `json.Marshaler` so it slots directly into
// go-openai's `ChatCompletionResponseFormatJSONSchema.Schema` field.
package schemas

import (
	"encoding/json"
	"fmt"
)

// Schema is a JSON Schema document Relicta sends to LLM providers for
// structured-output mode. Implementations must marshal to a JSON Schema
// 2020-12 compatible object.
type Schema interface {
	json.Marshaler

	// Name is the schema identifier sent to the provider. Must match
	// /^[a-zA-Z0-9_-]{1,64}$/ for OpenAI strict mode.
	Name() string

	// Description is a human-readable purpose statement.
	Description() string

	// Strict reports whether the provider should reject any output that
	// does not exactly conform to the schema. true is required for any
	// governance-sensitive path; false is acceptable for cosmetic prose.
	Strict() bool
}

// rawSchema is a small helper that lets schema authors define their schema
// as a JSON-encodable Go value (typically a map) and get json.Marshaler for free.
type rawSchema struct {
	name        string
	description string
	strict      bool
	doc         any
}

func (r rawSchema) Name() string                    { return r.name }
func (r rawSchema) Description() string             { return r.description }
func (r rawSchema) Strict() bool                    { return r.strict }
func (r rawSchema) MarshalJSON() ([]byte, error)    { return json.Marshal(r.doc) }

// GovernanceDecisionSchema is the schema for AI-generated governance decision
// summaries. Used by `relicta_evaluate` and risk-narrative tools.
//
// All fields are required (additional_properties: false) so the model cannot
// hallucinate extra fields downstream code does not expect.
func GovernanceDecisionSchema() Schema {
	return rawSchema{
		name:        "GovernanceDecision",
		description: "Structured governance decision with risk score, decision, and rationale.",
		strict:      true,
		doc: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"decision", "risk_score", "rationale", "required_actions"},
			"properties": map[string]any{
				"decision": map[string]any{
					"type":        "string",
					"enum":        []string{"approved", "approval_required", "rejected", "deferred"},
					"description": "The governance outcome.",
				},
				"risk_score": map[string]any{
					"type":        "number",
					"minimum":     0.0,
					"maximum":     1.0,
					"description": "Risk score in [0.0, 1.0].",
				},
				"rationale": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Reasoning entries supporting the decision.",
				},
				"required_actions": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Actions the user must take before publish.",
				},
			},
		},
	}
}

// ReleaseNotesSchema is the schema for AI-generated release notes payloads.
// strict=false because release notes are cosmetic prose; downstream code
// renders the markdown body whether or not the optional fields appear.
func ReleaseNotesSchema() Schema {
	return rawSchema{
		name:        "ReleaseNotes",
		description: "Structured release notes with categorized sections.",
		strict:      false,
		doc: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"summary", "sections"},
			"properties": map[string]any{
				"summary": map[string]any{
					"type":        "string",
					"description": "Brief release headline (1–3 sentences).",
				},
				"sections": map[string]any{
					"type":        "array",
					"items":       releaseSectionSchema(),
					"description": "Categorized changes in the release.",
				},
				"breaking_changes": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Free-form descriptions of breaking changes.",
				},
				"upgrade_notes": map[string]any{
					"type":        "string",
					"description": "Optional upgrade guidance.",
				},
			},
		},
	}
}

// DiffSummarySchema is the schema for diff-summary payloads. strict=true
// because diff summaries inform risk evaluation; missing fields would skew
// downstream risk scoring.
func DiffSummarySchema() Schema {
	return rawSchema{
		name:        "DiffSummary",
		description: "Structured diff summary with semantic risk signals.",
		strict:      true,
		doc: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"summary", "categories", "signals"},
			"properties": map[string]any{
				"summary": map[string]any{
					"type":        "string",
					"description": "1-paragraph overview of the change.",
				},
				"categories": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string", "enum": diffCategories()},
					"description": "Conventional-commit-aligned categories observed.",
				},
				"signals": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"touches_auth", "touches_secrets", "removes_guard", "schema_change"},
					"properties": map[string]any{
						"touches_auth":    map[string]any{"type": "boolean"},
						"touches_secrets": map[string]any{"type": "boolean"},
						"removes_guard":   map[string]any{"type": "boolean"},
						"schema_change":   map[string]any{"type": "boolean"},
					},
					"description": "Boolean risk signals fed into the risk calculator.",
				},
			},
		},
	}
}

// EvalJudgeSchema is the schema used by the LLM-as-judge to score AI output
// against a rubric. strict=true because rubric scoring drives CI gates;
// missing or out-of-range fields would silently degrade gate fidelity.
//
// Each rubric criterion produces one entry. Value is the integer score 1-5.
func EvalJudgeSchema() Schema {
	return rawSchema{
		name:        "EvalJudge",
		description: "Per-criterion scores produced by an LLM-as-judge.",
		strict:      true,
		doc: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"scores"},
			"properties": map[string]any{
				"scores": map[string]any{
					"type":        "array",
					"description": "One entry per rubric criterion. Order should mirror the rubric.",
					"items": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"required":             []string{"criterion", "value", "rationale"},
						"properties": map[string]any{
							"criterion": map[string]any{
								"type":        "string",
								"description": "Name of the criterion being scored (matches Rubric.Criteria[].Name).",
							},
							"value": map[string]any{
								"type":        "integer",
								"minimum":     1,
								"maximum":     5,
								"description": "Score on the 1-5 scale (1=poor, 5=excellent).",
							},
							"rationale": map[string]any{
								"type":        "string",
								"description": "One-sentence justification.",
							},
						},
					},
				},
			},
		},
	}
}

// AudienceNarrativeSchema covers audience-aware release narratives
// (engineering / product / executive / external).
func AudienceNarrativeSchema() Schema {
	return rawSchema{
		name:        "AudienceNarrative",
		description: "Audience-tailored release narrative with structured sections.",
		strict:      false,
		doc: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"audience", "headline", "body"},
			"properties": map[string]any{
				"audience": map[string]any{
					"type": "string",
					"enum": []string{"engineering", "product", "executive", "external"},
				},
				"headline": map[string]any{"type": "string"},
				"body":     map[string]any{"type": "string"},
				"call_to_action": map[string]any{
					"type":        "string",
					"description": "Optional next-step CTA for the audience.",
				},
			},
		},
	}
}

func releaseSectionSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"category", "items"},
		"properties": map[string]any{
			"category": map[string]any{
				"type": "string",
				"enum": []string{"features", "fixes", "performance", "security", "deprecations", "breaking", "internal"},
			},
			"items": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
	}
}

// diffCategories enumerates the diff-summary category vocabulary.
func diffCategories() []string {
	return []string{"feature", "fix", "perf", "refactor", "docs", "test", "chore", "ci", "build", "security", "breaking"}
}

// MustSchemaJSON marshals a Schema to indented JSON or panics. Useful for
// tests and debug logs only.
func MustSchemaJSON(s Schema) string {
	b, err := s.MarshalJSON()
	if err != nil {
		panic(fmt.Sprintf("schemas: marshal %s: %v", s.Name(), err))
	}
	var indented json.RawMessage
	_ = json.Unmarshal(b, &indented)
	pretty, _ := json.MarshalIndent(indented, "", "  ")
	return string(pretty)
}
