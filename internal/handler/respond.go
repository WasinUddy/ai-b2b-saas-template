package handler

import "github.com/gofiber/fiber/v2"

func respond(c *fiber.Ctx, status int, dto any) error {
	return c.Status(status).JSON(dto)
}

func noContent(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}
