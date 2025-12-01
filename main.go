package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"

	"nazarly-digital/config"
	"nazarly-digital/controllers"
	"nazarly-digital/models"
	"nazarly-digital/routes"
)

func main() {
	// load .env
	_ = godotenv.Load()

	cfg := config.LoadConfigFromEnv()

	// connect DB
	err := config.ConnectDB(cfg)
	if err != nil {
		log.Fatalf("db connect error: %v", err)
	}
	// auto migrate
	if err := config.DB.AutoMigrate(&models.User{}); err != nil {
		log.Println("auto migrate warning:", err)
	}

	// init google oauth
	controllers.InitGoogle(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)

	// fiber app
	app := fiber.New()

	// routes
	routes.Setup(app, cfg)

	port := cfg.Port
	if port == "" {
		port = "1000"
	}
	log.Println("listening on :", port)
	if err := app.Listen(":" + port); err != nil {
		log.Println("server error:", err)
		os.Exit(1)
	}
}
