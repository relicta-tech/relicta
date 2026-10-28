package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

var githubRepositoryRegexp = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// githubEnvironmentApproval is independently verified from GitHub's workflow
// run approval history; environment variables alone are never treated as proof.
type githubEnvironmentApproval struct {
	Environment string   `json:"environment"`
	Reviewers   []string `json:"reviewers"`
	RunURL      string   `json:"run_url"`
	HeadSHA     string   `json:"head_sha"`
	Ref         string   `json:"ref"`
}

type githubApprovalRecord struct {
	State string `json:"state"`
	User  struct {
		Login string `json:"login"`
	} `json:"user"`
	Environments []struct {
		Name string `json:"name"`
	} `json:"environments"`
}

type githubApprovalRequestContext struct {
	Token      string
	Repository string
	RunID      string
	SHA        string
	Ref        string
	APIURL     string
}

func approvedReviewers(records []githubApprovalRecord, environment string) []string {
	var reviewers []string
	for _, record := range records {
		if !strings.EqualFold(record.State, "approved") || record.User.Login == "" {
			continue
		}
		for _, approvedEnv := range record.Environments {
			if approvedEnv.Name == environment {
				reviewers = append(reviewers, record.User.Login)
				break
			}
		}
	}
	return reviewers
}

func verifyGitHubEnvironmentApproval(ctx context.Context, environment string) (githubEnvironmentApproval, error) {
	identity := githubApprovalRequestContext{
		Token: os.Getenv("GITHUB_TOKEN"), Repository: os.Getenv("GITHUB_REPOSITORY"),
		RunID: os.Getenv("GITHUB_RUN_ID"), SHA: os.Getenv("GITHUB_SHA"),
		Ref: os.Getenv("GITHUB_REF"), APIURL: os.Getenv("GITHUB_API_URL"),
	}
	return verifyGitHubEnvironmentApprovalWithClient(ctx, environment, identity, nil)
}

func verifyGitHubEnvironmentApprovalWithClient(ctx context.Context, environment string, identity githubApprovalRequestContext, client *http.Client) (githubEnvironmentApproval, error) {
	if identity.Token == "" || identity.Repository == "" || identity.RunID == "" || identity.SHA == "" || identity.Ref == "" || environment == "" {
		return githubEnvironmentApproval{}, fmt.Errorf("GitHub environment approval requires token, repository, run, SHA, ref, and environment")
	}
	if identity.APIURL == "" {
		identity.APIURL = "https://api.github.com"
	}
	parsed, err := url.Parse(identity.APIURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() != "api.github.com" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return githubEnvironmentApproval{}, fmt.Errorf("GitHub approval verification only supports the trusted api.github.com origin")
	}
	if !githubRepositoryRegexp.MatchString(identity.Repository) {
		return githubEnvironmentApproval{}, fmt.Errorf("invalid GitHub repository")
	}
	apiURL := strings.TrimRight(identity.APIURL, "/")
	if client == nil {
		client = &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Hostname() != "api.github.com" {
				return fmt.Errorf("refusing to forward GitHub token to a different host")
			}
			return http.ErrUseLastResponse
		}}
	}
	request := func(endpoint string, target any) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+identity.Token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("GitHub approval verification returned HTTP %d", resp.StatusCode)
		}
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return fmt.Errorf("decode GitHub approval response: %w", err)
		}
		return nil
	}
	var run struct {
		HeadSHA    string `json:"head_sha"`
		HeadBranch string `json:"head_branch"`
		Event      string `json:"event"`
		HTMLURL    string `json:"html_url"`
	}
	base := "/repos/" + identity.Repository + "/actions/runs/" + url.PathEscape(identity.RunID)
	if err := request(base, &run); err != nil {
		return githubEnvironmentApproval{}, fmt.Errorf("verify GitHub workflow run: %w", err)
	}
	if run.HeadSHA != identity.SHA {
		return githubEnvironmentApproval{}, fmt.Errorf("GitHub workflow run SHA does not match checked out SHA")
	}
	switch {
	case strings.HasPrefix(identity.Ref, "refs/tags/") && run.Event == "push":
		// GitHub's head_branch field is not consistently populated for tag runs.
		// The API-verified run SHA is matched against the actual tag commit by the
		// release workflow and Relicta's external publication gate.
	case strings.HasPrefix(identity.Ref, "refs/heads/") && (run.Event == "push" || run.Event == "workflow_dispatch") && run.HeadBranch == strings.TrimPrefix(identity.Ref, "refs/heads/"):
	default:
		return githubEnvironmentApproval{}, fmt.Errorf("GitHub workflow run ref/event does not authorize this release")
	}
	var records []githubApprovalRecord
	if err := request(base+"/approvals", &records); err != nil {
		return githubEnvironmentApproval{}, fmt.Errorf("verify GitHub environment reviewer: %w", err)
	}
	reviewers := approvedReviewers(records, environment)
	if len(reviewers) == 0 {
		return githubEnvironmentApproval{}, fmt.Errorf("GitHub has no approved reviewer record for environment %q in this workflow run", environment)
	}
	return githubEnvironmentApproval{Environment: environment, Reviewers: reviewers, RunURL: run.HTMLURL, HeadSHA: identity.SHA, Ref: identity.Ref}, nil
}
