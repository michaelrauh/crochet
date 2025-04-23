package main

import (
	"context"
	"crochet/config"
	"crochet/health"
	"crochet/middleware"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Global queue instance
var workQueue *WorkQueue

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// config key constant
const configKey contextKey = "config"

// Custom metrics for the workserver
var (
	queueDepthTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "workserver_queue_depth_total",
			Help: "Total number of items in the work queue",
		},
	)

	// Use a gauge vector instead of individual gauges for each shape
	queueDepthByShape = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "workserver_queue_depth_shape",
			Help: "Number of items in the work queue by shape",
		},
		[]string{"shape"},
	)
)

func init() {
	// Register all metrics with Prometheus
	prometheus.MustRegister(queueDepthTotal)
	prometheus.MustRegister(queueDepthByShape)
}

// updateQueueMetrics periodically updates the queue metrics
func updateQueueMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Update total queue depth
			queueDepthTotal.Set(float64(workQueue.Count()))

			// Update shape-specific metrics
			shapeCounts := workQueue.CountByShape()

			// Reset all shape gauges to avoid stale metrics
			queueDepthByShape.Reset()

			// Set metrics for all shapes in the queue
			for shape, count := range shapeCounts {
				queueDepthByShape.WithLabelValues(shape).Set(float64(count))
			}

			log.Printf("Updated metrics with queue data: %v", shapeCounts)
		}
	}
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

	// Start metrics update goroutine
	go updateQueueMetrics()

	// Create a router using the middleware package's functionality
	router, _, _, _, err := middleware.SetupCommonComponents(
		cfg.ServiceName,
		cfg.JaegerEndpoint,
		cfg.MetricsEndpoint,
		cfg.PyroscopeEndpoint,
	)

	if err != nil {
		log.Fatalf("Failed to setup common components: %v", err)
	}

	// Add the Prometheus handler explicitly
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

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

	// Start the server
	address := cfg.GetAddress()
	log.Printf("Workserver starting on %s...\n", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
