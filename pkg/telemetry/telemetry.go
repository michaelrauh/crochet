package telemetry

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
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

var (
	pingCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ping_requests_total",
		Help: "Total number of /ping requests received.",
	})

	RabbitMQQueueDepth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rabbitmq_queue_depth",
			Help: "Current depth of RabbitMQ queues.",
		},
		[]string{"queue"},
	)
)

func init() {
	prometheus.MustRegister(pingCounter)
	prometheus.MustRegister(RabbitMQQueueDepth)
}

func IncPingCounter() {
	pingCounter.Inc()
}
