package dsl

import (
	"fmt"
	"strings"
)

// Parser parses DSL tokens into an AST.
type Parser struct {
	tokens  []Token
	pos     int
	current Token
}

// NewParser creates a new parser for the given tokens.
func NewParser(tokens []Token) *Parser {
	p := &Parser{
		tokens: tokens,
		pos:    0,
	}
	if len(tokens) > 0 {
		p.current = tokens[0]
	}
	return p
}

// Parse parses the tokens and returns a PolicyFile AST.
func (p *Parser) Parse() (*PolicyFile, error) {
	file := &PolicyFile{
		Rules: make([]*RuleNode, 0),
	}

	for p.current.Type != TokenEOF {
		// Skip comments
		if p.current.Type == TokenComment {
			p.advance()
			continue
		}

		switch p.current.Type {
		case TokenRule:
			rule, err := p.parseRule()
			if err != nil {
				return nil, err
			}
			file.Rules = append(file.Rules, rule)

		case TokenDefaults:
			if file.Defaults != nil {
				return nil, p.error("duplicate defaults block")
			}
			defaults, err := p.parseDefaults()
			if err != nil {
				return nil, err
			}
			file.Defaults = defaults

		default:
			return nil, p.error("expected 'rule' or 'defaults', got %s", p.current.Type)
		}
	}

	return file, nil
}

func (p *Parser) parseRule() (*RuleNode, error) {
	rule := &RuleNode{
		Line:   p.current.Line,
		Column: p.current.Column,
	}

	// Consume 'rule' keyword
	if err := p.expect(TokenRule); err != nil {
		return nil, err
	}

	// Rule name (string)
	if p.current.Type != TokenString {
		return nil, p.error("expected rule name string, got %s", p.current.Type)
	}
	rule.Name = p.current.Value
	p.advance()

	// Opening brace
	if err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	// Parse rule body
	for p.current.Type != TokenRBrace && p.current.Type != TokenEOF {
		// Skip comments
		if p.current.Type == TokenComment {
			p.advance()
			continue
		}

		switch p.current.Type {
		case TokenPriority:
			p.advance()
			if err := p.expect(TokenAssign); err != nil {
				return nil, err
			}
			if p.current.Type != TokenNumber {
				return nil, p.error("expected number for priority, got %s", p.current.Type)
			}
			if v, ok := p.current.Literal.(int64); ok {
				rule.Priority = int(v)
			} else if v, ok := p.current.Literal.(float64); ok {
				rule.Priority = int(v)
			}
			p.advance()

		case TokenDescription:
			p.advance()
			if err := p.expect(TokenAssign); err != nil {
				return nil, err
			}
			if p.current.Type != TokenString {
				return nil, p.error("expected string for description, got %s", p.current.Type)
			}
			rule.Description = p.current.Value
			p.advance()

		case TokenEnabled:
			p.advance()
			if err := p.expect(TokenAssign); err != nil {
				return nil, err
			}
			if p.current.Type != TokenBool {
				return nil, p.error("expected boolean for enabled, got %s", p.current.Type)
			}
			enabled := p.current.Literal.(bool)
			rule.Enabled = &enabled
			p.advance()

		case TokenWhen:
			when, err := p.parseWhenBlock()
			if err != nil {
				return nil, err
			}
			rule.When = when

		case TokenThen:
			then, err := p.parseThenBlock()
			if err != nil {
				return nil, err
			}
			rule.Then = then

		default:
			return nil, p.error("unexpected token in rule body: %s", p.current.Type)
		}
	}

	// Closing brace
	if err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}

	return rule, nil
}

func (p *Parser) parseDefaults() (*DefaultsNode, error) {
	defaults := &DefaultsNode{
		Settings: make(map[string]any),
		Line:     p.current.Line,
		Column:   p.current.Column,
	}

	// Consume 'defaults' keyword
	if err := p.expect(TokenDefaults); err != nil {
		return nil, err
	}

	// Opening brace
	if err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	// Parse settings
	for p.current.Type != TokenRBrace && p.current.Type != TokenEOF {
		// Skip comments
		if p.current.Type == TokenComment {
			p.advance()
			continue
		}

		if p.current.Type != TokenIdent {
			return nil, p.error("expected setting name, got %s", p.current.Type)
		}
		name := p.current.Value
		p.advance()

		if err := p.expect(TokenAssign); err != nil {
			return nil, err
		}

		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		defaults.Settings[name] = value
	}

	// Closing brace
	if err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}

	return defaults, nil
}

