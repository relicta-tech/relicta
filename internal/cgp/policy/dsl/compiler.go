package dsl

import (
	"fmt"
	"strings"

	"github.com/relicta-tech/relicta/internal/cgp/policy"
)

// Compiler converts AST to Policy objects.
type Compiler struct {
	file *PolicyFile
}

// NewCompiler creates a new compiler for the given AST.
func NewCompiler(file *PolicyFile) *Compiler {
	return &Compiler{file: file}
}

// Compile converts the AST to a Policy.
func (c *Compiler) Compile(name string) (*policy.Policy, error) {
	p := policy.NewPolicy(name)
	p.Description = "Policy compiled from DSL"

	// Compile defaults
	if c.file.Defaults != nil {
		defaults, err := c.compileDefaults(c.file.Defaults)
		if err != nil {
			return nil, fmt.Errorf("compiling defaults: %w", err)
		}
		p.Defaults = *defaults
	}

	// Compile rules
	for i, ruleNode := range c.file.Rules {
		rule, err := c.compileRule(ruleNode, i)
		if err != nil {
			return nil, fmt.Errorf("compiling rule %q at line %d: %w", ruleNode.Name, ruleNode.Line, err)
		}
		p.Rules = append(p.Rules, *rule)
	}

	return p, nil
}

func (c *Compiler) compileDefaults(node *DefaultsNode) (*policy.Defaults, error) {
	defaults := &policy.Defaults{
		Decision:          policy.DecisionRequireReview,
		RequiredApprovers: 1,
	}

	for key, value := range node.Settings {
		switch key {
		case "decision":
			if s, ok := value.(string); ok {
				defaults.Decision = s
			} else {
				return nil, fmt.Errorf("decision must be a string")
			}
		case "required_approvers", "requiredApprovers":
			switch v := value.(type) {
			case int64:
				defaults.RequiredApprovers = int(v)
			case float64:
				defaults.RequiredApprovers = int(v)
			default:
				return nil, fmt.Errorf("required_approvers must be a number")
			}
		case "allowed_actors", "allowedActors":
			// Parse list of allowed actors
			if s, ok := value.(string); ok {
				defaults.AllowedActors = strings.Split(s, ",")
				for i := range defaults.AllowedActors {
					defaults.AllowedActors[i] = strings.TrimSpace(defaults.AllowedActors[i])
				}
			}
		}
	}

	return defaults, nil
}

func (c *Compiler) compileRule(node *RuleNode, index int) (*policy.Rule, error) {
	rule := policy.NewRule(
		toRuleID(node.Name),
		node.Name,
	)
	rule.Priority = node.Priority
	rule.Description = node.Description

	if node.Enabled != nil {
		rule.Enabled = *node.Enabled
	}

	// Compile conditions from when block
	if node.When != nil && node.When.Condition != nil {
		conditions, err := c.compileConditions(node.When.Condition)
		if err != nil {
			return nil, fmt.Errorf("compiling conditions: %w", err)
		}
		rule.Conditions = conditions
	} else {
		// Rule with no conditions always matches
		rule.Conditions = []policy.Condition{{
			Field:    "_always",
			Operator: policy.OperatorEqual,
			Value:    true,
		}}
	}

	// Compile actions from then block
	if node.Then != nil {
		actions, err := c.compileActions(node.Then.Actions)
		if err != nil {
			return nil, fmt.Errorf("compiling actions: %w", err)
		}
		rule.Actions = actions
	}

	return rule, nil
}

func (c *Compiler) compileConditions(expr Expression) ([]policy.Condition, error) {
	switch e := expr.(type) {
	case *BinaryExpr:
		return c.compileBinaryCondition(e)
	case *UnaryExpr:
		return c.compileUnaryCondition(e)
	case *IdentifierExpr:
		// Bare identifier treated as truthy check
		return []policy.Condition{{
			Field:    e.Name,
			Operator: policy.OperatorEqual,
			Value:    true,
		}}, nil
	case *CallExpr:
		return c.compileCallCondition(e)
	default:
		return nil, fmt.Errorf("unsupported expression type: %T", expr)
	}
}

func (c *Compiler) compileBinaryCondition(expr *BinaryExpr) ([]policy.Condition, error) {
	switch expr.Operator {
	case "and":
		// AND combines conditions
		leftConds, err := c.compileConditions(expr.Left)
		if err != nil {
			return nil, err
		}
		rightConds, err := c.compileConditions(expr.Right)
		if err != nil {
			return nil, err
		}
		return append(leftConds, rightConds...), nil

	case "or":
		// OR requires special handling - we use a composite condition
		// The engine evaluates all conditions with AND, so OR needs special encoding
		// We encode as: _or_group with structured value containing the alternatives
		leftConds, err := c.compileConditions(expr.Left)
		if err != nil {
			return nil, err
		}
		rightConds, err := c.compileConditions(expr.Right)
		if err != nil {
			return nil, err
		}
		return []policy.Condition{{
			Field:    "_or",
			Operator: "or",
			Value: map[string]any{
				"left":  conditionsToValue(leftConds),
				"right": conditionsToValue(rightConds),
			},
		}}, nil

	case "eq", "ne", "gt", "lt", "gte", "lte":
		field, err := c.extractField(expr.Left)
		if err != nil {
			return nil, fmt.Errorf("left side of comparison must be a field: %w", err)
		}
		value, err := c.extractValue(expr.Right)
		if err != nil {
			return nil, fmt.Errorf("right side of comparison must be a value: %w", err)
		}
		return []policy.Condition{{
			Field:    field,
			Operator: expr.Operator,
			Value:    value,
		}}, nil

	case "in":
		field, err := c.extractField(expr.Left)
		if err != nil {
			return nil, fmt.Errorf("left side of 'in' must be a field: %w", err)
		}
		list, ok := expr.Right.(*ListExpr)
		if !ok {
			return nil, fmt.Errorf("right side of 'in' must be a list")
		}
		values := make([]any, len(list.Elements))
		for i, elem := range list.Elements {
			val, err := c.extractValue(elem)
			if err != nil {
				return nil, err
			}
			values[i] = val
		}
		return []policy.Condition{{
			Field:    field,
			Operator: policy.OperatorIn,
			Value:    values,
		}}, nil

	default:
		return nil, fmt.Errorf("unsupported binary operator: %s", expr.Operator)
	}
}

