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

type StandardMetricNames struct {
	RequestDuration    string
	RequestCount       string
	ErrorCount         string
	ProcessingDuration string
}

func NewStandardMetricNames(servicePrefix string) *StandardMetricNames {
	return &StandardMetricNames{
		RequestDuration:    fmt.Sprintf("crochet_%s_request_duration_seconds", servicePrefix),
		RequestCount:       fmt.Sprintf("crochet_%s_request_count_total", servicePrefix),
		ErrorCount:         fmt.Sprintf("crochet_%s_error_count_total", servicePrefix),
		ProcessingDuration: fmt.Sprintf("crochet_%s_processing_duration_seconds", servicePrefix),
	}
}

type MeterProvider struct {
	provider *sdkmetric.MeterProvider
}

func InitMeter(serviceName string, metricsEndpoint string) (*MeterProvider, error) {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	exporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(metricsEndpoint),
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithTimeout(10*time.Second),
	)
	if err != nil {
		log.Printf("Failed to create OTLP metrics exporter: %v", err)
		log.Printf("Endpoint attempted: %s", metricsEndpoint)
		log.Printf("Continuing with a no-op exporter")

		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
		)
		otel.SetMeterProvider(mp)
		return &MeterProvider{provider: mp}, nil
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				exporter,
				sdkmetric.WithInterval(1*time.Second),
			),
		),
	)

	otel.SetMeterProvider(mp)

	return &MeterProvider{provider: mp}, nil
}

func (mp *MeterProvider) Meter(name string) metric.Meter {
	log.Printf("Creating meter with name: %s", name)
	return mp.provider.Meter(name)
}

func (mp *MeterProvider) Shutdown(ctx context.Context) error {
	if mp.provider == nil {
		return nil
	}
	return mp.provider.Shutdown(ctx)
}

func (mp *MeterProvider) ShutdownWithTimeout(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := mp.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down meter provider: %v", err)
	}
}
