package main

import (
	"crochet/config"
	"crochet/health"
	"crochet/middleware"
	"crochet/telemetry"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// OrthosConfig holds configuration specific to the orthos server
type OrthosConfig struct {
	config.BaseConfig
	QueueFlushInterval int `env:"QUEUE_FLUSH_INTERVAL" envDefault:"15"` // Seconds between queue flushes
	QueueBatchSize     int `env:"QUEUE_BATCH_SIZE" envDefault:"100"`    // Number of items to process in a batch
}

// Ortho represents the structure for orthogonal data
type Ortho struct {
	Grid     map[string]string `json:"grid"`
	Shape    []int             `json:"shape"`
	Position []int             `json:"position"`
	Shell    int               `json:"shell"`
	ID       string            `json:"id"`
}

// OrthosRequest represents a request containing multiple Ortho objects
type OrthosRequest struct {
	Orthos []Ortho `json:"orthos"`
}

// OrthosGetRequest represents a request to fetch orthos by IDs
type OrthosGetRequest struct {
	IDs []string `json:"ids"`
}

// Custom metrics for the orthos service
var (
	orthosTotalCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "orthos_total_count",
			Help: "Total number of orthos stored in the service",
		},
	)
	orthosCountByShape = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "orthos_count_by_shape",
			Help: "Number of orthos stored by shape",
		},
		[]string{"shape"},
	)
	orthosCountByLocation = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "orthos_count_by_location",
			Help: "Number of orthos stored by location within shape",
		},
		[]string{"shape", "position"},
	)
	orthosQueueDepth = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "orthos_queue_depth",
			Help: "Number of orthos waiting to be stored permanently",
		},
	)
	orthosFastPathHits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "orthos_fast_path_hits",
			Help: "Number of orthos that were handled by the fast path",
		},
	)
	orthosPermanentStoreAdds = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "orthos_permanent_store_adds",
			Help: "Number of orthos added to permanent storage",
		},
	)
	orthosFlushDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "orthos_flush_duration_seconds",
			Help:    "Time taken to flush the orthos queue in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)
	// New metrics for tracking accepted orthos by shape and position
	orthosAcceptedByShapePosition = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orthos_accepted_by_shape_position_total",
			Help: "Number of new orthos accepted (non-duplicate) by shape and position",
		},
		[]string{"shape", "position"},
	)
	// New metrics for tracking rejected orthos by shape and position
	orthosRejectedByShapePosition = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orthos_rejected_by_shape_position_total",
			Help: "Number of orthos rejected (duplicate) by shape and position",
		},
		[]string{"shape", "position"},
	)
)

func init() {
	// Register metrics with Prometheus
	prometheus.MustRegister(orthosTotalCount)
	prometheus.MustRegister(orthosCountByShape)
	prometheus.MustRegister(orthosCountByLocation)
	prometheus.MustRegister(orthosQueueDepth)
	prometheus.MustRegister(orthosFastPathHits)
	prometheus.MustRegister(orthosPermanentStoreAdds)
	prometheus.MustRegister(orthosFlushDuration)
	prometheus.MustRegister(orthosAcceptedByShapePosition)
	prometheus.MustRegister(orthosRejectedByShapePosition)
}

// updateOrthoMetrics updates the orthos metrics
func updateOrthoMetrics(storage *OrthosStorage) {
	// Get current orthos count by shape and location
	shapeCounts, locationCounts := storage.CountOrthosByShapeAndLocation()

	// Set total count
	totalCount := len(storage.orthos)
	orthosTotalCount.Set(float64(totalCount))

	// Reset metrics to avoid stale metrics
	orthosCountByShape.Reset()
	orthosCountByLocation.Reset()

	// Set shape-specific metrics
	for shape, count := range shapeCounts {
		orthosCountByShape.WithLabelValues(shape).Set(float64(count))
	}

	// Set location-specific metrics
	for key, count := range locationCounts {
		shape, position := key[0], key[1]
		orthosCountByLocation.WithLabelValues(shape, position).Set(float64(count))
	}

	// Add more detailed logging
	log.Printf("Updated orthos metrics: total=%d, shapes=%d, locations=%d",
		totalCount, len(shapeCounts), len(locationCounts))
}

