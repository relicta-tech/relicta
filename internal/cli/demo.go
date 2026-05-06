// Package cli provides the command-line interface for Relicta.
package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var (
	demoComposeFile string
	demoReset       bool
	demoDown        bool
)

func init() {
	demoCmd.Flags().StringVarP(&demoComposeFile, "file", "f", "docker-compose.demo.yml", "path to docker compose demo file")
	demoCmd.Flags().BoolVar(&demoReset, "reset", false, "reset demo data by running 'down -v' before startup")
	demoCmd.Flags().BoolVar(&demoDown, "down", false, "tear down demo environment")

	rootCmd.AddCommand(demoCmd)
}

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Manage the local Docker demo environment",
	Long: `Manage the local Docker demo environment used for product showcases.

Examples:
  relicta demo
  relicta demo --reset
  relicta demo --down
  relicta demo --file docker-compose.demo.yml`,
	RunE: runDemo,
}

func runDemo(cmd *cobra.Command, args []string) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker is required for demo environment: %w", err)
	}

	if demoDown {
		return runDockerCompose("down")
	}

	if demoReset {
		if err := runDockerCompose("down", "-v"); err != nil {
			return err
		}
	}

	if err := runDockerCompose("up", "-d"); err != nil {
		return err
	}

	printSuccess("Demo environment is up")
	printInfo("Next steps:")
	fmt.Println("  1) relicta plan --skip-cognitive")
	fmt.Println("  2) relicta notes")
	fmt.Println("  3) relicta demo --down")
	return nil
}

func runDockerCompose(args ...string) error {
	composeArgs := append([]string{"compose", "-f", demoComposeFile}, args...)
	compose := exec.Command("docker", composeArgs...) // #nosec G204 -- command and args are fixed/flag-controlled
	compose.Stdout = os.Stdout
	compose.Stderr = os.Stderr
	if err := compose.Run(); err != nil {
		return fmt.Errorf("docker %v failed: %w", composeArgs, err)
	}
	return nil
}
