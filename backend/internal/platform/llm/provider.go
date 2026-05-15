package llm

import (
	"context"
	"fmt"
	"os"
)

// LLMProvider — kontrak adapter ke vendor LLM (Gemini, Claude, dll).
// Composer pakai ini untuk LLM call mentah; prompt building & JSON validation
// dilakukan di layer composer (DRY antar provider).
//
// Generate menerima system prompt + user prompt + flag expectJSON.
// expectJSON adalah hint untuk provider supaya enforce JSON output bila support
// (Gemini pakai responseSchema; Claude tidak butuh — sudah di-instruct via prompt).
// Composer yang akan Validate() raw output bila perlu.
type LLMProvider interface {
	Generate(ctx context.Context, systemPrompt, userPrompt string, expectJSON bool) (string, error)
}

// NewProvider memilih implementasi LLM berdasarkan env LLM_PROVIDER.
// Default: "gemini". Fail fast kalau API key untuk provider aktif tidak ada
// atau nama provider tidak dikenal.
func NewProvider() (LLMProvider, error) {
	name := os.Getenv("LLM_PROVIDER")
	if name == "" {
		name = "gemini"
	}
	switch name {
	case "gemini":
		return newGeminiProvider()
	case "claude":
		return newClaudeProvider()
	default:
		return nil, fmt.Errorf("unknown LLM_PROVIDER: %q (expected: gemini | claude)", name)
	}
}
