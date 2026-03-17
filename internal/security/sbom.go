// Package security provides security utilities for the application.
package security

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SBOMFormat represents the SBOM output format.
type SBOMFormat string

const (
	// FormatCycloneDXJSON is CycloneDX in JSON format.
	FormatCycloneDXJSON SBOMFormat = "cyclonedx-json"
	// FormatCycloneDXXML is CycloneDX in XML format.
	FormatCycloneDXXML SBOMFormat = "cyclonedx-xml"
	// FormatSPDXJSON is SPDX in JSON format.
	FormatSPDXJSON SBOMFormat = "spdx-json"
)

// SBOMGenerator generates Software Bill of Materials.
type SBOMGenerator struct {
	// ModulePath is the Go module path to analyze.
	ModulePath string
	// Version is the version of the software.
	Version string
	// Name is the name of the software.
	Name string
}

// NewSBOMGenerator creates a new SBOM generator.
func NewSBOMGenerator(modulePath, name, version string) *SBOMGenerator {
	return &SBOMGenerator{
		ModulePath: modulePath,
		Name:       name,
		Version:    version,
	}
}

// CycloneDXBOM represents a CycloneDX Bill of Materials.
type CycloneDXBOM struct {
	XMLName     xml.Name             `xml:"bom" json:"-"`
	XMLNS       string               `xml:"xmlns,attr" json:"-"`
	Version     int                  `xml:"version,attr" json:"version"`
	SerialNum   string               `xml:"serialNumber,attr" json:"serialNumber"`
	BOMFormat   string               `json:"bomFormat"`
	SpecVersion string               `json:"specVersion"`
	Metadata    *CycloneDXMetadata   `xml:"metadata" json:"metadata"`
	Components  []CycloneDXComponent `xml:"components>component" json:"components"`
}

// CycloneDXMetadata contains metadata about the BOM.
type CycloneDXMetadata struct {
	Timestamp string              `xml:"timestamp" json:"timestamp"`
	Tools     []CycloneDXTool     `xml:"tools>tool" json:"tools,omitempty"`
	Component *CycloneDXComponent `xml:"component" json:"component,omitempty"`
}

// CycloneDXTool represents a tool used to generate the BOM.
type CycloneDXTool struct {
	Vendor  string `xml:"vendor" json:"vendor"`
	Name    string `xml:"name" json:"name"`
	Version string `xml:"version" json:"version"`
}

// CycloneDXComponent represents a software component.
type CycloneDXComponent struct {
	Type         string                 `xml:"type,attr" json:"type"`
	BOMRef       string                 `xml:"bom-ref,attr" json:"bom-ref"`
	Name         string                 `xml:"name" json:"name"`
	Version      string                 `xml:"version" json:"version"`
	PURL         string                 `xml:"purl,omitempty" json:"purl,omitempty"`
	Licenses     []CycloneDXLicense     `xml:"licenses>license,omitempty" json:"licenses,omitempty"`
	Hashes       []CycloneDXHash        `xml:"hashes>hash,omitempty" json:"hashes,omitempty"`
	ExternalRefs []CycloneDXExternalRef `xml:"externalReferences>reference,omitempty" json:"externalReferences,omitempty"`
}

// CycloneDXLicense represents a license.
type CycloneDXLicense struct {
	ID   string `xml:"id,omitempty" json:"id,omitempty"`
	Name string `xml:"name,omitempty" json:"name,omitempty"`
}

// CycloneDXHash represents a hash value.
type CycloneDXHash struct {
	Algorithm string `xml:"alg,attr" json:"alg"`
	Value     string `xml:",chardata" json:"content"`
}

// CycloneDXExternalRef represents an external reference.
type CycloneDXExternalRef struct {
	Type string `xml:"type,attr" json:"type"`
	URL  string `xml:"url" json:"url"`
}

