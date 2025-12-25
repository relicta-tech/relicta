package dsl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/internal/cgp/policy"
)

func TestLexer_BasicTokens(t *testing.T) {
	input := `rule "test" { }`
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	require.NoError(t, err)

	expected := []TokenType{TokenRule, TokenString, TokenLBrace, TokenRBrace, TokenEOF}
	require.Len(t, tokens, len(expected))
	for i, exp := range expected {
		assert.Equal(t, exp, tokens[i].Type, "token %d", i)
	}
}

func TestLexer_Operators(t *testing.T) {
	tests := []struct {
		input    string
		expected TokenType
	}{
		{"==", TokenEq},
		{"!=", TokenNe},
		{">", TokenGt},
		{"<", TokenLt},
		{">=", TokenGte},
		{"<=", TokenLte},
		{"&&", TokenAnd},
		{"||", TokenOr},
		{"!", TokenNot},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			tok := lexer.NextToken()
			assert.Equal(t, tt.expected, tok.Type)
		})
	}
}

func TestLexer_Keywords(t *testing.T) {
	tests := []struct {
		input    string
		expected TokenType
	}{
		{"rule", TokenRule},
		{"when", TokenWhen},
		{"then", TokenThen},
		{"defaults", TokenDefaults},
		{"AND", TokenAnd},
		{"and", TokenAnd},
		{"OR", TokenOr},
		{"or", TokenOr},
		{"NOT", TokenNot},
		{"not", TokenNot},
		{"in", TokenIn},
		{"contains", TokenContains},
		{"matches", TokenMatches},
		{"true", TokenBool},
		{"false", TokenBool},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			tok := lexer.NextToken()
			assert.Equal(t, tt.expected, tok.Type)
		})
	}
}

func TestLexer_Numbers(t *testing.T) {
	tests := []struct {
		input   string
		literal any
	}{
		{"123", int64(123)},
		{"0", int64(0)},
		{"0.5", float64(0.5)},
		{"1.234", float64(1.234)},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			tok := lexer.NextToken()
			assert.Equal(t, TokenNumber, tok.Type)
			assert.Equal(t, tt.literal, tok.Literal)
		})
	}
}

func TestLexer_Strings(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello"`, "hello"},
		{`"hello world"`, "hello world"},
		{`"with\nnewline"`, "with\nnewline"},
		{`"with\ttab"`, "with\ttab"},
		{`"with\"quote"`, `with"quote`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			tok := lexer.NextToken()
			assert.Equal(t, TokenString, tok.Type)
			assert.Equal(t, tt.expected, tok.Value)
		})
	}
}

func TestLexer_Identifiers(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"risk", "risk"},
		{"risk.score", "risk.score"},
		{"actor.kind", "actor.kind"},
		{"change.breaking", "change.breaking"},
		{"_private", "_private"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			tok := lexer.NextToken()
			assert.Equal(t, TokenIdent, tok.Type)
			assert.Equal(t, tt.expected, tok.Value)
		})
	}
}

func TestLexer_Comments(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"hash comment", "# this is a comment\nrule"},
		{"double slash comment", "// this is a comment\nrule"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			tok := lexer.NextToken()
			assert.Equal(t, TokenComment, tok.Type)

			tok = lexer.NextToken()
			assert.Equal(t, TokenRule, tok.Type)
		})
	}
}

func TestLexer_CompleteRule(t *testing.T) {
	input := `
rule "high-risk-release" {
  priority = 100
  description = "Require approval for high-risk releases"

  when {
    risk.score > 0.7 AND actor.kind == "agent"
  }

  then {
    require_approval(count: 2)
    add_rationale(message: "High-risk agent release")
  }
}
`
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	require.NoError(t, err)

	// Verify no error tokens
	for _, tok := range tokens {
		if tok.Type == TokenError {
			t.Errorf("unexpected error token: %s at line %d, column %d", tok.Value, tok.Line, tok.Column)
		}
	}

	// Check that we have the expected structure
	assert.True(t, len(tokens) > 0)
	assert.Equal(t, TokenEOF, tokens[len(tokens)-1].Type)
}

