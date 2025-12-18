package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

var pluginDevCmd = &cobra.Command{
	Use:   "dev [plugin-path]",
	Short: "Run a plugin in development mode",
	Long: `Run a plugin in development mode with optional file watching.

This command builds and installs a plugin from source for testing.
With --watch, it monitors for file changes and automatically rebuilds.

Examples:
  # Build and install plugin from current directory
  relicta plugin dev

  # Build and install plugin from specific path
  relicta plugin dev ./my-plugin

  # Watch for changes and auto-rebuild
  relicta plugin dev --watch

  # Specify output name
  relicta plugin dev --name my-custom-plugin`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPluginDev,
}

var (
	devWatch      bool
	devPluginName string
)

func init() {
	pluginCmd.AddCommand(pluginDevCmd)
	pluginDevCmd.Flags().BoolVarP(&devWatch, "watch", "w", false, "Watch for file changes and auto-rebuild")
	pluginDevCmd.Flags().StringVarP(&devPluginName, "name", "n", "", "Plugin name (defaults to directory name)")
}

func runPluginDev(cmd *cobra.Command, args []string) error {
	// Determine plugin path
	pluginPath := "."
	if len(args) > 0 {
		pluginPath = args[0]
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Verify it's a Go project
	goModPath := filepath.Join(absPath, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return fmt.Errorf("no go.mod found in %s - is this a Go plugin project?", absPath)
	}

	// Determine plugin name
	pluginName := devPluginName
	if pluginName == "" {
		pluginName = filepath.Base(absPath)
	}

	// Get destination path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	destPath := filepath.Join(homeDir, ".relicta", "plugins", pluginName)

	// Initial build
	if err := buildPlugin(absPath, destPath); err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Plugin %q built and installed to %s", pluginName, destPath))
	fmt.Println()
	fmt.Println("Enable with: relicta plugin enable", pluginName)
	fmt.Println("Test with:   relicta publish --dry-run")

	if !devWatch {
		return nil
	}

	// Watch mode
	fmt.Println()
	fmt.Println("Watching for changes... (press Ctrl+C to stop)")

	return watchAndRebuild(absPath, destPath, pluginName)
}

func buildPlugin(srcPath, destPath string) error {
	// Run go build
	cmd := exec.Command("go", "build", "-o", destPath, ".")
	cmd.Dir = srcPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Building plugin from %s...\n", srcPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Make executable
	if err := os.Chmod(destPath, 0o755); err != nil { // #nosec G302 -- plugins must be executable
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	return nil
}

func watchAndRebuild(srcPath, destPath, pluginName string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer func() { _ = watcher.Close() }()

	// Watch all .go files recursively
	err = filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip hidden directories and vendor
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to add watchers: %w", err)
	}

	// Handle interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Debounce rebuilds
	var lastBuild time.Time
	const debounceInterval = 500 * time.Millisecond

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Only rebuild on Go file changes
			if !strings.HasSuffix(event.Name, ".go") {
				continue
			}

			// Debounce
			if time.Since(lastBuild) < debounceInterval {
				continue
			}

			// Rebuild on write/create
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				lastBuild = time.Now()
				fmt.Printf("\n[%s] Change detected: %s\n", time.Now().Format("15:04:05"), filepath.Base(event.Name))
				if err := buildPlugin(srcPath, destPath); err != nil {
					printError(fmt.Sprintf("Build failed: %v", err))
				} else {
					printSuccess(fmt.Sprintf("Plugin %q rebuilt successfully", pluginName))
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "Watch error: %v\n", err)

		case <-sigCh:
			fmt.Println("\nStopping watch mode...")
			return nil

		case <-ctx.Done():
			return nil
		}
	}
}
