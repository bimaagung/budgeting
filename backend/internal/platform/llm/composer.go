package llm

import (
	"context"
	"log"

	"budgeting/internal/domain"
)

type composer struct {
	provider LLMProvider
}

func NewMessageComposer(p LLMProvider) domain.MessageComposer {
	return &composer{provider: p}
}

func (c *composer) ParseMessage(ctx context.Context, raw string) (*domain.ParseResult, error) {
	out, err := c.provider.Generate(ctx, parseSystemPrompt, raw, true)
	if err != nil {
		return nil, err
	}
	parsed, vErr := Validate([]byte(out))
	if vErr != nil {
		log.Printf("llm parse validation failed: raw=%s err=%v", out, vErr)
		return &domain.ParseResult{Intent: "unknown", Confidence: 0}, nil
	}
	return &parsed, nil
}

func (c *composer) FormatPostTransaction(ctx context.Context, rc domain.ReminderContext) (string, error) {
	return c.provider.Generate(ctx, reminderSystemPrompt, buildPostTransactionPrompt(rc), false)
}

func (c *composer) FormatDailyReminder(ctx context.Context, rc domain.ReminderContext) (string, error) {
	return c.provider.Generate(ctx, reminderSystemPrompt, buildDailyReminderPrompt(rc), false)
}
