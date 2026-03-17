package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var pluginCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new plugin from template",
	Long: `Create a new Relicta plugin project from a template.

This command scaffolds a complete plugin project with:
- Plugin implementation using the official SDK
- go.mod with correct dependencies
- Example configuration and hooks
- README with usage instructions

Examples:
  # Create a new plugin
  relicta plugin create my-notification

  # Create with specific hooks
  relicta plugin create my-plugin --hooks post-publish,on-success

  # Create in a specific directory
  relicta plugin create my-plugin --output ./plugins`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginCreate,
}

var (
	createHooks     []string
	createOutputDir string
	createAuthor    string
	createModule    string
)

func init() {
	pluginCmd.AddCommand(pluginCreateCmd)

	pluginCreateCmd.Flags().StringSliceVar(&createHooks, "hooks", []string{"post-publish"}, "Hooks the plugin responds to")
	pluginCreateCmd.Flags().StringVarP(&createOutputDir, "output", "o", ".", "Output directory for the plugin")
	pluginCreateCmd.Flags().StringVar(&createAuthor, "author", "", "Plugin author name")
	pluginCreateCmd.Flags().StringVar(&createModule, "module", "", "Go module path (default: github.com/yourname/relicta-plugin-<name>)")
}

func runPluginCreate(cmd *cobra.Command, args []string) error {
	pluginName := strings.ToLower(args[0])

	// Validate plugin name
	if !isValidPluginName(pluginName) {
		return fmt.Errorf("invalid plugin name %q: must contain only lowercase letters, numbers, and hyphens", pluginName)
	}

	// Determine output directory
	outputDir := filepath.Join(createOutputDir, pluginName)
	if createOutputDir != "." {
		outputDir = filepath.Join(createOutputDir, pluginName)
	}

	// Check if directory already exists
	if _, err := os.Stat(outputDir); err == nil {
		return fmt.Errorf("directory %q already exists", outputDir)
	}

	// Prepare template data
	data := pluginTemplateData{
		Name:        pluginName,
		NameTitle:   toTitle(pluginName),
		NamePascal:  toPascalCase(pluginName),
		Module:      createModule,
		Author:      createAuthor,
		Hooks:       createHooks,
		SDKVersion:  "v1.0.0",
		Description: fmt.Sprintf("A Relicta plugin for %s", pluginName),
	}

	if data.Module == "" {
		data.Module = fmt.Sprintf("github.com/yourname/relicta-plugin-%s", pluginName)
	}

	if data.Author == "" {
		data.Author = "Your Name"
	}

	// Create directory structure
	dirs := []string{
		outputDir,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil { // #nosec G301 -- plugin output needs exec
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Generate files
	files := map[string]string{
		"main.go":    pluginMainTemplate,
		"go.mod":     pluginGoModTemplate,
		"README.md":  pluginReadmeTemplate,
		".gitignore": pluginGitignoreTemplate,
	}

	for filename, tmplContent := range files {
		filePath := filepath.Join(outputDir, filename)
		if err := generateFile(filePath, tmplContent, data); err != nil {
			return fmt.Errorf("failed to generate %s: %w", filename, err)
		}
	}

	// Success message
	fmt.Println()
	printSuccess(fmt.Sprintf("Created plugin %q in %s", pluginName, outputDir))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. cd %s\n", outputDir)
	fmt.Println("  2. Update go.mod with your module path")
	fmt.Println("  3. Implement your plugin logic in main.go")
	fmt.Println("  4. go mod tidy")
	fmt.Println("  5. go build -o " + pluginName)
	fmt.Printf("  6. relicta plugin install ./%s\n", pluginName)
	fmt.Println()
	fmt.Println("Documentation: https://github.com/relicta-tech/relicta-plugin-sdk")

	return nil
}

type pluginTemplateData struct {
	Name        string
	NameTitle   string
	NamePascal  string
	Module      string
	Author      string
	Hooks       []string
	SDKVersion  string
	Description string
}

func isValidPluginName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		isLowercase := c >= 'a' && c <= 'z'
		isDigit := c >= '0' && c <= '9'
		isHyphen := c == '-'
		if !isLowercase && !isDigit && !isHyphen {
			return false
		}
	}
	// Can't start or end with hyphen
	if name[0] == '-' || name[len(name)-1] == '-' {
		return false
	}
	return true
}

func toTitle(s string) string {
	return cases.Title(language.English).String(strings.ReplaceAll(s, "-", " "))
}

func toPascalCase(s string) string {
	parts := strings.Split(s, "-")
	for i, part := range parts {
		parts[i] = cases.Title(language.English).String(part)
	}
	return strings.Join(parts, "")
}

