package middleware

import (
	"fmt"

	"crochet/telemetry"

	"github.com/gin-gonic/gin"
)

// SetupGlobalMiddleware applies all standard middleware to a Gin router
// This provides a convenient way to ensure consistent middleware usage
// across all Gin-based services
func SetupGlobalMiddleware(router *gin.Engine, serviceName string) {
	// Set the service name for use in middleware
	router.Use(func(c *gin.Context) {
		c.Set("service-name", serviceName)
		c.Next()
	})

	// Apply standard middleware in recommended order
	router.Use(Recovery())           // First to catch panics
	router.Use(Logger())             // Log all requests
	router.Use(ErrorHandler())       // Handle errors
	router.Use(Tracing(serviceName)) // Add tracing last to ensure it captures everything
}

// SetupCommonComponents initializes common components for a service and returns
// a configured Gin router ready for route registration.
// This reduces boilerplate in service main functions.
func SetupCommonComponents(serviceName string, jaegerEndpoint string) (*gin.Engine, *telemetry.TracerProvider, error) {
	// Initialize tracer
	tp, err := telemetry.InitTracer(serviceName, jaegerEndpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize OpenTelemetry: %w", err)
	}

	// Create router and setup middleware
	router := gin.New()
	SetupGlobalMiddleware(router, serviceName)

	// Return the router ready for the service to add routes
	return router, tp, nil
}
