package domain

import "context"

type ParseResult struct {
	Intent     string  `json:"intent"`
	Amount     int64   `json:"amount"`
	Category   string  `json:"category"`
	Note       string  `json:"note"`
	GoalName   string  `json:"goal_name"`
	Confidence float64 `json:"confidence"`
}

type ReminderContext struct {
	UserName        string
	Balance         int64
	LastCategory    string
	LastAmount      int64
	TodaySpend      int64
	SpendByCategory map[string]int64
	BudgetRemaining map[string]int64
	SavingsGoals    []SavingsGoal
}

type MessageComposer interface {
	ParseMessage(ctx context.Context, rawMessage string) (*ParseResult, error)
	FormatPostTransaction(ctx context.Context, rc ReminderContext) (string, error)
	FormatDailyReminder(ctx context.Context, rc ReminderContext) (string, error)
}
