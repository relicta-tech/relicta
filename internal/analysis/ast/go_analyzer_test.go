package ast

import (
	"context"
	"strings"
	"testing"
)

func TestGoAnalyzer_AddedExport(t *testing.T) {
	before := `package example

func helper() {}
`
	after := `package example

func helper() {}
func NewThing() {}
`

	analyzer := NewGoAnalyzer()
	result, err := analyzer.Analyze(context.Background(), []byte(before), []byte(after), "example.go")
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result == nil {
		t.Fatal("expected analysis result")
	}
	if len(result.AddedExports) != 1 || result.AddedExports[0] != "NewThing" {
		t.Errorf("AddedExports = %v, want NewThing", result.AddedExports)
	}
	if result.IsBreaking {
		t.Error("IsBreaking = true, want false")
	}
}

func TestGoAnalyzer_RemovedExport(t *testing.T) {
	before := `package example

func OldThing() {}
`
	after := `package example

func helper() {}
`

	analyzer := NewGoAnalyzer()
	result, err := analyzer.Analyze(context.Background(), []byte(before), []byte(after), "example.go")
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result == nil {
		t.Fatal("expected analysis result")
	}
	if len(result.RemovedExports) != 1 || result.RemovedExports[0] != "OldThing" {
		t.Errorf("RemovedExports = %v, want OldThing", result.RemovedExports)
	}
	if !result.IsBreaking {
		t.Error("IsBreaking = false, want true")
	}
	if !strings.Contains(strings.Join(result.BreakingReasons, " "), "removed") {
		t.Errorf("BreakingReasons = %v, want removal reason", result.BreakingReasons)
	}
}

func TestGoAnalyzer_ModifiedExport(t *testing.T) {
	before := `package example

func ChangeMe(a int) {}
`
	after := `package example

func ChangeMe(a string) {}
`

	analyzer := NewGoAnalyzer()
	result, err := analyzer.Analyze(context.Background(), []byte(before), []byte(after), "example.go")
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result == nil {
		t.Fatal("expected analysis result")
	}
	if len(result.ModifiedExports) != 1 || result.ModifiedExports[0] != "ChangeMe" {
		t.Errorf("ModifiedExports = %v, want ChangeMe", result.ModifiedExports)
	}
	if !result.IsBreaking {
		t.Error("IsBreaking = false, want true")
	}
}

func TestGoAnalyzer_SupportsFile(t *testing.T) {
	analyzer := NewGoAnalyzer()
	if analyzer.SupportsFile("example_test.go") {
		t.Error("SupportsFile returned true for test file")
	}
	if !analyzer.SupportsFile("example.go") {
		t.Error("SupportsFile returned false for .go file")
	}
	if analyzer.SupportsFile("example.ts") {
		t.Error("SupportsFile returned true for non-go file")
	}
}

func TestGoAnalyzer_EmptySource(t *testing.T) {
	analyzer := NewGoAnalyzer()
	result, err := analyzer.Analyze(context.Background(), []byte(""), []byte(""), "empty.go")
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result for empty source")
	}
}

func TestGoAnalyzer_InvalidSource(t *testing.T) {
	analyzer := NewGoAnalyzer()
	_, err := analyzer.Analyze(context.Background(), []byte("package main\nfunc("), []byte("package main"), "bad.go")
	if err == nil {
		t.Fatal("expected parse error for invalid source")
	}
}
