package health

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

// responseWriter is a wrapper around http.ResponseWriter that captures status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code and passes it to the underlying ResponseWriter
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// WithTracing wraps a health check handler with OpenTelemetry tracing
func WithTracing(h http.HandlerFunc, serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract trace context from incoming request
		ctx := r.Context()
		propagator := otel.GetTextMapPropagator()
		ctx = propagator.Extract(ctx, propagation.HeaderCarrier(r.Header))

		// Create a span for the health check
		tracer := otel.Tracer(serviceName)
		ctx, span := tracer.Start(ctx, "healthCheck")
		defer span.End()

		// Record the start time
		startTime := time.Now()

		// Create a wrapped response writer to capture status code
		wrappedWriter := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // Default status code
		}

		// Call the original handler
		h(wrappedWriter, r.WithContext(ctx))

		// Record metrics and attributes
		duration := time.Since(startTime)
		span.SetAttributes(
			attribute.Int64("health_check.duration_ms", duration.Milliseconds()),
			attribute.Int("http.status_code", wrappedWriter.statusCode),
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.String()),
			attribute.String("service.name", serviceName),
		)

		// Set span status based on HTTP status code
		if wrappedWriter.statusCode >= 200 && wrappedWriter.statusCode < 300 {
			span.SetStatus(codes.Ok, "Success")
		} else {
			span.SetStatus(codes.Error, "Health check failure")
		}
	}
}
