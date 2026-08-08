// Package versioning provides version calculation and application use cases.
package versioning

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
	"github.com/relicta-tech/relicta/v4/internal/fileutil"
)

// VersionFileWriter applies a new version to every configured manifest.
//
// Writes are all-or-nothing: every target is read, rendered and validated
// before anything touches the disk. A project that keeps two manifests in step
// is worse off with one of them updated than with neither, so a single bad
// target fails the whole operation with the working tree untouched.
type VersionFileWriter struct {
	repoRoot string
}

// NewVersionFileWriter creates a writer rooted at repoRoot.
func NewVersionFileWriter(repoRoot string) *VersionFileWriter {
	return &VersionFileWriter{repoRoot: repoRoot}
}

// pendingWrite is a rendered file waiting to be committed to disk.
type pendingWrite struct {
	path    string
	content []byte
	mode    os.FileMode
	target  config.VersionTarget
	value   string
}

// Apply writes v to every target. It returns the targets it wrote, in order.
func (w *VersionFileWriter) Apply(targets []config.VersionTarget, v version.SemanticVersion) ([]string, error) {
	if len(targets) == 0 {
		return nil, nil
	}

	// Phase 1: render everything, touching nothing.
	pending := make([]pendingWrite, 0, len(targets))
	for i, t := range targets {
		p, err := w.render(t, v)
		if err != nil {
			return nil, fmt.Errorf("version_files[%d] (%s): %w", i, t.Path, err)
		}
		pending = append(pending, *p)
	}

	// Phase 2: commit. Rendering is where the failures live — bad format, missing
	// key, unparseable file — so by here the remaining risk is I/O only.
	written := make([]string, 0, len(pending))
	for _, p := range pending {
		if err := fileutil.AtomicWriteFile(p.path, p.content, p.mode); err != nil {
			return written, fmt.Errorf("writing %s (%d of %d targets already written): %w",
				p.target.Path, len(written), len(pending), err)
		}
		written = append(written, p.target.Path)
	}

	return written, nil
}

// Plan renders every target and reports what would be written, without writing.
// Used by --dry-run so the rendered values are visible before they land.
func (w *VersionFileWriter) Plan(targets []config.VersionTarget, v version.SemanticVersion) ([]PlannedWrite, error) {
	planned := make([]PlannedWrite, 0, len(targets))
	for i, t := range targets {
		p, err := w.render(t, v)
		if err != nil {
			return nil, fmt.Errorf("version_files[%d] (%s): %w", i, t.Path, err)
		}
		planned = append(planned, PlannedWrite{
			Path:   t.Path,
			Format: string(w.format(t)),
			Key:    t.Key,
			Value:  p.value,
		})
	}
	return planned, nil
}

// PlannedWrite describes one pending version file update.
type PlannedWrite struct {
	Path   string `json:"path"`
	Format string `json:"format"`
	Key    string `json:"key,omitempty"`
	Value  string `json:"value"`
}

// format returns the target's format, defaulting to semver.
func (w *VersionFileWriter) format(t config.VersionTarget) config.VersionFileFormat {
	if t.Format == "" {
		return config.VersionFormatSemver
	}
	return t.Format
}

// render reads a target and produces its new content without writing it.
func (w *VersionFileWriter) render(t config.VersionTarget, v version.SemanticVersion) (*pendingWrite, error) {
	if t.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Confine targets to the repository: a version file is part of the project,
	// and a path escaping it is a config error rather than a use case.
	abs, err := w.resolve(t.Path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file does not exist")
		}
		return nil, err
	}
	mode := info.Mode().Perm()

	data, err := os.ReadFile(abs) //nolint:gosec // abs is confined to repoRoot by resolve
	if err != nil {
		return nil, err
	}

	kind := structuredKind(t.Path)

	// An integer target reads its own current value, since increment derives from
	// the file rather than from the semantic version.
	value, err := w.value(t, v, data, kind)
	if err != nil {
		return nil, err
	}

	var content []byte
	if kind == kindPlain {
		if t.Key != "" {
			return nil, fmt.Errorf("key %q given for a plain-text file; keys apply to json, yaml and toml targets", t.Key)
		}
		// Preserve a trailing newline if the file had one.
		if strings.HasSuffix(string(data), "\n") {
			content = []byte(value + "\n")
		} else {
			content = []byte(value)
		}
	} else {
		if t.Key == "" {
			return nil, fmt.Errorf("key is required for %s files; naming the field avoids writing the wrong one", kind)
		}
		content, err = setStructured(kind, data, t.Key, value, w.format(t))
		if err != nil {
			return nil, err
		}
	}

	return &pendingWrite{path: abs, content: content, mode: mode, target: t, value: value}, nil
}

