// Package ast provides language-specific AST analysis for commits.
package ast

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"

	"github.com/relicta-tech/relicta/internal/analysis"
	"github.com/relicta-tech/relicta/internal/domain/changes"
)

// GoAnalyzer implements AST analysis for Go files.
type GoAnalyzer struct{}

// NewGoAnalyzer creates a new Go analyzer.
func NewGoAnalyzer() *GoAnalyzer {
	return &GoAnalyzer{}
}

// SupportsFile returns true if the file is a Go source file (excluding tests).
func (g *GoAnalyzer) SupportsFile(path string) bool {
	if !strings.HasSuffix(path, ".go") {
		return false
	}
	return !strings.HasSuffix(path, "_test.go")
}

// Analyze compares before/after Go code and returns AST analysis.
func (g *GoAnalyzer) Analyze(_ context.Context, before, after []byte, path string) (*analysis.ASTAnalysis, error) {
	beforeExports, err := parseGoExports(before, path)
	if err != nil {
		return nil, err
	}
	afterExports, err := parseGoExports(after, path)
	if err != nil {
		return nil, err
	}

	if len(beforeExports) == 0 && len(afterExports) == 0 {
		return nil, nil
	}

	result := &analysis.ASTAnalysis{}

	for name, beforeInfo := range beforeExports {
		afterInfo, ok := afterExports[name]
		if !ok {
			result.RemovedExports = append(result.RemovedExports, name)
			continue
		}
		if beforeInfo.signature != afterInfo.signature {
			result.ModifiedExports = append(result.ModifiedExports, name)
		}
	}

	for name := range afterExports {
		if _, ok := beforeExports[name]; !ok {
			result.AddedExports = append(result.AddedExports, name)
		}
	}

	if len(result.RemovedExports) > 0 {
		result.IsBreaking = true
		result.BreakingReasons = append(result.BreakingReasons, "removed exported API")
	}
	if len(result.ModifiedExports) > 0 {
		result.IsBreaking = true
		result.BreakingReasons = append(result.BreakingReasons, "modified exported API")
	}

	switch {
	case result.IsBreaking:
		result.SuggestedType = changes.CommitTypeFeat
		result.Confidence = 0.9
	case len(result.AddedExports) > 0:
		result.SuggestedType = changes.CommitTypeFeat
		result.Confidence = 0.85
	default:
		result.SuggestedType = changes.CommitTypeRefactor
		result.Confidence = 0.6
	}

	return result, nil
}

type exportInfo struct {
	signature string
}

func parseGoExports(src []byte, path string) (map[string]exportInfo, error) {
	if len(bytes.TrimSpace(src)) == 0 {
		return map[string]exportInfo{}, nil
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	exports := make(map[string]exportInfo)
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name == nil || !d.Name.IsExported() {
				continue
			}
			exports[d.Name.Name] = exportInfo{signature: formatFuncSignature(fset, d)}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if typeSpec.Name == nil || !typeSpec.Name.IsExported() {
						continue
					}
					exports[typeSpec.Name.Name] = exportInfo{signature: formatNode(fset, typeSpec.Type)}
				}
			}
		}
	}

	return exports, nil
}

func formatFuncSignature(fset *token.FileSet, decl *ast.FuncDecl) string {
	var b strings.Builder
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		b.WriteString("(")
		b.WriteString(formatNode(fset, decl.Recv.List[0].Type))
		b.WriteString(") ")
	}
	b.WriteString(formatNode(fset, decl.Type))
	return b.String()
}

func formatNode(fset *token.FileSet, node ast.Node) string {
	if node == nil {
		return ""
	}
	var buf bytes.Buffer
	_ = format.Node(&buf, fset, node)
	return buf.String()
}