func (p *Parser) parseWhenBlock() (*WhenBlock, error) {
	when := &WhenBlock{
		Line:   p.current.Line,
		Column: p.current.Column,
	}

	// Consume 'when' keyword
	if err := p.expect(TokenWhen); err != nil {
		return nil, err
	}

	// Opening brace
	if err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	// Parse condition expression
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	when.Condition = expr

	// Closing brace
	if err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}

	return when, nil
}

func (p *Parser) parseThenBlock() (*ThenBlock, error) {
	then := &ThenBlock{
		Actions: make([]*ActionNode, 0),
		Line:    p.current.Line,
		Column:  p.current.Column,
	}

	// Consume 'then' keyword
	if err := p.expect(TokenThen); err != nil {
		return nil, err
	}

	// Opening brace
	if err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	// Parse actions
	for p.current.Type != TokenRBrace && p.current.Type != TokenEOF {
		// Skip comments
		if p.current.Type == TokenComment {
			p.advance()
			continue
		}

		action, err := p.parseAction()
		if err != nil {
			return nil, err
		}
		then.Actions = append(then.Actions, action)
	}

	// Closing brace
	if err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}

	return then, nil
}

func (p *Parser) parseAction() (*ActionNode, error) {
	action := &ActionNode{
		Args:   make(map[string]any),
		Line:   p.current.Line,
		Column: p.current.Column,
	}

	// Action name (identifier)
	if p.current.Type != TokenIdent {
		return nil, p.error("expected action name, got %s", p.current.Type)
	}
	action.Name = p.current.Value
	p.advance()

	// Optional arguments in parentheses
	if p.current.Type == TokenLParen {
		p.advance()

		for p.current.Type != TokenRParen && p.current.Type != TokenEOF {
			// Parse argument name
			if p.current.Type != TokenIdent {
				return nil, p.error("expected argument name, got %s", p.current.Type)
			}
			argName := p.current.Value
			p.advance()

			// Colon separator
			if err := p.expect(TokenColon); err != nil {
				return nil, err
			}

			// Argument value
			value, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			action.Args[argName] = value

			// Optional comma
			if p.current.Type == TokenComma {
				p.advance()
			}
		}

		if err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
	}

	return action, nil
}

func (p *Parser) parseValue() (any, error) {
	switch p.current.Type {
	case TokenString:
		value := p.current.Value
		p.advance()
		return value, nil
	case TokenNumber:
		value := p.current.Literal
		p.advance()
		return value, nil
	case TokenBool:
		value := p.current.Literal
		p.advance()
		return value, nil
	default:
		return nil, p.error("expected value, got %s", p.current.Type)
	}
}

// Expression parsing with operator precedence
func (p *Parser) parseExpression() (Expression, error) {
	return p.parseOrExpr()
}

func (p *Parser) parseOrExpr() (Expression, error) {
	left, err := p.parseAndExpr()
	if err != nil {
		return nil, err
	}

	for p.current.Type == TokenOr {
		op := p.current.Value
		line, col := p.current.Line, p.current.Column
		p.advance()

		right, err := p.parseAndExpr()
		if err != nil {
			return nil, err
		}

		left = &BinaryExpr{
			Left:     left,
			Operator: normalizeOperator(op),
			Right:    right,
			Line:     line,
			Column:   col,
		}
	}

	return left, nil
}

func (p *Parser) parseAndExpr() (Expression, error) {
	left, err := p.parseNotExpr()
	if err != nil {
		return nil, err
	}

	for p.current.Type == TokenAnd {
		op := p.current.Value
		line, col := p.current.Line, p.current.Column
		p.advance()

		right, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}

		left = &BinaryExpr{
			Left:     left,
			Operator: normalizeOperator(op),
			Right:    right,
			Line:     line,
			Column:   col,
		}
	}

	return left, nil
}

func (p *Parser) parseNotExpr() (Expression, error) {
	if p.current.Type == TokenNot {
		line, col := p.current.Line, p.current.Column
		p.advance()

		operand, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}

		return &UnaryExpr{
			Operator: "not",
			Operand:  operand,
			Line:     line,
			Column:   col,
		}, nil
	}

	return p.parseComparisonExpr()
}

