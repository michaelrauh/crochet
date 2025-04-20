package middleware

import (
	"net/http"

	"crochet/telemetry"

	"github.com/gin-gonic/gin"
)

// ErrorHandler is a middleware for Gin that handles custom errors
// This is moved from telemetry package to middleware package for better organization
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process request
		c.Next()

		// Check for errors
		if len(c.Errors) > 0 {
			// Get the last error
			err := c.Errors.Last()

			// Check if it's our custom error
			if serviceErr, ok := err.Err.(*telemetry.ServiceError); ok {
				c.JSON(serviceErr.Code, gin.H{
					"error":   serviceErr.Message,
					"service": serviceErr.Service,
				})
				return
			}

			// Handle standard errors
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   err.Error(),
				"service": "unknown",
			})
			return
		}
	}
}
