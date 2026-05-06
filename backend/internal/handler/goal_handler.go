package handler

import (
	"budgeting/internal/usecase"
	"budgeting/pkg/httputil"

	"github.com/gofiber/fiber/v2"
)

type GoalHandler struct {
	uc *usecase.GoalUsecase
}

func NewGoalHandler(uc *usecase.GoalUsecase) *GoalHandler {
	return &GoalHandler{uc}
}

type setGoalRequest struct {
	Phone        string `json:"phone"`
	Name         string `json:"name"`
	TargetAmount int64  `json:"target_amount"`
}

type setBudgetRequest struct {
	Phone    string `json:"phone"`
	Category string `json:"category"`
	Amount   int64  `json:"amount"`
}

func (h *GoalHandler) SetGoal(c *fiber.Ctx) error {
	var req setGoalRequest
	if err := c.BodyParser(&req); err != nil || req.Phone == "" {
		return httputil.Error(c, fiber.StatusBadRequest, "invalid request")
	}
	user, err := h.uc.ResolveUser(c.UserContext(), req.Phone)
	if err != nil || user == nil {
		return httputil.Error(c, fiber.StatusNotFound, "user not found")
	}
	reply, err := h.uc.SetGoal(c.UserContext(), user, req.Name, req.TargetAmount)
	if err != nil {
		return httputil.InternalError(c, err)
	}
	return c.JSON(fiber.Map{"reply": reply})
}

func (h *GoalHandler) SetBudget(c *fiber.Ctx) error {
	var req setBudgetRequest
	if err := c.BodyParser(&req); err != nil || req.Phone == "" {
		return httputil.Error(c, fiber.StatusBadRequest, "invalid request")
	}
	user, err := h.uc.ResolveUser(c.UserContext(), req.Phone)
	if err != nil || user == nil {
		return httputil.Error(c, fiber.StatusNotFound, "user not found")
	}
	reply, err := h.uc.SetBudget(c.UserContext(), user, req.Category, req.Amount)
	if err != nil {
		return httputil.InternalError(c, err)
	}
	return c.JSON(fiber.Map{"reply": reply})
}

func (h *GoalHandler) CheckGoals(c *fiber.Ctx) error {
	phone := c.Query("phone")
	if phone == "" {
		return httputil.Error(c, fiber.StatusBadRequest, "phone required")
	}
	user, err := h.uc.ResolveUser(c.UserContext(), phone)
	if err != nil || user == nil {
		return httputil.Error(c, fiber.StatusNotFound, "user not found")
	}
	reply, err := h.uc.CheckGoals(c.UserContext(), user)
	if err != nil {
		return httputil.InternalError(c, err)
	}
	return c.JSON(fiber.Map{"reply": reply})
}
