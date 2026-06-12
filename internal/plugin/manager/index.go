// Package manager provides plugin management functionality for Relicta.
package manager

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// IndexSchemaVersion is the registry index format version published by this
// host. The format follows the nox plugin registry (ADR-008): a static JSON
// file with per-plugin version lists and per-artifact digests plus Sigstore
// Cosign verification metadata — no registry server, no auth.
const IndexSchemaVersion = "2"

// DigestPlaceholder marks an artifact whose digest has not been stamped yet.
// Installation of artifacts carrying it is always refused.
const DigestPlaceholder = "sha256:tbd"

// Index is the published registry document (index.json).
type Index struct {
	SchemaVersion string        `json:"schema_version"`
	GeneratedAt   time.Time     `json:"generated_at"`
	Plugins       []IndexPlugin `json:"plugins"`
}

// IndexPlugin is one plugin's entry in the registry index.
type IndexPlugin struct {
	// Name of the plugin (e.g. "github").
	Name string `json:"name"`
	// Description of what the plugin does.
	Description string `json:"description"`
	// Homepage URL.
	Homepage string `json:"homepage,omitempty"`
	// Category groups plugins for discovery (vcs, notification, ...).
	Category string `json:"category,omitempty"`
	// Maintainers lists the maintaining users/orgs.
	Maintainers []string `json:"maintainers,omitempty"`
	// License SPDX identifier.
	License string `json:"license,omitempty"`
	// Repository is the source repository URL or owner/repo slug.
	Repository string `json:"repository"`
	// AuditStatus is the registry-level trust annotation
	// (verified | community).
	AuditStatus string `json:"audit_status,omitempty"`
	// Versions available, newest first.
	Versions []IndexVersion `json:"versions"`
}

// IndexVersion is one published version of a plugin.
type IndexVersion struct {
	// Version (semver, no leading v).
	Version string `json:"version"`
	// MinSDKVersion is the minimum plugin SDK version required.
	MinSDKVersion int `json:"min_sdk_version,omitempty"`
	// PublishedAt timestamp.
	PublishedAt time.Time `json:"published_at,omitempty"`
	// Hooks the plugin implements.
	Hooks []string `json:"hooks,omitempty"`
	// ConfigSchema documents the plugin's configuration options.
	ConfigSchema map[string]ConfigField `json:"config_schema,omitempty"`
	// Artifacts per platform.
	Artifacts []IndexArtifact `json:"artifacts"`
}

// IndexArtifact is one downloadable artifact of a plugin version.
type IndexArtifact struct {
	// OS (linux | darwin | windows).
	OS string `json:"os"`
	// Arch (amd64 | arm64).
	Arch string `json:"arch"`
	// URL of the release archive.
	URL string `json:"url"`
	// Size in bytes (0 = unknown).
	Size int64 `json:"size,omitempty"`
	// Digest of the archive, "sha256:<hex>". Mandatory for install.
	Digest string `json:"digest"`
	// CosignSigURL is the detached signature for the release checksums file.
	CosignSigURL string `json:"cosign_sig_url,omitempty"`
	// CosignBundleURL is the Sigstore bundle for keyless verification.
	CosignBundleURL string `json:"cosign_bundle_url,omitempty"`
	// CosignCertIdentityRegexp matches the signing workflow identity.
	CosignCertIdentityRegexp string `json:"cosign_cert_identity_regexp,omitempty"`
	// CosignOIDCIssuer is the expected OIDC issuer for keyless signatures.
	CosignOIDCIssuer string `json:"cosign_oidc_issuer,omitempty"`
}

// IsSigned reports whether the artifact carries Cosign keyless verification
// metadata.
func (a IndexArtifact) IsSigned() bool {
	return a.CosignBundleURL != "" && a.CosignCertIdentityRegexp != "" && a.CosignOIDCIssuer != ""
}

// HasValidDigest reports whether the artifact declares a usable digest.
func (a IndexArtifact) HasValidDigest() bool {
	return strings.HasPrefix(a.Digest, "sha256:") && a.Digest != DigestPlaceholder &&
		len(strings.TrimPrefix(a.Digest, "sha256:")) == 64
}

// ArtifactFor returns the artifact matching the given GOOS/GOARCH, or nil.
func (v *IndexVersion) ArtifactFor(goos, goarch string) *IndexArtifact {
	for i := range v.Artifacts {
		if v.Artifacts[i].OS == goos && v.Artifacts[i].Arch == goarch {
			return &v.Artifacts[i]
		}
	}
	return nil
}

