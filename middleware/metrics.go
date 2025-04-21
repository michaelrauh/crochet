package middleware

import (
	"time"

	"crochet/telemetry"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics middleware adds OpenTelemetry metrics for all HTTP requests
func Metrics(serviceName string, mp *telemetry.MeterProvider) gin.HandlerFunc {
	standardMetrics := telemetry.NewStandardMetricNames(serviceName)
	meter := mp.Meter(serviceName + ".http")

	// Create instruments
	requestCounter, _ := meter.Int64Counter(standardMetrics.RequestCount,
		metric.WithDescription("Number of HTTP requests"))

	requestDuration, _ := meter.Float64Histogram(standardMetrics.RequestDuration,
		metric.WithDescription("Duration of HTTP requests in milliseconds"))

	errorCounter, _ := meter.Int64Counter(standardMetrics.ErrorCount,
		metric.WithDescription("Number of HTTP request errors"))

	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// Common attributes for all metrics
		attrs := []attribute.KeyValue{
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.path", path),
		}

		// Process the request
		c.Next()

		// Record request count
		requestCounter.Add(c.Request.Context(), 1, metric.WithAttributes(attrs...))

		// Record latency
		duration := float64(time.Since(start)) / float64(time.Millisecond)
		requestDuration.Record(c.Request.Context(), duration, metric.WithAttributes(attrs...))

		// Record error count if there was an error
		statusCode := c.Writer.Status()
		if statusCode >= 400 {
			errorAttrs := append(attrs, attribute.Int("http.status_code", statusCode))
			errorCounter.Add(c.Request.Context(), 1, metric.WithAttributes(errorAttrs...))
		}
	}
}
