// Package telemetry provides shared OpenTelemetry functionality
package telemetry

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// StandardMetricNames contains common metric names used across services
type StandardMetricNames struct {
	RequestDuration    string
	RequestCount       string
	ErrorCount         string
	ProcessingDuration string
}

// NewStandardMetricNames creates a standardized set of metric names with service name prefix
func NewStandardMetricNames(servicePrefix string) *StandardMetricNames {
	// Use snake_case format with _total suffix for counters to match Prometheus conventions
	return &StandardMetricNames{
		RequestDuration:    fmt.Sprintf("crochet_%s_request_duration_seconds", servicePrefix),
		RequestCount:       fmt.Sprintf("crochet_%s_request_count_total", servicePrefix),
		ErrorCount:         fmt.Sprintf("crochet_%s_error_count_total", servicePrefix),
		ProcessingDuration: fmt.Sprintf("crochet_%s_processing_duration_seconds", servicePrefix),
	}
}

// MeterProvider is a struct that wraps the OpenTelemetry MeterProvider
type MeterProvider struct {
	provider *sdkmetric.MeterProvider
}

// TODO fix
func InitMeter(serviceName string, metricsEndpoint string) (*MeterProvider, error) {
	log.Printf("Initializing OpenTelemetry metrics for service: %s with endpoint: %s", serviceName, metricsEndpoint)

	ctx := context.Background()

	// Create a resource describing the service
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Debug log for troubleshooting
	log.Printf("Creating OTLP metrics exporter with endpoint: %s", metricsEndpoint)

	// Create OTLP exporter to send metrics to the collector
	exporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(metricsEndpoint),
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithTimeout(10*time.Second), // Add timeout for connection attempts
	)
	if err != nil {
		log.Printf("Failed to create OTLP metrics exporter: %v", err)
		log.Printf("Endpoint attempted: %s", metricsEndpoint)
		log.Printf("Continuing with a no-op exporter")
		// Create a meter provider without an exporter if we can't connect to the collector
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
		)
		otel.SetMeterProvider(mp)
		return &MeterProvider{provider: mp}, nil
	}

	log.Printf("OTLP metrics exporter created successfully")

	// Create MeterProvider with the exporter
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				exporter,
				sdkmetric.WithInterval(1*time.Second),
			),
		),
	)

	// Set global MeterProvider
	otel.SetMeterProvider(mp)

	log.Printf("OpenTelemetry metrics successfully initialized for service: %s", serviceName)
	return &MeterProvider{provider: mp}, nil
}

// Meter returns a named meter for creating instruments
func (mp *MeterProvider) Meter(name string) metric.Meter {
	log.Printf("Creating meter with name: %s", name)
	return mp.provider.Meter(name)
}

// Shutdown gracefully shuts down the meter provider
func (mp *MeterProvider) Shutdown(ctx context.Context) error {
	if mp.provider == nil {
		return nil
	}
	return mp.provider.Shutdown(ctx)
}

// ShutdownWithTimeout is a convenience method to shutdown the provider with a timeout
func (mp *MeterProvider) ShutdownWithTimeout(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := mp.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down meter provider: %v", err)
	}
}
