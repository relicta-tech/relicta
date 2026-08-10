package ai

// Compile-time interface compliance checks. If any provider's
// CompleteStructured method signature drifts from StructuredOutputService,
// these break the build immediately rather than at runtime.
//
// These now compile on every build. They did not before: the file carried
// "//go:build relicta_anthropic && relicta_openai && (...)", which requires both
// tags to be set explicitly, and nothing set them — not the default build, and
// none of the four combinations CI exercised. Its own comment claimed the default
// build included all three and that the sweep exercised it, and neither was true,
// so these assertions were never checked once. Removing the AI build tags is what
// made them real.

var (
	_ StructuredOutputService = (*openAIService)(nil)
	_ StructuredOutputService = (*anthropicService)(nil)
	_ StructuredOutputService = (*geminiService)(nil)
)
