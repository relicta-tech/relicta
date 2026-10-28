package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
)

const maxExternalEvidenceBytes = 1 << 20

var (
	commitSHARegexp = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	digestRegexp    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// externalReleaseEvidence is the publisher's reported outcome for a tag-push run.
type externalReleaseEvidence struct {
	Outcome   string            `json:"outcome"`
	Provider  string            `json:"provider"`
	RunURL    string            `json:"run_url"`
	Tag       string            `json:"tag"`
	Commit    string            `json:"commit"`
	Artifacts map[string]string `json:"artifacts"`
	Failure   string            `json:"failure,omitempty"`
}

func readExternalReleaseEvidence(path string) (externalReleaseEvidence, string, error) {
	file, err := os.Open(path) // #nosec G304 -- explicit CLI path supplied by the operator/workflow
	if err != nil {
		return externalReleaseEvidence{}, "", fmt.Errorf("open external release evidence: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxExternalEvidenceBytes+1))
	if err != nil {
		return externalReleaseEvidence{}, "", fmt.Errorf("read external release evidence: %w", err)
	}
	if len(data) > maxExternalEvidenceBytes {
		return externalReleaseEvidence{}, "", fmt.Errorf("external release evidence exceeds %d bytes", maxExternalEvidenceBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var evidence externalReleaseEvidence
	if err := decoder.Decode(&evidence); err != nil {
		return externalReleaseEvidence{}, "", fmt.Errorf("decode external release evidence: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return externalReleaseEvidence{}, "", fmt.Errorf("external release evidence must contain one JSON object")
	}
	if err := evidence.Validate(); err != nil {
		return externalReleaseEvidence{}, "", err
	}
	canonical, err := json.Marshal(evidence)
	if err != nil {
		return externalReleaseEvidence{}, "", fmt.Errorf("encode external release evidence: %w", err)
	}
	return evidence, string(canonical), nil
}

func (e externalReleaseEvidence) Validate() error {
	if e.Outcome != "published" && e.Outcome != "failed" {
		return fmt.Errorf("external release evidence outcome must be published or failed")
	}
	if e.Provider == "" || e.Tag == "" {
		return fmt.Errorf("external release evidence requires provider and tag")
	}
	if !commitSHARegexp.MatchString(e.Commit) {
		return fmt.Errorf("external release evidence commit must be a full Git SHA")
	}
	parsedURL, err := url.ParseRequestURI(e.RunURL)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return fmt.Errorf("external release evidence run_url must be an HTTPS URL")
	}
	if e.Outcome == "failed" && e.Failure == "" {
		return fmt.Errorf("failed external release evidence requires a failure description")
	}
	if e.Outcome == "published" && len(e.Artifacts) == 0 {
		return fmt.Errorf("external release evidence requires at least one artifact digest")
	}
	for name, digest := range e.Artifacts {
		if name == "" || !digestRegexp.MatchString(digest) {
			return fmt.Errorf("external release evidence artifact %q must have a sha256 digest", name)
		}
	}
	return nil
}
