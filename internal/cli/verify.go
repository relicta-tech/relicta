// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/security/attestation"
)

var (
	verifyRunID       string
	verifyFile        string
	verifyAllowUnsig  bool
	verifyMaxRisk     float64
	verifyMinApproval int
	verifyPublicKey   string
)

var verifyCmd = &cobra.Command{
	Use:   "verify [flags]",
	Short: "Verify governance attestation for a release",
	Long: `Verify the governance attestation attached to a release.

This command validates the cryptographic signature and governance
constraints of an in-toto attestation generated during publish.

Examples:
  # Verify the attestation for a specific run
  relicta verify --run-id abc123

  # Verify an attestation file directly
  relicta verify --file .relicta/releases/abc123/attestation.intoto.jsonl

  # Verify allowing unsigned attestations
  relicta verify --run-id abc123 --allow-unsigned

  # Verify with governance constraints
  relicta verify --file att.jsonl --max-risk-score 0.5 --min-approvals 2`,
	RunE: runVerify,
}

func init() {
	verifyCmd.Flags().StringVar(&verifyRunID, "run-id", "", "release run ID to verify")
	verifyCmd.Flags().StringVar(&verifyFile, "file", "", "path to attestation file")
	verifyCmd.Flags().BoolVar(&verifyAllowUnsig, "allow-unsigned", false, "accept unsigned attestations")
	verifyCmd.Flags().Float64Var(&verifyMaxRisk, "max-risk-score", 0, "maximum allowed risk score (0 = no limit)")
	verifyCmd.Flags().IntVar(&verifyMinApproval, "min-approvals", 0, "minimum required approvals (0 = no minimum)")
	verifyCmd.Flags().StringVar(&verifyPublicKey, "public-key", "", "path to public key PEM file for signature verification")

	rootCmd.AddCommand(verifyCmd)
}

// VerifyOutput represents the verify command output.
type VerifyOutput struct {
	Valid       bool    `json:"valid"`
	Unsigned    bool    `json:"unsigned,omitempty"`
	SignedBy    string  `json:"signed_by,omitempty"`
	Version     string  `json:"version,omitempty"`
	Decision    string  `json:"decision,omitempty"`
	RiskScore   float64 `json:"risk_score,omitempty"`
	Approvals   int     `json:"approvals,omitempty"`
	Repository  string  `json:"repository,omitempty"`
	CommitSHA   string  `json:"commit_sha,omitempty"`
	File        string  `json:"file"`
	Governance  string  `json:"governance,omitempty"`
	GovError    string  `json:"governance_error,omitempty"`
	ErrorDetail string  `json:"error,omitempty"`

	// AuditChain is what the repository's governance audit chain says about this
	// attestation: whether the chain itself holds up, and whether it still confirms
	// the position the attestation was sealed over.
	//
	// The predicate has carried auditChainHash and auditEntryCount since attestations
	// shipped, and nothing ever checked them — they were also always empty, so there
	// was nothing to check. These fields are that check's answer.
	AuditChain         string `json:"audit_chain,omitempty"`
	AuditChainEntries  int    `json:"audit_chain_entries,omitempty"`
	AuditChainAnchor   string `json:"audit_chain_anchor,omitempty"`
	AuditChainAnchorAt int    `json:"audit_chain_anchored_at,omitempty"`
	AuditChainError    string `json:"audit_chain_error,omitempty"`
}

