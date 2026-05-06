package handler

import (
	"context"

	"budgeting/internal/domain"
	"budgeting/internal/usecase"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func NewApp(msg *MessageHandler, reminder *ReminderHandler, goal *GoalHandler) *fiber.App {
	app := fiber.New()
	app.Use(logger.New())
	app.Use(recover.New())

	api := app.Group("/api")
	api.Post("/message", msg.Handle)
	api.Get("/reminder/:phone", reminder.Handle)
	api.Post("/goals", goal.SetGoal)
	api.Post("/budgets", goal.SetBudget)
	api.Get("/goals", goal.CheckGoals)

	return app
}

func resolveUser(ctx context.Context, phone string, uc *usecase.GoalUsecase) (*domain.User, error) {
	return uc.ResolveUser(ctx, phone)
}