// OrthoQueue represents a queue for background processing of orthos
type OrthoQueue struct {
	queue []Ortho
	mutex sync.RWMutex
}

// NewOrthoQueue creates a new queue for background processing
func NewOrthoQueue() *OrthoQueue {
	return &OrthoQueue{
		queue: make([]Ortho, 0),
	}
}

// Add adds an ortho to the queue
func (q *OrthoQueue) Add(ortho Ortho) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.queue = append(q.queue, ortho)
	// Update the queue depth metric
	orthosQueueDepth.Set(float64(len(q.queue)))
}

// GetBatch retrieves a batch of orthos from the queue
func (q *OrthoQueue) GetBatch(batchSize int) []Ortho {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if len(q.queue) == 0 {
		return []Ortho{}
	}

	// Determine how many items to take
	count := batchSize
	if count > len(q.queue) {
		count = len(q.queue)
	}

	// Get the batch
	batch := q.queue[:count]

	// Remove the batch from the queue
	q.queue = q.queue[count:]

	// Update the queue depth metric
	orthosQueueDepth.Set(float64(len(q.queue)))

	return batch
}

// Size returns the current size of the queue
func (q *OrthoQueue) Size() int {
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	return len(q.queue)
}

// Clear empties the queue and returns all items
func (q *OrthoQueue) Clear() []Ortho {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	// Get all items
	items := q.queue

	// Clear the queue
	q.queue = make([]Ortho, 0)

	// Update the queue depth metric
	orthosQueueDepth.Set(0)

	return items
}

// OrthosStorage provides thread-safe storage for orthos
type OrthosStorage struct {
	orthos           map[string]Ortho // Permanent storage
	idSet            map[string]bool  // Fast lookup set
	permanentMutex   sync.RWMutex     // For permanent storage
	idSetMutex       sync.RWMutex     // For ID set
	queue            *OrthoQueue      // Queue for background processing
	queueFlushSignal chan struct{}    // Signal channel for manual queue flushing
}

// NewOrthosStorage creates a new orthos storage
func NewOrthosStorage() *OrthosStorage {
	return &OrthosStorage{
		orthos:           make(map[string]Ortho),
		idSet:            make(map[string]bool),
		queue:            NewOrthoQueue(),
		queueFlushSignal: make(chan struct{}, 1), // Buffered channel to avoid blocking
	}
}

// CountOrthosByShape returns a count of orthos grouped by shape
func (s *OrthosStorage) CountOrthosByShape() map[string]int {
	s.permanentMutex.RLock()
	defer s.permanentMutex.RUnlock()
	shapeCounts := make(map[string]int)
	for _, ortho := range s.orthos {
		// Simply use the string representation of the shape array as the key
		shapeKey := fmt.Sprintf("%v", ortho.Shape)
		shapeCounts[shapeKey]++
	}
	log.Printf("CountOrthosByShape: counted %d different shapes from %d total orthos",
		len(shapeCounts), len(s.orthos))
	return shapeCounts
}

// CountOrthosByShapeAndLocation returns counts of orthos grouped by shape and by location within shape
func (s *OrthosStorage) CountOrthosByShapeAndLocation() (map[string]int, map[[2]string]int) {
	s.permanentMutex.RLock()
	defer s.permanentMutex.RUnlock()

	shapeCounts := make(map[string]int)
	locationCounts := make(map[[2]string]int)

	for _, ortho := range s.orthos {
		// Get the shape as a string
		shapeKey := fmt.Sprintf("%v", ortho.Shape)
		shapeCounts[shapeKey]++

		// Get the position as a string
		posKey := fmt.Sprintf("%v", ortho.Position)

		// Create a composite key for shape+position
		locationKey := [2]string{shapeKey, posKey}
		locationCounts[locationKey]++
	}

	return shapeCounts, locationCounts
}

// FastCheckExists quickly checks if an ortho ID exists
func (s *OrthosStorage) FastCheckExists(id string) bool {
	s.idSetMutex.RLock()
	defer s.idSetMutex.RUnlock()
	_, exists := s.idSet[id]
	return exists
}

// AddToFastSet adds an ortho ID to the fast lookup set
func (s *OrthosStorage) AddToFastSet(id string) {
	s.idSetMutex.Lock()
	defer s.idSetMutex.Unlock()
	s.idSet[id] = true
}

