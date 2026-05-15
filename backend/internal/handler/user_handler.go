package handler

import (
	"context"

	"budgeting/pkg/httputil"

	"github.com/gofiber/fiber/v2"
)

type userPhoneQuerier interface {
	GetAllPhones(ctx context.Context) ([]string, error)
}

type UserHandler struct {
	q userPhoneQuerier
}

func NewUserHandler(q userPhoneQuerier) *UserHandler {
	return &UserHandler{q: q}
}

func (h *UserHandler) Handle(c *fiber.Ctx) error {
	phones, err := h.q.GetAllPhones(c.UserContext())
	if err != nil {
		return httputil.InternalError(c, err)
	}
	if phones == nil {
		phones = []string{}
	}
	return c.JSON(fiber.Map{"phones": phones})
}