func TestParser_SimpleRule(t *testing.T) {
	input := `
rule "test-rule" {
  priority = 50
  description = "A test rule"

  when {
    risk.score > 0.5
  }

  then {
    require_approval(count: 1)
  }
}
`
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	require.NoError(t, err)

	parser := NewParser(tokens)
	file, err := parser.Parse()
	require.NoError(t, err)

	require.Len(t, file.Rules, 1)
	rule := file.Rules[0]
	assert.Equal(t, "test-rule", rule.Name)
	assert.Equal(t, 50, rule.Priority)
	assert.Equal(t, "A test rule", rule.Description)
	assert.NotNil(t, rule.When)
	assert.NotNil(t, rule.Then)
	assert.Len(t, rule.Then.Actions, 1)
}

func TestParser_RuleWithAndCondition(t *testing.T) {
	input := `
rule "compound" {
  when {
    risk.score > 0.7 AND actor.kind == "agent"
  }
  then {
    block()
  }
}
`
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	require.NoError(t, err)

	parser := NewParser(tokens)
	file, err := parser.Parse()
	require.NoError(t, err)

	require.Len(t, file.Rules, 1)
	rule := file.Rules[0]
	assert.NotNil(t, rule.When)

	// Check that condition is a binary AND expression
	binExpr, ok := rule.When.Condition.(*BinaryExpr)
	require.True(t, ok, "expected BinaryExpr")
	assert.Equal(t, "and", binExpr.Operator)
}

func TestParser_RuleWithOrCondition(t *testing.T) {
	input := `
rule "or-test" {
  when {
    risk.score > 0.9 OR change.breaking == true
  }
  then {
    require_approval(count: 2)
  }
}
`
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	require.NoError(t, err)

	parser := NewParser(tokens)
	file, err := parser.Parse()
	require.NoError(t, err)

	require.Len(t, file.Rules, 1)
	binExpr, ok := file.Rules[0].When.Condition.(*BinaryExpr)
	require.True(t, ok)
	assert.Equal(t, "or", binExpr.Operator)
}

func TestParser_RuleWithInOperator(t *testing.T) {
	input := `
rule "in-test" {
  when {
    actor.kind in ("agent", "bot", "system")
  }
  then {
    require_approval(count: 1)
  }
}
`
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	require.NoError(t, err)

	parser := NewParser(tokens)
	file, err := parser.Parse()
	require.NoError(t, err)

	require.Len(t, file.Rules, 1)
	binExpr, ok := file.Rules[0].When.Condition.(*BinaryExpr)
	require.True(t, ok)
	assert.Equal(t, "in", binExpr.Operator)

	listExpr, ok := binExpr.Right.(*ListExpr)
	require.True(t, ok)
	assert.Len(t, listExpr.Elements, 3)
}

func TestParser_Defaults(t *testing.T) {
	input := `
defaults {
  decision = "approve"
  required_approvers = 1
}
`
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	require.NoError(t, err)

	parser := NewParser(tokens)
	file, err := parser.Parse()
	require.NoError(t, err)

	require.NotNil(t, file.Defaults)
	assert.Equal(t, "approve", file.Defaults.Settings["decision"])
}

func TestParser_MultipleRules(t *testing.T) {
	input := `
rule "first" {
  when { risk.score > 0.5 }
  then { require_approval(count: 1) }
}

rule "second" {
  when { risk.score > 0.8 }
  then { require_approval(count: 2) }
}

defaults {
  decision = "approve"
}
`
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	require.NoError(t, err)

	parser := NewParser(tokens)
	file, err := parser.Parse()
	require.NoError(t, err)

	assert.Len(t, file.Rules, 2)
	assert.NotNil(t, file.Defaults)
}

func TestCompiler_SimpleRule(t *testing.T) {
	input := `
rule "test" {
  priority = 100
  description = "Test rule"

  when {
    risk.score > 0.7
  }

  then {
    require_approval(count: 2)
  }
}
`
	pol, err := Parse(input, "test-policy")
	require.NoError(t, err)

	assert.Equal(t, "test-policy", pol.Name)
	require.Len(t, pol.Rules, 1)

	rule := pol.Rules[0]
	assert.Equal(t, "test", rule.ID)
	assert.Equal(t, "test", rule.Name)
	assert.Equal(t, 100, rule.Priority)
	assert.Equal(t, "Test rule", rule.Description)
	assert.True(t, rule.Enabled)

	// Check condition
	require.Len(t, rule.Conditions, 1)
	assert.Equal(t, "risk.score", rule.Conditions[0].Field)
	assert.Equal(t, "gt", rule.Conditions[0].Operator)
	assert.Equal(t, 0.7, rule.Conditions[0].Value)

	// Check action
	require.Len(t, rule.Actions, 1)
	assert.Equal(t, policy.ActionRequireApproval, rule.Actions[0].Type)
	assert.Equal(t, int64(2), rule.Actions[0].Params["count"])
}

