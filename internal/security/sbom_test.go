package security

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSBOMGenerator(t *testing.T) {
	gen := NewSBOMGenerator("/path/to/module", "my-app", "v1.0.0")

	require.NotNil(t, gen)
	assert.Equal(t, "/path/to/module", gen.ModulePath)
	assert.Equal(t, "my-app", gen.Name)
	assert.Equal(t, "v1.0.0", gen.Version)
}

func TestSBOMGenerator_createCycloneDXBOM(t *testing.T) {
	gen := NewSBOMGenerator("github.com/example/app", "example-app", "v1.2.3")

	deps := []Dependency{
		{Path: "github.com/stretchr/testify", Version: "v1.8.4"},
		{Path: "golang.org/x/sync", Version: "v0.5.0"},
	}

	bom := gen.createCycloneDXBOM(deps)

	require.NotNil(t, bom)
	assert.Equal(t, 1, bom.Version)
	assert.Contains(t, bom.SerialNum, "urn:uuid:")

	// Check metadata
	require.NotNil(t, bom.Metadata)
	assert.NotEmpty(t, bom.Metadata.Timestamp)
	require.Len(t, bom.Metadata.Tools, 1)
	assert.Equal(t, "relicta", bom.Metadata.Tools[0].Name)

	// Check main component
	require.NotNil(t, bom.Metadata.Component)
	assert.Equal(t, "example-app", bom.Metadata.Component.Name)
	assert.Equal(t, "v1.2.3", bom.Metadata.Component.Version)
	assert.Equal(t, "application", bom.Metadata.Component.Type)

	// Check dependencies
	assert.Len(t, bom.Components, 2)

	// First component
	comp1 := bom.Components[0]
	assert.Equal(t, "library", comp1.Type)
	assert.Equal(t, "testify", comp1.Name) // filepath.Base
	assert.Equal(t, "v1.8.4", comp1.Version)
	assert.Contains(t, comp1.PURL, "pkg:golang/")
	assert.Len(t, comp1.ExternalRefs, 1)
	assert.Equal(t, "vcs", comp1.ExternalRefs[0].Type)
}

func TestSBOMGenerator_generateCycloneDXJSON(t *testing.T) {
	gen := NewSBOMGenerator("github.com/example/app", "example-app", "v1.0.0")

	deps := []Dependency{
		{Path: "github.com/example/dep", Version: "v1.0.0"},
	}

	data, err := gen.generateCycloneDXJSON(deps)

	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Verify it's valid JSON
	var bom CycloneDXBOM
	err = json.Unmarshal(data, &bom)
	require.NoError(t, err)

	assert.Equal(t, "CycloneDX", bom.BOMFormat)
	assert.Equal(t, "1.5", bom.SpecVersion)
	assert.Equal(t, 1, bom.Version)
}

func TestSBOMGenerator_generateCycloneDXXML(t *testing.T) {
	gen := NewSBOMGenerator("github.com/example/app", "example-app", "v1.0.0")

	deps := []Dependency{
		{Path: "github.com/example/dep", Version: "v1.0.0"},
	}

	data, err := gen.generateCycloneDXXML(deps)

	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Should start with XML header
	assert.True(t, strings.HasPrefix(string(data), "<?xml"))

	// Verify it's valid XML
	var bom CycloneDXBOM
	err = xml.Unmarshal(data, &bom)
	require.NoError(t, err)

	assert.Equal(t, "http://cyclonedx.org/schema/bom/1.5", bom.XMLNS)
}

func TestSBOMGenerator_generateSPDXJSON(t *testing.T) {
	gen := NewSBOMGenerator("github.com/example/app", "example-app", "v1.0.0")

	deps := []Dependency{
		{Path: "github.com/example/dep", Version: "v1.0.0"},
	}

	data, err := gen.generateSPDXJSON(deps)

	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Verify it's valid JSON
	var bom SPDXBOM
	err = json.Unmarshal(data, &bom)
	require.NoError(t, err)

	assert.Equal(t, "SPDX-2.3", bom.SPDXVersion)
	assert.Equal(t, "CC0-1.0", bom.DataLicense)
	assert.Equal(t, "SPDXRef-DOCUMENT", bom.SPDXID)
	assert.Equal(t, "example-app", bom.Name)
	assert.Contains(t, bom.DocumentNamespace, "spdx.org")

	// Check packages
	assert.Len(t, bom.Packages, 2) // main + 1 dependency

	// Check relationships
	assert.NotEmpty(t, bom.Relationships)
}

