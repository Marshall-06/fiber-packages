package routes

import (
	"github.com/gofiber/fiber/v2"

	"nazarly-digital/config"
	"nazarly-digital/controllers"
	"nazarly-digital/middlewares"
)

func Setup(app *fiber.App, cfg config.Config) {
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Fiber auth simple")
	})

	app.Get("/auth/google/login", controllers.GoogleLogin)
	app.Get("/auth/google/callback", controllers.GoogleCallback(cfg.JWTSecret))

	// protected route example
	app.Get("/me", middleware.RequireAuth(cfg.JWTSecret), func(c *fiber.Ctx) error {
		claims := c.Locals("user")
		return c.JSON(fiber.Map{"me": claims})
	})
}