func TestCompiler_AndCondition(t *testing.T) {
	input := `
rule "compound" {
  when {
    risk.score > 0.7 AND actor.kind == "agent"
  }
  then {
    block()
  }
}
`
	pol, err := Parse(input, "test")
	require.NoError(t, err)

	require.Len(t, pol.Rules, 1)
	rule := pol.Rules[0]

	// AND conditions are flattened
	require.Len(t, rule.Conditions, 2)
	assert.Equal(t, "risk.score", rule.Conditions[0].Field)
	assert.Equal(t, "actor.kind", rule.Conditions[1].Field)
}

func TestCompiler_OrCondition(t *testing.T) {
	input := `
rule "or-test" {
  when {
    risk.score > 0.9 OR change.breaking == true
  }
  then {
    require_approval(count: 2)
  }
}
`
	pol, err := Parse(input, "test")
	require.NoError(t, err)

	require.Len(t, pol.Rules, 1)
	rule := pol.Rules[0]

	// OR creates a special _or condition
	require.Len(t, rule.Conditions, 1)
	assert.Equal(t, "_or", rule.Conditions[0].Field)
	assert.Equal(t, "or", rule.Conditions[0].Operator)
}

func TestCompiler_InOperator(t *testing.T) {
	input := `
rule "in-test" {
  when {
    actor.kind in ("agent", "bot")
  }
  then {
    require_approval(count: 1)
  }
}
`
	pol, err := Parse(input, "test")
	require.NoError(t, err)

	require.Len(t, pol.Rules, 1)
	rule := pol.Rules[0]

	require.Len(t, rule.Conditions, 1)
	assert.Equal(t, "actor.kind", rule.Conditions[0].Field)
	assert.Equal(t, policy.OperatorIn, rule.Conditions[0].Operator)

	values, ok := rule.Conditions[0].Value.([]any)
	require.True(t, ok)
	assert.Len(t, values, 2)
}

func TestCompiler_ContainsOperator(t *testing.T) {
	input := `
rule "contains-test" {
  when {
    change.files contains "api/"
  }
  then {
    add_rationale(message: "API changes detected")
  }
}
`
	pol, err := Parse(input, "test")
	require.NoError(t, err)

	require.Len(t, pol.Rules, 1)
	rule := pol.Rules[0]

	require.Len(t, rule.Conditions, 1)
	assert.Equal(t, "change.files", rule.Conditions[0].Field)
	assert.Equal(t, policy.OperatorContains, rule.Conditions[0].Operator)
	assert.Equal(t, "api/", rule.Conditions[0].Value)
}

func TestCompiler_Defaults(t *testing.T) {
	input := `
defaults {
  decision = "approve"
  required_approvers = 2
}
`
	pol, err := Parse(input, "test")
	require.NoError(t, err)

	assert.Equal(t, "approve", pol.Defaults.Decision)
	assert.Equal(t, 2, pol.Defaults.RequiredApprovers)
}

func TestCompiler_MultipleActions(t *testing.T) {
	input := `
rule "multi-action" {
  when {
    risk.score > 0.8
  }
  then {
    require_approval(count: 2)
    add_rationale(message: "High risk requires review")
    add_reviewer(team: "security")
  }
}
`
	pol, err := Parse(input, "test")
	require.NoError(t, err)

	require.Len(t, pol.Rules, 1)
	rule := pol.Rules[0]

	require.Len(t, rule.Actions, 3)
	assert.Equal(t, policy.ActionRequireApproval, rule.Actions[0].Type)
	assert.Equal(t, policy.ActionAddRationale, rule.Actions[1].Type)
	assert.Equal(t, policy.ActionAddReviewer, rule.Actions[2].Type)
}