func runVerify(cmd *cobra.Command, _ []string) error {
	// Resolve attestation file path
	attPath, err := resolveAttestationPath()
	if err != nil {
		return err
	}

	// Load the attestation
	att, err := loadAttestation(attPath)
	if err != nil {
		return fmt.Errorf("failed to load attestation from %s: %w", attPath, err)
	}

	// Build verifier options
	var verifierOpts []attestation.VerifierOption
	if verifyAllowUnsig {
		verifierOpts = append(verifierOpts, attestation.WithAllowUnsigned(true))
	}
	if verifyPublicKey != "" {
		pubPEM, err := os.ReadFile(verifyPublicKey)
		if err != nil {
			return fmt.Errorf("failed to read public key file: %w", err)
		}
		verifierOpts = append(verifierOpts, attestation.WithPublicKey(pubPEM))
	}

	// Verify
	verifier := attestation.NewVerifier(verifierOpts...)
	result, err := verifier.Verify(cmd.Context(), att)

	output := &VerifyOutput{File: attPath}

	if err != nil {
		output.Valid = false
		output.ErrorDetail = err.Error()
		return outputVerifyResult(output)
	}

	output.Valid = result.Valid
	output.Unsigned = result.Unsigned
	output.SignedBy = result.SignedBy

	if result.Predicate != nil {
		output.Version = result.Predicate.Version
		output.Decision = result.Predicate.Decision
		output.RiskScore = result.Predicate.RiskScore
		output.Approvals = len(result.Predicate.Approvals)
		output.Repository = result.Predicate.Repository
		output.CommitSHA = result.Predicate.CommitSHA
	}

	// Check the attestation against the governance audit chain it names.
	//
	// After the signature check and before the governance constraints, because it is a
	// question about the same predicate the constraints read and it can invalidate the
	// answer: a risk score inside a chain that no longer verifies is a number, not
	// evidence.
	if result.Predicate != nil {
		verifyAuditChain(cmd.Context(), output,
			result.Predicate.AuditChainHash, result.Predicate.AuditEntryCount)
	}

	// Check governance constraints
	constraints := attestation.GovernanceConstraints{
		MaxRiskScore: verifyMaxRisk,
		MinApprovals: verifyMinApproval,
	}
	if constraints.MaxRiskScore > 0 || constraints.MinApprovals > 0 {
		if govErr := verifier.CheckGovernance(result, constraints); govErr != nil {
			output.Governance = "failed"
			output.GovError = govErr.Error()
		} else {
			output.Governance = "passed"
		}
	}

	return outputVerifyResult(output)
}

func resolveAttestationPath() (string, error) {
	if verifyFile != "" {
		return verifyFile, nil
	}

	if verifyRunID != "" {
		// Look in the standard release directory
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}
		path := filepath.Join(cwd, ".relicta", "releases", verifyRunID, "attestation.intoto.jsonl")
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("attestation not found at %s: %w", path, err)
		}
		return path, nil
	}

	// Try to find the latest attestation
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	releasesDir := filepath.Join(cwd, ".relicta", "releases")
	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		return "", fmt.Errorf("no releases found (specify --file or --run-id): %w", err)
	}

	// Walk in reverse to find the most recent run with an attestation
	for i := len(entries) - 1; i >= 0; i-- {
		if !entries[i].IsDir() {
			continue
		}
		path := filepath.Join(releasesDir, entries[i].Name(), "attestation.intoto.jsonl")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no attestation found in %s (specify --file or --run-id)", releasesDir)
}

func loadAttestation(path string) (*attestation.SignedAttestation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var att attestation.SignedAttestation
	if err := json.Unmarshal(data, &att); err != nil {
		return nil, fmt.Errorf("invalid attestation format: %w", err)
	}

	return &att, nil
}

func outputVerifyResult(output *VerifyOutput) error {
	if outputJSON {
		if err := outputVerifyJSON(output); err != nil {
			return err
		}
		// The audit chain findings fail the command in JSON mode too.
		//
		// This mode otherwise always exits 0 — even for an invalid attestation — which
		// is a pre-existing asymmetry left alone here. It is not extended to the chain:
		// a tampered governance record that exits 0 in CI is a check that runs and
		// cannot fail, which is the shape of the defect this whole change is about. The
		// object is written first, so a machine reader still gets audit_chain and
		// audit_chain_error alongside the non-zero status.
		return auditChainFailure(output)
	}
	return outputVerifyText(output)
}

// auditChainFailure reports the two audit chain findings that mean the governance record
// changed after the release was signed for.
//
// An unavailable chain is not one of them: it says the check did not happen, which is
// what verifying a downloaded attestation outside a repository always produces.
func auditChainFailure(output *VerifyOutput) error {
	switch {
	case output.AuditChain == string(chainBroken):
		return fmt.Errorf("audit chain verification failed: %s", output.AuditChainError)
	case output.AuditChain != "" && output.AuditChainAnchor == string(anchorMismatched):
		return fmt.Errorf("audit chain verification failed: %s", output.AuditChainError)
	default:
		return nil
	}
}

