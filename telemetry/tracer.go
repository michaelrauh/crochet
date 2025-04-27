// Package telemetry provides shared OpenTelemetry tracing functionality
package telemetry

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

// TracerProvider is a struct that wraps the OpenTelemetry TracerProvider
type TracerProvider struct {
	provider *tracesdk.TracerProvider
}

// TODO fix
func InitTracer(serviceName string, jaegerEndpoint string) (*TracerProvider, error) {
	// Create OTLP exporter
	ctx := context.Background()
	client := otlptracegrpc.NewClient(
		otlptracegrpc.WithEndpoint(jaegerEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create TracerProvider with the exporter
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exporter),
		tracesdk.WithResource(resource.NewWithAttributes(
			"",
			attribute.String("service.name", serviceName),
		)),
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
	)

	// Set global TracerProvider and TextMapPropagator
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Printf("OpenTelemetry tracer initialized for service: %s", serviceName)

	return &TracerProvider{provider: tp}, nil
}

// Shutdown gracefully shuts down the tracer provider
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if tp.provider == nil {
		return nil
	}

	return tp.provider.Shutdown(ctx)
}

// ShutdownWithTimeout is a convenience method to shutdown the provider with a timeout
func (tp *TracerProvider) ShutdownWithTimeout(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := tp.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down tracer provider: %v", err)
	}
}