func TestCompiler_RuleWithEnabled(t *testing.T) {
	input := `
rule "disabled" {
  enabled = false
  when {
    risk.score > 0.5
  }
  then {
    block()
  }
}
`
	pol, err := Parse(input, "test")
	require.NoError(t, err)

	require.Len(t, pol.Rules, 1)
	assert.False(t, pol.Rules[0].Enabled)
}

func TestCompiler_ComplexPolicy(t *testing.T) {
	input := `
# Policy for AI-assisted releases
rule "high-risk-agent" {
  priority = 100
  description = "Block high-risk AI releases without approval"

  when {
    risk.score > 0.8 AND actor.kind == "agent"
  }

  then {
    require_approval(count: 2)
    add_reviewer(team: "security")
    add_rationale(message: "High-risk AI release requires security review")
  }
}

rule "breaking-changes" {
  priority = 90
  description = "Require approval for breaking changes"

  when {
    change.breaking == true
  }

  then {
    require_approval(count: 1)
    add_rationale(message: "Breaking changes require approval")
  }
}

// Default behavior
defaults {
  decision = "approve"
  required_approvers = 1
}
`
	pol, err := Parse(input, "ai-release-policy")
	require.NoError(t, err)

	assert.Equal(t, "ai-release-policy", pol.Name)
	require.Len(t, pol.Rules, 2)

	// First rule (ID has underscores from name conversion)
	assert.Equal(t, "high_risk_agent", pol.Rules[0].ID)
	assert.Equal(t, 100, pol.Rules[0].Priority)
	assert.Len(t, pol.Rules[0].Conditions, 2)
	assert.Len(t, pol.Rules[0].Actions, 3)

	// Second rule
	assert.Equal(t, "breaking_changes", pol.Rules[1].ID)
	assert.Equal(t, 90, pol.Rules[1].Priority)

	// Defaults
	assert.Equal(t, "approve", pol.Defaults.Decision)
	assert.Equal(t, 1, pol.Defaults.RequiredApprovers)
}

func TestLexer_ErrorUnterminatedString(t *testing.T) {
	input := `"unterminated`
	lexer := NewLexer(input)
	_, err := lexer.Tokenize()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unterminated string")
}

func TestParser_ErrorMissingBrace(t *testing.T) {
	input := `rule "test" {`
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	require.NoError(t, err)

	parser := NewParser(tokens)
	_, err = parser.Parse()
	require.Error(t, err)
}

func TestParser_ErrorInvalidToken(t *testing.T) {
	input := `invalid_keyword "test" {}`
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	require.NoError(t, err)

	parser := NewParser(tokens)
	_, err = parser.Parse()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected 'rule' or 'defaults'")
}

func TestCompiler_AllComparisonOperators(t *testing.T) {
	tests := []struct {
		condition string
		operator  string
	}{
		{"risk.score == 0.5", "eq"},
		{"risk.score != 0.5", "ne"},
		{"risk.score > 0.5", "gt"},
		{"risk.score < 0.5", "lt"},
		{"risk.score >= 0.5", "gte"},
		{"risk.score <= 0.5", "lte"},
	}

	for _, tt := range tests {
		t.Run(tt.operator, func(t *testing.T) {
			input := `rule "test" { when { ` + tt.condition + ` } then { block() } }`
			pol, err := Parse(input, "test")
			require.NoError(t, err)

			require.Len(t, pol.Rules, 1)
			require.Len(t, pol.Rules[0].Conditions, 1)
			assert.Equal(t, tt.operator, pol.Rules[0].Conditions[0].Operator)
		})
	}
}

func TestCompiler_NotOperator(t *testing.T) {
	input := `
rule "not-test" {
  when {
    NOT actor.trusted
  }
  then {
    require_approval(count: 1)
  }
}
`
	pol, err := Parse(input, "test")
	require.NoError(t, err)

	require.Len(t, pol.Rules, 1)
	rule := pol.Rules[0]

	require.Len(t, rule.Conditions, 1)
	assert.Equal(t, "_not", rule.Conditions[0].Field)
	assert.Equal(t, "not", rule.Conditions[0].Operator)
}