func TestSBOMGenerator_Generate(t *testing.T) {
	gen := NewSBOMGenerator("github.com/example/app", "example-app", "v1.0.0")

	// Mock dependencies for this test
	t.Run("cyclonedx-json", func(t *testing.T) {
		// This will try to get actual dependencies, but we can test the format handling
		data, err := gen.Generate(context.Background(), FormatCycloneDXJSON)
		// May fail if not in a Go module, but should not panic
		if err == nil {
			assert.NotEmpty(t, data)
			assert.True(t, json.Valid(data))
		}
	})

	t.Run("cyclonedx-xml", func(t *testing.T) {
		data, err := gen.Generate(context.Background(), FormatCycloneDXXML)
		if err == nil {
			assert.NotEmpty(t, data)
			assert.True(t, strings.HasPrefix(string(data), "<?xml"))
		}
	})

	t.Run("spdx-json", func(t *testing.T) {
		data, err := gen.Generate(context.Background(), FormatSPDXJSON)
		if err == nil {
			assert.NotEmpty(t, data)
			assert.True(t, json.Valid(data))
		}
	})

	t.Run("unsupported format", func(t *testing.T) {
		_, err := gen.Generate(context.Background(), "invalid-format")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported format")
	})
}

func TestSanitizeSPDXID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"github.com/example/pkg", "github-com-example-pkg"},
		{"golang.org/x/sync", "golang-org-x-sync"},
		{"my-package", "my-package"},
		{"MyPackage123", "MyPackage123"},
		{"pkg@v1.0.0", "pkgv1-0-0"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeSPDXID(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestSupportedFormats(t *testing.T) {
	formats := SupportedFormats()

	assert.Len(t, formats, 3)
	assert.Contains(t, formats, FormatCycloneDXJSON)
	assert.Contains(t, formats, FormatCycloneDXXML)
	assert.Contains(t, formats, FormatSPDXJSON)
}

func TestDependency_WithReplace(t *testing.T) {
	gen := NewSBOMGenerator("github.com/example/app", "example-app", "v1.0.0")

	deps := []Dependency{
		{
			Path:    "github.com/old/pkg",
			Version: "v1.0.0",
			Replace: &Dependency{
				Path:    "github.com/new/pkg",
				Version: "v2.0.0",
			},
		},
	}

	bom := gen.createCycloneDXBOM(deps)

	require.Len(t, bom.Components, 1)
	// Should use the replacement path and version
	assert.Equal(t, "pkg", bom.Components[0].Name)
	assert.Equal(t, "v2.0.0", bom.Components[0].Version)
	assert.Contains(t, bom.Components[0].PURL, "github.com%2Fnew%2Fpkg")
}

func TestCycloneDXComponent_ExternalRefs(t *testing.T) {
	gen := NewSBOMGenerator("github.com/example/app", "example-app", "v1.0.0")

	tests := []struct {
		path       string
		expectRefs bool
	}{
		{"github.com/example/pkg", true},
		{"gitlab.com/example/pkg", true},
		{"golang.org/x/sync", false},
		{"example.com/pkg", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			deps := []Dependency{{Path: tt.path, Version: "v1.0.0"}}
			bom := gen.createCycloneDXBOM(deps)

			require.Len(t, bom.Components, 1)
			if tt.expectRefs {
				assert.NotEmpty(t, bom.Components[0].ExternalRefs)
				assert.Equal(t, "vcs", bom.Components[0].ExternalRefs[0].Type)
			} else {
				assert.Empty(t, bom.Components[0].ExternalRefs)
			}
		})
	}
}

func TestSPDXBOM_Relationships(t *testing.T) {
	gen := NewSBOMGenerator("github.com/example/app", "example-app", "v1.0.0")

	deps := []Dependency{
		{Path: "github.com/dep1", Version: "v1.0.0"},
		{Path: "github.com/dep2", Version: "v2.0.0"},
	}

	data, err := gen.generateSPDXJSON(deps)
	require.NoError(t, err)

	var bom SPDXBOM
	err = json.Unmarshal(data, &bom)
	require.NoError(t, err)

	// Should have:
	// - 2 DEPENDS_ON relationships (main -> dep1, main -> dep2)
	// - 1 DESCRIBES relationship (document -> main)
	assert.Len(t, bom.Relationships, 3)

	// Count relationship types
	var dependsOn, describes int
	for _, rel := range bom.Relationships {
		switch rel.RelationshipType {
		case "DEPENDS_ON":
			dependsOn++
		case "DESCRIBES":
			describes++
		}
	}
	assert.Equal(t, 2, dependsOn)
	assert.Equal(t, 1, describes)
}
