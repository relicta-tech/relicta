package ast

import (
	"context"
	"strings"
	"testing"
)

func TestTypeScriptAnalyzer_AddedExport(t *testing.T) {
	before := `
function helper() {}
`
	after := `
function helper() {}
export function newFeature() {}
`

	analyzer := NewTypeScriptAnalyzer()
	result, err := analyzer.Analyze(context.Background(), []byte(before), []byte(after), "example.ts")
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result == nil {
		t.Fatal("expected analysis result")
	}
	if len(result.AddedExports) != 1 || result.AddedExports[0] != "newFeature" {
		t.Errorf("AddedExports = %v, want [newFeature]", result.AddedExports)
	}
	if result.IsBreaking {
		t.Error("IsBreaking = true, want false")
	}
}

func TestTypeScriptAnalyzer_RemovedExport(t *testing.T) {
	before := `
export function oldFeature() {}
`
	after := `
function helper() {}
`

	analyzer := NewTypeScriptAnalyzer()
	result, err := analyzer.Analyze(context.Background(), []byte(before), []byte(after), "example.ts")
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result == nil {
		t.Fatal("expected analysis result")
	}
	if len(result.RemovedExports) != 1 || result.RemovedExports[0] != "oldFeature" {
		t.Errorf("RemovedExports = %v, want [oldFeature]", result.RemovedExports)
	}
	if !result.IsBreaking {
		t.Error("IsBreaking = false, want true")
	}
	if !strings.Contains(strings.Join(result.BreakingReasons, " "), "removed") {
		t.Errorf("BreakingReasons = %v, want removal reason", result.BreakingReasons)
	}
}

func TestTypeScriptAnalyzer_ModifiedExport(t *testing.T) {
	before := `
export function changeMe(a: number) {}
`
	after := `
export function changeMe(a: string, b: number) {}
`

	analyzer := NewTypeScriptAnalyzer()
	result, err := analyzer.Analyze(context.Background(), []byte(before), []byte(after), "example.ts")
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result == nil {
		t.Fatal("expected analysis result")
	}
	if len(result.ModifiedExports) != 1 || result.ModifiedExports[0] != "changeMe" {
		t.Errorf("ModifiedExports = %v, want [changeMe]", result.ModifiedExports)
	}
	if !result.IsBreaking {
		t.Error("IsBreaking = false, want true")
	}
}

func TestTypeScriptAnalyzer_ExportTypes(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name:     "export function",
			code:     "export function myFunc() {}",
			expected: []string{"myFunc"},
		},
		{
			name:     "export async function",
			code:     "export async function asyncFunc() {}",
			expected: []string{"asyncFunc"},
		},
		{
			name:     "export const",
			code:     "export const myConst = 42;",
			expected: []string{"myConst"},
		},
		{
			name:     "export const with type",
			code:     "export const typed: string = 'hello';",
			expected: []string{"typed"},
		},
		{
			name:     "export class",
			code:     "export class MyClass {}",
			expected: []string{"MyClass"},
		},
		{
			name:     "export abstract class",
			code:     "export abstract class AbstractClass {}",
			expected: []string{"AbstractClass"},
		},
		{
			name:     "export interface",
			code:     "export interface MyInterface {}",
			expected: []string{"MyInterface"},
		},
		{
			name:     "export type",
			code:     "export type MyType = string;",
			expected: []string{"MyType"},
		},
		{
			name:     "export enum",
			code:     "export enum Status { Active, Inactive }",
			expected: []string{"Status"},
		},
		{
			name:     "export const enum",
			code:     "export const enum Direction { Up, Down }",
			expected: []string{"Direction"},
		},
		{
			name:     "export default",
			code:     "export default function main() {}",
			expected: []string{"default:main"},
		},
		{
			name:     "re-export",
			code:     "export { foo, bar as baz };",
			expected: []string{"foo", "baz"},
		},
		{
			name:     "generic function",
			code:     "export function generic<T>(val: T): T {}",
			expected: []string{"generic"},
		},
		{
			name:     "generic interface",
			code:     "export interface Container<T> {}",
			expected: []string{"Container"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exports := parseTypeScriptExports([]byte(tc.code))
			for _, exp := range tc.expected {
				if _, ok := exports[exp]; !ok {
					t.Errorf("expected export %q not found in %v", exp, exports)
				}
			}
		})
	}
}

func TestTypeScriptAnalyzer_SupportsFile(t *testing.T) {
	analyzer := NewTypeScriptAnalyzer()

	supported := []string{
		"example.ts",
		"example.tsx",
		"example.js",
		"example.jsx",
		"example.mjs",
		"example.mts",
		"src/lib/utils.ts",
	}

	notSupported := []string{
		"example.test.ts",
		"example.spec.ts",
		"__tests__/example.ts",
		"__mocks__/example.ts",
		"example.go",
		"example.py",
	}

	for _, file := range supported {
		if !analyzer.SupportsFile(file) {
			t.Errorf("SupportsFile(%q) = false, want true", file)
		}
	}

	for _, file := range notSupported {
		if analyzer.SupportsFile(file) {
			t.Errorf("SupportsFile(%q) = true, want false", file)
		}
	}
}

func TestTypeScriptAnalyzer_EmptySource(t *testing.T) {
	analyzer := NewTypeScriptAnalyzer()
	result, err := analyzer.Analyze(context.Background(), []byte(""), []byte(""), "empty.ts")
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result for empty source, got %+v", result)
	}
}

func TestTypeScriptAnalyzer_NoExports(t *testing.T) {
	code := `
function internal() {}
const local = 42;
class Helper {}
`

	analyzer := NewTypeScriptAnalyzer()
	result, err := analyzer.Analyze(context.Background(), []byte(code), []byte(code), "internal.ts")
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result for no exports, got %+v", result)
	}
}

func TestTypeScriptAnalyzer_SkipsComments(t *testing.T) {
	code := `
// export function commented() {}
/* export function blockCommented() {} */
/**
 * export function docComment() {}
 */
export function actual() {}
`

	exports := parseTypeScriptExports([]byte(code))
	if _, ok := exports["commented"]; ok {
		t.Error("should not detect export in line comment")
	}
	if _, ok := exports["blockCommented"]; ok {
		t.Error("should not detect export in block comment")
	}
	if _, ok := exports["actual"]; !ok {
		t.Error("should detect actual export")
	}
}
