// Package cli provides the command-line interface for Relicta.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/security/attestation"
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
		return outputVerifyJSON(output)
	}
	return outputVerifyText(output)
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
