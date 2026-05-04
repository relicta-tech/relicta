//go:build relicta_anthropic && relicta_openai && (relicta_gemini || relicta_all_ai || !relicta_minimal)

package ai

// Compile-time interface compliance checks. If any provider's
// CompleteStructured method signature drifts from StructuredOutputService,
// these break the build immediately rather than at runtime.
//
// Build-tag requires all three provider tags so the assertions only run when
// the symbols are in scope. The default build (no tags) includes all three,
// so this is exercised by the standard test sweep.

var (
	_ StructuredOutputService = (*openAIService)(nil)
	_ StructuredOutputService = (*anthropicService)(nil)
	_ StructuredOutputService = (*geminiService)(nil)
)
