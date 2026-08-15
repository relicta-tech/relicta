// Package versioning provides version calculation and application use cases.
package versioning

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

// Apply writes v to every target. It returns the paths it wrote, in the order
// they were first mentioned.
func (w *VersionFileWriter) Apply(targets []config.VersionTarget, v version.SemanticVersion) ([]string, error) {
	if len(targets) == 0 {
		return nil, nil
	}

	// Phase 1: render everything, touching nothing.
	//
	// Targets sharing a path compose rather than compete: each is rendered
	// against the result of the previous one, and the path is written once at the
	// end. Rendering every target against the on-disk content instead would make
	// the last write win and silently drop the others — which is exactly what
	// Android needs (versionName and versionCode in one build.gradle) and what
	// Helm needs (Chart.yaml's version and appVersion).
	staged, order, err := w.stage(targets, v)
	if err != nil {
		return nil, err
	}

	// Phase 2: commit. Rendering is where the failures live — bad format, missing
	// key, unparseable file — so by here the remaining risk is I/O only.
	written := make([]string, 0, len(order))
	for _, rel := range order {
		p := staged[rel]
		if err := fileutil.AtomicWriteFile(p.path, p.content, p.mode); err != nil {
			return written, fmt.Errorf("writing %s (%d of %d files already written): %w",
				rel, len(written), len(order), err)
		}
		written = append(written, rel)
	}

	return written, nil
}

// stage renders every target in order, accumulating changes per path, and
// returns the staged content by path plus the order paths were first seen.
func (w *VersionFileWriter) stage(targets []config.VersionTarget, v version.SemanticVersion) (map[string]pendingWrite, []string, error) {
	staged := make(map[string]pendingWrite, len(targets))
	order := make([]string, 0, len(targets))

	for i, t := range targets {
		var prior []byte
		if existing, ok := staged[t.Path]; ok {
			prior = existing.content
		} else {
			order = append(order, t.Path)
		}

		p, err := w.renderFrom(t, v, prior)
		if err != nil {
			return nil, nil, fmt.Errorf("version_files[%d] (%s): %w", i, t.Path, err)
		}
		staged[t.Path] = *p
	}

	return staged, order, nil
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

// render reads a target from disk and produces its new content without writing.
func (w *VersionFileWriter) render(t config.VersionTarget, v version.SemanticVersion) (*pendingWrite, error) {
	return w.renderFrom(t, v, nil)
}

// renderFrom produces a target's new content. When prior is non-nil it is used
// instead of the file's on-disk content, so several targets on one path build on
// each other.
func (w *VersionFileWriter) renderFrom(t config.VersionTarget, v version.SemanticVersion, prior []byte) (*pendingWrite, error) {
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

	data := prior
	if data == nil {
		data, err = os.ReadFile(abs) //nolint:gosec // abs is confined to repoRoot by resolve
		if err != nil {
			return nil, err
		}
	}

	kind := structuredKind(t.Path)

	// An integer target reads its own current value, since increment derives from
	// the file rather than from the semantic version.
	value, err := w.value(t, v, data, kind)
	if err != nil {
		return nil, err
	}

	var content []byte
	switch kind {
	case kindGradle:
		if t.Key == "" {
			return nil, fmt.Errorf("key is required for gradle files; name the property to update, e.g. versionName")
		}
		content, err = setGradle(data, t.Key, value)
		if err != nil {
			return nil, err
		}

	case kindPlain:
		if t.Key != "" {
			return nil, fmt.Errorf("key %q given for a plain-text file; keys apply to json, yaml, toml and gradle targets", t.Key)
		}
		// Preserve a trailing newline if the file had one.
		if strings.HasSuffix(string(data), "\n") {
			content = []byte(value + "\n")
		} else {
			content = []byte(value)
		}

	default:
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
	return resolveInRepo(w.repoRoot, rel)
}

// resolveInRepo turns a repo-relative path into an absolute one, refusing paths
// that escape the repository root. Package-level because reading a version out
// of a manifest needs the same confinement as writing one into it, and a path
// that is unsafe to write is equally unsafe to read.
func resolveInRepo(repoRoot, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path must be relative to the repository root")
	}

	root, err := filepath.Abs(repoRoot)
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

	switch kind {
	case kindPlain:
		current = strings.TrimSpace(string(data))
	case kindGradle:
		got, err := getGradle(data, key)
		if err != nil {
			return "", err
		}
		current = got
	default:
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
	kindJSON   fileKind = "json"
	kindYAML   fileKind = "yaml"
	kindTOML   fileKind = "toml"
	kindGradle fileKind = "gradle"
	kindPlain  fileKind = "plain"
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
	case ".gradle", ".kts":
		// Gradle build scripts are Groovy/Kotlin, not a data format: the version
		// lives in an assignment such as `versionName "2.7.15"`. Handled by
		// targeted line rewriting rather than parsing the script.
		return kindGradle
	default:
		return kindPlain
	}
}

// gradleAssignment matches the assignment for a named Gradle property, capturing
// everything before the value and the value itself, so the rewrite preserves the
// original indentation, assignment style and quoting.
//
// Covers the four forms that appear in practice:
//
//	versionName "2.7.15"      versionName '2.7.15'
//	versionName = "2.7.15"    versionCode 42
func gradleAssignment(key string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^(\s*` + regexp.QuoteMeta(key) + `\s*(?:=\s*)?)(["']?)([^"'\r\n]*)(["']?)(\s*)$`)
}

// getGradle reads the current value of a Gradle property.
func getGradle(data []byte, key string) (string, error) {
	m := gradleAssignment(key).FindSubmatch(data)
	if m == nil {
		return "", fmt.Errorf("no assignment for %q found in the gradle script", key)
	}
	return strings.TrimSpace(string(m[3])), nil
}

// setGradle rewrites the value of a Gradle property, keeping the surrounding
// syntax byte-for-byte. Exactly one assignment must match: a build script that
// sets the same property in two places (say per flavor) needs a decision this
// code should not make silently.
func setGradle(data []byte, key, value string) ([]byte, error) {
	re := gradleAssignment(key)

	matches := re.FindAllSubmatchIndex(data, -1)
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no assignment for %q found in the gradle script", key)
	case 1:
	default:
		return nil, fmt.Errorf("%q is assigned %d times in the gradle script; "+
			"relicta will not guess which one to update", key, len(matches))
	}

	out := re.ReplaceAll(data, []byte("${1}${2}"+value+"${4}${5}"))
	return out, nil
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
