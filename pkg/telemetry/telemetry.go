package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type Params struct {
	ServiceName string
}

func NewTracerProvider(p Params) (*trace.TracerProvider, error) {
	exp, err := otlptracehttp.New(context.Background(), otlptracehttp.WithEndpoint("jaeger:4318"), otlptracehttp.WithInsecure())
	if err != nil {
		return nil, err
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(p.ServiceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exp),
		trace.WithResource(res),
	)

	return tp, nil
}

func RegisterGlobal(tp *trace.TracerProvider) {
	otel.SetTracerProvider(tp)
}

func init() {
	fmt.Println("In telemetry init")
}
