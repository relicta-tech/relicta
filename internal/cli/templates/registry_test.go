package templates

import (
	"sort"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	if reg == nil {
		t.Fatal("NewRegistry() returned nil")
	}
}

func TestRegistry_All(t *testing.T) {
	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	all := reg.All()
	if len(all) == 0 {
		t.Error("All() returned empty list")
	}
}

func TestRegistry_Count(t *testing.T) {
	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	count := reg.Count()
	if count == 0 {
		t.Error("Count() returned 0")
	}
	if count != len(reg.All()) {
		t.Errorf("Count() = %d, All() len = %d", count, len(reg.All()))
	}
}

func TestRegistry_Names(t *testing.T) {
	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	names := reg.Names()
	if len(names) == 0 {
		t.Error("Names() returned empty list")
	}

	// Verify sorted
	if !sort.StringsAreSorted(names) {
		t.Error("Names() not sorted")
	}
}

func TestFilterByLanguage(t *testing.T) {
	filter := FilterByLanguage(LanguageGo)

	// Template with Go support
	goTemplate := &Template{
		SupportedLanguages: []Language{LanguageGo, LanguageNode},
	}
	if !filter(goTemplate) {
		t.Error("expected Go template to match")
	}

	// Template without Go support
	pyTemplate := &Template{
		SupportedLanguages: []Language{LanguagePython},
	}
	if filter(pyTemplate) {
		t.Error("expected Python-only template not to match")
	}

	// Template with no language restriction
	anyTemplate := &Template{}
	if !filter(anyTemplate) {
		t.Error("expected unrestricted template to match")
	}
}

func TestFilterByProjectType(t *testing.T) {
	filter := FilterByProjectType(ProjectTypeLibrary)

	libTemplate := &Template{
		SupportedProjectTypes: []ProjectType{ProjectTypeLibrary},
	}
	if !filter(libTemplate) {
		t.Error("expected library template to match")
	}

	appTemplate := &Template{
		SupportedProjectTypes: []ProjectType{ProjectTypeSaaS},
	}
	if filter(appTemplate) {
		t.Error("expected app-only template not to match")
	}

	anyTemplate := &Template{}
	if !filter(anyTemplate) {
		t.Error("expected unrestricted template to match")
	}
}

func TestFilterByTag(t *testing.T) {
	filter := FilterByTag("ci")

	tagged := &Template{Tags: []string{"ci", "release"}}
	if !filter(tagged) {
		t.Error("expected tagged template to match")
	}

	untagged := &Template{Tags: []string{"release"}}
	if filter(untagged) {
		t.Error("expected untagged template not to match")
	}

	empty := &Template{}
	if filter(empty) {
		t.Error("expected empty template not to match")
	}
}
