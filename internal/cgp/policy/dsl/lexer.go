package dsl

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Lexer tokenizes DSL source code.
type Lexer struct {
	input   string
	pos     int
	line    int
	column  int
	start   int
	startLn int
	startCl int
}

// NewLexer creates a new lexer for the given input.
func NewLexer(input string) *Lexer {
	return &Lexer{
		input:  input,
		pos:    0,
		line:   1,
		column: 1,
	}
}

// Tokenize returns all tokens from the input.
func (l *Lexer) Tokenize() ([]Token, error) {
	var tokens []Token
	for {
		tok := l.NextToken()
		if tok.Type == TokenError {
			return nil, fmt.Errorf("lexer error at line %d, column %d: %s", tok.Line, tok.Column, tok.Value)
		}
		tokens = append(tokens, tok)
		if tok.Type == TokenEOF {
			break
		}
	}
	return tokens, nil
}

// NextToken returns the next token from the input.
func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	if l.pos >= len(l.input) {
		return Token{Type: TokenEOF, Line: l.line, Column: l.column}
	}

	l.start = l.pos
	l.startLn = l.line
	l.startCl = l.column

	ch := l.input[l.pos]

	// Comments
	if ch == '#' || (ch == '/' && l.peek() == '/') {
		return l.scanComment()
	}

	// String literals
	if ch == '"' {
		return l.scanString()
	}

	// Numbers
	if isDigit(ch) || (ch == '.' && l.peek() != 0 && isDigit(l.peek())) {
		return l.scanNumber()
	}

	// Identifiers and keywords
	if isLetter(ch) || ch == '_' {
		return l.scanIdentifier()
	}

	// Two-character operators
	if l.pos+1 < len(l.input) {
		two := l.input[l.pos : l.pos+2]
		switch two {
		case "==":
			l.advance()
			l.advance()
			return Token{Type: TokenEq, Value: two, Line: l.startLn, Column: l.startCl}
		case "!=":
			l.advance()
			l.advance()
			return Token{Type: TokenNe, Value: two, Line: l.startLn, Column: l.startCl}
		case ">=":
			l.advance()
			l.advance()
			return Token{Type: TokenGte, Value: two, Line: l.startLn, Column: l.startCl}
		case "<=":
			l.advance()
			l.advance()
			return Token{Type: TokenLte, Value: two, Line: l.startLn, Column: l.startCl}
		case "&&":
			l.advance()
			l.advance()
			return Token{Type: TokenAnd, Value: two, Line: l.startLn, Column: l.startCl}
		case "||":
			l.advance()
			l.advance()
			return Token{Type: TokenOr, Value: two, Line: l.startLn, Column: l.startCl}
		}
	}

	// Single-character tokens
	l.advance()
	switch ch {
	case '{':
		return Token{Type: TokenLBrace, Value: "{", Line: l.startLn, Column: l.startCl}
	case '}':
		return Token{Type: TokenRBrace, Value: "}", Line: l.startLn, Column: l.startCl}
	case '(':
		return Token{Type: TokenLParen, Value: "(", Line: l.startLn, Column: l.startCl}
	case ')':
		return Token{Type: TokenRParen, Value: ")", Line: l.startLn, Column: l.startCl}
	case ',':
		return Token{Type: TokenComma, Value: ",", Line: l.startLn, Column: l.startCl}
	case ':':
		return Token{Type: TokenColon, Value: ":", Line: l.startLn, Column: l.startCl}
	case '=':
		return Token{Type: TokenAssign, Value: "=", Line: l.startLn, Column: l.startCl}
	case '>':
		return Token{Type: TokenGt, Value: ">", Line: l.startLn, Column: l.startCl}
	case '<':
		return Token{Type: TokenLt, Value: "<", Line: l.startLn, Column: l.startCl}
	case '!':
		return Token{Type: TokenNot, Value: "!", Line: l.startLn, Column: l.startCl}
	}

	return Token{
		Type:   TokenError,
		Value:  fmt.Sprintf("unexpected character: %c", ch),
		Line:   l.startLn,
		Column: l.startCl,
	}
}

func (l *Lexer) advance() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	ch := l.input[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
	return ch
}

func (l *Lexer) peek() byte {
	if l.pos+1 >= len(l.input) {
		return 0
	}
	return l.input[l.pos+1]
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			l.advance()
		} else {
			break
		}
	}
}

func (l *Lexer) scanComment() Token {
	start := l.pos
	// Skip # or //
	if l.input[l.pos] == '#' {
		l.advance()
	} else {
		l.advance()
		l.advance()
	}

	// Read until end of line
	for l.pos < len(l.input) && l.input[l.pos] != '\n' {
		l.advance()
	}

	return Token{
		Type:   TokenComment,
		Value:  strings.TrimSpace(l.input[start:l.pos]),
		Line:   l.startLn,
		Column: l.startCl,
	}
}

func (l *Lexer) scanString() Token {
	l.advance() // Skip opening quote
	var sb strings.Builder

	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '"' {
			l.advance() // Skip closing quote
			return Token{
				Type:    TokenString,
				Value:   sb.String(),
				Line:    l.startLn,
				Column:  l.startCl,
				Literal: sb.String(),
			}
		}
		if ch == '\\' && l.pos+1 < len(l.input) {
			l.advance()
			next := l.advance()
			switch next {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case '"':
				sb.WriteByte('"')
			case '\\':
				sb.WriteByte('\\')
			default:
				sb.WriteByte('\\')
				sb.WriteByte(next)
			}
		} else if ch == '\n' {
			return Token{
				Type:   TokenError,
				Value:  "unterminated string",
				Line:   l.startLn,
				Column: l.startCl,
			}
		} else {
			sb.WriteByte(l.advance())
		}
	}

	return Token{
		Type:   TokenError,
		Value:  "unterminated string",
		Line:   l.startLn,
		Column: l.startCl,
	}
}

func (l *Lexer) scanNumber() Token {
	start := l.pos
	hasDecimal := false

	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if isDigit(ch) {
			l.advance()
		} else if ch == '.' && !hasDecimal {
			hasDecimal = true
			l.advance()
		} else {
			break
		}
	}

	value := l.input[start:l.pos]
	var literal any
	var err error

	if hasDecimal {
		literal, err = strconv.ParseFloat(value, 64)
	} else {
		literal, err = strconv.ParseInt(value, 10, 64)
	}

	if err != nil {
		return Token{
			Type:   TokenError,
			Value:  fmt.Sprintf("invalid number: %s", value),
			Line:   l.startLn,
			Column: l.startCl,
		}
	}

	return Token{
		Type:    TokenNumber,
		Value:   value,
		Line:    l.startLn,
		Column:  l.startCl,
		Literal: literal,
	}
}

func (l *Lexer) scanIdentifier() Token {
	start := l.pos

	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if isLetter(ch) || isDigit(ch) || ch == '_' || ch == '.' {
			l.advance()
		} else {
			break
		}
	}

	value := l.input[start:l.pos]
	tokType := LookupKeyword(value)

	tok := Token{
		Type:   tokType,
		Value:  value,
		Line:   l.startLn,
		Column: l.startCl,
	}

	// Set literal for booleans
	if tokType == TokenBool {
		tok.Literal = value == "true"
	}

	return tok
}

func isLetter(ch byte) bool {
	return unicode.IsLetter(rune(ch))
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}
