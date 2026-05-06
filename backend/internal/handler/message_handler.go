package handler

import (
	"budgeting/internal/usecase"
	"budgeting/pkg/httputil"

	"github.com/gofiber/fiber/v2"
)

type MessageHandler struct {
	uc *usecase.MessageUsecase
}

func NewMessageHandler(uc *usecase.MessageUsecase) *MessageHandler {
	return &MessageHandler{uc}
}

type messageRequest struct {
	Phone   string `json:"phone"`
	Message string `json:"message"`
}

func (h *MessageHandler) Handle(c *fiber.Ctx) error {
	var req messageRequest
	if err := c.BodyParser(&req); err != nil || req.Phone == "" || req.Message == "" {
		return httputil.Error(c, fiber.StatusBadRequest, "phone and message are required")
	}

	result, err := h.uc.Handle(c.UserContext(), req.Phone, req.Message)
	if err != nil {
		return httputil.InternalError(c, err)
	}

	return c.JSON(fiber.Map{
		"reply":         result.Reply,
		"needs_confirm": result.NeedsConfirm,
	})
}
