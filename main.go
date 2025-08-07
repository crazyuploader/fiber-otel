package main

import (
	"errors"
	"log"
	"time"

	// Import Fiber related packages
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	// Initialize Fiber app instance
	app := fiber.New(fiber.Config{
		AppName:      "API Server with OTel metrics.",
		ServerHeader: "Go Fiber",
	})

	// Use Fiber logger middleware for logging
	app.Use(logger.New())

	// Root Endpoint
	// GET <> Endpoint: /
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Endpoint to simulate an error
	// GET <> Endpoint: /error
	errorCount := 0
	app.Get("/error", func(c *fiber.Ctx) error {
		errorCount++
		log.Println("Errors:", errorCount)
		return errors.New("error")
	})

	// Endpoint to check the processed latency; simulated by time.Sleep() function
	// GET <> Endpoint: latency
	app.Get("/latency", func(c *fiber.Ctx) error {
		initialTime := time.Now()
		time.Sleep(time.Second * 3)
		return c.JSON(fiber.Map{"status": "ok",
			"response_time": time.Since(initialTime).String(),
		})
	})

	// Endpoint to check items in cart, initial value is zero
	// GET <> Endpoint: /cart
	totalItemsInCart := 0
	app.Get("/cart", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok",
			"total_items": totalItemsInCart})
	})

	// Endpoint to add items in cart, makes use of query parameters
	// GET <> Endpoint: /cart/add?count=integer_number
	app.Get("/cart/add", func(c *fiber.Ctx) error {
		countInt := c.QueryInt("count")
		if countInt == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid count, it should be a number"})
		}
		totalItemsInCart += countInt
		return c.JSON(fiber.Map{"status": "ok", "message": "Added items", "updated_items": totalItemsInCart})
	})

	// Endpoint to reduce items in cart, makes use of query parameters
	// GET <> Endpoint: /cart/reduce?count=integer_number
	app.Get("/cart/reduce", func(c *fiber.Ctx) error {
		countInt := c.QueryInt("count")
		if countInt == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid count, it should be a number"})
		}
		totalItemsInCart -= countInt
		return c.JSON(fiber.Map{"status": "ok", "message": "Reduced items", "updated_items": totalItemsInCart})
	})

	// Start our API Server
	log.Fatal(app.Listen(":3100"))
}
