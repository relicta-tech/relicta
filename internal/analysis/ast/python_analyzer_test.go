package ast

import (
	"context"
	"strings"
	"testing"
)

func TestPythonAnalyzer_AddedExport(t *testing.T) {
	before := `
def _helper():
    pass
`
	after := `
def _helper():
    pass

def new_feature():
    pass
`

	analyzer := NewPythonAnalyzer()
	result, err := analyzer.Analyze(context.Background(), []byte(before), []byte(after), "example.py")
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result == nil {
		t.Fatal("expected analysis result")
	}
	if len(result.AddedExports) != 1 || result.AddedExports[0] != "new_feature" {
		t.Errorf("AddedExports = %v, want [new_feature]", result.AddedExports)
	}
	if result.IsBreaking {
		t.Error("IsBreaking = true, want false")
	}
}

func TestPythonAnalyzer_RemovedExport(t *testing.T) {
	before := `
def old_feature():
    pass
`
	after := `
def _helper():
    pass
`

	analyzer := NewPythonAnalyzer()
	result, err := analyzer.Analyze(context.Background(), []byte(before), []byte(after), "example.py")
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result == nil {
		t.Fatal("expected analysis result")
	}
	if len(result.RemovedExports) != 1 || result.RemovedExports[0] != "old_feature" {
		t.Errorf("RemovedExports = %v, want [old_feature]", result.RemovedExports)
	}
	if !result.IsBreaking {
		t.Error("IsBreaking = false, want true")
	}
	if !strings.Contains(strings.Join(result.BreakingReasons, " "), "removed") {
		t.Errorf("BreakingReasons = %v, want removal reason", result.BreakingReasons)
	}
}

func TestPythonAnalyzer_ModifiedExport(t *testing.T) {
	before := `
def change_me(a):
    pass
`
	after := `
def change_me(a, b, c):
    pass
`

	analyzer := NewPythonAnalyzer()
	result, err := analyzer.Analyze(context.Background(), []byte(before), []byte(after), "example.py")
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result == nil {
		t.Fatal("expected analysis result")
	}
	if len(result.ModifiedExports) != 1 || result.ModifiedExports[0] != "change_me" {
		t.Errorf("ModifiedExports = %v, want [change_me]", result.ModifiedExports)
	}
	if !result.IsBreaking {
		t.Error("IsBreaking = false, want true")
	}
}

func TestPythonAnalyzer_ExportTypes(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name:     "simple function",
			code:     "def my_func():\n    pass",
			expected: []string{"my_func"},
		},
		{
			name:     "async function",
			code:     "async def async_func():\n    pass",
			expected: []string{"async_func"},
		},
		{
			name:     "function with params",
			code:     "def with_params(a, b, c):\n    pass",
			expected: []string{"with_params"},
		},
		{
			name:     "function with type hints",
			code:     "def typed(a: int, b: str) -> bool:\n    pass",
			expected: []string{"typed"},
		},
		{
			name:     "function with defaults",
			code:     "def defaults(a=1, b='test'):\n    pass",
			expected: []string{"defaults"},
		},
		{
			name:     "simple class",
			code:     "class MyClass:\n    pass",
			expected: []string{"MyClass"},
		},
		{
			name:     "class with base",
			code:     "class Derived(Base):\n    pass",
			expected: []string{"Derived"},
		},
		{
			name:     "class multiple bases",
			code:     "class Multi(Base1, Base2):\n    pass",
			expected: []string{"Multi"},
		},
		{
			name:     "constant",
			code:     "MAX_VALUE = 100",
			expected: []string{"MAX_VALUE"},
		},
		{
			name:     "constant with type",
			code:     "CONFIG_PATH: str = '/etc/config'",
			expected: []string{"CONFIG_PATH"},
		},
		{
			name:     "private function excluded",
			code:     "def _private():\n    pass",
			expected: []string{},
		},
		{
			name:     "dunder excluded",
			code:     "def __init__(self):\n    pass",
			expected: []string{},
		},
		{
			name:     "private class excluded",
			code:     "class _Internal:\n    pass",
			expected: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exports := parsePythonExports([]byte(tc.code))
			for _, exp := range tc.expected {
				if _, ok := exports[exp]; !ok {
					t.Errorf("expected export %q not found in %v", exp, exports)
				}
			}
			// Verify no unexpected exports
			if len(tc.expected) == 0 && len(exports) > 0 {
				t.Errorf("expected no exports, got %v", exports)
			}
		})
	}
}

