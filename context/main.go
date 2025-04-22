package main

import (
	"log"
	"net/http"
	"time"

	"crochet/config"
	"crochet/health"
	"crochet/middleware"
	"crochet/telemetry"
	"crochet/types"

	"github.com/gin-gonic/gin"
)

// Global state
var store *types.ContextMemoryStore
var versionCounter int

func initStore() {
	store = types.InitContextStore()
	versionCounter = 1
	log.Println("In-memory store initialized successfully")
}

// Handle input from the ingestor service
func ginHandleInput(c *gin.Context) {
	log.Printf("Received request: %v", c.Request)

	var input types.ContextInput
	if err := c.ShouldBindJSON(&input); err != nil {
		telemetry.LogAndError(c, err, "context", "Invalid JSON format")
		return
	}

	// Process vocabulary
	newVocabulary := types.SaveVocabularyToStore(store, input.Vocabulary)

	// Process subphrases
	newSubphrases := types.SaveSubphrasesToStore(store, input.Subphrases)

	response := gin.H{
		"newVocabulary": newVocabulary,
		"newSubphrases": newSubphrases,
		"version":       versionCounter,
	}

	versionCounter++
	log.Printf("Sending response to ingestor: %v", response)
	log.Println("Flushing logs to ensure visibility")
	c.JSON(http.StatusOK, response)
}

// Handler for getting the current version
func ginGetVersion(c *gin.Context) {
	log.Printf("Received version request: %v", c.Request)

	response := gin.H{
		"version": versionCounter,
	}

	log.Printf("Sending version response: %v", response)
	c.JSON(http.StatusOK, response)
}

func main() {
	log.Println("Starting context service...")

	// Load configuration using the unified config package
	cfg, err := config.LoadContextConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Log configuration details
	log.Printf("Using unified configuration management: %+v", cfg)
	config.LogConfig(cfg.BaseConfig)

	// Set up common components using the updated shared helper with profiling
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

	initStore()

	// Register Gin routes
	router.POST("/input", ginHandleInput)
	router.GET("/version", ginGetVersion)

	// Set up health check
	healthCheck := health.New(health.Options{
		ServiceName: cfg.ServiceName,
		Version:     "1.0.0",
		Details: map[string]string{
			"description": "Manages context data for the Crochet system",
		},
	})

	// DO NOT add Jaeger dependency check since it's a gRPC service, not HTTP
	// Jaeger health should be monitored separately

	// Register health check handler in the gin router
	router.GET("/health", gin.WrapF(healthCheck.Handler()))

	address := cfg.GetAddress()
	log.Printf("Context service starting on %s...\n", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Context service failed to start: %v", err)
	}
}
