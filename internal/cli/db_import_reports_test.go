package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v5/internal/application/releasehistory"
	"github.com/relicta-tech/relicta/v5/internal/config"
	"github.com/relicta-tech/relicta/v5/internal/container"
	"github.com/relicta-tech/relicta/v5/internal/domain/release/domain"
)

func captureImportReportOutput(t *testing.T, render func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()
	render()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = oldStdout
	var out bytes.Buffer
	if _, err := out.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestDBImportReportsDescribeDryRunAndCompletedMigration(t *testing.T) {
	into := container.ImportedInto{
		Backend: config.BackendSQLite, Location: "relicta.db",
		GovernanceLocation: "relicta.db", CGPLocation: "relicta.db",
		IdentityLocation: "relicta.db", AnalyticsLocation: "relicta.db",
	}
	report := releasehistory.Report{
		RepoRoot: "/work/project", Runs: 2, Created: 1, Replaced: 1,
		Latest: domain.RunID("release-2"), LatestTransferred: true,
		ForeignRoots: []domain.RunID{"/old/checkout/release-1"},
	}
	dry := report
	dry.DryRun = true
	dry.Latest = ""
	out := captureImportReportOutput(t, func() {
		printImportReport(dry, into)
		printImportReport(report, into)
		printImportReport(releasehistory.Report{RepoRoot: "/empty"}, into)
		printPartialImport(releasehistory.Report{RepoRoot: "/work/project", Runs: 3, Created: 1})
		printPartialImport(releasehistory.Report{Runs: 0})
	})
	for _, want := range []string{
		"Would import 2 release runs", "no current release pointer",
		"Nothing was written", "1 new, 1 replaced", "Current release pointer set to release-2",
		"different repository root", "No release history found", "1 of 3 runs were written",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("migration report omitted %q:\n%s", want, out)
		}
	}
	if got := runCount(1); got != "1 release run" {
		t.Errorf("runCount(1) = %q", got)
	}
	if got := runCount(4); got != "4 release runs" {
		t.Errorf("runCount(4) = %q", got)
	}
}

func TestDBImportGovernanceAndProtocolReports(t *testing.T) {
	into := container.ImportedInto{Backend: config.BackendPostgres, GovernanceLocation: "database", CGPLocation: "database"}
	out := captureImportReportOutput(t, func() {
		printGovernanceImportReport(releasehistory.GovernanceReport{}, into)
		printGovernanceImportReport(releasehistory.GovernanceReport{
			Releases: 2, Incidents: 1, Decisions: 3, Authorizations: 1,
			Repositories: []string{"/checkout", "github.com/acme/service"}, Deployments: 2, DryRun: true,
		}, into)
		printGovernanceImportReport(releasehistory.GovernanceReport{Releases: 1}, into)
		printPartialGovernanceImport(releasehistory.GovernanceReport{Releases: 1, Decisions: 2})
		printPartialGovernanceImport(releasehistory.GovernanceReport{})
		printCGPImportReport(releasehistory.CGPProtocolReport{}, into)
		printCGPImportReport(releasehistory.CGPProtocolReport{Proposals: 1, Decisions: 2, Authorizations: 1, DryRun: true}, into)
		printCGPImportReport(releasehistory.CGPProtocolReport{Proposals: 1}, into)
		printPartialCGPImport(releasehistory.CGPProtocolReport{Proposals: 1, Decisions: 1})
		printPartialCGPImport(releasehistory.CGPProtocolReport{})
	})
	for _, want := range []string{
		"No governance memory found", "Would import governance memory",
		"2 release record(s), 1 incident(s), 3 decision(s), 1 authorization(s)",
		"holds records under 2 repository identities", "2 deployment record(s) were not imported",
		"were written before the governance import stopped", "No CGP protocol proposals found",
		"Would import CGP protocol records", "Imported CGP protocol records",
		"rerunning `relicta db import` is safe",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("governance/protocol report omitted %q:\n%s", want, out)
		}
	}
	if got := governanceCounts(releasehistory.GovernanceReport{Releases: 1, Incidents: 2, Decisions: 3, Authorizations: 4}); got != "1 release record(s), 2 incident(s), 3 decision(s), 4 authorization(s)" {
		t.Errorf("governanceCounts() = %q", got)
	}
}

func TestDBImportAnalyticsAndIdentityReports(t *testing.T) {
	into := container.ImportedInto{Backend: config.BackendSQLite, IdentityLocation: "db", AnalyticsLocation: "db"}
	out := captureImportReportOutput(t, func() {
		printAnalyticsImportReport(releasehistory.AnalyticsReport{}, into)
		printAnalyticsImportReport(releasehistory.AnalyticsReport{Events: 5, DryRun: true}, into)
		printAnalyticsImportReport(releasehistory.AnalyticsReport{Events: 5, Imported: 3, Existing: 2}, into)
		printPartialAnalyticsImport(releasehistory.AnalyticsReport{Imported: 3})
		printPartialAnalyticsImport(releasehistory.AnalyticsReport{})
		printIdentityImportReport(releasehistory.IdentityRegistryReport{}, into)
		printIdentityImportReport(releasehistory.IdentityRegistryReport{Identities: 2, DryRun: true}, into)
		printIdentityImportReport(releasehistory.IdentityRegistryReport{Identities: 2}, into)
		printPartialIdentityImport(releasehistory.IdentityRegistryReport{Identities: 1})
		printPartialIdentityImport(releasehistory.IdentityRegistryReport{})
	})
	for _, want := range []string{
		"No analytics events found", "Would import 5 analytics event(s)",
		"Imported 5 analytics event(s)", "3 analytics event(s) were imported before",
		"Would import 2 actor identity grant(s)", "Imported 2 actor identity grant(s)",
		"1 actor identity grant(s) were written before import stopped",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("analytics/identity report omitted %q:\n%s", want, out)
		}
	}
}
