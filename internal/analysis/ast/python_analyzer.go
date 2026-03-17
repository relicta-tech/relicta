// Package ast provides language-specific AST analysis for commits.
package ast

import (
	"bufio"
	"bytes"
	"context"
	"regexp"
	"strings"

	"github.com/relicta-tech/relicta/internal/analysis"
	"github.com/relicta-tech/relicta/internal/domain/changes"
)

// PythonAnalyzer implements export analysis for Python files.
// It uses pattern matching to detect public functions, classes, and variables.
// In Python, names not starting with underscore are considered public.
type PythonAnalyzer struct{}

// NewPythonAnalyzer creates a new Python analyzer.
func NewPythonAnalyzer() *PythonAnalyzer {
	return &PythonAnalyzer{}
}

// SupportsFile returns true if the file is a Python source file.
func (p *PythonAnalyzer) SupportsFile(path string) bool {
	lower := strings.ToLower(path)

	// Must be a .py file (not .pyi stub files)
	if !strings.HasSuffix(lower, ".py") || strings.HasSuffix(lower, ".pyi") {
		return false
	}

	// Exclude test files by name pattern
	base := lower
	if idx := strings.LastIndex(lower, "/"); idx >= 0 {
		base = lower[idx+1:]
	}
	if strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") {
		return false
	}
	if base == "conftest.py" {
		return false
	}

	// Exclude files in test directories
	if strings.Contains(lower, "/tests/") || strings.Contains(lower, "/test/") {
		return false
	}

	return true
}

// Analyze compares before/after Python code and returns analysis.
func (p *PythonAnalyzer) Analyze(_ context.Context, before, after []byte, _ string) (*analysis.ASTAnalysis, error) {
	beforeExports := parsePythonExports(before)
	afterExports := parsePythonExports(after)

	if len(beforeExports) == 0 && len(afterExports) == 0 {
		return nil, nil
	}

	result := &analysis.ASTAnalysis{}

	// Find removed and modified exports
	for name, beforeSig := range beforeExports {
		afterSig, exists := afterExports[name]
		if !exists {
			result.RemovedExports = append(result.RemovedExports, name)
			continue
		}
		if beforeSig != afterSig {
			result.ModifiedExports = append(result.ModifiedExports, name)
		}
	}

	// Find added exports
	for name := range afterExports {
		if _, exists := beforeExports[name]; !exists {
			result.AddedExports = append(result.AddedExports, name)
		}
	}

	// Determine breaking changes
	if len(result.RemovedExports) > 0 {
		result.IsBreaking = true
		result.BreakingReasons = append(result.BreakingReasons, "removed public API")
	}
	if len(result.ModifiedExports) > 0 {
		result.IsBreaking = true
		result.BreakingReasons = append(result.BreakingReasons, "modified public API signature")
	}

	// Infer commit type and confidence
	switch {
	case result.IsBreaking:
		result.SuggestedType = changes.CommitTypeFeat
		result.Confidence = 0.85
	case len(result.AddedExports) > 0:
		result.SuggestedType = changes.CommitTypeFeat
		result.Confidence = 0.80
	default:
		result.SuggestedType = changes.CommitTypeRefactor
		result.Confidence = 0.55
	}

	return result, nil
}

// Python patterns for public API detection
var (
	// def function_name(params) or async def function_name(params)
	// Must be at column 0 (module-level) to be considered public
	pyFuncRe = regexp.MustCompile(`^(async\s+)?def\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(([^)]*)\)`)
	// class ClassName or class ClassName(bases)
	// Must be at column 0 (module-level)
	pyClassRe = regexp.MustCompile(`^class\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:\(([^)]*)\))?`)
	// CONSTANT = value (uppercase names at module level)
	pyConstRe = regexp.MustCompile(`^([A-Z][A-Z0-9_]*)\s*(?::\s*[^=]+)?\s*=`)
	// __all__ = [...] - explicit exports
	pyAllRe = regexp.MustCompile(`^__all__\s*=\s*\[([^\]]*)\]`)
)

// pyExportInfo holds export name and signature for comparison.
type pyExportInfo = string

func parsePythonExports(src []byte) map[string]pyExportInfo {
	if len(bytes.TrimSpace(src)) == 0 {
		return map[string]pyExportInfo{}
	}

	exports := make(map[string]pyExportInfo)
	var explicitAll []string
	scanner := bufio.NewScanner(bytes.NewReader(src))

	for scanner.Scan() {
		line := scanner.Text()

		// Skip comments and empty lines
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check for __all__ definition
		if matches := pyAllRe.FindStringSubmatch(line); matches != nil {
			// Parse __all__ list
			content := matches[1]
			// Handle both 'name' and "name" quoted strings
			allRe := regexp.MustCompile(`['"]([^'"]+)['"]`)
			for _, m := range allRe.FindAllStringSubmatch(content, -1) {
				explicitAll = append(explicitAll, m[1])
			}
			continue
		}

		// Only consider module-level definitions (no leading whitespace)
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			continue
		}

		// Check for function definitions
		if matches := pyFuncRe.FindStringSubmatch(line); matches != nil {
			name := matches[2]
			// Skip private functions (starting with _)
			if strings.HasPrefix(name, "_") {
				continue
			}
			isAsync := matches[1] != ""
			params := normalizePyParams(matches[3])
			sig := "def(" + params + ")"
			if isAsync {
				sig = "async " + sig
			}
			exports[name] = sig
			continue
		}

		// Check for class definitions
		if matches := pyClassRe.FindStringSubmatch(line); matches != nil {
			name := matches[1]
			// Skip private classes (starting with _)
			if strings.HasPrefix(name, "_") {
				continue
			}
			bases := strings.TrimSpace(matches[2])
			sig := "class"
			if bases != "" {
				sig = "class(" + bases + ")"
			}
			exports[name] = sig
			continue
		}

		// Check for constants (UPPERCASE names)
		if matches := pyConstRe.FindStringSubmatch(line); matches != nil {
			name := matches[1]
			exports[name] = "const"
		}
	}

	// If __all__ is defined, filter exports to only those listed
	if len(explicitAll) > 0 {
		filtered := make(map[string]pyExportInfo)
		for _, name := range explicitAll {
			if sig, ok := exports[name]; ok {
				filtered[name] = sig
			} else {
				// Listed in __all__ but definition not found (might be imported)
				filtered[name] = "export"
			}
		}
		return filtered
	}

	return exports
}

// normalizePyParams removes whitespace variations and type hints for comparison.
func normalizePyParams(params string) string {
	params = strings.TrimSpace(params)

	// Remove type hints for simpler comparison (just keep param names)
	var simplified []string
	for _, param := range strings.Split(params, ",") {
		param = strings.TrimSpace(param)
		if param == "" {
			continue
		}
		// Remove type annotation
		if idx := strings.Index(param, ":"); idx > 0 {
			param = strings.TrimSpace(param[:idx])
		}
		// Remove default value
		if idx := strings.Index(param, "="); idx > 0 {
			param = strings.TrimSpace(param[:idx])
		}
		simplified = append(simplified, param)
	}

	return strings.Join(simplified, ", ")
}
