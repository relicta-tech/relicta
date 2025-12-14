// Package main is the entry point for the relicta CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/relicta-tech/relicta/internal/cli"
)

// Version information set by ldflags during build.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// shutdownTimeout is the maximum time to wait for graceful shutdown.
const shutdownTimeout = 30 * time.Second

func main() {
	// Set up context with graceful shutdown handling
	ctx, cancel := context.WithCancel(context.Background())

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Track completion for coordinated shutdown
	var wg sync.WaitGroup
	done := make(chan struct{})

	// Handle shutdown signals in a goroutine
	go func() {
		sig := <-sigChan
		fmt.Fprintf(os.Stderr, "\nReceived signal %v, initiating graceful shutdown...\n", sig)
		cancel()

		// Start shutdown timeout
		shutdownTimer := time.NewTimer(shutdownTimeout)
		defer shutdownTimer.Stop()

		// Wait for either: graceful completion, timeout, or second signal
		select {
		case <-done:
			// Graceful shutdown completed
			return
		case <-shutdownTimer.C:
			fmt.Fprintf(os.Stderr, "\nShutdown timeout (%v) exceeded, forcing exit\n", shutdownTimeout)
			os.Exit(1)
		case sig = <-sigChan:
			fmt.Fprintf(os.Stderr, "\nReceived second signal %v, forcing exit\n", sig)
			os.Exit(1)
		}
	}()

	cli.SetVersionInfo(version, commit, date)

	// Run CLI in tracked goroutine for coordinated shutdown
	var exitCode int
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := cli.ExecuteContext(ctx); err != nil {
			// Check if it was a context cancellation (user interrupted)
			if ctx.Err() != nil {
				fmt.Fprintln(os.Stderr, "Operation canceled")
				exitCode = 130 // Standard exit code for SIGINT
				return
			}
			// Print the error since SilenceErrors is enabled in cobra
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exitCode = 1
		}
	}()

	// Wait for CLI to complete
	wg.Wait()

	// Signal completion and allow cleanup
	close(done)
	cancel() // Ensure context is canceled for cleanup

	// Cleanup CLI resources (e.g., log file handles)
	cli.Cleanup()

	os.Exit(exitCode)
}
