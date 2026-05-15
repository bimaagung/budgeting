package llm

import (
	"context"
	"os"
	"testing"
)

func TestNewGeminiProvider_RequiresAPIKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	_, err := newGeminiProvider()
	if err == nil {
		t.Fatal("expected error when GEMINI_API_KEY is empty, got nil")
	}
}

func TestNewGeminiProvider_WithAPIKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "dummy-key")
	t.Setenv("GEMINI_MODEL", "")
	p, err := newGeminiProvider()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("provider is nil")
	}
	if p.model != "gemini-2.5-flash" {
		t.Errorf("expected default model 'gemini-2.5-flash', got %q", p.model)
	}
}

func TestNewGeminiProvider_CustomModel(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "dummy-key")
	t.Setenv("GEMINI_MODEL", "gemini-2.5-flash-lite")
	p, err := newGeminiProvider()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.model != "gemini-2.5-flash-lite" {
		t.Errorf("expected model 'gemini-2.5-flash-lite', got %q", p.model)
	}
}

// TestGeminiProvider_GenerateLive — smoke test real API, skip kalau no key.
func TestGeminiProvider_GenerateLive(t *testing.T) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" || key == "dummy-key" {
		t.Skip("GEMINI_API_KEY not set; live test skipped")
	}
	p, err := newGeminiProvider()
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	out, err := p.Generate(context.Background(),
		"Kamu balas pesan dengan satu kata dalam Bahasa Indonesia.",
		"Sapa saya!",
		false)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	t.Logf("gemini response: %q", out)
}
