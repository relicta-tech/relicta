package dsl

// Node is the base interface for all AST nodes.
type Node interface {
	node()
}

// PolicyFile represents the root of a parsed DSL file.
type PolicyFile struct {
	Rules    []*RuleNode
	Defaults *DefaultsNode
}

func (p *PolicyFile) node() {}

// RuleNode represents a policy rule definition.
type RuleNode struct {
	Name        string
	Priority    int
	Description string
	Enabled     *bool // nil means default (true)
	When        *WhenBlock
	Then        *ThenBlock
	Line        int
	Column      int
}

func (r *RuleNode) node() {}

// DefaultsNode represents default policy settings.
type DefaultsNode struct {
	Settings map[string]any
	Line     int
	Column   int
}

func (d *DefaultsNode) node() {}

// WhenBlock contains conditions for rule activation.
type WhenBlock struct {
	Condition Expression
	Line      int
	Column    int
}

func (w *WhenBlock) node() {}

// ThenBlock contains actions to execute when rule matches.
type ThenBlock struct {
	Actions []*ActionNode
	Line    int
	Column  int
}

func (t *ThenBlock) node() {}

// ActionNode represents an action to execute.
type ActionNode struct {
	Name   string
	Args   map[string]any
	Line   int
	Column int
}

func (a *ActionNode) node() {}

// Expression is the interface for condition expressions.
type Expression interface {
	Node
	expr()
}

// BinaryExpr represents a binary operation (AND, OR, comparisons).
type BinaryExpr struct {
	Left     Expression
	Operator string
	Right    Expression
	Line     int
	Column   int
}

func (b *BinaryExpr) node() {}
func (b *BinaryExpr) expr() {}

// UnaryExpr represents a unary operation (NOT).
type UnaryExpr struct {
	Operator string
	Operand  Expression
	Line     int
	Column   int
}

func (u *UnaryExpr) node() {}
func (u *UnaryExpr) expr() {}

// IdentifierExpr represents a field reference (e.g., risk.score).
type IdentifierExpr struct {
	Name   string
	Line   int
	Column int
}

func (i *IdentifierExpr) node() {}
func (i *IdentifierExpr) expr() {}

// LiteralExpr represents a literal value.
type LiteralExpr struct {
	Value  any
	Line   int
	Column int
}

func (l *LiteralExpr) node() {}
func (l *LiteralExpr) expr() {}

// CallExpr represents a function call in conditions (e.g., contains, matches).
type CallExpr struct {
	Function string
	Args     []Expression
	Line     int
	Column   int
}

func (c *CallExpr) node() {}
func (c *CallExpr) expr() {}

// ListExpr represents a list of values for 'in' operator.
type ListExpr struct {
	Elements []Expression
	Line     int
	Column   int
}

func (l *ListExpr) node() {}
func (l *ListExpr) expr() {}
