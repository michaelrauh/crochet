package main

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"crochet/types"

	"github.com/gin-gonic/gin"
)

// versionedRem stores a remediation tuple with a version for change tracking
type versionedRem struct {
	types.RemediationTuple
	Version int64
}

// RemediationQueueStore manages a queue and store of remediations with versioning
type RemediationQueueStore struct {
	queue          chan types.RemediationTuple
	store          []versionedRem
	existingMap    map[string]map[string]struct{}
	currentVersion int64
	mu             sync.RWMutex
	flushSize      int
	flushInterval  time.Duration
}

// NewRemediationQueueStore creates a new RemediationQueueStore with given parameters
func NewRemediationQueueStore(qSize, flushSize int, flushInterval time.Duration) *RemediationQueueStore {
	store := &RemediationQueueStore{
		queue:          make(chan types.RemediationTuple, qSize),
		store:          make([]versionedRem, 0),
		existingMap:    make(map[string]map[string]struct{}),
		currentVersion: 0,
		flushSize:      flushSize,
		flushInterval:  flushInterval,
	}

	// Launch the background flusher in a goroutine
	go store.backgroundFlusher()

	return store
}

// backgroundFlusher processes items from the queue in batches
func (s *RemediationQueueStore) backgroundFlusher() {
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	batch := make([]types.RemediationTuple, 0, s.flushSize)

	for {
		select {
		case r := <-s.queue:
			// Add the item to the current batch
			batch = append(batch, r)

			// If the batch is full, flush it
			if len(batch) >= s.flushSize {
				s.flush(batch)
				batch = make([]types.RemediationTuple, 0, s.flushSize)
			}

		case <-ticker.C:
			// Time to flush whatever we have, even if the batch isn't full
			if len(batch) > 0 {
				s.flush(batch)
				batch = make([]types.RemediationTuple, 0, s.flushSize)
			}
		}
	}
}

// flush adds a batch of remediations to the store with proper locking
func (s *RemediationQueueStore) flush(batch []types.RemediationTuple) {
	if len(batch) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Increment the current version for this batch
	s.currentVersion++
	currentVersion := s.currentVersion

	for _, r := range batch {
		// Check if we've seen this hash before
		if _, exists := s.existingMap[r.Hash]; !exists {
			s.existingMap[r.Hash] = make(map[string]struct{})
		}

		// Create pair key for lookup
		pairKey := createPairKey(r.Pair)

		// Check if this specific remediation exists already
		if _, exists := s.existingMap[r.Hash][pairKey]; !exists {
			// This is a new remediation, add it to the store with the current version
			s.store = append(s.store, versionedRem{
				RemediationTuple: r,
				Version:          currentVersion,
			})

			// Mark it as existing in our map
			s.existingMap[r.Hash][pairKey] = struct{}{}
		}
	}
}

// AddHandler handles HTTP POST requests to add remediations to the queue
func (s *RemediationQueueStore) AddHandler(c *gin.Context) {
	var incoming []types.RemediationTuple
	if err := c.ShouldBindJSON(&incoming); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}

	// Enqueue all incoming remediations
	// This should never block due to channel buffering
	enqueuedCount := 0
	for _, r := range incoming {
		select {
		case s.queue <- r:
			enqueuedCount++
		default:
			// Queue is full, this shouldn't happen with proper sizing
			// We could log this condition, but we'll just continue
		}
	}

	c.JSON(http.StatusAccepted, gin.H{
		"status":   "accepted",
		"enqueued": enqueuedCount,
	})
}

// GetHandler handles HTTP GET requests to retrieve remediations
func (s *RemediationQueueStore) GetHandler(c *gin.Context) {
	// Parse the optional since_version parameter
	sinceVersion := int64(0)
	if sinceStr := c.Query("since_version"); sinceStr != "" {
		var err error
		sinceVersion, err = strconv.ParseInt(sinceStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Invalid since_version parameter",
				"error":   err.Error(),
			})
			return
		}
	}

	// Drain the queue into a local slice to ensure we process all pending items
	pending := make([]types.RemediationTuple, 0)
	draining := true

	for draining {
		select {
		case r := <-s.queue:
			pending = append(pending, r)
		default:
			draining = false
		}
	}

	// Lock for consistent read and to flush the pending items
	s.mu.Lock()

	// Flush the pending items first
	if len(pending) > 0 {
		s.flush(pending)
	}

	// Build a response with items newer than the requested version
	response := make([]types.RemediationTuple, 0)
	for _, item := range s.store {
		if item.Version > sinceVersion {
			response = append(response, item.RemediationTuple)
		}
	}

	// Get the current version to return to the client
	currentVersion := s.currentVersion

	// Unlock now that we have our local copy
	s.mu.Unlock()

	// Return the response as JSON
	c.JSON(http.StatusOK, gin.H{
		"status":          "ok",
		"current_version": currentVersion,
		"remediations":    response,
	})
}

// createPairKey creates a string key from a string slice for map lookups
// This is the same implementation as in types/remediation_store.go
func createPairKey(pair []string) string {
	if len(pair) == 0 {
		return ""
	}
	result := pair[0]
	for i := 1; i < len(pair); i++ {
		result += ":" + pair[i]
	}
	return result
}
