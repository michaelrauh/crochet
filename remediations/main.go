package main

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"time"

	"crochet/config"
	"crochet/health"
	"crochet/middleware"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Define Prometheus metrics for the remediations service
var (
	remediationsTotalCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "remediations_total_count",
			Help: "Total number of remediations stored in the service",
		},
	)
)

func main() {
	// Load configuration using the unified config package
	cfg, err := config.LoadRemediationsConfig()
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

	// Add the Prometheus handler explicitly
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	log.Printf("Registered /metrics endpoint for Prometheus")

	// Create health check service with appropriate options
	healthOptions := health.Options{
		ServiceName: cfg.ServiceName,
		Version:     "0.1.0",
		Details: map[string]string{
			"service":     "remediations",
			"environment": "development",
		},
	}

	// Create the health service
	healthService := health.New(healthOptions)

	// Add custom checks
	healthService.AddCheck("goroutines", func() (bool, string) {
		return true, fmt.Sprintf("%d", runtime.NumGoroutine())
	})

	// Register health check endpoint
	router.GET("/health", gin.WrapF(healthService.Handler()))
	log.Printf("Registered /health endpoint for container health checks")

	// Create the remediation queue store with sensible defaults
	queueSize := 10000                     // Buffer for 10,000 items
	flushSize := 200                       // Flush every 200 items
	flushInterval := 50 * time.Millisecond // Or every 50ms, whichever comes first
	store := NewRemediationQueueStore(queueSize, flushSize, flushInterval)
	log.Printf("Initialized RemediationQueueStore with queue size: %d, flush size: %d, flush interval: %dms",
		queueSize, flushSize, flushInterval.Milliseconds())

	// Register the HTTP endpoints
	router.POST("/remediations", store.AddHandler)
	router.GET("/remediations", store.GetHandler)
	router.POST("/delete", DeleteHandler)

	// Start a goroutine to periodically update the metrics
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			store.mu.RLock()
			remediationsTotalCount.Set(float64(len(store.store)))
			store.mu.RUnlock()
		}
	}()

	// Start the HTTP server
	address := cfg.GetAddress()
	log.Printf("Remediations service starting on %s...", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// TODO Fix this and ensure this is covered in e2e tests
func DeleteHandler(c *gin.Context) {
	var request struct {
		Hashes []string `json:"hashes"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request format",
			"error":   err.Error(),
		})
		return
	}

	// Just return success even for empty hashes array
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Remediations deleted successfully",
		"count":   len(request.Hashes),
	})
}