// AddToPermanentStore adds an ortho to the permanent storage
func (s *OrthosStorage) AddToPermanentStore(ortho Ortho) {
	s.permanentMutex.Lock()
	defer s.permanentMutex.Unlock()
	s.orthos[ortho.ID] = ortho
	orthosPermanentStoreAdds.Inc()
}

// QueueForStorage adds an ortho to the background queue
func (s *OrthosStorage) QueueForStorage(ortho Ortho) {
	s.queue.Add(ortho)
}

// ProcessQueue processes a batch of items from the queue
func (s *OrthosStorage) ProcessQueue(batchSize int) int {
	// Get a batch of items
	batch := s.queue.GetBatch(batchSize)
	if len(batch) == 0 {
		return 0
	}

	// Process each item
	for _, ortho := range batch {
		s.AddToPermanentStore(ortho)
	}

	log.Printf("Processed %d orthos from queue", len(batch))
	return len(batch)
}

// FlushQueue immediately processes all queued items
func (s *OrthosStorage) FlushQueue() int {
	timer := prometheus.NewTimer(orthosFlushDuration)
	defer timer.ObserveDuration()

	// Get all items from the queue
	items := s.queue.Clear()
	if len(items) == 0 {
		return 0
	}

	// Process each item
	for _, ortho := range items {
		s.AddToPermanentStore(ortho)
	}

	log.Printf("Flushed queue: processed %d orthos", len(items))
	return len(items)
}

// TriggerQueueFlush signals that the queue should be flushed
func (s *OrthosStorage) TriggerQueueFlush() {
	// Try to send a signal non-blocking
	select {
	case s.queueFlushSignal <- struct{}{}:
		log.Printf("Queue flush signal sent")
	default:
		log.Printf("Queue flush already pending, skipping signal")
	}
}

// AddOrthos adds multiple orthos using the fast path approach and returns new IDs
func (s *OrthosStorage) AddOrthos(orthos []Ortho) []string {
	if len(orthos) == 0 {
		return []string{}
	}

	newIDs := make([]string, 0, len(orthos))

	for _, ortho := range orthos {
		// Fast check if the ID exists
		if !s.FastCheckExists(ortho.ID) {
			// ID is new - add to fast set, queue for storage, and add to newIDs
			s.AddToFastSet(ortho.ID)
			s.QueueForStorage(ortho)
			newIDs = append(newIDs, ortho.ID)
			orthosFastPathHits.Inc()

			// Update accepted metrics
			shapeKey := fmt.Sprintf("%v", ortho.Shape)
			posKey := fmt.Sprintf("%v", ortho.Position)
			orthosAcceptedByShapePosition.WithLabelValues(shapeKey, posKey).Inc()
		} else {
			// Update rejected metrics
			shapeKey := fmt.Sprintf("%v", ortho.Shape)
			posKey := fmt.Sprintf("%v", ortho.Position)
			orthosRejectedByShapePosition.WithLabelValues(shapeKey, posKey).Inc()
		}
	}

	log.Printf("Fast path: processed %d orthos, %d are new", len(orthos), len(newIDs))
	return newIDs
}

// GetOrthosByIDs returns orthos matching the provided IDs, ensuring queue is flushed first
func (s *OrthosStorage) GetOrthosByIDs(ids []string) []Ortho {
	if len(ids) == 0 {
		return []Ortho{}
	}

	// Flush the queue before reading to ensure consistency
	s.FlushQueue()

	// Now read from permanent storage
	s.permanentMutex.RLock()
	defer s.permanentMutex.RUnlock()

	result := make([]Ortho, 0, len(ids))
	for _, id := range ids {
		if ortho, exists := s.orthos[id]; exists {
			result = append(result, ortho)
		}
	}

	return result
}

