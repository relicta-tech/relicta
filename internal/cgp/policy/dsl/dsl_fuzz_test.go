package dsl

import (
	"testing"
)

// FuzzParsePolicy tests the policy DSL parser with fuzzing.
// Run with: go test -fuzz=FuzzParsePolicy -fuzztime=30s
func FuzzParsePolicy(f *testing.F) {
	// Add seed corpus with valid and invalid policy DSL inputs
	seeds := []string{
		// Valid policies
		`rule "test" {
			priority = 1
			description = "test rule"
			when {
				risk_score > 5
			}
			then {
				require_approval(count: 2)
			}
		}`,
		`defaults {
			approval_count = 1
			notify = true
		}`,
		`rule "breaking-changes" {
			priority = 10
			when {
				has_breaking_changes == true
			}
			then {
				block(reason: "Breaking changes require review")
			}
		}`,
		`rule "high-risk" {
			enabled = false
			when {
				risk_score >= 8 and commit_count > 50
			}
			then {
				require_approval(count: 3)
				notify(channel: "releases")
			}
		}`,
		`rule "scope-check" {
			when {
				scope in ("api", "core", "auth")
			}
			then {
				require_approval(count: 1)
			}
		}`,
		`rule "pattern-match" {
			when {
				branch matches "release/.*"
			}
			then {
				require_approval(count: 2)
			}
		}`,
		`rule "negation" {
			when {
				not is_hotfix
			}
			then {
				require_approval(count: 1)
			}
		}`,
		`rule "complex" {
			when {
				(risk_score > 5 or has_breaking_changes) and not is_hotfix
			}
			then {
				block(reason: "complex condition")
			}
		}`,
		// Edge cases
		"",
		"rule",
		`rule "test"`,
		`rule "test" {`,
		`rule "test" { }`,
		`rule "test" { when { } }`,
		`rule "test" { then { } }`,
		"defaults { }",
		"defaults { } defaults { }",
		// Comments
		`# This is a comment
		rule "test" {
			# Another comment
			priority = 1
			when {
				risk_score > 0
			}
			then {
				notify(channel: "test")
			}
		}`,
		// Invalid inputs
		"not a policy",
		"{ }",
		`rule 123 { }`,
		`rule "test" { priority = "not a number" }`,
		`rule "test" { unknown_field = 1 }`,
		// Injection attempts
		`rule "test; rm -rf /" { }`,
		`rule "test" { description = "$(whoami)" }`,
		// Unicode
		`rule "テスト" { }`,
		`rule "test" { description = "описание" }`,
		// Nested braces
		`rule "test" { when { { } } }`,
		// Missing closing brace
		`rule "test" {`,
		`rule "test" { when {`,
		// Extra tokens
		`rule "test" { } extra`,
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// The primary goal is to ensure the lexer and parser don't panic
		lexer := NewLexer(input)
		tokens, err := lexer.Tokenize()

		if err != nil {
			// Lexer error is fine, just ensure no panic
			return
		}

		// Parse tokens
		parser := NewParser(tokens)
		policyFile, err := parser.Parse()

		if err != nil {
			// Parser error is fine, just ensure no panic
			return
		}

		// If we got a result, verify it's internally consistent
		if policyFile != nil {
			// Access all fields to ensure no nil pointer panics
			_ = len(policyFile.Rules)

			for _, rule := range policyFile.Rules {
				_ = rule.Name
				_ = rule.Priority
				_ = rule.Description
				_ = rule.Line
				_ = rule.Column

				if rule.When != nil {
					_ = rule.When.Condition
				}
				if rule.Then != nil {
					for _, action := range rule.Then.Actions {
						_ = action.Name
						_ = len(action.Args)
					}
				}
			}

			if policyFile.Defaults != nil {
				_ = len(policyFile.Defaults.Settings)
			}
		}
	})
}

// FuzzLexer tests the DSL lexer with fuzzing.
// Run with: go test -fuzz=FuzzLexer -fuzztime=30s
func FuzzLexer(f *testing.F) {
	seeds := []string{
		`rule "test" { }`,
		`== != >= <= > < && || !`,
		`123 45.67 0 999999`,
		`"hello" "world" "escaped\"quote"`,
		`true false`,
		`rule defaults when then priority description enabled`,
		`and or not in contains matches`,
		`{ } ( ) , : =`,
		`# comment line`,
		`// another comment`,
		"",
		" \t\n\r",
		`"unterminated string`,
		`"string with
		newline"`,
		`@#$%^&`,
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		lexer := NewLexer(input)
		// Tokenize should not panic
		tokens, _ := lexer.Tokenize()

		// If tokenization succeeded, all tokens should be accessible
		for _, tok := range tokens {
			_ = tok.Type
			_ = tok.Value
			_ = tok.Line
			_ = tok.Column
			_ = tok.Literal
		}
	})
}
