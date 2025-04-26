package main

import (
	"bytes"
	"encoding/json"
	"fmt" // Add missing fmt import
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter() (*gin.Engine, *OrthosStorage) {
	// Use test mode to avoid debug logs
	gin.SetMode(gin.TestMode)

	// Create a test router
	router := gin.New()
	router.Use(gin.Recovery())

	// Create a test storage
	storage := NewOrthosStorage()

	// Set up routes
	router.POST("/orthos", func(c *gin.Context) {
		// Use our test storage instance
		orthosStorage = storage
		handleOrthos(c)
	})

	router.POST("/orthos/get", func(c *gin.Context) {
		// Use our test storage instance
		orthosStorage = storage
		handleGetOrthosByIDs(c)
	})

	router.POST("/flush", func(c *gin.Context) {
		orthosStorage = storage
		orthosStorage.TriggerQueueFlush()
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Queue flush triggered",
		})
	})

	return router, storage
}

func TestFastPathIdentifiesNewOrthos(t *testing.T) {
	router, storage := setupTestRouter() // Keep the router variable since we use it later

	// Create some test orthos
	orthos := []Ortho{
		{
			ID:       "test-id-1",
			Shape:    []int{1, 2},
			Position: []int{3, 4},
			Shell:    5,
			Grid:     map[string]string{"key1": "value1"},
		},
		{
			ID:       "test-id-2",
			Shape:    []int{6, 7},
			Position: []int{8, 9},
			Shell:    10,
			Grid:     map[string]string{"key2": "value2"},
		},
	}

	// Create request body
	reqBody, _ := json.Marshal(OrthosRequest{Orthos: orthos})

	// Create request
	req := httptest.NewRequest("POST", "/orthos", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Perform request
	router.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse response
	var response struct {
		Status  string   `json:"status"`
		Message string   `json:"message"`
		Count   int      `json:"count"`
		NewIDs  []string `json:"newIDs"`
	}

	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify all IDs were reported as new
	assert.Equal(t, "success", response.Status)
	assert.Equal(t, 2, response.Count)
	assert.Equal(t, 2, len(response.NewIDs))
	assert.Contains(t, response.NewIDs, "test-id-1")
	assert.Contains(t, response.NewIDs, "test-id-2")

	// Verify they were added to the fast set but not yet to permanent storage
	assert.True(t, storage.FastCheckExists("test-id-1"))
	assert.True(t, storage.FastCheckExists("test-id-2"))

	// Check queue size (should contain the items)
	assert.Equal(t, 2, storage.queue.Size())

	// The permanent storage should be empty still
	storage.permanentMutex.RLock()
	assert.Equal(t, 0, len(storage.orthos))
	storage.permanentMutex.RUnlock()
}

func TestQueueProcessorCorrectlyStoresOrthos(t *testing.T) {
	_, storage := setupTestRouter() // Don't need router var

	// Create a test config
	cfg := OrthosConfig{
		QueueFlushInterval: 1, // 1 second for tests
		QueueBatchSize:     10,
	}

	// Start the queue processor in background
	go startQueueProcessor(storage, cfg)

	// Create a test ortho
	orthos := []Ortho{
		{
			ID:       "queue-test-id",
			Shape:    []int{1, 2},
			Position: []int{3, 4},
			Shell:    5,
			Grid:     map[string]string{"key": "value"},
		},
	}

	// Add it using the fast path
	storage.AddOrthos(orthos)

	// Wait for queue processor to run
	time.Sleep(2 * time.Second)

	// Verify it was added to permanent storage
	storage.permanentMutex.RLock()
	assert.Equal(t, 1, len(storage.orthos))
	assert.Contains(t, storage.orthos, "queue-test-id")
	storage.permanentMutex.RUnlock()
}

func TestReadOperationsFlushQueue(t *testing.T) {
	router, storage := setupTestRouter()

	// Create some test orthos
	orthos := []Ortho{
		{
			ID:       "flush-test-id-1",
			Shape:    []int{1, 2},
			Position: []int{3, 4},
			Shell:    5,
			Grid:     map[string]string{"key1": "value1"},
		},
		{
			ID:       "flush-test-id-2",
			Shape:    []int{6, 7},
			Position: []int{8, 9},
			Shell:    10,
			Grid:     map[string]string{"key2": "value2"},
		},
	}

	// Add them using the fast path
	storage.AddOrthos(orthos)

	// Verify they are in the queue
	assert.Equal(t, 2, storage.queue.Size())

	// Create a request to get the orthos
	reqBody, _ := json.Marshal(OrthosGetRequest{IDs: []string{"flush-test-id-1", "flush-test-id-2"}})

	// Create request
	req := httptest.NewRequest("POST", "/orthos/get", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Perform request
	router.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	// Queue should be empty now
	assert.Equal(t, 0, storage.queue.Size())

	// Permanent storage should contain the orthos
	storage.permanentMutex.RLock()
	assert.Equal(t, 2, len(storage.orthos))
	assert.Contains(t, storage.orthos, "flush-test-id-1")
	assert.Contains(t, storage.orthos, "flush-test-id-2")
	storage.permanentMutex.RUnlock()

	// Parse response
	var response struct {
		Status  string  `json:"status"`
		Message string  `json:"message"`
		Count   int     `json:"count"`
		Orthos  []Ortho `json:"orthos"`
	}

	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify response contains both orthos
	assert.Equal(t, "success", response.Status)
	assert.Equal(t, 2, response.Count)
	assert.Equal(t, 2, len(response.Orthos))
}

func TestConcurrentOperations(t *testing.T) {
	_, storage := setupTestRouter()

	// Test concurrent addition of orthos
	var wg sync.WaitGroup
	numGoroutines := 10
	orthosPerGoroutine := 10

	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer wg.Done()

			orthos := make([]Ortho, orthosPerGoroutine)
			for j := 0; j < orthosPerGoroutine; j++ {
				orthos[j] = Ortho{
					ID:       fmt.Sprintf("concurrent-test-id-%d-%d", routineID, j),
					Shape:    []int{routineID, j},
					Position: []int{routineID + 1, j + 1},
					Shell:    routineID + j,
					Grid:     map[string]string{"key": fmt.Sprintf("value-%d-%d", routineID, j)},
				}
			}

			// Use fast path to add orthos
			storage.AddOrthos(orthos)
		}(i)
	}

	wg.Wait()

	// Verify all orthos are in the fast set
	assert.Equal(t, numGoroutines*orthosPerGoroutine, len(storage.idSet))

	// Flush the queue
	storage.FlushQueue()

	// Verify all orthos are in permanent storage
	storage.permanentMutex.RLock()
	assert.Equal(t, numGoroutines*orthosPerGoroutine, len(storage.orthos))
	storage.permanentMutex.RUnlock()
}

