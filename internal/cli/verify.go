// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v5/internal/security/attestation"
)

var (
	verifyRunID       string
	verifyFile        string
	verifyAllowUnsig  bool
	verifyMaxRisk     float64
	verifyMinApproval int
	verifyPublicKey   string
	verifyTagName     string
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

  # Verify an unsigned attestation (signed attestations still require --public-key)
  relicta verify --run-id abc123 --allow-unsigned

  # Verify with governance constraints
  relicta verify --file att.jsonl --max-risk-score 0.5 --min-approvals 2

  # Verify both the attestation and a signed Git tag
  relicta verify --run-id abc123 --tag v1.2.3`,
	RunE: runVerify,
}

func init() {
	verifyCmd.Flags().StringVar(&verifyRunID, "run-id", "", "release run ID to verify")
	verifyCmd.Flags().StringVar(&verifyFile, "file", "", "path to attestation file")
	verifyCmd.Flags().BoolVar(&verifyAllowUnsig, "allow-unsigned", false, "accept attestations with no signature (signed attestations still require --public-key)")
	verifyCmd.Flags().Float64Var(&verifyMaxRisk, "max-risk-score", 0, "maximum allowed risk score (0 = no limit)")
	verifyCmd.Flags().IntVar(&verifyMinApproval, "min-approvals", 0, "minimum required approvals (0 = no minimum)")
	verifyCmd.Flags().StringVar(&verifyPublicKey, "public-key", "", "path to public key PEM file for signature verification")
	verifyCmd.Flags().StringVar(&verifyTagName, "tag", "", "Git tag name to verify with git verify-tag")

	rootCmd.AddCommand(verifyCmd)
}

// VerifyOutput represents the verify command output.
type VerifyOutput struct {
	Valid        bool                `json:"valid"`
	OverallValid bool                `json:"overall_valid"`
	Unsigned     bool                `json:"unsigned,omitempty"`
	SignedBy     string              `json:"signed_by,omitempty"`
	Version      string              `json:"version,omitempty"`
	Decision     string              `json:"decision,omitempty"`
	RiskScore    float64             `json:"risk_score,omitempty"`
	Approvals    int                 `json:"approvals,omitempty"`
	Repository   string              `json:"repository,omitempty"`
	CommitSHA    string              `json:"commit_sha,omitempty"`
	File         string              `json:"file"`
	Governance   string              `json:"governance,omitempty"`
	GovError     string              `json:"governance_error,omitempty"`
	ErrorDetail  string              `json:"error,omitempty"`
	TagSignature *TagSignatureOutput `json:"tag_signature,omitempty"`

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

// TagSignatureOutput reports Git's verification result for a release tag.
type TagSignatureOutput struct {
	Tag                string `json:"tag"`
	Valid              bool   `json:"valid"`
	TargetCommit       string `json:"target_commit,omitempty"`
	BindingChecked     bool   `json:"binding_checked,omitempty"`
	MatchesAttestation bool   `json:"matches_attestation"`
	Output             string `json:"output,omitempty"`
	Error              string `json:"error,omitempty"`
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
		if verifyTagName != "" {
			output.TagSignature = verifyGitTag(cmd.Context(), verifyTagName, "")
		}
		return outputVerifyResult(output)
	}

	output.Valid = result.Valid
	output.Unsigned = result.Unsigned
	output.SignedBy = result.SignedBy
	if verifyTagName != "" {
		expectedCommit := ""
		if result.Predicate != nil {
			expectedCommit = result.Predicate.CommitSHA
		}
		output.TagSignature = verifyGitTag(cmd.Context(), verifyTagName, expectedCommit)
	}

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
		output.OverallValid = verifyOverallValid(output)
		if err := outputVerifyJSON(output); err != nil {
			return err
		}
		// Write the structured result before returning a failure so CI and other
		// machine callers get both the details and a reliable non-zero status.
		if output.ErrorDetail != "" {
			return fmt.Errorf("attestation verification failed: %s", output.ErrorDetail)
		}
		if !output.Valid {
			return fmt.Errorf("attestation verification failed: attestation is invalid")
		}
		if err := auditChainFailure(output); err != nil {
			return err
		}
		if err := tagSignatureFailure(output); err != nil {
			return err
		}
		if output.Governance == "failed" {
			return fmt.Errorf("governance check failed: %s", output.GovError)
		}
		return nil
	}
	return outputVerifyText(output)
}

func verifyOverallValid(output *VerifyOutput) bool {
	return output != nil && output.Valid && output.ErrorDetail == "" &&
		auditChainFailure(output) == nil && tagSignatureFailure(output) == nil &&
		output.Governance != "failed"
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

	if output.TagSignature != nil {
		if output.TagSignature.Valid && (!output.TagSignature.BindingChecked || output.TagSignature.MatchesAttestation) {
			printSuccess(fmt.Sprintf("Git tag %s signature is valid", output.TagSignature.Tag))
			if output.TagSignature.BindingChecked {
				printSuccess("Git tag target matches the attested release commit")
			}
			if output.TagSignature.Output != "" {
				printInfo(output.TagSignature.Output)
			}
		} else {
			printError(fmt.Sprintf("Git tag %s verification failed: %s", output.TagSignature.Tag, output.TagSignature.Error))
			return tagSignatureFailure(output)
		}
	}
	if output.ErrorDetail != "" {
		printError(fmt.Sprintf("Verification failed: %s", output.ErrorDetail))
		return fmt.Errorf("attestation verification failed")
	}

	if output.Valid {
		printSuccess("Attestation is valid")
	} else {
		printError("Attestation is invalid")
		return fmt.Errorf("attestation verification failed: attestation is invalid")
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

func verifyGitTag(parent context.Context, name, expectedCommit string) *TagSignatureOutput {
	result := &TagSignatureOutput{Tag: name}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	cwd, err := os.Getwd()
	if err != nil {
		result.Error = fmt.Sprintf("resolve repository directory: %v", err)
		return result
	}
	cmd := exec.CommandContext(ctx, "git", "verify-tag", "--", name)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	result.Output = strings.TrimSpace(string(output))
	if err != nil {
		result.Error = err.Error()
		if result.Output != "" {
			result.Error += ": " + result.Output
		}
		return result
	}
	result.Valid = true
	tagRef := strings.TrimPrefix(name, "refs/tags/")
	targetCmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "--end-of-options", "refs/tags/"+tagRef+"^{}")
	targetCmd.Dir = cwd
	target, err := targetCmd.Output()
	if err != nil {
		result.Valid = false
		result.Error = fmt.Sprintf("could not resolve tag target: %v", err)
		return result
	}
	result.TargetCommit = strings.TrimSpace(string(target))
	if expectedCommit != "" {
		result.BindingChecked = true
		result.MatchesAttestation = strings.EqualFold(result.TargetCommit, expectedCommit)
		if !result.MatchesAttestation {
			result.Error = fmt.Sprintf("tag target %s does not match attested commit %s", result.TargetCommit, expectedCommit)
		}
	}
	return result
}

func tagSignatureFailure(output *VerifyOutput) error {
	if output.TagSignature != nil {
		if !output.TagSignature.Valid {
			return fmt.Errorf("git tag signature verification failed: %s", output.TagSignature.Error)
		}
		if output.TagSignature.BindingChecked && !output.TagSignature.MatchesAttestation {
			return fmt.Errorf("git tag is not bound to the attested release commit: %s", output.TagSignature.Error)
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
