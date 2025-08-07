package main

import (
	// Standard packages
	"context"
	"errors"
	"log"
	"time"

	// Import Fiber related packages
	"github.com/crazyuploader/fiber-otel/internal/metrics"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"

	// Import OpenTelemetry related packages
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

// Function to initialize Meter Provider
func initMeter() (*sdkmetric.MeterProvider, error) {
	// Create an exporter
	exporter, err := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
	if err != nil {
		return nil, err
	}

	// Create a Meter Provider
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("fiber-app"),
		)),
	)

	// Set Meter Provider globally
	otel.SetMeterProvider(mp)

	// Return Meter Provider
	return mp, nil
}

// Main function of the Go Fiber API Server
func main() {
	// Initialize OpenTelemetry MeterProvider
	mp, err := initMeter()
	if err != nil {
		log.Fatal("Failed to initialize Meter Provider", err)
	}

	// Make sure to shut down the Meter Provider
	defer func() {
		if err := mp.Shutdown(context.Background()); err != nil {
			log.Println("Error shutting down Meter Provider", err)
		}
	}()

	// Initialize our custom metrics
	appMetrics, err := metrics.NewMetric("fiber-app")
	if err != nil {
		log.Fatal("Failed to initialize custom metrics", err)
	}

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
	var errorCount int
	app.Get("/error", func(c *fiber.Ctx) error {
		ctx := c.UserContext()
		if ctx == nil {
			ctx = context.Background()
		}

		// Increment error counter
		errorCount++

		// Increment error metrics counter
		appMetrics.ErrorCount.Add(ctx, 1)
		log.Println("Errors:", errorCount)
		return errors.New("simulated error")
	})

	// Endpoint to check the processed latency; simulated by time.Sleep() function
	// GET <> Endpoint: /latency
	app.Get("/latency", func(c *fiber.Ctx) error {
		ctx := c.UserContext()
		if ctx == nil {
			ctx = context.Background()
		}

		// Store current time
		initialTime := time.Now()

		// Sleep for 3 seconds
		time.Sleep(time.Second * 3)

		// Get latest time
		duration := time.Since(initialTime).Milliseconds()

		// Record elapsed duration in metrics histogram
		appMetrics.RequestLatency.Record(ctx, float64(duration))
		return c.JSON(fiber.Map{"status": "ok",
			"response_time": time.Since(initialTime).String(),
		})
	})

	// Endpoint to check items in cart, initial value is zero
	// GET <> Endpoint: /cart
	app.Get("/cart", func(c *fiber.Ctx) error {
		// Get latest cart value from gauge metrics which we stored internally
		totalItemsInCart := appMetrics.GetTotalItemsInCart()
		return c.JSON(fiber.Map{"status": "ok",
			"total_items": totalItemsInCart})
	})

	// Endpoint to add items in cart, makes use of query parameters
	// GET <> Endpoint: /cart/add?count=integer_number
	app.Get("/cart/add", func(c *fiber.Ctx) error {
		ctx := c.UserContext()
		if ctx == nil {
			ctx = context.Background()
		}

		// Get integer number from query params
		countInt := c.QueryInt("count")

		// Return an error if count from query is invalid
		if countInt == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid count, it should be a number"})
		}

		// Add items in cart
		appMetrics.AddItemsInCart(ctx, int64(countInt))
		return c.JSON(fiber.Map{"status": "ok", "message": "Added items", "updated_items": appMetrics.GetTotalItemsInCart()})
	})

	// Endpoint to reduce items in cart, makes use of query parameters
	// GET <> Endpoint: /cart/reduce?count=integer_number
	app.Get("/cart/reduce", func(c *fiber.Ctx) error {
		ctx := c.UserContext()
		if ctx == nil {
			ctx = context.Background()
		}

		// Get integer number from query params
		countInt := c.QueryInt("count")

		// Return an error if count from query is invalid
		if countInt == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid count, it should be a number"})
		}

		// Remove items in cart
		appMetrics.ReduceItemsInCart(ctx, int64(countInt))
		return c.JSON(fiber.Map{"status": "ok", "message": "Reduced items", "updated_items": appMetrics.GetTotalItemsInCart()})
	})

	// Start our API Server
	log.Println("Starting API Server on 3100")
	log.Fatal(app.Listen(":3100"))
}
