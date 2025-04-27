package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"crochet/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRemediationQueueStore_Deduplication(t *testing.T) {
	// Create a store with small values for testing
	store := NewRemediationQueueStore(100, 5, 10*time.Millisecond)

	// Create some test data with duplicates
	testData := []types.RemediationTuple{
		{Pair: []string{"a", "b"}},
		{Pair: []string{"a", "b"}}, // Duplicate
		{Pair: []string{"c", "d"}},
		{Pair: []string{"e", "f"}},
		{Pair: []string{"c", "d"}}, // Duplicate
	}

	// Add to queue
	for _, r := range testData {
		store.queue <- r
	}

	// Sleep to let the backgroundFlusher process the queue
	time.Sleep(20 * time.Millisecond)

	// Check store contents with a lock to be safe
	store.mu.RLock()
	defer store.mu.RUnlock()

	// We should have 3 unique remediations in the store
	assert.Equal(t, 3, len(store.store), "Should have 3 unique remediations after deduplication")

	// Check the map
	assert.Equal(t, 3, len(store.existingMap), "Should have 3 unique pair keys in the map")
}

func TestRemediationQueueStore_GetHandler(t *testing.T) {
	// Set up Gin in test mode
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create a store with small values for testing
	store := NewRemediationQueueStore(100, 5, 10*time.Millisecond)

	// Register the handler
	router.GET("/remediations", store.GetHandler)

	// Create test data
	testData := []types.RemediationTuple{
		{Pair: []string{"a", "b"}},
		{Pair: []string{"c", "d"}},
		{Pair: []string{"e", "f"}},
	}

	// Add to queue and ensure they're flushed
	for _, r := range testData {
		select {
		case store.queue <- r:
			// Successfully queued
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Failed to queue test data - channel might be full or deadlocked")
		}
	}

	// Sleep to let the backgroundFlusher process the queue
	time.Sleep(50 * time.Millisecond)

	// Record the current version with proper locking
	var firstVersion int64
	store.mu.RLock()
	firstVersion = store.currentVersion
	store.mu.RUnlock()

	// Test getting all remediations (since_version=0)
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/remediations", nil)
	router.ServeHTTP(w1, req1)

	assert.Equal(t, http.StatusOK, w1.Code)

	// Parse response
	var response1 struct {
		Status         string                   `json:"status"`
		CurrentVersion int64                    `json:"current_version"`
		Remediations   []types.RemediationTuple `json:"remediations"`
	}
	err := json.Unmarshal(w1.Body.Bytes(), &response1)
	assert.NoError(t, err, "Should parse JSON response successfully")

	// Should have all remediations
	assert.Equal(t, 3, len(response1.Remediations), "Should have all 3 remediations")

	// Add more data with timeout to prevent deadlock
	moreData := []types.RemediationTuple{
		{Pair: []string{"g", "h"}},
		{Pair: []string{"i", "j"}},
	}

	for _, r := range moreData {
		select {
		case store.queue <- r:
			// Successfully queued
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Failed to queue additional data - channel might be full or deadlocked")
		}
	}

	// Sleep to let the backgroundFlusher process the queue
	time.Sleep(50 * time.Millisecond)

	// Now test with since_version parameter
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/remediations?since_version="+strconv.FormatInt(firstVersion, 10), nil)
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)

	// Parse response
	var response2 struct {
		Status         string                   `json:"status"`
		CurrentVersion int64                    `json:"current_version"`
		Remediations   []types.RemediationTuple `json:"remediations"`
	}
	err = json.Unmarshal(w2.Body.Bytes(), &response2)
	assert.NoError(t, err, "Should parse JSON response successfully")

	// Verify we got new remediations
	assert.GreaterOrEqual(t, len(response2.Remediations), 2,
		"Should have at least the newer remediations with version filter")
}

func TestAddHandler(t *testing.T) {
	// Set up Gin in test mode
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create a store with small values for testing
	store := NewRemediationQueueStore(100, 5, 10*time.Millisecond)

	// Register the handler
	router.POST("/remediations", store.AddHandler)

	// Test data
	testData := []types.RemediationTuple{
		{Pair: []string{"a", "b"}},
		{Pair: []string{"c", "d"}},
	}

	// Create request body
	body, _ := json.Marshal(testData)

	// Make request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/remediations", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusAccepted, w.Code)

	// Parse response
	var response struct {
		Status   string `json:"status"`
		Enqueued int    `json:"enqueued"`
	}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Should have enqueued 2 items
	assert.Equal(t, 2, response.Enqueued, "Should have enqueued 2 items")
	assert.Equal(t, "accepted", response.Status)

	// Sleep to let the backgroundFlusher process the queue
	time.Sleep(20 * time.Millisecond)

	// Check store contents
	store.mu.RLock()
	assert.Equal(t, 2, len(store.store), "Should have 2 items in the store")
	store.mu.RUnlock()
}