func generateFile(path, tmplContent string, data pluginTemplateData) error {
	tmpl, err := template.New(filepath.Base(path)).Funcs(template.FuncMap{
		"join": strings.Join,
		"hookConst": func(hook string) string {
			// Convert hook name to constant name
			// e.g., "post-publish" -> "HookPostPublish"
			parts := strings.Split(hook, "-")
			for i, part := range parts {
				parts[i] = cases.Title(language.English).String(part)
			}
			return "Hook" + strings.Join(parts, "")
		},
	}).Parse(tmplContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	f, err := os.Create(path) // #nosec G304 -- path is constructed from validated plugin name
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

const pluginMainTemplate = `// Package main implements a Relicta plugin.
package main

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta-plugin-sdk/helpers"
	"github.com/relicta-tech/relicta-plugin-sdk/plugin"
)

func main() {
	plugin.Serve(&{{.NamePascal}}Plugin{})
}

// {{.NamePascal}}Plugin implements the Relicta plugin interface.
type {{.NamePascal}}Plugin struct{}

// GetInfo returns plugin metadata.
func (p *{{.NamePascal}}Plugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "{{.Name}}",
		Version:     "1.0.0",
		Description: "{{.Description}}",
		Author:      "{{.Author}}",
		Hooks: []plugin.Hook{
			{{- range .Hooks}}
			plugin.{{hookConst .}},
			{{- end}}
		},
		ConfigSchema: ` + "`" + `{
			"type": "object",
			"properties": {
				"api_key": {
					"type": "string",
					"description": "API key for authentication"
				},
				"enabled": {
					"type": "boolean",
					"description": "Enable or disable the plugin",
					"default": true
				}
			}
		}` + "`" + `,
	}
}

// Execute runs the plugin for a given hook.
func (p *{{.NamePascal}}Plugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	cfg := helpers.NewConfigParser(req.Config)

	// Check if plugin is enabled
	if !cfg.GetBool("enabled", true) {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Plugin disabled, skipping",
		}, nil
	}

	// Respect dry-run mode
	if req.DryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Would execute %s hook (dry-run)", req.Hook),
		}, nil
	}

	switch req.Hook {
	{{- range .Hooks}}
	case plugin.{{hookConst .}}:
		return p.handle{{hookConst .}}(ctx, req, cfg)
	{{- end}}
	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

{{range .Hooks}}
func (p *{{$.NamePascal}}Plugin) handle{{hookConst .}}(ctx context.Context, req plugin.ExecuteRequest, cfg *helpers.ConfigParser) (*plugin.ExecuteResponse, error) {
	// TODO: Implement your plugin logic here
	//
	// Available context:
	//   req.Context.Version          - New version (e.g., "1.2.3")
	//   req.Context.PreviousVersion  - Previous version
	//   req.Context.TagName          - Git tag (e.g., "v1.2.3")
	//   req.Context.ReleaseType      - Type: major, minor, patch
	//   req.Context.RepositoryOwner  - Repository owner
	//   req.Context.RepositoryName   - Repository name
	//   req.Context.Changelog        - Full changelog markdown
	//   req.Context.ReleaseNotes     - Public release notes

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Executed {{.}} hook for version %s", req.Context.Version),
		Outputs: map[string]any{
			"executed": true,
		},
	}, nil
}
{{end}}

// Validate validates the plugin configuration.
func (p *{{.NamePascal}}Plugin) Validate(ctx context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	v := helpers.NewValidationBuilder()

	// Add your validation rules here
	// Example: v.RequireString(config, "api_key", "MY_API_KEY")

	return v.Build(), nil
}
`

const pluginGoModTemplate = `module {{.Module}}

go 1.22

require github.com/relicta-tech/relicta-plugin-sdk {{.SDKVersion}}
`

const pluginReadmeTemplate = `# {{.NameTitle}} Plugin

{{.Description}}

## Installation

` + "```" + `bash
# Build the plugin
go build -o {{.Name}}

# Install it
relicta plugin install ./{{.Name}}

# Enable it
relicta plugin enable {{.Name}}
` + "```" + `

## Configuration

Add to your ` + "`" + `.relicta.yaml` + "`" + `:

` + "```" + `yaml
plugins:
  {{.Name}}:
    enabled: true
    # api_key: ${MY_API_KEY}
` + "```" + `

## Hooks

This plugin responds to the following hooks:
{{range .Hooks}}
- ` + "`" + `{{.}}` + "`" + `
{{- end}}

## Development

` + "```" + `bash
# Install dependencies
go mod tidy

# Build
go build -o {{.Name}}

# Test locally
relicta plugin install ./{{.Name}}
relicta publish --dry-run
` + "```" + `

## License

MIT
`

const pluginGitignoreTemplate = `# Binaries
{{.Name}}
*.exe
*.exe~
*.dll
*.so
*.dylib

# Test binary
*.test

# Output of the go coverage tool
*.out

# Go workspace file
go.work

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db
`
