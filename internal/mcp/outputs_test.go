package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.klarlabs.de/mcp/schema"
)

// TestOutputSchemasGenerate guards every type advertised via OutputSchema at
// tool registration. mcp-go runs schema.Generate when OutputSchema is set, and
// on error the ToolBuilder records the failure and never registers the tool —
// silently dropping it from the server. This test fails loudly at build time if
// any advertised output type stops being schema-generatable.
func TestOutputSchemasGenerate(t *testing.T) {
	tests := []struct {
		name    string
		example any
	}{
		{"relicta_status", StatusToolOutput{}},
		{"relicta_plan", PlanToolOutput{}},
		{"relicta_blast_radius", BlastRadiusOutput{}},
		{"relicta_infer_version", InferVersionToolOutput{}},
		{"relicta_validate_release", ValidateReleaseToolOutput{}},
		{"cgp_status", CGPStatusToolOutput{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := schema.Generate(tt.example)
			require.NoError(t, err, "schema.Generate must succeed or the tool is silently dropped at registration")
			require.NotNil(t, s)
			assert.Equal(t, "object", s.Type, "structured output schema must describe a JSON object")
		})
	}
}

// TestDataToolsAdvertiseOutputSchema verifies that every converted tool has an
// output schema attached to its registered Tool. mcp-go's tools/call responder
// emits structuredContent precisely when tool.OutputSchema() is non-nil and the
// handler returns a non-string value, so a non-nil OutputSchema here is the
// registration-side guarantee that structuredContent will be produced on the
// wire. (mcp-go's testutil client is a simplified stub that does not model the
// structuredContent path, so it cannot be used to assert this.)
func TestDataToolsAdvertiseOutputSchema(t *testing.T) {
	srv, err := NewServer("1.0.0")
	require.NoError(t, err)

	for _, name := range []string{
		"relicta_status",
		"relicta_plan",
		"relicta_blast_radius",
		"relicta_infer_version",
		"relicta_validate_release",
		"cgp_status",
	} {
		t.Run(name, func(t *testing.T) {
			tool, ok := srv.server.GetTool(name)
			require.True(t, ok, "tool must be registered (OutputSchema errors silently drop it)")
			assert.NotNil(t, tool.OutputSchema(), "tool must have an outputSchema attached")
		})
	}
}

// TestValidateReleaseReturnsStructuredValue exercises the real handler path via
// Tool.Execute and confirms the result is a typed struct (not a JSON string)
// carrying the expected structured fields. Combined with a non-nil OutputSchema,
// this is exactly the input mcp-go promotes to structuredContent.
func TestValidateReleaseReturnsStructuredValue(t *testing.T) {
	srv, err := NewServer("1.0.0")
	require.NoError(t, err)

	tool, ok := srv.server.GetTool("relicta_validate_release")
	require.True(t, ok)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)

	// Minimal (no-adapter) path returns a typed ValidateReleaseToolOutput, not
	// a string. A string here would silently disable structuredContent.
	out, ok := result.(ValidateReleaseToolOutput)
	require.True(t, ok, "handler must return a typed struct, got %T", result)
	assert.True(t, out.Valid)
	assert.True(t, out.CanProceed)
	require.Len(t, out.Checks, 1)
	assert.Equal(t, "basic", out.Checks[0].Name)
}
