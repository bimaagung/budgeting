package handler

import (
	"context"
	"errors"
	"time"

	"budgeting/internal/usecase"
	"budgeting/pkg/httputil"

	"github.com/gofiber/fiber/v2"
)

// messageUsecase is the subset of *usecase.MessageUsecase the handler depends on.
// Declared as an interface here for fakes in tests.
type messageUsecase interface {
	Handle(ctx context.Context, phone, message, receivedAt string) (*usecase.MessageResult, error)
}

type MessageHandler struct {
	uc messageUsecase
}

func NewMessageHandler(uc *usecase.MessageUsecase) *MessageHandler {
	return &MessageHandler{uc: uc}
}

type messageRequest struct {
	Phone      string `json:"phone"`
	Message    string `json:"message"`
	ReceivedAt string `json:"received_at"`
}

func (h *MessageHandler) Handle(c *fiber.Ctx) error {
	var req messageRequest
	if err := c.BodyParser(&req); err != nil {
		return httputil.Error(c, fiber.StatusBadRequest, "malformed JSON")
	}
	if req.Phone == "" {
		return httputil.Error(c, fiber.StatusBadRequest, "phone is required")
	}
	if req.Message == "" {
		return httputil.Error(c, fiber.StatusBadRequest, "message is required")
	}
	if req.ReceivedAt == "" {
		return httputil.Error(c, fiber.StatusBadRequest, "received_at is required")
	}
	if _, err := time.Parse(time.RFC3339, req.ReceivedAt); err != nil {
		return httputil.Error(c, fiber.StatusBadRequest, "received_at must be RFC3339")
	}

	normalized, err := normalizePhone(req.Phone)
	if err != nil {
		return httputil.Error(c, fiber.StatusBadRequest, "invalid phone format")
	}

	result, err := h.uc.Handle(c.UserContext(), normalized, req.Message, req.ReceivedAt)
	if err != nil {
		if errors.Is(err, usecase.ErrUserNotRegistered) {
			return httputil.Error(c, fiber.StatusNotFound, "user not registered")
		}
		return httputil.InternalError(c, err)
	}

	return c.JSON(fiber.Map{
		"reply":         result.Reply,
		"needs_confirm": result.NeedsConfirm,
	})
}
