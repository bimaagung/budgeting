package llm

import (
	"context"
	"fmt"
	"log"
	"os"

	"budgeting/internal/domain"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const model = anthropic.ModelClaudeHaiku4_5_20251001

type composer struct {
	client anthropic.Client
}

func NewMessageComposer() domain.MessageComposer {
	return &composer{
		client: anthropic.NewClient(
			option.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")),
		),
	}
}

func (c *composer) ParseMessage(ctx context.Context, rawMessage string) (*domain.ParseResult, error) {
	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 256,
		System: []anthropic.TextBlockParam{
			{Text: parseSystemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(rawMessage)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude api: %w", err)
	}

	raw := []byte(msg.Content[0].Text)
	parsed, vErr := Validate(raw)
	if vErr != nil {
		log.Printf("llm parse validation failed: raw=%s err=%v", string(raw), vErr)
		return &domain.ParseResult{Intent: "unknown", Confidence: 0}, nil
	}
	return &parsed, nil
}

func (c *composer) FormatPostTransaction(ctx context.Context, rc domain.ReminderContext) (string, error) {
	return c.compose(ctx, buildPostTransactionPrompt(rc))
}

func (c *composer) FormatDailyReminder(ctx context.Context, rc domain.ReminderContext) (string, error) {
	return c.compose(ctx, buildDailyReminderPrompt(rc))
}

func (c *composer) compose(ctx context.Context, userPrompt string) (string, error) {
	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 512,
		System: []anthropic.TextBlockParam{
			{Text: reminderSystemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("claude api: %w", err)
	}
	return msg.Content[0].Text, nil
}
