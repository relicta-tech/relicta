package dsl

import "testing"

// TestASTNodeInterfaces verifies that all AST types implement the Node interface.
func TestASTNodeInterfaces(t *testing.T) {
	// All these types should satisfy the Node interface
	nodes := []Node{
		&PolicyFile{},
		&RuleNode{},
		&DefaultsNode{},
		&WhenBlock{},
		&ThenBlock{},
		&ActionNode{},
		&BinaryExpr{},
		&UnaryExpr{},
		&IdentifierExpr{},
		&LiteralExpr{},
		&CallExpr{},
		&ListExpr{},
	}

	for _, n := range nodes {
		// Call node() to ensure it doesn't panic and satisfies interface
		n.node()
	}
}

// TestASTExprInterfaces verifies that expression types implement the Expression interface.
func TestASTExprInterfaces(t *testing.T) {
	// All these types should satisfy the Expression interface
	exprs := []Expression{
		&BinaryExpr{},
		&UnaryExpr{},
		&IdentifierExpr{},
		&LiteralExpr{},
		&CallExpr{},
		&ListExpr{},
	}

	for _, e := range exprs {
		// Call expr() to ensure it doesn't panic and satisfies interface
		e.expr()
		e.node() // Expression embeds Node
	}
}

// TestASTNodeFields verifies AST node fields can be set and read.
func TestASTNodeFields(t *testing.T) {
	pf := &PolicyFile{
		Rules:    []*RuleNode{{Name: "test-rule"}},
		Defaults: &DefaultsNode{Settings: map[string]any{"key": "value"}},
	}
	if len(pf.Rules) != 1 {
		t.Errorf("Rules count = %d, want 1", len(pf.Rules))
	}
	if pf.Rules[0].Name != "test-rule" {
		t.Errorf("Rule name = %s, want test-rule", pf.Rules[0].Name)
	}

	rn := &RuleNode{
		Name:        "rule1",
		Priority:    100,
		Description: "Test rule",
		Line:        10,
		Column:      5,
	}
	if rn.Priority != 100 {
		t.Errorf("Priority = %d, want 100", rn.Priority)
	}

	wb := &WhenBlock{
		Condition: &LiteralExpr{Value: true},
		Line:      20,
	}
	if wb.Line != 20 {
		t.Errorf("Line = %d, want 20", wb.Line)
	}

	tb := &ThenBlock{
		Actions: []*ActionNode{{Name: "block"}},
	}
	if len(tb.Actions) != 1 {
		t.Errorf("Actions count = %d, want 1", len(tb.Actions))
	}

	an := &ActionNode{
		Name: "require_approval",
		Args: map[string]any{"count": 2},
	}
	if an.Args["count"] != 2 {
		t.Errorf("Args[count] = %v, want 2", an.Args["count"])
	}
}

// TestExpressionFields verifies expression node fields.
func TestExpressionFields(t *testing.T) {
	be := &BinaryExpr{
		Left:     &LiteralExpr{Value: 1},
		Operator: ">",
		Right:    &LiteralExpr{Value: 0},
		Line:     5,
		Column:   10,
	}
	if be.Operator != ">" {
		t.Errorf("Operator = %s, want >", be.Operator)
	}

	ue := &UnaryExpr{
		Operator: "not",
		Operand:  &LiteralExpr{Value: false},
	}
	if ue.Operator != "not" {
		t.Errorf("Operator = %s, want not", ue.Operator)
	}

	ie := &IdentifierExpr{Name: "risk.score"}
	if ie.Name != "risk.score" {
		t.Errorf("Name = %s, want risk.score", ie.Name)
	}

	le := &LiteralExpr{Value: "test"}
	if le.Value != "test" {
		t.Errorf("Value = %v, want test", le.Value)
	}

	ce := &CallExpr{
		Function: "contains",
		Args:     []Expression{&LiteralExpr{Value: "needle"}},
	}
	if ce.Function != "contains" {
		t.Errorf("Function = %s, want contains", ce.Function)
	}

	lex := &ListExpr{
		Elements: []Expression{&LiteralExpr{Value: 1}, &LiteralExpr{Value: 2}},
	}
	if len(lex.Elements) != 2 {
		t.Errorf("Elements count = %d, want 2", len(lex.Elements))
	}
}