// startQueueProcessor starts a background goroutine to process the queue
func startQueueProcessor(storage *OrthosStorage, config OrthosConfig) {
	// Ensure we have valid values - fix to prevent panic with non-positive ticker interval
	if config.QueueFlushInterval <= 0 {
		log.Printf("Warning: Invalid QueueFlushInterval value %d, using default of 15 seconds", config.QueueFlushInterval)
		config.QueueFlushInterval = 15
	}

	if config.QueueBatchSize <= 0 {
		log.Printf("Warning: Invalid QueueBatchSize value %d, using default of 100", config.QueueBatchSize)
		config.QueueBatchSize = 100
	}

	log.Printf("Starting orthos queue processor with flush interval %d seconds and batch size %d", config.QueueFlushInterval, config.QueueBatchSize)

	// Create a ticker for regular processing
	ticker := time.NewTicker(time.Duration(config.QueueFlushInterval) * time.Second)

	// Create a dedicated goroutine for queue processing
	go func() {
		// Make sure the ticker is stopped when this goroutine exits
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Regular processing at intervals
				queueSize := storage.queue.Size()
				if queueSize > 0 {
					log.Printf("Processing queue: %d items pending", queueSize)
					storage.ProcessQueue(config.QueueBatchSize)
				}

			case <-storage.queueFlushSignal:
				// Manually triggered flush
				log.Printf("Manual queue flush triggered")
				storage.FlushQueue()
			}
		}
	}()

	log.Printf("Queue processor goroutine started successfully")
}

// Global storage instance
var orthosStorage *OrthosStorage

// startMetricsUpdater periodically updates the orthos metrics
func startMetricsUpdater(storage *OrthosStorage) {
	log.Printf("Starting metrics updater background task")

	// Immediately update metrics on startup
	updateOrthoMetrics(storage)

	// Create a ticker for periodic updates
	ticker := time.NewTicker(15 * time.Second)

	// Create a dedicated goroutine for metrics updating
	go func() {
		// Make sure the ticker is stopped when this goroutine exits
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.Printf("Metrics update triggered by ticker")
				updateOrthoMetrics(storage)
			}
		}
	}()

	log.Printf("Metrics updater goroutine started successfully")
}

func handleRoot(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Hello World",
	})
}

func handleOrthos(c *gin.Context) {
	var request OrthosRequest

	// Bind JSON to struct
	if err := c.ShouldBindJSON(&request); err != nil {
		telemetry.LogAndError(c, err, "orthos", "Error parsing orthos request")
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request format",
		})
		return
	}

	// Log the received orthos
	log.Printf("Received %d orthos", len(request.Orthos))
	for i, ortho := range request.Orthos {
		log.Printf("Ortho %d: ID=%s, Shell=%d, Position=%v, Shape=%v",
			i, ortho.ID, ortho.Shell, ortho.Position, ortho.Shape)
	}

	// Use fast path for adding orthos
	newIDs := orthosStorage.AddOrthos(request.Orthos)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Orthos saved successfully",
		"count":   len(request.Orthos),
		"newIDs":  newIDs,
	})
}

func handleGetOrthosByIDs(c *gin.Context) {
	var request OrthosGetRequest

	// Bind JSON to struct
	if err := c.ShouldBindJSON(&request); err != nil {
		telemetry.LogAndError(c, err, "orthos", "Error parsing orthos get request")
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request format",
		})
		return
	}

	// This will trigger a queue flush before getting orthos
	matchedOrthos := orthosStorage.GetOrthosByIDs(request.IDs)
	log.Printf("Found %d orthos matching %d requested IDs", len(matchedOrthos), len(request.IDs))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Retrieved orthos successfully",
		"count":   len(matchedOrthos),
		"orthos":  matchedOrthos,
	})
}

func main() {
	// Initialize orthos storage
	orthosStorage = NewOrthosStorage()

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

	// Start background workers - just call the functions directly since they now handle goroutine creation internally
	startMetricsUpdater(orthosStorage)
	startQueueProcessor(orthosStorage, cfg)

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

	// Add the Prometheus handler explicitly
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Log that we're registering the metrics endpoint
	log.Printf("Registered /metrics endpoint for Prometheus")

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
	router.POST("/orthos", handleOrthos)
	router.POST("/orthos/get", handleGetOrthosByIDs)

	// Add an endpoint to manually trigger queue flushing
	router.POST("/flush", func(c *gin.Context) {
		orthosStorage.TriggerQueueFlush()
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Queue flush triggered",
		})
	})

	// Start the server
	address := cfg.GetAddress()
	log.Printf("Orthos server starting on %s...\n", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