// Generate creates an SBOM in the specified format.
func (g *SBOMGenerator) Generate(ctx context.Context, format SBOMFormat) ([]byte, error) {
	// Validate format first
	switch format {
	case FormatCycloneDXJSON, FormatCycloneDXXML, FormatSPDXJSON:
		// Valid format, continue
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}

	// Get dependency information
	deps, err := g.getDependencies(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependencies: %w", err)
	}

	switch format {
	case FormatCycloneDXJSON:
		return g.generateCycloneDXJSON(deps)
	case FormatCycloneDXXML:
		return g.generateCycloneDXXML(deps)
	case FormatSPDXJSON:
		return g.generateSPDXJSON(deps)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// Dependency represents a Go module dependency.
type Dependency struct {
	Path    string
	Version string
	Sum     string
	Replace *Dependency
}

// getDependencies retrieves the module dependencies.
func (g *SBOMGenerator) getDependencies(ctx context.Context) ([]Dependency, error) {
	// First, try to get dependencies from the build info embedded in the binary
	if buildInfo, ok := debug.ReadBuildInfo(); ok && len(buildInfo.Deps) > 0 {
		deps := make([]Dependency, 0, len(buildInfo.Deps))
		for _, dep := range buildInfo.Deps {
			d := Dependency{
				Path:    dep.Path,
				Version: dep.Version,
				Sum:     dep.Sum,
			}
			if dep.Replace != nil {
				d.Replace = &Dependency{
					Path:    dep.Replace.Path,
					Version: dep.Replace.Version,
					Sum:     dep.Replace.Sum,
				}
			}
			deps = append(deps, d)
		}
		return deps, nil
	}

	// Fallback: use go list to get dependencies
	return g.getDependenciesFromGoList(ctx)
}

// getDependenciesFromGoList uses go list to get dependencies.
func (g *SBOMGenerator) getDependenciesFromGoList(ctx context.Context) ([]Dependency, error) {
	// Find the module root
	dir := g.ModulePath
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	// Run go list -m -json all
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-json", "all")
	cmd.Dir = dir

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go list failed: %w", err)
	}

	// Parse JSON output (multiple JSON objects)
	var deps []Dependency
	decoder := json.NewDecoder(&stdout)
	for decoder.More() {
		var mod struct {
			Path    string `json:"Path"`
			Version string `json:"Version"`
			Main    bool   `json:"Main"`
			Replace *struct {
				Path    string `json:"Path"`
				Version string `json:"Version"`
			} `json:"Replace"`
		}
		if err := decoder.Decode(&mod); err != nil {
			continue
		}

		// Skip the main module
		if mod.Main {
			continue
		}

		d := Dependency{
			Path:    mod.Path,
			Version: mod.Version,
		}
		if mod.Replace != nil {
			d.Replace = &Dependency{
				Path:    mod.Replace.Path,
				Version: mod.Replace.Version,
			}
		}
		deps = append(deps, d)
	}

	return deps, nil
}

// generateCycloneDXJSON generates a CycloneDX JSON SBOM.
func (g *SBOMGenerator) generateCycloneDXJSON(deps []Dependency) ([]byte, error) {
	bom := g.createCycloneDXBOM(deps)
	bom.BOMFormat = "CycloneDX"
	bom.SpecVersion = "1.5"

	return json.MarshalIndent(bom, "", "  ")
}

// generateCycloneDXXML generates a CycloneDX XML SBOM.
func (g *SBOMGenerator) generateCycloneDXXML(deps []Dependency) ([]byte, error) {
	bom := g.createCycloneDXBOM(deps)
	bom.XMLNS = "http://cyclonedx.org/schema/bom/1.5"

	output, err := xml.MarshalIndent(bom, "", "  ")
	if err != nil {
		return nil, err
	}

	// Add XML header
	return append([]byte(xml.Header), output...), nil
}

// createCycloneDXBOM creates the CycloneDX BOM structure.
func (g *SBOMGenerator) createCycloneDXBOM(deps []Dependency) *CycloneDXBOM {
	serialNum := fmt.Sprintf("urn:uuid:%s", uuid.New().String())

	bom := &CycloneDXBOM{
		Version:   1,
		SerialNum: serialNum,
		Metadata: &CycloneDXMetadata{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Tools: []CycloneDXTool{
				{
					Vendor:  "relicta-tech",
					Name:    "relicta",
					Version: g.Version,
				},
			},
		},
		Components: make([]CycloneDXComponent, 0, len(deps)+1),
	}

	// Add main component
	if g.Name != "" {
		mainComp := CycloneDXComponent{
			Type:    "application",
			BOMRef:  g.Name,
			Name:    g.Name,
			Version: g.Version,
		}
		if g.ModulePath != "" {
			mainComp.PURL = fmt.Sprintf("pkg:golang/%s@%s", g.ModulePath, g.Version)
		}
		bom.Metadata.Component = &mainComp
	}

	// Add dependencies as components
	for _, dep := range deps {
		path := dep.Path
		version := dep.Version
		if dep.Replace != nil {
			path = dep.Replace.Path
			version = dep.Replace.Version
		}

		comp := CycloneDXComponent{
			Type:    "library",
			BOMRef:  fmt.Sprintf("%s@%s", path, version),
			Name:    filepath.Base(path),
			Version: version,
			PURL:    fmt.Sprintf("pkg:golang/%s@%s", strings.ReplaceAll(path, "/", "%2F"), version),
		}

		// Add VCS reference if it looks like a GitHub/GitLab URL
		if strings.HasPrefix(path, "github.com/") || strings.HasPrefix(path, "gitlab.com/") {
			comp.ExternalRefs = []CycloneDXExternalRef{
				{
					Type: "vcs",
					URL:  fmt.Sprintf("https://%s", path),
				},
			}
		}

		bom.Components = append(bom.Components, comp)
	}

	return bom
}

// SPDXBOM represents an SPDX Bill of Materials.
type SPDXBOM struct {
	SPDXVersion       string             `json:"spdxVersion"`
	DataLicense       string             `json:"dataLicense"`
	SPDXID            string             `json:"SPDXID"`
	Name              string             `json:"name"`
	DocumentNamespace string             `json:"documentNamespace"`
	CreationInfo      SPDXCreationInfo   `json:"creationInfo"`
	Packages          []SPDXPackage      `json:"packages"`
	Relationships     []SPDXRelationship `json:"relationships"`
}

// SPDXCreationInfo contains creation metadata.
type SPDXCreationInfo struct {
	Created  string   `json:"created"`
	Creators []string `json:"creators"`
}

// SPDXPackage represents a package in SPDX.
type SPDXPackage struct {
	SPDXID           string            `json:"SPDXID"`
	Name             string            `json:"name"`
	VersionInfo      string            `json:"versionInfo"`
	DownloadLocation string            `json:"downloadLocation"`
	FilesAnalyzed    bool              `json:"filesAnalyzed"`
	ExternalRefs     []SPDXExternalRef `json:"externalRefs,omitempty"`
}

// SPDXExternalRef represents an external reference.
type SPDXExternalRef struct {
	ReferenceCategory string `json:"referenceCategory"`
	ReferenceType     string `json:"referenceType"`
	ReferenceLocator  string `json:"referenceLocator"`
}

// SPDXRelationship represents a relationship between packages.
type SPDXRelationship struct {
	SpdxElementID      string `json:"spdxElementId"`
	RelationshipType   string `json:"relationshipType"`
	RelatedSpdxElement string `json:"relatedSpdxElement"`
}

// generateSPDXJSON generates an SPDX JSON SBOM.
func (g *SBOMGenerator) generateSPDXJSON(deps []Dependency) ([]byte, error) {
	docID := uuid.New().String()

	bom := &SPDXBOM{
		SPDXVersion:       "SPDX-2.3",
		DataLicense:       "CC0-1.0",
		SPDXID:            "SPDXRef-DOCUMENT",
		Name:              g.Name,
		DocumentNamespace: fmt.Sprintf("https://spdx.org/spdxdocs/%s-%s-%s", g.Name, g.Version, docID),
		CreationInfo: SPDXCreationInfo{
			Created:  time.Now().UTC().Format(time.RFC3339),
			Creators: []string{"Tool: relicta-" + g.Version},
		},
		Packages:      make([]SPDXPackage, 0, len(deps)+1),
		Relationships: make([]SPDXRelationship, 0, len(deps)),
	}

	// Add main package
	mainPkg := SPDXPackage{
		SPDXID:           "SPDXRef-Package-" + sanitizeSPDXID(g.Name),
		Name:             g.Name,
		VersionInfo:      g.Version,
		DownloadLocation: "NOASSERTION",
		FilesAnalyzed:    false,
	}
	bom.Packages = append(bom.Packages, mainPkg)

	// Add dependencies
	for _, dep := range deps {
		path := dep.Path
		version := dep.Version
		if dep.Replace != nil {
			path = dep.Replace.Path
			version = dep.Replace.Version
		}

		pkgID := "SPDXRef-Package-" + sanitizeSPDXID(path)
		pkg := SPDXPackage{
			SPDXID:           pkgID,
			Name:             path,
			VersionInfo:      version,
			DownloadLocation: fmt.Sprintf("https://%s", path),
			FilesAnalyzed:    false,
			ExternalRefs: []SPDXExternalRef{
				{
					ReferenceCategory: "PACKAGE-MANAGER",
					ReferenceType:     "purl",
					ReferenceLocator:  fmt.Sprintf("pkg:golang/%s@%s", strings.ReplaceAll(path, "/", "%2F"), version),
				},
			},
		}
		bom.Packages = append(bom.Packages, pkg)

		// Add relationship
		bom.Relationships = append(bom.Relationships, SPDXRelationship{
			SpdxElementID:      mainPkg.SPDXID,
			RelationshipType:   "DEPENDS_ON",
			RelatedSpdxElement: pkgID,
		})
	}

	// Add document describes relationship
	bom.Relationships = append(bom.Relationships, SPDXRelationship{
		SpdxElementID:      "SPDXRef-DOCUMENT",
		RelationshipType:   "DESCRIBES",
		RelatedSpdxElement: mainPkg.SPDXID,
	})

	return json.MarshalIndent(bom, "", "  ")
}

// sanitizeSPDXID converts a string to a valid SPDX ID.
func sanitizeSPDXID(s string) string {
	// SPDX IDs can only contain letters, numbers, and hyphens
	var result strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			result.WriteRune(r)
		case r == '-', r == '.', r == '/':
			result.WriteRune('-')
		default:
			// Skip invalid characters
		}
	}
	return result.String()
}

// WriteToFile writes the SBOM to a file.
func (g *SBOMGenerator) WriteToFile(ctx context.Context, format SBOMFormat, outputPath string) error {
	data, err := g.Generate(ctx, format)
	if err != nil {
		return err
	}

	return os.WriteFile(outputPath, data, 0644)
}

// SupportedFormats returns the list of supported SBOM formats.
func SupportedFormats() []SBOMFormat {
	return []SBOMFormat{
		FormatCycloneDXJSON,
		FormatCycloneDXXML,
		FormatSPDXJSON,
	}
}