func outputVerifyJSON(output *VerifyOutput) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputVerifyText(output *VerifyOutput) error {
	printTitle("Attestation Verification")
	fmt.Println()

	fmt.Printf("  File: %s\n", output.File)
	fmt.Println()

	if output.ErrorDetail != "" {
		printError(fmt.Sprintf("Verification failed: %s", output.ErrorDetail))
		return fmt.Errorf("attestation verification failed")
	}

	if output.Valid {
		printSuccess("Attestation is valid")
	} else {
		printError("Attestation is invalid")
	}

	if output.Unsigned {
		printWarning("Attestation is unsigned")
	} else if output.SignedBy != "" {
		printInfo(fmt.Sprintf("Signed by: %s", output.SignedBy))
	}

	fmt.Println()

	if output.Version != "" {
		fmt.Printf("  Version:    %s\n", output.Version)
	}
	if output.Decision != "" {
		fmt.Printf("  Decision:   %s\n", output.Decision)
	}
	if output.RiskScore > 0 {
		fmt.Printf("  Risk Score: %.2f\n", output.RiskScore)
	}
	if output.Approvals > 0 {
		fmt.Printf("  Approvals:  %d\n", output.Approvals)
	}
	if output.Repository != "" {
		fmt.Printf("  Repository: %s\n", output.Repository)
	}
	if output.CommitSHA != "" {
		fmt.Printf("  Commit:     %s\n", output.CommitSHA)
	}

	if chainErr := outputVerifyAuditChain(output); chainErr != nil {
		return chainErr
	}

	if output.Governance != "" {
		fmt.Println()
		if output.Governance == "passed" {
			printSuccess("Governance constraints: passed")
		} else {
			printError(fmt.Sprintf("Governance constraints: %s", output.GovError))
			return fmt.Errorf("governance check failed")
		}
	}

	return nil
}

// verifyAuditChain fills in what the governance audit chain says about this attestation.
//
// Two separate questions, and the answer to each is recorded separately because they fail
// for different reasons and an operator fixes them differently. Does the repository's
// chain verify at all — is every entry still the entry it was? And does the chain still
// confirm the position this attestation was sealed over?
//
// A chain that does not verify makes the second question unanswerable, so it is reported
// as a mismatch rather than passed over: an attestation whose anchor cannot be confirmed
// is not an attestation whose anchor is fine.
func verifyAuditChain(ctx context.Context, output *VerifyOutput, hash string, count int) {
	report := readAuditChain(ctx)
	output.AuditChain = string(report.Status)
	output.AuditChainEntries = report.Entries

	anchor, detail := checkAuditChainAnchor(report, hash, count)
	output.AuditChainAnchor = string(anchor)
	if anchor == anchorMatched {
		// The index the attestation pinned, not the chain's current length: later
		// releases append more entries, so the two diverge immediately and only the
		// first says anything about this release.
		output.AuditChainAnchorAt = count - 1
	}

	switch {
	case report.Status == chainBroken:
		output.AuditChainError = report.Detail
	case anchor == anchorMismatched:
		output.AuditChainError = detail
	case report.Status == chainUnavailable:
		// Not an error field: `relicta verify` has to work on an attestation
		// downloaded from a release page, with no repository around it, and the
		// signature check is still worth something there. Reported as unavailable and
		// rendered as a warning.
		output.AuditChainError = report.Detail
	}
}

// outputVerifyAuditChain renders the audit chain findings and fails the command on the two
// that mean the governance record changed.
//
// A broken chain and a mismatched anchor are hard failures with a non-zero exit, because
// the entire value of a hash chain is that a break is not something a reader has to
// notice. An unavailable chain is a warning: it says the check did not happen, which is
// honest, and it does not claim the attestation is bad.
func outputVerifyAuditChain(output *VerifyOutput) error {
	if output.AuditChain == "" {
		return nil
	}

	fmt.Println()

	// Order matters: a chain that could not be read is reported as unavailable before
	// anything is said about the anchor, because an unconfirmed anchor is not a failed
	// one, and only the two cases below it mean the governance record changed.
	switch {
	case output.AuditChain == string(chainBroken):
		printError(fmt.Sprintf("Audit chain: INTEGRITY FAILURE — %s", output.AuditChainError))
		return auditChainFailure(output)

	case output.AuditChain == string(chainUnavailable):
		printWarning(fmt.Sprintf("Audit chain: not checked — %s", output.AuditChainError))

	case output.AuditChainAnchor == string(anchorMismatched):
		printError(fmt.Sprintf("Audit chain: anchor mismatch — %s", output.AuditChainError))
		return auditChainFailure(output)

	case output.AuditChainAnchor == string(anchorAbsent):
		printWarning("Audit chain: this attestation records no chain position, so the " +
			"governance events behind it are not anchored to anything")

	default:
		printSuccess(fmt.Sprintf(
			"Audit chain: verified, %d entries, this release anchored at entry %d",
			output.AuditChainEntries, output.AuditChainAnchorAt))
	}

	return nil
}
