package main

import (
	"crochet/config"
	"crochet/health"
	"crochet/middleware"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// OrthosConfig holds configuration specific to the orthos server
type OrthosConfig struct {
	config.BaseConfig
}

// OrthosService implements the health.ServiceChecker interface
type OrthosService struct{}

// Check implements the health.ServiceChecker interface
func (s *OrthosService) Check() error {
	// For now, this is a simple health check that always passes
	return nil
}

// GetName implements the health.ServiceChecker interface
func (s *OrthosService) GetName() string {
	return "orthos"
}
func handleRoot(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Hello World",
	})
}
func main() {
	// Load configuration
	var cfg OrthosConfig
	// Set service name before loading config
	cfg.ServiceName = "orthos"
	// Process environment variables with the appropriate prefix
	if err := config.LoadConfig("ORTHOS", &cfg); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	// Log configuration details
	log.Printf("Using configuration: %+v", cfg)
	config.LogConfig(cfg.BaseConfig)
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
	// Create health check service with appropriate options
	healthOptions := health.Options{
		ServiceName: cfg.ServiceName,
		Version:     "0.1.0", // Set a version
		Details: map[string]string{
			"environment": "development",
		},
	}
	// Create the health service
	healthService := health.New(healthOptions)
	// Register health check endpoint
	router.GET("/health", gin.WrapF(healthService.Handler()))
	// Register routes
	router.GET("/", handleRoot)
	// Start the server
	address := cfg.GetAddress()
	log.Printf("Orthos server starting on %s...\n", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