// platformKeyToOSArch converts a registry.yaml checksum key
// ("darwin_aarch64") into index (os, arch) pairs ("darwin", "arm64").
func platformKeyToOSArch(key string) (string, string, bool) {
	parts := strings.SplitN(key, "_", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	goos := parts[0]
	var goarch string
	switch parts[1] {
	case "aarch64":
		goarch = "arm64"
	case "x86_64":
		goarch = "amd64"
	default:
		goarch = parts[1]
	}
	return goos, goarch, true
}

// archiveExtension returns the release archive extension per OS.
func archiveExtension(goos string) string {
	if goos == "windows" {
		return "zip"
	}
	return "tar.gz"
}

// BuildIndexFromRegistry converts the legacy registry.yaml document into a
// v2 index. registry.yaml remains the source the index is generated from
// until all consumers migrate (ADR-008); each legacy entry becomes a
// single-version plugin with one artifact per checksum platform.
func BuildIndexFromRegistry(reg *Registry, generatedAt time.Time) (*Index, error) {
	if reg == nil {
		return nil, fmt.Errorf("nil registry")
	}

	idx := &Index{
		SchemaVersion: IndexSchemaVersion,
		GeneratedAt:   generatedAt.UTC(),
	}

	for _, p := range reg.Plugins {
		version := strings.TrimPrefix(p.Version, "v")
		if version == "" {
			return nil, fmt.Errorf("plugin %q: missing version", p.Name)
		}

		hooks := make([]string, 0, len(p.Hooks))
		for _, h := range p.Hooks {
			hooks = append(hooks, string(h))
		}

		var artifacts []IndexArtifact
		platforms := make([]string, 0, len(p.Checksums))
		for key := range p.Checksums {
			platforms = append(platforms, key)
		}
		sort.Strings(platforms)
		for _, key := range platforms {
			goos, goarch, ok := platformKeyToOSArch(key)
			if !ok {
				return nil, fmt.Errorf("plugin %q: unrecognized platform key %q", p.Name, key)
			}
			repoSlug := p.Repository
			artifacts = append(artifacts, IndexArtifact{
				OS:   goos,
				Arch: goarch,
				URL: fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s_%s_%s.%s",
					repoSlug, version, p.Name, version, key, archiveExtension(goos)),
				Digest: "sha256:" + p.Checksums[key],
			})
		}

		var maintainers []string
		if p.Author != "" {
			maintainers = []string{p.Author}
		}

		idx.Plugins = append(idx.Plugins, IndexPlugin{
			Name:        p.Name,
			Description: p.Description,
			Homepage:    p.Homepage,
			Category:    p.Category,
			Maintainers: maintainers,
			License:     p.License,
			Repository:  p.Repository,
			AuditStatus: auditStatusOf(p),
			Versions: []IndexVersion{{
				Version:       version,
				MinSDKVersion: p.MinSDKVersion,
				Hooks:         hooks,
				ConfigSchema:  p.ConfigSchema,
				Artifacts:     artifacts,
			}},
		})
	}

	sort.Slice(idx.Plugins, func(i, j int) bool { return idx.Plugins[i].Name < idx.Plugins[j].Name })
	return idx, nil
}

// MarshalIndex renders the index as stable, indented JSON.
func MarshalIndex(idx *Index) ([]byte, error) {
	out, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal index: %w", err)
	}
	return append(out, '\n'), nil
}

// ParseIndex parses an index document and validates its schema version.
func ParseIndex(data []byte) (*Index, error) {
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse index: %w", err)
	}
	if idx.SchemaVersion != IndexSchemaVersion {
		return nil, fmt.Errorf("unsupported index schema_version %q (want %q)", idx.SchemaVersion, IndexSchemaVersion)
	}
	return &idx, nil
}

// auditStatusOf extracts the audit status from a legacy registry entry.
func auditStatusOf(p PluginInfo) string {
	return p.AuditStatus
}

// LoadRegistryFromFile reads a legacy registry.yaml document from disk.
func LoadRegistryFromFile(path string) (*Registry, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- operator-supplied registry path
	if err != nil {
		return nil, fmt.Errorf("read registry: %w", err)
	}
	var reg Registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}
	return &reg, nil
}