func (p *Parser) parseComparisonExpr() (Expression, error) {
	left, err := p.parsePrimaryExpr()
	if err != nil {
		return nil, err
	}

	// Check for comparison operators
	switch p.current.Type {
	case TokenEq, TokenNe, TokenGt, TokenLt, TokenGte, TokenLte:
		op := p.current.Value
		line, col := p.current.Line, p.current.Column
		p.advance()

		right, err := p.parsePrimaryExpr()
		if err != nil {
			return nil, err
		}

		return &BinaryExpr{
			Left:     left,
			Operator: normalizeOperator(op),
			Right:    right,
			Line:     line,
			Column:   col,
		}, nil

	case TokenIn:
		line, col := p.current.Line, p.current.Column
		p.advance()

		// Parse list
		if p.current.Type != TokenLParen {
			return nil, p.error("expected '(' after 'in', got %s", p.current.Type)
		}
		p.advance()

		elements := make([]Expression, 0)
		for p.current.Type != TokenRParen && p.current.Type != TokenEOF {
			elem, err := p.parsePrimaryExpr()
			if err != nil {
				return nil, err
			}
			elements = append(elements, elem)

			if p.current.Type == TokenComma {
				p.advance()
			}
		}

		if err := p.expect(TokenRParen); err != nil {
			return nil, err
		}

		return &BinaryExpr{
			Left:     left,
			Operator: "in",
			Right: &ListExpr{
				Elements: elements,
				Line:     line,
				Column:   col,
			},
			Line:   line,
			Column: col,
		}, nil

	case TokenContains:
		line, col := p.current.Line, p.current.Column
		p.advance()

		right, err := p.parsePrimaryExpr()
		if err != nil {
			return nil, err
		}

		return &CallExpr{
			Function: "contains",
			Args:     []Expression{left, right},
			Line:     line,
			Column:   col,
		}, nil

	case TokenMatches:
		line, col := p.current.Line, p.current.Column
		p.advance()

		right, err := p.parsePrimaryExpr()
		if err != nil {
			return nil, err
		}

		return &CallExpr{
			Function: "matches",
			Args:     []Expression{left, right},
			Line:     line,
			Column:   col,
		}, nil
	}

	return left, nil
}

func (p *Parser) parsePrimaryExpr() (Expression, error) {
	switch p.current.Type {
	case TokenLParen:
		p.advance()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
		return expr, nil

	case TokenIdent:
		ident := &IdentifierExpr{
			Name:   p.current.Value,
			Line:   p.current.Line,
			Column: p.current.Column,
		}
		p.advance()
		return ident, nil

	case TokenString:
		lit := &LiteralExpr{
			Value:  p.current.Value,
			Line:   p.current.Line,
			Column: p.current.Column,
		}
		p.advance()
		return lit, nil

	case TokenNumber:
		lit := &LiteralExpr{
			Value:  p.current.Literal,
			Line:   p.current.Line,
			Column: p.current.Column,
		}
		p.advance()
		return lit, nil

	case TokenBool:
		lit := &LiteralExpr{
			Value:  p.current.Literal,
			Line:   p.current.Line,
			Column: p.current.Column,
		}
		p.advance()
		return lit, nil

	default:
		return nil, p.error("unexpected token in expression: %s", p.current.Type)
	}
}

func (p *Parser) advance() {
	p.pos++
	if p.pos < len(p.tokens) {
		p.current = p.tokens[p.pos]
	} else {
		p.current = Token{Type: TokenEOF}
	}
}

func (p *Parser) expect(t TokenType) error {
	if p.current.Type != t {
		return p.error("expected %s, got %s", t, p.current.Type)
	}
	p.advance()
	return nil
}

func (p *Parser) error(format string, args ...any) error {
	return fmt.Errorf("parse error at line %d, column %d: %s",
		p.current.Line, p.current.Column, fmt.Sprintf(format, args...))
}

// normalizeOperator converts DSL operators to policy engine operators.
func normalizeOperator(op string) string {
	switch strings.ToLower(op) {
	case "and", "&&":
		return "and"
	case "or", "||":
		return "or"
	case "==":
		return "eq"
	case "!=":
		return "ne"
	case ">":
		return "gt"
	case "<":
		return "lt"
	case ">=":
		return "gte"
	case "<=":
		return "lte"
	default:
		return op
	}
}
