package llm

import (
	"testing"
)

func TestNewClaudeProvider_RequiresAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	_, err := newClaudeProvider()
	if err == nil {
		t.Fatal("expected error when ANTHROPIC_API_KEY is empty, got nil")
	}
}

func TestNewClaudeProvider_WithAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-dummy")
	p, err := newClaudeProvider()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("provider is nil")
	}
}

