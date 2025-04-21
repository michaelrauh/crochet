package middleware

import (
	"fmt"

	"crochet/telemetry"

	"github.com/gin-gonic/gin"
)

// SetupGlobalMiddleware applies all standard middleware to a Gin router
// This provides a convenient way to ensure consistent middleware usage
// across all Gin-based services
func SetupGlobalMiddleware(router *gin.Engine, serviceName string, mp *telemetry.MeterProvider) {
	// Set the service name for use in middleware
	router.Use(func(c *gin.Context) {
		c.Set("service-name", serviceName)
		c.Next()
	})

	// Apply standard middleware in recommended order
	router.Use(Recovery())           // First to catch panics
	router.Use(Logger())             // Log all requests
	router.Use(ErrorHandler())       // Handle errors
	router.Use(Tracing(serviceName)) // Add tracing

	// Add metrics middleware if a meter provider is provided
	if mp != nil {
		router.Use(Metrics(serviceName, mp)) // Add metrics collection
	}
}

// SetupCommonComponents initializes common components for a service and returns
// a configured Gin router ready for route registration.
// This reduces boilerplate in service main functions.
func SetupCommonComponents(serviceName string, jaegerEndpoint string, metricsEndpoint string, pyroscopeEndpoint string) (*gin.Engine, *telemetry.TracerProvider, *telemetry.MeterProvider, *telemetry.ProfilerProvider, error) {
	// Initialize tracer
	tp, err := telemetry.InitTracer(serviceName, jaegerEndpoint)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to initialize OpenTelemetry tracer: %w", err)
	}

	// Initialize meter if metrics endpoint is provided
	var mp *telemetry.MeterProvider
	if metricsEndpoint != "" {
		mp, err = telemetry.InitMeter(serviceName, metricsEndpoint)
		if err != nil {
			// Log error but continue without metrics
			fmt.Printf("Failed to initialize OpenTelemetry metrics: %v\n", err)
		}
	}

	// Initialize profiler with pyroscope endpoint
	var pp *telemetry.ProfilerProvider
	if pyroscopeEndpoint != "" {
		fmt.Printf("Initializing Pyroscope profiler for service %s with endpoint %s\n", serviceName, pyroscopeEndpoint)
		pp, err = telemetry.InitProfiler(serviceName, pyroscopeEndpoint)
		if err != nil {
			// Log error but continue without profiling
			fmt.Printf("Failed to initialize Pyroscope profiler: %v\n", err)
		} else {
			fmt.Printf("Pyroscope profiler successfully initialized for service %s\n", serviceName)
		}
	}

	// Create router and setup middleware
	router := gin.New()
	SetupGlobalMiddleware(router, serviceName, mp)

	// Return the router ready for the service to add routes
	return router, tp, mp, pp, nil
}

// SetupCommonComponentsLegacy maintains backward compatibility
// Can be removed after all services are updated to use the new method
func SetupCommonComponentsLegacy(serviceName string, jaegerEndpoint string, metricsEndpoint string) (*gin.Engine, *telemetry.TracerProvider, *telemetry.MeterProvider, error) {
	router, tp, mp, _, err := SetupCommonComponents(serviceName, jaegerEndpoint, metricsEndpoint, "")
	return router, tp, mp, err
}