func (c *Compiler) compileUnaryCondition(expr *UnaryExpr) ([]policy.Condition, error) {
	if expr.Operator == "not" {
		innerConds, err := c.compileConditions(expr.Operand)
		if err != nil {
			return nil, err
		}
		// Encode NOT as a special condition
		return []policy.Condition{{
			Field:    "_not",
			Operator: "not",
			Value:    conditionsToValue(innerConds),
		}}, nil
	}
	return nil, fmt.Errorf("unsupported unary operator: %s", expr.Operator)
}

func (c *Compiler) compileCallCondition(expr *CallExpr) ([]policy.Condition, error) {
	if len(expr.Args) < 2 {
		return nil, fmt.Errorf("%s requires at least 2 arguments", expr.Function)
	}

	field, err := c.extractField(expr.Args[0])
	if err != nil {
		return nil, fmt.Errorf("first argument must be a field: %w", err)
	}

	value, err := c.extractValue(expr.Args[1])
	if err != nil {
		return nil, fmt.Errorf("second argument must be a value: %w", err)
	}

	switch expr.Function {
	case "contains":
		return []policy.Condition{{
			Field:    field,
			Operator: policy.OperatorContains,
			Value:    value,
		}}, nil
	case "matches":
		return []policy.Condition{{
			Field:    field,
			Operator: policy.OperatorMatches,
			Value:    value,
		}}, nil
	default:
		return nil, fmt.Errorf("unsupported function: %s", expr.Function)
	}
}

func (c *Compiler) compileActions(nodes []*ActionNode) ([]policy.Action, error) {
	actions := make([]policy.Action, 0, len(nodes))

	for _, node := range nodes {
		action, err := c.compileAction(node)
		if err != nil {
			return nil, fmt.Errorf("compiling action %q at line %d: %w", node.Name, node.Line, err)
		}
		actions = append(actions, *action)
	}

	return actions, nil
}

func (c *Compiler) compileAction(node *ActionNode) (*policy.Action, error) {
	// Map DSL action names to policy action types
	actionType := mapActionName(node.Name)

	// Validate action type
	switch actionType {
	case policy.ActionSetDecision, policy.ActionRequireApproval,
		policy.ActionAddReviewer, policy.ActionBlock,
		policy.ActionAddRationale, policy.ActionAddCondition:
		// Valid action
	default:
		return nil, fmt.Errorf("unknown action: %s", node.Name)
	}

	return &policy.Action{
		Type:   actionType,
		Params: node.Args,
	}, nil
}

func (c *Compiler) extractField(expr Expression) (string, error) {
	if ident, ok := expr.(*IdentifierExpr); ok {
		return ident.Name, nil
	}
	return "", fmt.Errorf("expected identifier, got %T", expr)
}

func (c *Compiler) extractValue(expr Expression) (any, error) {
	switch e := expr.(type) {
	case *LiteralExpr:
		return e.Value, nil
	case *IdentifierExpr:
		// Identifiers as values are treated as field references
		return map[string]string{"$ref": e.Name}, nil
	default:
		return nil, fmt.Errorf("expected value, got %T", expr)
	}
}

// Helper functions

func toRuleID(name string) string {
	// Convert rule name to ID: lowercase, replace spaces with underscores
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, "-", "_")
	return id
}

func mapActionName(name string) string {
	// Map DSL action names to policy action types
	switch name {
	case "set_decision", "setDecision":
		return policy.ActionSetDecision
	case "require_approval", "requireApproval":
		return policy.ActionRequireApproval
	case "add_reviewer", "addReviewer":
		return policy.ActionAddReviewer
	case "block":
		return policy.ActionBlock
	case "add_rationale", "addRationale":
		return policy.ActionAddRationale
	case "add_condition", "addCondition":
		return policy.ActionAddCondition
	default:
		return name
	}
}

func conditionsToValue(conds []policy.Condition) []map[string]any {
	result := make([]map[string]any, len(conds))
	for i, c := range conds {
		result[i] = map[string]any{
			"field":    c.Field,
			"operator": c.Operator,
			"value":    c.Value,
		}
	}
	return result
}

// Parse parses DSL source code and returns a Policy.
func Parse(source string, policyName string) (*policy.Policy, error) {
	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return nil, fmt.Errorf("tokenizing: %w", err)
	}

	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("parsing: %w", err)
	}

	compiler := NewCompiler(ast)
	pol, err := compiler.Compile(policyName)
	if err != nil {
		return nil, fmt.Errorf("compiling: %w", err)
	}

	return pol, nil
}

// ParseFile parses DSL from a file and returns a Policy.
func ParseFile(filename string, source string) (*policy.Policy, error) {
	return Parse(source, filename)
}
