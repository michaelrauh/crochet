package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	"crochet/config"
	"crochet/health"
	"crochet/middleware"
	"crochet/types"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Global state
var store *types.RemediationMemoryStore
var rwLock sync.RWMutex   // Protect reads with RWLock
var writeMutex sync.Mutex // Mutex specifically for write operations
var writeInProgress sync.WaitGroup

// Prometheus metrics
var (
	remediationsCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "remediations_total_count",
			Help: "Total number of remediations stored in the service",
		},
	)
)

func init() {
	// Register metrics with Prometheus
	prometheus.MustRegister(remediationsCount)
}

// updateRemediationsMetrics updates the metric for total remediation count
func updateRemediationsMetrics() {
	rwLock.RLock()
	count := len(store.Remediations)
	rwLock.RUnlock()

	remediationsCount.Set(float64(count))
	log.Printf("Updated remediations metrics: total=%d", count)
}

func initStore() {
	store = types.InitRemediationStore()
	log.Println("Remediation in-memory store initialized successfully")
	// Initialize metrics after store setup
	updateRemediationsMetrics()
}

// findHashesForPairs searches the store for all hashes that match the given pairs
func findHashesForPairs(pairs [][]string) []string {
	if len(pairs) == 0 {
		return []string{}
	}

	// Create a map to ensure unique hashes
	uniqueHashes := make(map[string]struct{})

	// For each pair in the request, find matching remediations
	for _, pair := range pairs {
		for _, remediation := range store.Remediations {
			if types.CompareStringSlices(remediation.Pair, pair) {
				uniqueHashes[remediation.Hash] = struct{}{}
			}
		}
	}

	// Convert the map keys to a slice
	hashes := make([]string, 0, len(uniqueHashes))
	for hash := range uniqueHashes {
		hashes = append(hashes, hash)
	}

	return hashes
}

func ginGetRemediationsHandler(c *gin.Context) {
	// Use the new helper function to process remediation pairs
	pairs, err := types.ProcessRemediationPairs(c, "remediations")
	if err != nil {
		// ProcessRemediationPairs already sets the appropriate error response
		return
	}

	pairs_count := len(pairs)
	log.Printf("Received %d pairs for remediation lookup", pairs_count)

	// Log the pairs we received
	for i, pair := range pairs {
		log.Printf("Pair %d: %v", i+1, pair)
	}

	// Wait for any ongoing writes to complete and acquire a read lock
	// This ensures read time consistency while allowing multiple concurrent reads
	writeInProgress.Wait() // Wait for any ongoing writes to complete
	rwLock.RLock()         // Acquire read lock - this blocks new writes
	defer rwLock.RUnlock() // Ensure we release the read lock when done

	// Find matching hashes in the store
	hashes := findHashesForPairs(pairs)
	log.Printf("Found %d matching hashes", len(hashes))

	c.JSON(http.StatusOK, types.RemediationResponse{
		Status: "OK",
		Hashes: hashes,
	})
}

// SafeSaveRemediationsToStore safely adds remediations to the store with proper synchronization
func SafeSaveRemediationsToStore(remediations []types.RemediationTuple) int {
	// Lock only when actually modifying the store to prevent data corruption
	writeMutex.Lock()
	defer writeMutex.Unlock()

	addedCount := types.SaveRemediationsToStore(store, remediations)
	// Update metrics after modifying the store
	updateRemediationsMetrics()
	return addedCount
}

// SafeDeleteRemediationsFromStore safely removes remediations from the store with proper synchronization
func SafeDeleteRemediationsFromStore(hashes []string) int {
	// Lock only when actually modifying the store to prevent data corruption
	writeMutex.Lock()
	defer writeMutex.Unlock()

	deletedCount := types.DeleteRemediationsFromStore(store, hashes)
	// Update metrics after modifying the store
	updateRemediationsMetrics()
	return deletedCount
}

func ginAddRemediationHandler(c *gin.Context) {
	// We never want to block on writes, so don't try to get a full lock
	// Instead, just signal that a write is in progress
	writeInProgress.Add(1)
	defer writeInProgress.Done()

	// Process the request without blocking other writers
	addRemediationWithoutBlockingWrites(c)
}

// Helper function to process add remediation requests
func addRemediationWithoutBlockingWrites(c *gin.Context) {
	// Use the helper function to process add remediation request
	request, err := types.ProcessAddRemediationRequest(c, "remediations")
	if err != nil {
		// ProcessAddRemediationRequest already sets the appropriate error response
		return
	}

	remediation_count := len(request.Remediations)
	log.Printf("Received %d remediations to add", remediation_count)

	// Log the remediations we received
	for i, remediation := range request.Remediations {
		log.Printf("Remediation %d: Pair %v with Hash %s", i+1, remediation.Pair, remediation.Hash)
	}

	// Save remediations to store using the thread-safe helper
	addedCount := SafeSaveRemediationsToStore(request.Remediations)
	log.Printf("Added %d new remediations to store. Total remediations in store: %d",
		addedCount, len(store.Remediations))

	c.JSON(http.StatusOK, types.AddRemediationResponse{
		Status:  "OK",
		Message: "Remediations added successfully",
		Count:   addedCount,
	})
}

func ginDeleteRemediationHandler(c *gin.Context) {
	// Signal that a write is in progress
	writeInProgress.Add(1)
	defer writeInProgress.Done()

	// Process the delete remediation request
	request, err := types.ProcessDeleteRemediationRequest(c, "remediations")
	if err != nil {
		// ProcessDeleteRemediationRequest already sets the appropriate error response
		return
	}

	hash_count := len(request.Hashes)
	log.Printf("Received %d hashes to delete", hash_count)

	// Log the hashes we received
	for i, hash := range request.Hashes {
		log.Printf("Hash %d to delete: %s", i+1, hash)
	}

	// Delete remediations from store using the thread-safe helper
	deletedCount := SafeDeleteRemediationsFromStore(request.Hashes)
	log.Printf("Deleted %d remediations from store. Total remediations remaining in store: %d",
		deletedCount, len(store.Remediations))

	c.JSON(http.StatusOK, types.DeleteRemediationResponse{
		Status:  "OK",
		Message: "Remediations deleted successfully",
		Count:   deletedCount,
	})
}

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

	// Initialize the remediation store
	initStore()

	// Add the Prometheus handler explicitly
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	log.Printf("Registered /metrics endpoint for Prometheus")

	// Register Gin routes
	router.GET("/", ginGetRemediationsHandler)
	router.POST("/add", ginAddRemediationHandler)
	router.POST("/delete", ginDeleteRemediationHandler)

	// Set up health check using our local health package
	healthCheck := health.New(health.Options{
		ServiceName: cfg.ServiceName,
		Version:     "1.0.0",
		Details: map[string]string{
			"description": "Manages remediations for the Crochet system",
		},
	})

	// Add dependency checks if needed
	if cfg.JaegerEndpoint != "" {
		healthCheck.AddDependencyCheck("jaeger", "http://"+cfg.JaegerEndpoint+"/health")
	}

	// Register health check handler in the gin router
	router.GET("/health", gin.WrapF(healthCheck.Handler()))

	// Start periodic metrics update in the background
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				updateRemediationsMetrics()
			}
		}
	}()

	address := cfg.GetAddress()
	log.Printf("Remediations service starting on %s...", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
