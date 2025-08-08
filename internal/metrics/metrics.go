// internal/metrics/metrics.go
package metrics

import (
	// Standard packages
	"context"
	"sync/atomic"

	// Import OpenTelemetry related packages
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Define metrics struct with our custom metrics
type Metrics struct {
	// Different types of metrics used by the API Server
	ErrorCount     metric.Int64Counter
	RequestLatency metric.Float64Histogram
	CartItems      metric.Int64Observable

	// Store items count in a variable locally
	totalCartItems int64
}

// Function to initialize a meter provider, with our custom metrics
func NewMetric(meterName string) (*Metrics, error) {
	meter := otel.Meter(meterName)

	// Add a counter for storing error count
	errorCount, err := meter.Int64Counter("showcase.fiber.api.requests_errors_total",
		metric.WithDescription("Total number of errors encountered by the API Server"))
	if err != nil {
		return nil, err
	}

	// Add a histogram for aggregating response time over the time
	requestLatency, err := meter.Float64Histogram("showcase.fiber.http.request_duration_seconds",
		metric.WithDescription("Duration of HTTP requests in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
		))
	if err != nil {
		return nil, err
	}

	// Add a gauge type metric for storing total number of items in our cart
	cartItems, err := meter.Int64ObservableGauge("showcase.fiber.user.cart_items_count",
		metric.WithDescription("Total items of user cart in the API Server"))
	if err != nil {
		return nil, err
	}

	// Ready our metrics struct
	m := &Metrics{
		ErrorCount:     errorCount,
		RequestLatency: requestLatency,
		CartItems:      cartItems,
	}

	// Invoke a callback function for handling our observable gauge
	_, err = meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		currentValue := atomic.LoadInt64(&m.totalCartItems)
		observer.ObserveInt64(m.CartItems, currentValue)
		return nil
	}, cartItems)

	// Return meter provider
	return m, err
}

// Function to set items in cart initially
func (m *Metrics) SetItemsInCart(ctx context.Context, value int64) {
	atomic.StoreInt64(&m.totalCartItems, value)
}

// Function to add items in cart
func (m *Metrics) AddItemsInCart(ctx context.Context, value int64) {
	atomic.AddInt64(&m.totalCartItems, value)
}

// Function to reduce items in cart
func (m *Metrics) ReduceItemsInCart(ctx context.Context, value int64) {
	atomic.AddInt64(&m.totalCartItems, -value)
}

// Function to get total items in cart
func (m *Metrics) GetTotalItemsInCart() int64 {
	return atomic.LoadInt64(&m.totalCartItems)
}
