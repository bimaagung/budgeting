package llm

import (
	"strings"
	"testing"
)

func TestNewProvider_DefaultsToGemini(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "")
	t.Setenv("GEMINI_API_KEY", "dummy")
	t.Setenv("GEMINI_MODEL", "")
	p, err := NewProvider()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*geminiProvider); !ok {
		t.Errorf("expected *geminiProvider, got %T", p)
	}
}

func TestNewProvider_SelectsClaude(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "claude")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	p, err := NewProvider()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*claudeProvider); !ok {
		t.Errorf("expected *claudeProvider, got %T", p)
	}
}

func TestNewProvider_UnknownProvider(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "openai")
	_, err := NewProvider()
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown LLM_PROVIDER") {
		t.Errorf("error message unexpected: %v", err)
	}
}

func TestNewProvider_GeminiMissingKey(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "gemini")
	t.Setenv("GEMINI_API_KEY", "")
	_, err := NewProvider()
	if err == nil {
		t.Fatal("expected error for missing GEMINI_API_KEY")
	}
}

func TestNewProvider_ClaudeMissingKey(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "claude")
	t.Setenv("ANTHROPIC_API_KEY", "")
	_, err := NewProvider()
	if err == nil {
		t.Fatal("expected error for missing ANTHROPIC_API_KEY")
	}
}
