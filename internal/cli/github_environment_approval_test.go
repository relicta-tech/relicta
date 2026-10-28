package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestVerifyGitHubEnvironmentApprovalBindsRunAndReviewer(t *testing.T) {
	identity := githubApprovalRequestContext{
		Token: "test-token", Repository: "owner/repo", RunID: "42",
		SHA: "0123456789abcdef0123456789abcdef01234567", Ref: "refs/tags/v1.2.3",
	}
	responses := []string{
		`{"head_sha":"0123456789abcdef0123456789abcdef01234567","head_branch":"","event":"push","html_url":"https://github.com/owner/repo/actions/runs/42"}`,
		`[{"state":"approved","user":{"login":"reviewer"},"environments":[{"name":"release-approval"}]}]`,
	}
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "api.github.com" || request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("request did not target trusted API with token: %s %s", request.URL, request.Header.Get("Authorization"))
		}
		if requests > 1 {
			t.Fatalf("unexpected request: %s", request.URL)
		}
		body := responses[requests]
		requests++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	proof, err := verifyGitHubEnvironmentApprovalWithClient(context.Background(), "release-approval", identity, client)
	if err != nil {
		t.Fatalf("verifyGitHubEnvironmentApprovalWithClient() error = %v", err)
	}
	if requests != 2 {
		t.Fatalf("made %d API requests, want 2", requests)
	}
	if proof.Environment != "release-approval" || proof.HeadSHA != identity.SHA || proof.RunURL != "https://github.com/owner/repo/actions/runs/42" || len(proof.Reviewers) != 1 || proof.Reviewers[0] != "reviewer" {
		t.Fatalf("unexpected verified approval: %+v", proof)
	}
}

func TestVerifyGitHubEnvironmentApprovalRejectsRunMismatchAndUntrustedOrigin(t *testing.T) {
	identity := githubApprovalRequestContext{Token: "token", Repository: "owner/repo", RunID: "42", SHA: "expected", Ref: "refs/tags/v1.2.3"}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `{"head_sha":"different","event":"push"}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	if _, err := verifyGitHubEnvironmentApprovalWithClient(context.Background(), "release-approval", identity, client); err == nil || !strings.Contains(err.Error(), "SHA") {
		t.Fatalf("run SHA mismatch error = %v, want SHA rejection", err)
	}
	identity.APIURL = "https://attacker.example"
	if _, err := verifyGitHubEnvironmentApprovalWithClient(context.Background(), "release-approval", identity, client); err == nil {
		t.Fatal("untrusted API origin was accepted")
	}
}

func TestApprovedReviewersRequiresApprovedMatchingEnvironmentAndUser(t *testing.T) {
	var records []githubApprovalRecord
	if err := json.Unmarshal([]byte(`[
		{"state":"approved","user":{"login":"release-reviewer"},"environments":[{"name":"release-approval"}]},
		{"state":"rejected","user":{"login":"rejector"},"environments":[{"name":"release-approval"}]},
		{"state":"approved","user":{"login":"wrong-environment"},"environments":[{"name":"staging"}]},
		{"state":"approved","user":{},"environments":[{"name":"release-approval"}]}
	]`), &records); err != nil {
		t.Fatal(err)
	}
	got := approvedReviewers(records, "release-approval")
	if len(got) != 1 || got[0] != "release-reviewer" {
		t.Fatalf("approvedReviewers() = %v, want [release-reviewer]", got)
	}
}

func TestApprovedReviewersNoMatchingApproval(t *testing.T) {
	var records []githubApprovalRecord
	if err := json.Unmarshal([]byte(`[{"state":"approved","user":{"login":"reviewer"},"environments":[{"name":"other"}]}]`), &records); err != nil {
		t.Fatal(err)
	}
	if got := approvedReviewers(records, "release-approval"); len(got) != 0 {
		t.Fatalf("approvedReviewers() = %v, want no reviewers", got)
	}
}
