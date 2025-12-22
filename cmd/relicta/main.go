// Package main is the entry point for the relicta CLI.
package main

import (
	"context"
	"fmt"
	"io"
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

var exitFunc = os.Exit

func main() {
	ctx := context.Background()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	exitCode := run(ctx, sigChan, cli.ExecuteContext, cli.Cleanup, os.Stderr, exitFunc)
	exitFunc(exitCode)
}

func run(ctx context.Context, sigChan <-chan os.Signal, execute func(context.Context) error, cleanup func(), stderr io.Writer, exitFn func(int)) int {
	// Set up context with graceful shutdown handling
	ctx, cancel := context.WithCancel(ctx)

	// Track completion for coordinated shutdown
	var wg sync.WaitGroup
	done := make(chan struct{})

	// Handle shutdown signals in a goroutine
	if sigChan != nil {
		go func() {
			sig := <-sigChan
			fmt.Fprintf(stderr, "\nReceived signal %v, initiating graceful shutdown...\n", sig)
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
				fmt.Fprintf(stderr, "\nShutdown timeout (%v) exceeded, forcing exit\n", shutdownTimeout)
				exitFn(1)
			case sig = <-sigChan:
				fmt.Fprintf(stderr, "\nReceived second signal %v, forcing exit\n", sig)
				exitFn(1)
			}
		}()
	}

	cli.SetVersionInfo(version, commit, date)

	// Run CLI in tracked goroutine for coordinated shutdown
	var exitCode int
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := execute(ctx); err != nil {
			// Check if it was a context cancellation (user interrupted)
			if ctx.Err() != nil {
				fmt.Fprintln(stderr, "Operation canceled")
				exitCode = 130 // Standard exit code for SIGINT
				return
			}
			// Print the error since SilenceErrors is enabled in cobra
			fmt.Fprintf(stderr, "Error: %v\n", err)
			exitCode = 1
		}
	}()

	// Wait for CLI to complete
	wg.Wait()

	// Signal completion and allow cleanup
	close(done)
	cancel() // Ensure context is canceled for cleanup

	// Cleanup CLI resources (e.g., log file handles)
	if cleanup != nil {
		cleanup()
	}

	return exitCode
}
