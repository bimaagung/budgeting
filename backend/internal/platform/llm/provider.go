package llm

import (
	// Import genai SDK for provider abstraction support.
	// Currently using Anthropic, but genai is imported to support
	// future Gemini provider implementation.
	_ "google.golang.org/genai"
)
