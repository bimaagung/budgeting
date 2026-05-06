package httputil

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

func Error(c *fiber.Ctx, status int, msg string) error {
	return c.Status(status).JSON(fiber.Map{"error": msg})
}

func InternalError(c *fiber.Ctx, err error) error {
	log.Printf("internal error: %v", err)
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
}
