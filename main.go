package main

import (
	// Standard packages
	"context"
	"errors"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	// Internal packages
	"github.com/crazyuploader/fiber-otel/internal/metrics"
	"google.golang.org/grpc/credentials"

	// Import Fiber related packages
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"

	// Import OpenTelemetry related packages
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

// Prepare our environment variable(s)
var (
	serviceName  = os.Getenv("SERVICE_NAME")
	collectorURL = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	insecure     = os.Getenv("INSECURE_MODE")
)

// Function to initialize Meter Provider
func initMeter() (*sdkmetric.MeterProvider, error) {
	var secureOptions otlpmetricgrpc.Option

	if strings.ToLower(insecure) == "false" {
		secureOptions = otlpmetricgrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, ""))
	} else {
		secureOptions = otlpmetricgrpc.WithInsecure()
	}

	// Create an exporter
	exporter, err := otlpmetricgrpc.New(
		context.Background(),
		secureOptions,
		otlpmetricgrpc.WithEndpoint(collectorURL))
	if err != nil {
		return nil, err
	}

	// Create a Meter Provider
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(time.Second*5))),
		sdkmetric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
			attribute.String("environment", "demo"),
			attribute.String("language", "go"),
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
	appMetrics, err := metrics.NewMetric("fiber-api-server")
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
		appMetrics.ErrorCount.Add(ctx, 1, metric.WithAttributes(
			attribute.String("endpoint", "/error"),
			attribute.String("method", "GET"),
		))
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

		// Random int from 0 to 100
		source := rand.NewSource(time.Now().Unix())
		r := rand.New(source)
		number := r.Intn(100)

		// Sleep for a few milliseconds
		time.Sleep(time.Millisecond * time.Duration(number))

		// Get latest time
		duration := time.Since(initialTime).Seconds()

		// Record elapsed duration in metrics histogram
		appMetrics.RequestLatency.Record(ctx, float64(duration), metric.WithAttributes(
			attribute.String("endpoint", "/latency"),
			attribute.String("method", "GET"),
		))
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
