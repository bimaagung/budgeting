package llm

import (
	"context"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type claudeProvider struct {
	client anthropic.Client
	model  anthropic.Model
}

func newClaudeProvider() (*claudeProvider, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is required when LLM_PROVIDER=claude")
	}
	return &claudeProvider{
		client: anthropic.NewClient(option.WithAPIKey(key)),
		model:  anthropic.ModelClaudeHaiku4_5_20251001,
	}, nil
}

func (c *claudeProvider) Generate(ctx context.Context, system, user string, expectJSON bool) (string, error) {
	maxTok := int64(512)
	if expectJSON {
		maxTok = 256
	}
	// expectJSON tidak butuh special handling: output JSON-only sudah di-enforce
	// via prompt content, dan composer yang Validate() output.
	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: maxTok,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(user))},
	})
	if err != nil {
		return "", fmt.Errorf("claude api: %w", err)
	}
	return msg.Content[0].Text, nil
}
