package vanta

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	if _, err := NewClient(ClientConfig{APIToken: "  "}); err == nil {
		t.Error("expected error when APIToken is whitespace")
	}
}

func TestNewClient_DefaultsBaseURL(t *testing.T) {
	c, err := NewClient(ClientConfig{APIToken: "tok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.cfg.BaseURL != DefaultBaseURL {
		t.Errorf("expected default base URL; got %q", c.cfg.BaseURL)
	}
	if c.cfg.UserAgent != DefaultUserAgent {
		t.Errorf("expected default user agent")
	}
	if c.httpClient == nil {
		t.Error("expected default http client")
	}
}

func TestPushEvidence_ValidatesPayload(t *testing.T) {
	c, _ := NewClient(ClientConfig{APIToken: "tok"})
	if _, err := c.PushEvidence(context.Background(), Evidence{}); err == nil {
		t.Error("expected validation error for empty evidence")
	}
}

func TestPushEvidence_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom-evidence" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("auth header: got %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("content-type: got %q", got)
		}

		body, _ := io.ReadAll(r.Body)
		var ev Evidence
		if err := json.Unmarshal(body, &ev); err != nil {
			t.Errorf("body not valid JSON: %v", err)
		}
		if ev.Title == "" {
			t.Errorf("title empty in body")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"vanta-ev-123"}`))
	}))
	defer srv.Close()

	c, _ := NewClient(ClientConfig{
		APIToken: "test-token",
		BaseURL:  srv.URL,
	})

	id, err := c.PushEvidence(context.Background(), Evidence{
		Title:            "Test",
		Description:      "test evidence",
		Type:             EvidenceTypeAuditLog,
		CollectedAt:      time.Now().UTC(),
		Source:           "relicta",
		SystemIdentifier: "acme/test",
		Data:             map[string]any{"k": "v"},
	})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if id != "vanta-ev-123" {
		t.Errorf("expected returned id; got %q", id)
	}
}

func TestPushEvidence_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	c, _ := NewClient(ClientConfig{APIToken: "tok", BaseURL: srv.URL})
	_, err := c.PushEvidence(context.Background(), Evidence{
		Title:            "Test",
		Type:             EvidenceTypeAuditLog,
		CollectedAt:      time.Now().UTC(),
		SystemIdentifier: "acme/test",
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("status: %d", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Body, "forbidden") {
		t.Errorf("expected body in error: %q", apiErr.Body)
	}
}

func TestPushBatch_StopsOnFirstError(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"server"}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"id-` + string(rune('0'+calls)) + `"}`))
	}))
	defer srv.Close()

	c, _ := NewClient(ClientConfig{APIToken: "tok", BaseURL: srv.URL})

	evidence := []Evidence{
		validEvidence("first"),
		validEvidence("second"),
		validEvidence("third"),
	}

	ids, err := c.PushBatch(context.Background(), evidence)
	if err == nil {
		t.Fatal("expected error on second call")
	}
	if len(ids) != 1 {
		t.Errorf("expected 1 partial success id, got %d (ids=%v)", len(ids), ids)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestEvidence_Validate(t *testing.T) {
	cases := []struct {
		name    string
		ev      Evidence
		wantErr bool
	}{
		{"valid", validEvidence("ok"), false},
		{"missing title", Evidence{Type: EvidenceTypeAuditLog, CollectedAt: time.Now(), SystemIdentifier: "x"}, true},
		{"missing type", Evidence{Title: "t", CollectedAt: time.Now(), SystemIdentifier: "x"}, true},
		{"missing collectedAt", Evidence{Title: "t", Type: EvidenceTypeAuditLog, SystemIdentifier: "x"}, true},
		{"missing systemId", Evidence{Title: "t", Type: EvidenceTypeAuditLog, CollectedAt: time.Now()}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.ev.Validate()
			if c.wantErr && err == nil {
				t.Errorf("expected error")
			}
			if !c.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestMapArticle12LogEntries(t *testing.T) {
	now := time.Now().UTC()
	report := &compliance.Article12Report{
		SystemIdentifier: "acme/payments",
		LogEntries: []compliance.Article12LogEntry{
			{
				EntryID:          "art12:1",
				EventTimestamp:   now,
				SystemIdentifier: "acme/payments",
				Version:          "1.0.0",
				Actor:            cgp.Actor{Kind: cgp.ActorKindAgent, ID: "claude-code-1"},
				OutputDecision:   "approved",
				RiskScore:        0.4,
				AuditChainHash:   "abc123",
			},
			{
				EntryID:          "art12:2",
				EventTimestamp:   now,
				SystemIdentifier: "acme/payments",
				Actor:            cgp.Actor{Kind: cgp.ActorKindHuman, ID: "alice@example.com"},
				OutputDecision:   "approval_required",
				Verifiers: []compliance.Verifier{
					{Kind: "human", ID: "bob@example.com"},
				},
			},
		},
	}

	out := MapArticle12LogEntries(report)
	if len(out) != 2 {
		t.Fatalf("expected 2 evidence records, got %d", len(out))
	}

	for i, ev := range out {
		if ev.Type != EvidenceTypeAuditLog {
			t.Errorf("ev[%d] type: got %q", i, ev.Type)
		}
		if ev.SystemIdentifier != "acme/payments" {
			t.Errorf("ev[%d] systemId: got %q", i, ev.SystemIdentifier)
		}
		if len(ev.Frameworks) == 0 {
			t.Errorf("ev[%d] frameworks empty", i)
		}
		// EU AI Act + SOC2 + ISO 27001 should all be present
		var foundAct, foundSoc, foundIso bool
		for _, fw := range ev.Frameworks {
			switch fw.Framework {
			case "EU-AI-Act":
				foundAct = true
			case "SOC2":
				foundSoc = true
			case "ISO-27001":
				foundIso = true
			}
		}
		if !foundAct || !foundSoc || !foundIso {
			t.Errorf("ev[%d] missing expected frameworks: act=%v soc=%v iso=%v", i, foundAct, foundSoc, foundIso)
		}
	}

	if out[0].IntegrityHash != "abc123" {
		t.Errorf("expected integrity hash to flow through, got %q", out[0].IntegrityHash)
	}
}

func TestMapArticle12LogEntries_NilReport(t *testing.T) {
	if got := MapArticle12LogEntries(nil); got != nil {
		t.Errorf("expected nil for nil report; got %v", got)
	}
}

func TestMapSOC2(t *testing.T) {
	report := &compliance.Report{
		Type:        compliance.ReportSOC2,
		Repository:  "acme/widgets",
		Period:      compliance.Period{Label: "2026-Q1"},
		GeneratedAt: time.Now().UTC(),
		SOC2: &compliance.SOC2Report{
			ChangeLog:        []compliance.ChangeLogEntry{{ID: "1"}},
			ApprovalEvidence: []compliance.ApprovalEvidence{{ReleaseID: "r1"}},
			RiskAssessments:  []compliance.RiskAssessment{{ReleaseID: "r1"}},
		},
	}

	out := MapSOC2(report)
	if len(out) != 3 {
		t.Fatalf("expected 3 evidence records, got %d", len(out))
	}

	types := map[EvidenceType]bool{}
	for _, ev := range out {
		types[ev.Type] = true
		if ev.Source != "relicta" {
			t.Errorf("source: got %q", ev.Source)
		}
		if len(ev.Frameworks) == 0 || ev.Frameworks[0].Framework != "SOC2" {
			t.Errorf("expected SOC2 framework on every record")
		}
	}

	for _, want := range []EvidenceType{EvidenceTypeChangeManagement, EvidenceTypeAccessReview, EvidenceTypeRiskAssessment} {
		if !types[want] {
			t.Errorf("missing evidence type %q", want)
		}
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

func TestDescribeEntry_Verifiers(t *testing.T) {
	one := describeEntry(compliance.Article12LogEntry{
		Actor:    cgp.Actor{Kind: cgp.ActorKindAgent, ID: "x"},
		OutputDecision: "approved",
		Verifiers: []compliance.Verifier{{Kind: "human", ID: "a"}},
	})
	if !strings.Contains(one, "1 verifier") {
		t.Errorf("expected '1 verifier' in description, got %q", one)
	}

	none := describeEntry(compliance.Article12LogEntry{
		Actor: cgp.Actor{Kind: cgp.ActorKindAgent, ID: "y"}, OutputDecision: "denied",
	})
	if !strings.Contains(none, "no verifiers") {
		t.Errorf("expected 'no verifiers' phrasing, got %q", none)
	}

	many := describeEntry(compliance.Article12LogEntry{
		Actor: cgp.Actor{Kind: cgp.ActorKindAgent, ID: "z"}, OutputDecision: "approved",
		Verifiers: []compliance.Verifier{{Kind: "human", ID: "a"}, {Kind: "human", ID: "b"}, {Kind: "human", ID: "c"}},
	})
	if !strings.Contains(many, "3 verifiers") {
		t.Errorf("expected '3 verifiers' in description, got %q", many)
	}
}

func validEvidence(title string) Evidence {
	return Evidence{
		Title:            title,
		Type:             EvidenceTypeAuditLog,
		CollectedAt:      time.Now().UTC(),
		SystemIdentifier: "acme/test",
		Source:           "relicta",
		Data:             map[string]any{"k": "v"},
	}
}
