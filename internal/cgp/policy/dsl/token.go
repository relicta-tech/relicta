// Package dsl provides a domain-specific language for defining governance policies.
//
// The DSL provides a human-readable syntax for writing policy rules:
//
//	rule "high-risk-release" {
//	  priority = 100
//	  description = "Require approval for high-risk releases"
//
//	  when {
//	    risk.score > 0.7 AND actor.kind == "agent"
//	  }
//
//	  then {
//	    require_approval(count: 2)
//	    add_rationale("High-risk agent release requires security review")
//	  }
//	}
//
//	defaults {
//	  decision = "approve"
//	  required_approvers = 1
//	}
package dsl

// TokenType represents the type of a lexical token.
type TokenType int

const (
	// Special tokens
	TokenEOF TokenType = iota
	TokenError
	TokenComment

	// Literals
	TokenIdent  // identifiers like risk.score
	TokenString // "quoted strings"
	TokenNumber // 123, 0.5
	TokenBool   // true, false

	// Keywords
	TokenRule
	TokenWhen
	TokenThen
	TokenDefaults
	TokenPriority
	TokenDescription
	TokenEnabled

	// Operators
	TokenAnd      // AND, &&
	TokenOr       // OR, ||
	TokenNot      // NOT, !
	TokenEq       // ==
	TokenNe       // !=
	TokenGt       // >
	TokenLt       // <
	TokenGte      // >=
	TokenLte      // <=
	TokenIn       // in
	TokenContains // contains
	TokenMatches  // matches

	// Delimiters
	TokenLBrace // {
	TokenRBrace // }
	TokenLParen // (
	TokenRParen // )
	TokenComma  // ,
	TokenColon  // :
	TokenAssign // =
)

// Token represents a lexical token.
type Token struct {
	Type    TokenType
	Value   string
	Line    int
	Column  int
	Literal any // Parsed literal value (for numbers, bools)
}

// String returns the token type name.
func (t TokenType) String() string {
	names := map[TokenType]string{
		TokenEOF:         "EOF",
		TokenError:       "ERROR",
		TokenComment:     "COMMENT",
		TokenIdent:       "IDENT",
		TokenString:      "STRING",
		TokenNumber:      "NUMBER",
		TokenBool:        "BOOL",
		TokenRule:        "rule",
		TokenWhen:        "when",
		TokenThen:        "then",
		TokenDefaults:    "defaults",
		TokenPriority:    "priority",
		TokenDescription: "description",
		TokenEnabled:     "enabled",
		TokenAnd:         "AND",
		TokenOr:          "OR",
		TokenNot:         "NOT",
		TokenEq:          "==",
		TokenNe:          "!=",
		TokenGt:          ">",
		TokenLt:          "<",
		TokenGte:         ">=",
		TokenLte:         "<=",
		TokenIn:          "in",
		TokenContains:    "contains",
		TokenMatches:     "matches",
		TokenLBrace:      "{",
		TokenRBrace:      "}",
		TokenLParen:      "(",
		TokenRParen:      ")",
		TokenComma:       ",",
		TokenColon:       ":",
		TokenAssign:      "=",
	}
	if name, ok := names[t]; ok {
		return name
	}
	return "UNKNOWN"
}

// keywords maps keyword strings to token types.
var keywords = map[string]TokenType{
	"rule":        TokenRule,
	"when":        TokenWhen,
	"then":        TokenThen,
	"defaults":    TokenDefaults,
	"priority":    TokenPriority,
	"description": TokenDescription,
	"enabled":     TokenEnabled,
	"AND":         TokenAnd,
	"and":         TokenAnd,
	"OR":          TokenOr,
	"or":          TokenOr,
	"NOT":         TokenNot,
	"not":         TokenNot,
	"in":          TokenIn,
	"contains":    TokenContains,
	"matches":     TokenMatches,
	"true":        TokenBool,
	"false":       TokenBool,
}

// LookupKeyword returns the token type for an identifier.
func LookupKeyword(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return TokenIdent
}
