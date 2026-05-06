package handler

import (
	"budgeting/internal/usecase"
	"budgeting/pkg/httputil"

	"github.com/gofiber/fiber/v2"
)

type ReminderHandler struct {
	uc *usecase.ReminderUsecase
}

func NewReminderHandler(uc *usecase.ReminderUsecase) *ReminderHandler {
	return &ReminderHandler{uc}
}

func (h *ReminderHandler) Handle(c *fiber.Ctx) error {
	phone := c.Params("phone")
	if phone == "" {
		return httputil.Error(c, fiber.StatusBadRequest, "phone required")
	}

	message, err := h.uc.BuildDailyReminder(c.UserContext(), phone)
	if err != nil {
		return httputil.InternalError(c, err)
	}

	return c.JSON(fiber.Map{"message": message})
}
