package middleware

import (
	"log"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// Tracing is a middleware for Gin that adds OpenTelemetry tracing
// This is a wrapper around the otelgin middleware to make it
// consistent with our other middleware naming conventions
func Tracing(serviceName string) gin.HandlerFunc {
	log.Printf("Initializing OpenTelemetry tracing middleware for service: %s", serviceName)
	return otelgin.Middleware(serviceName)
}
