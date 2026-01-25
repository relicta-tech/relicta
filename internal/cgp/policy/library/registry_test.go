package library

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/cgp/policy"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r.templates == nil {
		t.Error("templates map not initialized")
	}
	if r.byCategory == nil {
		t.Error("byCategory map not initialized")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	template := &PolicyTemplate{
		ID:          "test-template",
		Name:        "Test Template",
		Description: "A test template",
		Category:    "test",
		Tags:        []string{"test"},
		Build: func(opts TemplateOptions) *policy.Policy {
			return policy.NewPolicy("test")
		},
	}

	err := r.Register(template)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Verify template is registered
	got, ok := r.Get("test-template")
	if !ok {
		t.Error("template not found after registration")
	}
	if got.Name != "Test Template" {
		t.Errorf("got name %s, want Test Template", got.Name)
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	r := NewRegistry()

	template := &PolicyTemplate{
		ID:   "dup-test",
		Name: "Duplicate Test",
		Build: func(opts TemplateOptions) *policy.Policy {
			return policy.NewPolicy("test")
		},
	}

	err := r.Register(template)
	if err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	err = r.Register(template)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestRegistry_RegisterValidation(t *testing.T) {
	r := NewRegistry()

	// Missing ID
	err := r.Register(&PolicyTemplate{
		Name: "No ID",
		Build: func(opts TemplateOptions) *policy.Policy {
			return policy.NewPolicy("test")
		},
	})
	if err == nil {
		t.Error("expected error for missing ID")
	}

	// Missing Build function
	err = r.Register(&PolicyTemplate{
		ID:   "no-build",
		Name: "No Build",
	})
	if err == nil {
		t.Error("expected error for missing Build function")
	}
}

func TestRegistry_ListByCategory(t *testing.T) {
	r := NewRegistry()

	r.Register(&PolicyTemplate{
		ID:       "cat-a-1",
		Category: "category-a",
		Build:    func(opts TemplateOptions) *policy.Policy { return policy.NewPolicy("a1") },
	})
	r.Register(&PolicyTemplate{
		ID:       "cat-a-2",
		Category: "category-a",
		Build:    func(opts TemplateOptions) *policy.Policy { return policy.NewPolicy("a2") },
	})
	r.Register(&PolicyTemplate{
		ID:       "cat-b-1",
		Category: "category-b",
		Build:    func(opts TemplateOptions) *policy.Policy { return policy.NewPolicy("b1") },
	})

	catA := r.ListByCategory("category-a")
	if len(catA) != 2 {
		t.Errorf("expected 2 templates in category-a, got %d", len(catA))
	}

	catB := r.ListByCategory("category-b")
	if len(catB) != 1 {
		t.Errorf("expected 1 template in category-b, got %d", len(catB))
	}

	catC := r.ListByCategory("category-c")
	if len(catC) != 0 {
		t.Errorf("expected 0 templates in category-c, got %d", len(catC))
	}
}

func TestRegistry_Build(t *testing.T) {
	r := NewRegistry()

	r.Register(&PolicyTemplate{
		ID:   "builder-test",
		Name: "Builder Test",
		Build: func(opts TemplateOptions) *policy.Policy {
			p := policy.NewPolicy(opts.PolicyName)
			if opts.PolicyName == "" {
				p = policy.NewPolicy("default-name")
			}
			return p
		},
	})

	// Build with custom name
	p, err := r.Build("builder-test", TemplateOptions{PolicyName: "custom-name"})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if p.Name != "custom-name" {
		t.Errorf("expected name 'custom-name', got '%s'", p.Name)
	}

	// Build with default
	p, err = r.Build("builder-test", TemplateOptions{})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if p.Name != "default-name" {
		t.Errorf("expected name 'default-name', got '%s'", p.Name)
	}

	// Build non-existent
	_, err = r.Build("non-existent", TemplateOptions{})
	if err == nil {
		t.Error("expected error for non-existent template")
	}
}

func TestDefaultRegistry_HasBuiltins(t *testing.T) {
	// Verify built-in templates are registered
	templates := DefaultRegistry.List()
	if len(templates) == 0 {
		t.Fatal("DefaultRegistry has no templates")
	}

	// Check for expected categories
	categories := DefaultRegistry.Categories()
	expectedCategories := []string{CategorySecurity, CategoryStability, CategorySpeed, CategoryEnterprise}

	categorySet := make(map[string]bool)
	for _, c := range categories {
		categorySet[c] = true
	}

	for _, expected := range expectedCategories {
		if !categorySet[expected] {
			t.Errorf("expected category %s not found", expected)
		}
	}
}

func TestDefaultTemplateOptions(t *testing.T) {
	opts := DefaultTemplateOptions()

	if opts.RiskThreshold == 0 {
		t.Error("RiskThreshold should have default value")
	}
	if opts.RequiredApprovers == 0 {
		t.Error("RequiredApprovers should have default value")
	}
	if len(opts.ProductionBranches) == 0 {
		t.Error("ProductionBranches should have default values")
	}
	if opts.MaxFilesWithoutReview == 0 {
		t.Error("MaxFilesWithoutReview should have default value")
	}
	if opts.MaxLinesWithoutReview == 0 {
		t.Error("MaxLinesWithoutReview should have default value")
	}
}
