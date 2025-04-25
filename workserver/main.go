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

	queueDepthQueued = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "workserver_queue_depth_queued",
			Help: "Number of queued items waiting to be processed",
		},
	)

	queueDepthInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "workserver_queue_depth_in_flight",
			Help: "Number of items currently being processed",
		},
	)

	// Use gauge vectors for shape-specific metrics
	queueDepthByShape = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "workserver_queue_depth_shape",
			Help: "Number of items in the work queue by shape",
		},
		[]string{"shape"},
	)

	queueDepthQueuedByShape = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "workserver_queue_depth_queued_shape",
			Help: "Number of queued items waiting to be processed by shape",
		},
		[]string{"shape"},
	)

	queueDepthInFlightByShape = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "workserver_queue_depth_in_flight_shape",
			Help: "Number of items currently being processed by shape",
		},
		[]string{"shape"},
	)

	queueDepthQueuedByShapePosition = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "workserver_queue_depth_queued_shape_position",
			Help: "Number of queued items waiting to be processed by shape and position",
		},
		[]string{"shape", "position"},
	)

	// Add a new metric for tracking processing time by shape and position
	orthoProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "crochet_ortho_processing_duration_seconds",
			Help:    "Histogram of ortho processing duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"shape", "position"},
	)
)

func init() {
	// Register all metrics with Prometheus
	prometheus.MustRegister(queueDepthTotal)
	prometheus.MustRegister(queueDepthQueued)
	prometheus.MustRegister(queueDepthInFlight)
	prometheus.MustRegister(queueDepthByShape)
	prometheus.MustRegister(queueDepthQueuedByShape)
	prometheus.MustRegister(queueDepthInFlightByShape)
	prometheus.MustRegister(queueDepthQueuedByShapePosition)
	prometheus.MustRegister(orthoProcessingDuration)
}

// updateQueueMetrics periodically updates the queue metrics
func updateQueueMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Update total queue depth metrics
			queueDepthTotal.Set(float64(workQueue.Count()))
			queueDepthQueued.Set(float64(workQueue.CountQueued()))
			queueDepthInFlight.Set(float64(workQueue.CountInFlight()))

			// Update shape-specific metrics for all items
			shapeCounts := workQueue.CountByShape()
			queueDepthByShape.Reset()
			for shape, count := range shapeCounts {
				queueDepthByShape.WithLabelValues(shape).Set(float64(count))
			}

			// Update shape-specific metrics for queued items
			queuedShapeCounts := workQueue.CountQueuedByShape()
			queueDepthQueuedByShape.Reset()
			for shape, count := range queuedShapeCounts {
				queueDepthQueuedByShape.WithLabelValues(shape).Set(float64(count))
			}

			// Update shape-specific metrics for in-flight items
			inFlightShapeCounts := workQueue.CountInFlightByShape()
			queueDepthInFlightByShape.Reset()
			for shape, count := range inFlightShapeCounts {
				queueDepthInFlightByShape.WithLabelValues(shape).Set(float64(count))
			}

			// Update shape+position metrics for queued items
			queuedPositionCounts := workQueue.CountQueuedByShapeAndPosition()
			queueDepthQueuedByShapePosition.Reset()
			for key, count := range queuedPositionCounts {
				shape, position := key[0], key[1]
				log.Printf("Debug: Shape×Position metric: shape=%s, position=%s, count=%d",
					shape, position, count)
				queueDepthQueuedByShapePosition.WithLabelValues(shape, position).Set(float64(count))
			}

			log.Printf("Updated metrics: total=%d, queued=%d, in-flight=%d, shapes=%v",
				workQueue.Count(), workQueue.CountQueued(), workQueue.CountInFlight(), shapeCounts)
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
