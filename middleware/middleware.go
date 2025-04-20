package middleware

import (
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
