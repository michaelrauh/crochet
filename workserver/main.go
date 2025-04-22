package main

import (
	"context"
	"crochet/config"
	"crochet/health"
	"crochet/middleware"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// Global queue instance
var workQueue *WorkQueue

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// config key constant
const configKey contextKey = "config"

// setupRoutes configures the API endpoints
func setupRoutes(router *gin.Engine, cfg *config.WorkServerConfig) {
	// Register health check endpoint
	healthOptions := health.Options{
		ServiceName: cfg.ServiceName,
		Version:     "0.1.0",
		Details: map[string]string{
			"environment": "development",
		},
	}

	// Create the health service
	healthService := health.New(healthOptions)

	// Note: Not adding any dependency checks to avoid failures during Docker health checks

	router.GET("/health", gin.WrapF(healthService.Handler()))

	// Register API routes
	router.GET("/", handleRoot)
	router.POST("/push", func(c *gin.Context) {
		// Add config to context for potential future use
		ctxWithConfig := context.WithValue(c.Request.Context(), configKey, cfg)
		c.Request = c.Request.WithContext(ctxWithConfig)
		handlePush(c)
	})
	router.POST("/pop", func(c *gin.Context) {
		ctxWithConfig := context.WithValue(c.Request.Context(), configKey, cfg)
		c.Request = c.Request.WithContext(ctxWithConfig)
		handlePop(c)
	})
	router.POST("/ack", func(c *gin.Context) {
		ctxWithConfig := context.WithValue(c.Request.Context(), configKey, cfg)
		c.Request = c.Request.WithContext(ctxWithConfig)
		handleAck(c)
	})
	router.POST("/nack", func(c *gin.Context) {
		ctxWithConfig := context.WithValue(c.Request.Context(), configKey, cfg)
		c.Request = c.Request.WithContext(ctxWithConfig)
		handleNack(c)
	})
}

func main() {
	// Load configuration using the helper
	cfg, err := config.LoadWorkServerConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Log configuration details
	log.Printf("Using configuration: %+v", cfg)
	config.LogConfig(cfg.BaseConfig)

	// Initialize work queue with timeout from config
	workQueue = NewWorkQueue(cfg.RequeueTimeoutSeconds)

	// Set up common components using the shared helper
	router, tp, mp, pp, err := middleware.SetupCommonComponents(
		cfg.ServiceName,
		cfg.JaegerEndpoint,
		cfg.MetricsEndpoint,
		cfg.PyroscopeEndpoint,
	)
	if err != nil {
		log.Fatalf("Failed to set up application: %v", err)
	}

	// Ensure resources are properly cleaned up
	defer tp.ShutdownWithTimeout(5 * time.Second)
	if mp != nil {
		defer mp.ShutdownWithTimeout(5 * time.Second)
	}
	if pp != nil {
		defer pp.StopWithTimeout(5 * time.Second)
	}

	// Set up routes
	setupRoutes(router, cfg)

	// Start the server
	address := cfg.GetAddress()
	log.Printf("Workserver starting on %s...\n", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
