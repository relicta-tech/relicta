package drata

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/compliance"
)

func TestNewClient_RequiresAPIToken(t *testing.T) {
	if _, err := NewClient(ClientConfig{}); err == nil {
		t.Error("expected error when APIToken missing")
	}
}

func TestPushEvidence_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/evidence" {
			t.Errorf("path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("auth: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Drata-Workspace") != "ws-1" {
			t.Errorf("expected workspace header")
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"drata-1"}`))
	}))
	defer srv.Close()

	c, _ := NewClient(ClientConfig{APIToken: "tok", BaseURL: srv.URL, WorkspaceID: "ws-1"})

	id, err := c.PushEvidence(context.Background(), Evidence{
		Title:            "T",
		Type:             EvidenceTypeAuditLog,
		CollectedAt:      time.Now(),
		SystemIdentifier: "acme/x",
		Source:           "relicta",
		Data:             map[string]any{"k": "v"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "drata-1" {
		t.Errorf("id: %q", id)
	}
}

func TestPushEvidence_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauth"}`))
	}))
	defer srv.Close()

	c, _ := NewClient(ClientConfig{APIToken: "tok", BaseURL: srv.URL})
	_, err := c.PushEvidence(context.Background(), Evidence{
		Title:            "T",
		Type:             EvidenceTypeAuditLog,
		CollectedAt:      time.Now(),
		SystemIdentifier: "x",
	})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: %d", apiErr.StatusCode)
	}
}

func TestPushBatch_StopsOnError(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"drata-` + string(rune('0'+calls)) + `"}`))
	}))
	defer srv.Close()

	c, _ := NewClient(ClientConfig{APIToken: "tok", BaseURL: srv.URL})

	ev := Evidence{
		Title:            "T",
		Type:             EvidenceTypeAuditLog,
		CollectedAt:      time.Now(),
		SystemIdentifier: "x",
	}

	ids, err := c.PushBatch(context.Background(), []Evidence{ev, ev, ev})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(ids) != 1 {
		t.Errorf("ids: %v", ids)
	}
}

func TestEvidence_Validate(t *testing.T) {
	cases := map[string]Evidence{
		"missing title":       {Type: EvidenceTypeAuditLog, CollectedAt: time.Now(), SystemIdentifier: "x"},
		"missing type":        {Title: "T", CollectedAt: time.Now(), SystemIdentifier: "x"},
		"missing collectedAt": {Title: "T", Type: EvidenceTypeAuditLog, SystemIdentifier: "x"},
		"missing systemId":    {Title: "T", Type: EvidenceTypeAuditLog, CollectedAt: time.Now()},
	}
	for name, ev := range cases {
		t.Run(name, func(t *testing.T) {
			if err := ev.Validate(); err == nil {
				t.Errorf("expected validation error")
			}
		})
	}
}

func TestMapArticle12LogEntries(t *testing.T) {
	report := &compliance.Article12Report{
		LogEntries: []compliance.Article12LogEntry{
			{
				EntryID:          "art12:1",
				EventTimestamp:   time.Now(),
				SystemIdentifier: "acme/x",
				Version:          "1.0.0",
				Actor:            cgp.Actor{Kind: cgp.ActorKindAgent, ID: "claude-1"},
				OutputDecision:   "approved",
				RiskScore:        0.3,
				AuditChainHash:   "h1",
			},
		},
	}
	out := MapArticle12LogEntries(report)
	if len(out) != 1 {
		t.Fatalf("expected 1 evidence, got %d", len(out))
	}
	if out[0].Type != EvidenceTypeAuditLog {
		t.Errorf("type: %q", out[0].Type)
	}
	if out[0].IntegrityHash != "h1" {
		t.Errorf("expected integrity hash through-flow")
	}
	if !strings.Contains(strings.Join(frameworkNames(out[0].Frameworks), ","), "EU-AI-Act") {
		t.Errorf("missing EU-AI-Act framework")
	}
}

func TestMapArticle12LogEntries_NilReport(t *testing.T) {
	if got := MapArticle12LogEntries(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestMapSOC2(t *testing.T) {
	r := &compliance.Report{
		Type:        compliance.ReportSOC2,
		Repository:  "acme/x",
		Period:      compliance.Period{Label: "2026-Q1"},
		GeneratedAt: time.Now(),
		SOC2:        &compliance.SOC2Report{ChangeLog: []compliance.ChangeLogEntry{{ID: "1"}}},
	}
	out := MapSOC2(r)
	if len(out) != 3 {
		t.Errorf("expected 3 evidence, got %d", len(out))
	}
}

func TestMapSOC2_NilOrMissing(t *testing.T) {
	if got := MapSOC2(nil); got != nil {
		t.Errorf("nil report should return nil")
	}
	if got := MapSOC2(&compliance.Report{Type: compliance.ReportDORA}); got != nil {
		t.Errorf("non-soc2 report should return nil")
	}
}

func TestDescribeEntry_VerifierCount(t *testing.T) {
	cases := []struct {
		v    int
		want string
	}{
		{0, "no verifiers"},
		{1, "1 verifier"},
		{3, "3 verifiers"},
	}
	for _, c := range cases {
		entry := compliance.Article12LogEntry{
			Actor:          cgp.Actor{Kind: cgp.ActorKindAgent, ID: "x"},
			OutputDecision: "approved",
		}
		for i := 0; i < c.v; i++ {
			entry.Verifiers = append(entry.Verifiers, compliance.Verifier{Kind: "human", ID: "x"})
		}
		got := describeEntry(entry)
		if !strings.Contains(got, c.want) {
			t.Errorf("verifierCount=%d: got %q, want substring %q", c.v, got, c.want)
		}
	}
}

func frameworkNames(fws []FrameworkMapping) []string {
	out := make([]string, 0, len(fws))
	for _, f := range fws {
		out = append(out, f.Framework)
	}
	return out
}
