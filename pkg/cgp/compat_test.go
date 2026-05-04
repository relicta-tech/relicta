package cgp

import (
	"bytes"
	"embed"
	"encoding/json"
	"io/fs"
	"path"
	"strings"
	"testing"
)

// FF#2 — CGP wire-format stability fitness function.
//
// Asserts golden JSON envelopes from `testdata/v0.1/*.json` round-trip
// through current Marshal/Unmarshal byte-identical (after JSON canonicalisation).
//
// Why this matters: pkg/cgp is a public SDK. External consumers — third-party
// governance tools, federated Hub deployments, agent SDKs — depend on a stable
// wire format at protocol version 0.1. Silently changing the JSON shape (field
// rename, dropped omitempty, added required field) breaks them without warning.
//
// To deliberately bump the wire format, copy testdata to `testdata/v0.2/` and
// add corresponding test cases — the v0.1 cases stay green forever.

//go:embed testdata/v0.1/*.json
var goldenV01 embed.FS

func TestCGPWireFormat_v01_RoundTripStable(t *testing.T) {
	const dir = "testdata/v0.1"

	entries, err := fs.ReadDir(goldenV01, dir)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			golden, err := fs.ReadFile(goldenV01, path.Join(dir, name))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}

			// 1. Decode the golden envelope into a typed message.
			msg, err := Unmarshal(golden)
			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// 2. Re-marshal that message through the public Marshal API.
			reemitted, err := Marshal(msg)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			// 3. Canonicalize both sides (strip whitespace, normalise key order)
			//    so cosmetic JSON formatting differences don't trigger false alarms.
			canonGolden := canonicalJSON(t, golden)
			canonReemit := canonicalJSON(t, reemitted)

			if !bytes.Equal(canonGolden, canonReemit) {
				t.Errorf("wire format drift detected for %s\n--- golden\n%s\n--- re-emitted\n%s",
					name, string(canonGolden), string(canonReemit))
			}
		})
	}
}

func TestCGPWireFormat_v01_VersionedConstants(t *testing.T) {
	if ProtocolVersion != "0.1" {
		t.Errorf("ProtocolVersion changed to %q — bump testdata to a new version directory before changing this constant",
			ProtocolVersion)
	}
}

func TestCGPWireFormat_v01_TypeStringsStable(t *testing.T) {
	wantStrings := map[MessageType]string{
		TypeChangeProposal:         "change.proposal",
		TypeGovernanceEvaluation:   "change.evaluation",
		TypeGovernanceDecision:     "change.decision",
		TypeExecutionAuthorization: "change.execution_authorized",
	}
	for k, want := range wantStrings {
		if string(k) != want {
			t.Errorf("MessageType drift: %v -> %q, expected %q", k, k, want)
		}
	}
}

// canonicalJSON parses then re-marshals JSON to a stable canonical form
// (sorted keys, no insignificant whitespace) so semantic equality survives
// formatting differences.
func canonicalJSON(t *testing.T, data []byte) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	out, err := json.Marshal(sortKeys(v))
	if err != nil {
		t.Fatalf("canonicalize re-marshal: %v", err)
	}
	return out
}

// sortKeys recursively converts maps with sorted keys so json.Marshal emits
// deterministic byte output. Slices and primitives pass through unchanged.
func sortKeys(v any) any {
	switch x := v.(type) {
	case map[string]any:
		// json.Marshal already sorts map keys alphabetically — pass through.
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = sortKeys(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = sortKeys(val)
		}
		return out
	default:
		return x
	}
}
