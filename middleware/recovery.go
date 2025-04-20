package middleware

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery is a middleware for Gin that recovers from panics
// and returns a 500 error response with the error message
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the stack trace
				log.Printf("[PANIC RECOVERED] %v\n%s", err, debug.Stack())

				// Return a 500 error
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":   fmt.Sprintf("Internal Server Error: %v", err),
					"service": c.GetString("service-name"),
				})
			}
		}()

		c.Next()
	}
}
