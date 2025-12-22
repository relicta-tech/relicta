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

// TypeScriptAnalyzer implements export analysis for TypeScript/JavaScript files.
// It uses pattern matching rather than full AST parsing to avoid external dependencies.
type TypeScriptAnalyzer struct{}

// NewTypeScriptAnalyzer creates a new TypeScript/JavaScript analyzer.
func NewTypeScriptAnalyzer() *TypeScriptAnalyzer {
	return &TypeScriptAnalyzer{}
}

// SupportsFile returns true if the file is a TypeScript or JavaScript source file.
func (t *TypeScriptAnalyzer) SupportsFile(path string) bool {
	lower := strings.ToLower(path)
	// Exclude test files
	if strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.") {
		return false
	}
	if strings.Contains(lower, "__tests__") || strings.Contains(lower, "__mocks__") {
		return false
	}
	return strings.HasSuffix(lower, ".ts") ||
		strings.HasSuffix(lower, ".tsx") ||
		strings.HasSuffix(lower, ".js") ||
		strings.HasSuffix(lower, ".jsx") ||
		strings.HasSuffix(lower, ".mjs") ||
		strings.HasSuffix(lower, ".mts")
}

// Analyze compares before/after TypeScript/JavaScript code and returns analysis.
func (t *TypeScriptAnalyzer) Analyze(_ context.Context, before, after []byte, _ string) (*analysis.ASTAnalysis, error) {
	beforeExports := parseTypeScriptExports(before)
	afterExports := parseTypeScriptExports(after)

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
		result.BreakingReasons = append(result.BreakingReasons, "removed exported API")
	}
	if len(result.ModifiedExports) > 0 {
		result.IsBreaking = true
		result.BreakingReasons = append(result.BreakingReasons, "modified exported API signature")
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

// Export patterns for TypeScript/JavaScript
var (
	// export function name(...)
	tsExportFuncRe = regexp.MustCompile(`^\s*export\s+(?:async\s+)?function\s+(\w+)\s*(<[^>]*>)?\s*\(([^)]*)\)`)
	// export const/let/var name = ...
	tsExportVarRe = regexp.MustCompile(`^\s*export\s+(?:const|let|var)\s+(\w+)\s*(?::\s*([^=]+))?\s*=`)
	// export class name
	tsExportClassRe = regexp.MustCompile(`^\s*export\s+(?:abstract\s+)?class\s+(\w+)(?:\s*<[^>]*>)?(?:\s+extends\s+\w+)?(?:\s+implements\s+[^{]+)?`)
	// export interface name
	tsExportInterfaceRe = regexp.MustCompile(`^\s*export\s+interface\s+(\w+)(?:\s*<[^>]*>)?(?:\s+extends\s+[^{]+)?`)
	// export type name
	tsExportTypeRe = regexp.MustCompile(`^\s*export\s+type\s+(\w+)(?:\s*<[^>]*>)?\s*=`)
	// export enum name
	tsExportEnumRe = regexp.MustCompile(`^\s*export\s+(?:const\s+)?enum\s+(\w+)`)
	// export default (function/class/expression)
	tsExportDefaultRe = regexp.MustCompile(`^\s*export\s+default\s+(?:(?:async\s+)?function|class)?\s*(\w+)?`)
	// export { name, name2 as alias }
	tsReExportRe = regexp.MustCompile(`^\s*export\s*\{([^}]+)\}`)
)

// tsExportInfo holds export name and signature for comparison.
type tsExportInfo = string

func parseTypeScriptExports(src []byte) map[string]tsExportInfo {
	if len(bytes.TrimSpace(src)) == 0 {
		return map[string]tsExportInfo{}
	}

	exports := make(map[string]tsExportInfo)
	scanner := bufio.NewScanner(bytes.NewReader(src))

	for scanner.Scan() {
		line := scanner.Text()

		// Skip comments
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			continue
		}

		// Try each pattern
		if matches := tsExportFuncRe.FindStringSubmatch(line); matches != nil {
			name := matches[1]
			generics := matches[2]
			params := normalizeParams(matches[3])
			exports[name] = "function" + generics + "(" + params + ")"
			continue
		}

		if matches := tsExportVarRe.FindStringSubmatch(line); matches != nil {
			name := matches[1]
			typeAnnotation := strings.TrimSpace(matches[2])
			exports[name] = "const:" + typeAnnotation
			continue
		}

		if matches := tsExportClassRe.FindStringSubmatch(line); matches != nil {
			name := matches[1]
			exports[name] = "class"
			continue
		}

		if matches := tsExportInterfaceRe.FindStringSubmatch(line); matches != nil {
			name := matches[1]
			exports[name] = "interface"
			continue
		}

		if matches := tsExportTypeRe.FindStringSubmatch(line); matches != nil {
			name := matches[1]
			exports[name] = "type"
			continue
		}

		if matches := tsExportEnumRe.FindStringSubmatch(line); matches != nil {
			name := matches[1]
			exports[name] = "enum"
			continue
		}

		if matches := tsExportDefaultRe.FindStringSubmatch(line); matches != nil {
			name := "default"
			if matches[1] != "" {
				name = "default:" + matches[1]
			}
			exports[name] = "default"
			continue
		}

		if matches := tsReExportRe.FindStringSubmatch(line); matches != nil {
			// Parse re-exports: { name, name2 as alias }
			parts := strings.Split(matches[1], ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				// Handle "name as alias"
				if idx := strings.Index(part, " as "); idx > 0 {
					alias := strings.TrimSpace(part[idx+4:])
					exports[alias] = "reexport"
				} else {
					exports[part] = "reexport"
				}
			}
		}
	}

	return exports
}

// normalizeParams removes whitespace variations from parameter lists for comparison.
func normalizeParams(params string) string {
	// Remove extra whitespace
	params = strings.TrimSpace(params)
	// Normalize multiple spaces to single space
	for strings.Contains(params, "  ") {
		params = strings.ReplaceAll(params, "  ", " ")
	}
	return params
}
