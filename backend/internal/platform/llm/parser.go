package llm

import (
	"encoding/json"
	"fmt"

	"budgeting/internal/domain"
)

var validIntents = map[string]struct{}{
	"expense":     {},
	"income":      {},
	"balance":     {},
	"report":      {},
	"delete_last": {},
	"set_goal":    {},
	"set_budget":  {},
	"check_goal":  {},
	"unknown":     {},
}

// Validate parses raw JSON from Claude API into ParseResult and enforces:
// - well-formed JSON
// - intent is one of the 9 allowed values
// - amount >= 0
// - 0.0 <= confidence <= 1.0
//
// Caller (composer) logs raw bytes on failure and downgrades to {Intent: "unknown", Confidence: 0}.
func Validate(raw []byte) (domain.ParseResult, error) {
	var p domain.ParseResult
	if err := json.Unmarshal(raw, &p); err != nil {
		return domain.ParseResult{}, fmt.Errorf("invalid JSON: %w", err)
	}
	if _, ok := validIntents[p.Intent]; !ok {
		return domain.ParseResult{}, fmt.Errorf("unknown intent: %q", p.Intent)
	}
	if p.Amount < 0 {
		return domain.ParseResult{}, fmt.Errorf("negative amount: %d", p.Amount)
	}
	if p.Confidence < 0 || p.Confidence > 1 {
		return domain.ParseResult{}, fmt.Errorf("confidence out of range: %f", p.Confidence)
	}
	return p, nil
}
