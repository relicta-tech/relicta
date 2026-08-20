package monorepo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Surgical manifest edits.
//
// A version bump changes one value. Everything else in the file — key order, indentation,
// comments, trailing whitespace — belongs to the project, and rewriting it turns a one-line
// release commit into a whole-file diff that reviewers stop reading.
//
// This is not only cosmetic. The writers these replace were reachable from nothing until
// per-package versioning was wired, and two of them were wrong in ways that only a real
// repository would show:
//
//   - package.json was decoded into a map and re-marshaled, and Go marshals map keys in
//     alphabetical order. Every bump would have permanently reordered the manifest, moving
//     `name` below `dependencies`.
//   - Cargo.toml and pyproject.toml were rewritten with `^(\s*version\s*=\s*)"[^"]+"` applied
//     to the whole file. A dependency declared as its own table —
//
//     [dependencies.serde]
//     version = "1.0"
//
//     is a line beginning with `version =`, so the package's version was written over the
//     dependency's. That is silent corruption of a file the build reads.

// setJSONStringField replaces one top-level string field, leaving every other byte of the
// document untouched.
//
// It walks the token stream to find the value's offsets rather than decoding and re-encoding,
// which is what makes key order, indentation and formatting survive. Only depth-1 keys are
// considered, so a "version" inside dependencies is not the one that moves.
func setJSONStringField(data []byte, key, value string) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(data))

	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("not a JSON object")
	}

	for dec.More() {
		keyTok, keyErr := dec.Token()
		if keyErr != nil {
			return nil, fmt.Errorf("parsing JSON: %w", keyErr)
		}
		name, _ := keyTok.(string)

		// The value begins after the colon that follows the key, and ends where the decoder
		// stops. Whitespace between the two belongs to the file, so the splice starts at the
		// first byte of the value itself.
		start := int(dec.InputOffset())
		valTok, valErr := decodeValue(dec)
		if valErr != nil {
			return nil, fmt.Errorf("parsing JSON: %w", valErr)
		}
		end := int(dec.InputOffset())

		if name != key {
			continue
		}
		if _, isString := valTok.(string); !isString {
			return nil, fmt.Errorf("%q is not a string", key)
		}

		// InputOffset after a key sits before the colon that follows it, so the colon and the
		// whitespace around it are stepped over to leave them exactly as the file had them.
		start = skipJSONSpace(data, start, end)
		if start < end && data[start] == ':' {
			start++
		}
		start = skipJSONSpace(data, start, end)

		quoted, quoteErr := json.Marshal(value)
		if quoteErr != nil {
			return nil, quoteErr
		}

		out := make([]byte, 0, len(data)+len(quoted))
		out = append(out, data[:start]...)
		out = append(out, quoted...)
		out = append(out, data[end:]...)
		return out, nil
	}

	return nil, fmt.Errorf("no %q field", key)
}

// skipJSONSpace advances past insignificant whitespace.
func skipJSONSpace(data []byte, from, to int) int {
	for from < to {
		switch data[from] {
		case ' ', '\t', '\n', '\r':
			from++
		default:
			return from
		}
	}
	return from
}

// decodeValue reads one complete value, descending through nested objects and arrays so that
// the next key the caller reads is the next one at the same level.
func decodeValue(dec *json.Decoder) (json.Token, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}

	delim, ok := tok.(json.Delim)
	if !ok {
		return tok, nil
	}

	depth := 1
	for depth > 0 {
		next, nextErr := dec.Token()
		if nextErr != nil {
			if nextErr == io.EOF {
				return nil, fmt.Errorf("unexpected end of JSON")
			}
			return nil, nextErr
		}
		if d, isDelim := next.(json.Delim); isDelim {
			switch d {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		}
	}
	return delim, nil
}

// setTOMLTableValue replaces a quoted value inside one named table, and only there.
//
// Line-oriented rather than a TOML round-trip because a round-trip discards comments and
// reorders keys — Cargo.toml carries both, and a release must not rewrite a file it was only
// asked to renumber.
//
// Returns false when the table or the key is absent, which callers must treat as a failure:
// silently writing nothing is how a package ends up tagged at a version its manifest never
// claimed.
func setTOMLTableValue(data []byte, table, key, value string) ([]byte, bool) {
	lines := strings.Split(string(data), "\n")
	inTable := table == "" // an empty table name means the document's top level

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "[") {
			// [package] and [package.metadata] are different tables; only the exact header
			// opens the one being edited. Array-of-table headers ([[bin]]) are section
			// headers too, and equally not it.
			header := strings.Trim(trimmed, "[]")
			inTable = header == table
			continue
		}

		if !inTable {
			continue
		}

		name, _, found := strings.Cut(trimmed, "=")
		if !found || strings.TrimSpace(name) != key {
			continue
		}

		// Only the text between the quotes is replaced. Rebuilding the line as `key = "value"`
		// would be simpler and would delete whatever follows it — Cargo.toml commonly carries
		// a trailing comment on the version line, and a release must not eat it.
		eq := strings.Index(line, "=")
		open := strings.Index(line[eq:], `"`)
		if open < 0 {
			continue
		}
		open += eq
		closing := strings.Index(line[open+1:], `"`)
		if closing < 0 {
			continue
		}
		closing += open + 1

		lines[i] = line[:open+1] + value + line[closing:]
		return []byte(strings.Join(lines, "\n")), true
	}

	return data, false
}

// writePreservingMode rewrites a manifest with the permissions it already had.
//
// A fixed 0644 would widen a manifest somebody deliberately kept private, and a release is not
// the moment to change who can read a file.
func writePreservingMode(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(path, data, mode)
}