// resolve turns a repo-relative target path into an absolute one, refusing paths
// that escape the repository root.
func (w *VersionFileWriter) resolve(rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path must be relative to the repository root")
	}

	root, err := filepath.Abs(w.repoRoot)
	if err != nil {
		return "", err
	}
	abs := filepath.Clean(filepath.Join(root, rel))

	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes the repository root")
	}
	return abs, nil
}

// value renders the value to write for a target.
func (w *VersionFileWriter) value(t config.VersionTarget, v version.SemanticVersion, data []byte, kind fileKind) (string, error) {
	format := w.format(t)

	if t.Strategy == config.StrategyIncrement {
		if format != config.VersionFormatInteger {
			return "", fmt.Errorf("strategy 'increment' requires format 'integer', got %q", format)
		}
		return incrementInteger(data, t.Key, kind)
	}

	switch format {
	case config.VersionFormatSemver:
		return v.String(), nil

	case config.VersionFormatSemverBuild:
		// Four-part form. The fourth component is a build counter that semver
		// has no room for, so it starts at 0 rather than being invented.
		return fmt.Sprintf("%d.%d.%d.0", v.Major(), v.Minor(), v.Patch()), nil

	case config.VersionFormatInteger:
		// Without increment, derive a monotonic integer from the version so it
		// rises with each release: 2.7.15 -> 2007015.
		return strconv.FormatUint(v.Major()*1_000_000+v.Minor()*1_000+v.Patch(), 10), nil

	case config.VersionFormatTemplate:
		if t.Template == "" {
			return "", fmt.Errorf("format 'template' requires a template")
		}
		return renderTemplate(t.Template, v), nil

	default:
		return "", fmt.Errorf("unknown format %q (want semver, semver.build, integer or template)", format)
	}
}

// renderTemplate substitutes version components into a template.
func renderTemplate(tmpl string, v version.SemanticVersion) string {
	r := strings.NewReplacer(
		"${major}", strconv.FormatUint(v.Major(), 10),
		"${minor}", strconv.FormatUint(v.Minor(), 10),
		"${patch}", strconv.FormatUint(v.Patch(), 10),
		"${prerelease}", string(v.Prerelease()),
		"${build}", string(v.Metadata()),
		"${version}", v.String(),
	)
	return r.Replace(tmpl)
}

// incrementInteger reads the target's current integer value and adds one.
func incrementInteger(data []byte, key string, kind fileKind) (string, error) {
	var current string

	if kind == kindPlain {
		current = strings.TrimSpace(string(data))
	} else {
		got, err := getStructured(kind, data, key)
		if err != nil {
			return "", err
		}
		current = got
	}

	if current == "" {
		return "", fmt.Errorf("cannot increment: no current value found")
	}

	n, err := strconv.ParseInt(current, 10, 64)
	if err != nil {
		return "", fmt.Errorf("cannot increment %q: not an integer", current)
	}
	return strconv.FormatInt(n+1, 10), nil
}

// fileKind is the structured format of a target file, inferred from extension.
type fileKind string

const (
	kindJSON  fileKind = "json"
	kindYAML  fileKind = "yaml"
	kindTOML  fileKind = "toml"
	kindPlain fileKind = "plain"
)

// structuredKind infers a file's kind from its extension. Anything unrecognized
// is treated as plain text, where the whole file is the version.
func structuredKind(path string) fileKind {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return kindJSON
	case ".yaml", ".yml":
		return kindYAML
	case ".toml":
		return kindTOML
	default:
		return kindPlain
	}
}

