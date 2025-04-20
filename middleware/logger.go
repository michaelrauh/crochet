package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger is a middleware for Gin that provides standardized request logging
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Format the raw query if it exists
		if raw != "" {
			path = path + "?" + raw
		}

		// Calculate latency
		latency := time.Since(start)

		// Get status code
		statusCode := c.Writer.Status()

		// Get client IP
		clientIP := c.ClientIP()

		// Get request method
		method := c.Request.Method

		// Log the request details
		log.Printf("[GIN] %s | %3d | %v | %s | %s",
			method,
			statusCode,
			latency,
			clientIP,
			path,
		)

		// Add trace information if errors occurred
		if len(c.Errors) > 0 {
			for _, e := range c.Errors.Errors() {
				log.Printf("[GIN-ERROR] %s", e)
			}
		}
	}
}
