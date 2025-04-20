# Health Check Package 

This package provides a standardized way to implement health checks across all services in the Crochet project.

## Features

- Common health check implementation that can be reused across services
- Built-in support for OpenTelemetry tracing
- Default checks for memory usage and service uptime
- Easy to add custom health checks for dependencies or other service components
- JSON response format with detailed health status

## Usage Example

Here's how to integrate the health check package into a service:

```go
import (
	"crochet/health"
	"net/http"
)

func main() {
	// Create health check instance with service details
	healthCheck := health.New(health.Options{
		ServiceName: "context-service",
		Version:     "1.0.0",
		Details: map[string]string{
			"description": "Manages context data for the Crochet system",
		},
	})
	
	// Add dependency checks if needed
	healthCheck.AddDependencyCheck("jaeger", "http://jaeger:4317/health")
	
	// Add a custom check function
	healthCheck.AddCheck("database", func() (bool, string) {
		// Check database connection
		return true, "Connected"
	})
	
	// Register HTTP handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/input", handleInput)
	
	// Register health check handler
	healthCheck.RegisterHandler(mux, "/health")
	
	// Start the server
	http.ListenAndServe(":8081", mux)
}
```

## Example Implementation for Context Service

To update the Context service to use the new health check package:

1. Update the go.mod file to include the health package
2. Replace the current health check implementation with the new one
3. Add any service-specific health checks

See the documentation in the `health` package for more detailed information.