// splitKey turns a key into path segments. A leading "/" selects RFC 6901 JSON
// Pointer syntax, so keys containing dots or slashes remain addressable;
// anything else is a dotted path.
func splitKey(key string) []string {
	if strings.HasPrefix(key, "/") {
		parts := strings.Split(strings.TrimPrefix(key, "/"), "/")
		for i, p := range parts {
			// RFC 6901 escapes: ~1 is "/", ~0 is "~", in that order.
			p = strings.ReplaceAll(p, "~1", "/")
			parts[i] = strings.ReplaceAll(p, "~0", "~")
		}
		return parts
	}
	return strings.Split(key, ".")
}

// decodeStructured parses data according to kind.
func decodeStructured(kind fileKind, data []byte) (map[string]any, error) {
	out := map[string]any{}
	switch kind {
	case kindJSON:
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, fmt.Errorf("parsing json: %w", err)
		}
	case kindYAML:
		if err := yaml.Unmarshal(data, &out); err != nil {
			return nil, fmt.Errorf("parsing yaml: %w", err)
		}
	case kindTOML:
		if err := toml.Unmarshal(data, &out); err != nil {
			return nil, fmt.Errorf("parsing toml: %w", err)
		}
	default:
		return nil, fmt.Errorf("not a structured file")
	}
	return out, nil
}

// getStructured reads the string form of the value at key.
func getStructured(kind fileKind, data []byte, key string) (string, error) {
	doc, err := decodeStructured(kind, data)
	if err != nil {
		return "", err
	}

	segments := splitKey(key)
	var cur any = doc
	for i, seg := range segments {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", fmt.Errorf("key %q: %q is not an object", key, strings.Join(segments[:i], "."))
		}
		cur, ok = m[seg]
		if !ok {
			return "", fmt.Errorf("key %q not found in file", key)
		}
	}

	switch t := cur.(type) {
	case string:
		return t, nil
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	case int:
		return strconv.Itoa(t), nil
	default:
		return "", fmt.Errorf("key %q holds %T, which is not a version value", key, cur)
	}
}

// setStructured writes value at key and re-encodes the document.
//
// Integer formats are written as numbers rather than strings, because a manifest
// that declares an integer field (Android's versionCode, for instance) rejects a
// quoted value.
func setStructured(kind fileKind, data []byte, key, value string, format config.VersionFileFormat) ([]byte, error) {
	doc, err := decodeStructured(kind, data)
	if err != nil {
		return nil, err
	}

	segments := splitKey(key)
	cur := doc
	for _, seg := range segments[:len(segments)-1] {
		next, ok := cur[seg]
		if !ok {
			return nil, fmt.Errorf("key %q not found in file", key)
		}
		m, ok := next.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("key %q: %q is not an object", key, seg)
		}
		cur = m
	}

	leaf := segments[len(segments)-1]
	if _, ok := cur[leaf]; !ok {
		return nil, fmt.Errorf("key %q not found in file", key)
	}

	if format == config.VersionFormatInteger {
		n, convErr := strconv.ParseInt(value, 10, 64)
		if convErr != nil {
			return nil, fmt.Errorf("integer format produced %q: %w", value, convErr)
		}
		cur[leaf] = n
	} else {
		cur[leaf] = value
	}

	return encodeStructured(kind, doc, data)
}

// encodeStructured re-encodes a document, preserving the indentation and
// trailing newline of the original where the format allows.
func encodeStructured(kind fileKind, doc map[string]any, original []byte) ([]byte, error) {
	switch kind {
	case kindJSON:
		indent := detectJSONIndent(original)
		out, err := json.MarshalIndent(doc, "", indent)
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(string(original), "\n") {
			out = append(out, '\n')
		}
		return out, nil

	case kindYAML:
		out, err := yaml.Marshal(doc)
		if err != nil {
			return nil, err
		}
		return out, nil

	case kindTOML:
		out, err := toml.Marshal(doc)
		if err != nil {
			return nil, err
		}
		return out, nil

	default:
		return nil, fmt.Errorf("not a structured file")
	}
}

// detectJSONIndent returns the indentation used by the first indented line, so
// rewriting package.json does not reformat the whole file. Defaults to two
// spaces, which is what npm itself writes.
func detectJSONIndent(data []byte) string {
	for _, line := range strings.Split(string(data), "\n")[1:] {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" || trimmed == "}" {
			continue
		}
		if indent := line[:len(line)-len(trimmed)]; indent != "" {
			return indent
		}
		break
	}
	return "  "
}
