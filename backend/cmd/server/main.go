package main

import (
	"log"
	"os"

	"budgeting/internal/handler"
	"budgeting/internal/platform/llm"
	"budgeting/internal/platform/postgres"
	"budgeting/internal/usecase"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	db, err := postgres.NewDB()
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}

	// platform
	composer := llm.NewMessageComposer()

	// repositories
	userRepo := postgres.NewUserRepository(db)
	txRepo := postgres.NewTransactionRepository(db)
	goalRepo := postgres.NewSavingsGoalRepository(db)
	budgetRepo := postgres.NewBudgetTargetRepository(db)

	// usecases
	reminderUC := usecase.NewReminderUsecase(txRepo, goalRepo, budgetRepo, userRepo, composer)
	goalUC := usecase.NewGoalUsecase(goalRepo, budgetRepo, userRepo)
	messageUC := usecase.NewMessageUsecase(txRepo, userRepo, composer, reminderUC, goalUC)

	// app
	app := handler.NewApp(
		handler.NewMessageHandler(messageUC),
		handler.NewReminderHandler(reminderUC),
		handler.NewGoalHandler(goalUC),
	)

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	log.Fatal(app.Listen(":" + port))
}
