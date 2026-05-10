package handler

// Response is the tagged-union body returned by POST /api/message after
// parse succeeds. ReplyType discriminates which optional fields are present.
//
// reply_type=confirm  -> Transaction, Context populated; Data absent.
// reply_type=info     -> Data populated; Transaction, Context absent.
// reply_type=clarify  -> only ReplyType, Persisted, ReplyText.
// reply_type=error    -> only ReplyType, Persisted, ReplyText.
//
// ReplyText is always non-empty.
type Response struct {
	ReplyType   string           `json:"reply_type"`
	Persisted   bool             `json:"persisted"`
	ReplyText   string           `json:"reply_text"`
	Transaction *TransactionView `json:"transaction,omitempty"`
	Context     *ContextView     `json:"context,omitempty"`
	Data        map[string]any   `json:"data,omitempty"`
}

type TransactionView struct {
	ID       string `json:"id"`
	Amount   int64  `json:"amount"`
	Category string `json:"category"`
	Type     string `json:"type"` // "expense" | "income"
}

// ContextView is the post-transaction context for confirm replies.
// CategoryBudget is *intentionally* a pointer with no omitempty so it
// serializes as `null` when the user has no budget for the transaction's
// category (per spec Decision 5).
type ContextView struct {
	Balance        int64               `json:"balance"`
	CategoryBudget *CategoryBudgetView `json:"category_budget"`
	SavingsGoals   []SavingsGoalView   `json:"savings_goals"`
}

type CategoryBudgetView struct {
	Category  string `json:"category"`
	Spent     int64  `json:"spent"`
	Limit     int64  `json:"limit"`
	Remaining int64  `json:"remaining"`
}

type SavingsGoalView struct {
	Name        string `json:"name"`
	Saved       int64  `json:"saved"`
	Target      int64  `json:"target"`
	ProgressPct int    `json:"progress_pct"`
}