func TestExplicitFlushEndpoint(t *testing.T) {
	router, storage := setupTestRouter()

	// Create some test orthos
	orthos := []Ortho{
		{
			ID:       "endpoint-flush-test-id-1",
			Shape:    []int{1, 2},
			Position: []int{3, 4},
			Shell:    5,
			Grid:     map[string]string{"key1": "value1"},
		},
		{
			ID:       "endpoint-flush-test-id-2",
			Shape:    []int{6, 7},
			Position: []int{8, 9},
			Shell:    10,
			Grid:     map[string]string{"key2": "value2"},
		},
	}

	// Add them using the fast path
	storage.AddOrthos(orthos)

	// Verify they are in the queue
	assert.Equal(t, 2, storage.queue.Size())

	// Create request to flush
	req := httptest.NewRequest("POST", "/flush", nil)

	// Create response recorder
	w := httptest.NewRecorder()

	// Perform request
	router.ServeHTTP(w, req)

	// Signal has been sent to channel, now flush directly
	storage.FlushQueue()

	// Queue should be empty now
	assert.Equal(t, 0, storage.queue.Size())

	// Permanent storage should contain the orthos
	storage.permanentMutex.RLock()
	assert.Equal(t, 2, len(storage.orthos))
	assert.Contains(t, storage.orthos, "endpoint-flush-test-id-1")
	assert.Contains(t, storage.orthos, "endpoint-flush-test-id-2")
	storage.permanentMutex.RUnlock()
}
