package dsl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/relicta-tech/relicta/internal/cgp/policy"
)

// LoaderOptions configures policy loading behavior.
type LoaderOptions struct {
	// IgnoreErrors continues loading even if some files fail to parse.
	IgnoreErrors bool
	// Recursive searches subdirectories for policy files.
	Recursive bool
}

// LoadResult contains the outcome of loading policies.
type LoadResult struct {
	// Policies contains successfully loaded policies.
	Policies []*policy.Policy
	// Errors contains errors for files that failed to load.
	Errors []LoadError
}

// LoadError represents an error loading a specific policy file.
type LoadError struct {
	File  string
	Error error
}

// Loader loads policy files from the filesystem.
type Loader struct {
	opts LoaderOptions
}

// NewLoader creates a new policy loader with the given options.
func NewLoader(opts LoaderOptions) *Loader {
	return &Loader{opts: opts}
}

// LoadDir loads all policy files from a directory.
func (l *Loader) LoadDir(dir string) (*LoadResult, error) {
	result := &LoadResult{
		Policies: make([]*policy.Policy, 0),
		Errors:   make([]LoadError, 0),
	}

	// Check if directory exists
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		// Directory doesn't exist, return empty result
		return result, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	// Find policy files
	var files []string
	if l.opts.Recursive {
		err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && isPolicyFile(path) {
				files = append(files, path)
			}
			return nil
		})
	} else {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && isPolicyFile(entry.Name()) {
				files = append(files, filepath.Join(dir, entry.Name()))
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	// Load each file
	for _, file := range files {
		pol, err := l.LoadFile(file)
		if err != nil {
			result.Errors = append(result.Errors, LoadError{
				File:  file,
				Error: err,
			})
			if !l.opts.IgnoreErrors {
				return result, fmt.Errorf("failed to load %s: %w", file, err)
			}
			continue
		}
		result.Policies = append(result.Policies, pol)
	}

	return result, nil
}

// LoadFile loads a single policy file.
func (l *Loader) LoadFile(path string) (*policy.Policy, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Use filename (without extension) as policy name
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, filepath.Ext(name))

	pol, err := Parse(string(content), name)
	if err != nil {
		return nil, err
	}

	return pol, nil
}

// LoadString loads a policy from a string.
func (l *Loader) LoadString(source, name string) (*policy.Policy, error) {
	return Parse(source, name)
}

// isPolicyFile checks if a file is a policy file based on its extension.
func isPolicyFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".policy" || ext == ".cgp"
}

// ValidateDir validates all policy files in a directory without loading them.
func ValidateDir(dir string) ([]LoadError, error) {
	loader := NewLoader(LoaderOptions{IgnoreErrors: true})
	result, err := loader.LoadDir(dir)
	if err != nil {
		return nil, err
	}
	return result.Errors, nil
}

// ValidateFile validates a single policy file.
func ValidateFile(path string) error {
	loader := NewLoader(LoaderOptions{})
	_, err := loader.LoadFile(path)
	return err
}

// ValidateString validates a policy DSL string.
func ValidateString(source string) error {
	_, err := Parse(source, "validation")
	return err
}

// MustLoadDir loads policies from a directory or panics.
// This is useful for tests.
func MustLoadDir(dir string) []*policy.Policy {
	loader := NewLoader(LoaderOptions{})
	result, err := loader.LoadDir(dir)
	if err != nil {
		panic(fmt.Sprintf("failed to load policies from %s: %v", dir, err))
	}
	if len(result.Errors) > 0 {
		panic(fmt.Sprintf("failed to load some policies: %v", result.Errors))
	}
	return result.Policies
}

// MustLoadFile loads a single policy file or panics.
// This is useful for tests.
func MustLoadFile(path string) *policy.Policy {
	loader := NewLoader(LoaderOptions{})
	pol, err := loader.LoadFile(path)
	if err != nil {
		panic(fmt.Sprintf("failed to load policy from %s: %v", path, err))
	}
	return pol
}

// DefaultPolicyDir returns the default policy directory path.
func DefaultPolicyDir() string {
	return ".relicta/policies"
}

// DefaultPolicyPaths returns the list of paths to search for policies.
func DefaultPolicyPaths() []string {
	return []string{
		".relicta/policies",
		".github/relicta/policies",
		"policies",
	}
}
