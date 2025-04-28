package telemetry

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	// Gin framework for HTTP handling
	"github.com/gin-gonic/gin"
)

type ServiceError struct {
	Code    int    // HTTP status code
	Message string // Error message
	Service string // Service that generated the error
}

func (e *ServiceError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Service, e.Message)
}

func NewServiceError(service string, code int, message string) *ServiceError {
	return &ServiceError{
		Service: service,
		Code:    code,
		Message: message,
	}
}

func LogAndError(c *gin.Context, err error, serviceName string, message string) bool {
	if err == nil {
		return false
	}

	log.Printf("%s: %v", message, err)

	if serviceErr, ok := err.(*ServiceError); ok {
		c.Error(serviceErr)
	} else {
		c.Error(NewServiceError(serviceName, http.StatusInternalServerError, message))
	}

	return true
}

func GinErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last()

			if serviceErr, ok := err.Err.(*ServiceError); ok {
				c.JSON(serviceErr.Code, gin.H{
					"error":   serviceErr.Message,
					"service": serviceErr.Service,
				})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   err.Error(),
				"service": "unknown",
			})
			return
		}
	}
}

func WriteJSONError(w http.ResponseWriter, serviceName string, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	response := map[string]interface{}{
		"error":   message,
		"service": serviceName,
	}

	json.NewEncoder(w).Encode(response)
}
