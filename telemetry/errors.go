package telemetry

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	// Gin framework for HTTP handling
	"github.com/gin-gonic/gin"
)

// ServiceError is a common error type for handling HTTP errors with status codes
type ServiceError struct {
	Code    int    // HTTP status code
	Message string // Error message
	Service string // Service that generated the error
}

// Error implements the error interface
func (e *ServiceError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Service, e.Message)
}

// NewServiceError creates a new ServiceError
func NewServiceError(service string, code int, message string) *ServiceError {
	return &ServiceError{
		Service: service,
		Code:    code,
		Message: message,
	}
}

// TODO: Fix
func LogAndError(c *gin.Context, err error, serviceName string, message string) bool {
	if err == nil {
		return false
	}

	// Log the error
	if message != "" {
		log.Printf("%s: %v", message, err)
	} else {
		log.Printf("%v", err)
	}

	// If it's already a ServiceError, use it directly; otherwise, wrap it
	if serviceErr, ok := err.(*ServiceError); ok {
		c.Error(serviceErr)
	} else {
		c.Error(NewServiceError(serviceName, http.StatusInternalServerError, message))
	}

	return true
}

// ErrorHandler is a middleware for Gin that handles custom errors
func GinErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process request
		c.Next()

		// Check for errors
		if len(c.Errors) > 0 {
			// Get the last error
			err := c.Errors.Last()

			// Check if it's our custom error
			if serviceErr, ok := err.Err.(*ServiceError); ok {
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

// WriteJSONError sends a standardized error response for standard http handlers
func WriteJSONError(w http.ResponseWriter, serviceName string, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	response := map[string]interface{}{
		"error":   message,
		"service": serviceName,
	}

	json.NewEncoder(w).Encode(response)
}