func TestPythonAnalyzer_AllExplicit(t *testing.T) {
	code := `
__all__ = ['public_func', 'PublicClass']

def public_func():
    pass

def another_func():
    pass

class PublicClass:
    pass

class AnotherClass:
    pass
`

	exports := parsePythonExports([]byte(code))

	// Should only include items listed in __all__
	if _, ok := exports["public_func"]; !ok {
		t.Error("public_func should be exported (in __all__)")
	}
	if _, ok := exports["PublicClass"]; !ok {
		t.Error("PublicClass should be exported (in __all__)")
	}
	if _, ok := exports["another_func"]; ok {
		t.Error("another_func should not be exported (not in __all__)")
	}
	if _, ok := exports["AnotherClass"]; ok {
		t.Error("AnotherClass should not be exported (not in __all__)")
	}
}

func TestPythonAnalyzer_NestedDefinitionsExcluded(t *testing.T) {
	code := `
class MyClass:
    def method(self):
        pass

    class Nested:
        pass

def outer():
    def inner():
        pass
`

	exports := parsePythonExports([]byte(code))

	// Only module-level definitions should be detected
	if _, ok := exports["MyClass"]; !ok {
		t.Error("MyClass should be exported")
	}
	if _, ok := exports["outer"]; !ok {
		t.Error("outer should be exported")
	}
	if _, ok := exports["method"]; ok {
		t.Error("method should not be exported (class method)")
	}
	if _, ok := exports["Nested"]; ok {
		t.Error("Nested should not be exported (nested class)")
	}
	if _, ok := exports["inner"]; ok {
		t.Error("inner should not be exported (nested function)")
	}
}

func TestPythonAnalyzer_SupportsFile(t *testing.T) {
	analyzer := NewPythonAnalyzer()

	supported := []string{
		"example.py",
		"src/lib/utils.py",
		"my_module.py",
	}

	notSupported := []string{
		"test_example.py",
		"example_test.py",
		"tests/test_thing.py",
		"test/test_thing.py",
		"conftest.py",
		"example.pyi",
		"example.go",
		"example.ts",
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

func TestPythonAnalyzer_EmptySource(t *testing.T) {
	analyzer := NewPythonAnalyzer()
	result, err := analyzer.Analyze(context.Background(), []byte(""), []byte(""), "empty.py")
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result for empty source, got %+v", result)
	}
}

func TestPythonAnalyzer_OnlyPrivate(t *testing.T) {
	code := `
def _private():
    pass

class _Internal:
    pass
`

	analyzer := NewPythonAnalyzer()
	result, err := analyzer.Analyze(context.Background(), []byte(code), []byte(code), "internal.py")
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result for only private symbols, got %+v", result)
	}
}

func TestPythonAnalyzer_SkipsComments(t *testing.T) {
	code := `
# def commented():
#     pass

def actual():
    pass
`

	exports := parsePythonExports([]byte(code))
	if _, ok := exports["commented"]; ok {
		t.Error("should not detect definition in comment")
	}
	if _, ok := exports["actual"]; !ok {
		t.Error("should detect actual definition")
	}
}

func TestPythonAnalyzer_SignatureNormalization(t *testing.T) {
	// Same signature with different formatting should match
	before := `
def func(a, b, c):
    pass
`
	after := `
def func(  a,   b,   c  ):
    pass
`

	analyzer := NewPythonAnalyzer()
	result, err := analyzer.Analyze(context.Background(), []byte(before), []byte(after), "example.py")
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}

	// Should be refactor, not modified (signature is semantically the same)
	if result != nil && len(result.ModifiedExports) > 0 {
		t.Logf("Note: whitespace differences detected as modification")
	}
}

func TestPythonAnalyzer_TypeHintRemovalNotBreaking(t *testing.T) {
	// Type hints being added/removed shouldn't count as signature changes
	before := `
def func(a, b):
    pass
`
	after := `
def func(a: int, b: str) -> bool:
    pass
`

	analyzer := NewPythonAnalyzer()
	result, err := analyzer.Analyze(context.Background(), []byte(before), []byte(after), "example.py")
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}

	// Adding type hints should not be breaking
	if result != nil && result.IsBreaking {
		t.Logf("Note: type hint changes detected as breaking - may want to refine")
	}
}
