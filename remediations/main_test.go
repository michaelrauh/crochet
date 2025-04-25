// filepath: /Users/michaelrauh/dev/crochet/remediations/main_test.go
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

func setupRouter() (*gin.Engine, *RemediationQueueStore) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	// Create a queue store with small values for testing
	queueStore := NewRemediationQueueStore(100, 5, 10*time.Millisecond)

	// Add some test data
	testRemediations := []types.RemediationTuple{
		{Pair: []string{"word1", "word2"}, Hash: "hash1"},
		{Pair: []string{"word3", "word4"}, Hash: "hash2"},
		{Pair: []string{"word5", "word6"}, Hash: "hash3"},
		{Pair: []string{"word1", "word2"}, Hash: "hash4"}, // Same pair, different hash
	}

	// Add to queue
	for _, r := range testRemediations {
		queueStore.queue <- r
	}

	// Give time for background flusher to process
	time.Sleep(50 * time.Millisecond)

	// Register the handlers
	router.POST("/remediations", queueStore.AddHandler)
	router.GET("/remediations", queueStore.GetHandler)

	return router, queueStore
}

func TestRemediationEndpoints(t *testing.T) {
	router, queueStore := setupRouter()

	// Verify initial data was added correctly
	queueStore.mu.RLock()
	assert.Equal(t, 4, len(queueStore.store), "Should have 4 remediations in store")
	queueStore.mu.RUnlock()

	// Test GET remediations (all)
	t.Run("Get all remediations", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/remediations", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Status         string                   `json:"status"`
			CurrentVersion int64                    `json:"current_version"`
			Remediations   []types.RemediationTuple `json:"remediations"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)

		assert.Equal(t, "ok", response.Status)
		assert.Equal(t, 4, len(response.Remediations), "Should have all 4 remediations")

		// Save the current version for next test
		currentVersion := response.CurrentVersion

		// Test GET with since_version
		t.Run("Get remediations since version", func(t *testing.T) {
			// Add a new remediation to increment the version
			newRemediation := types.RemediationTuple{
				Pair: []string{"word7", "word8"},
				Hash: "hash5",
			}
			queueStore.queue <- newRemediation

			// Give time for background flusher to process
			time.Sleep(50 * time.Millisecond)

			// Get only remediations since our saved version
			req, _ := http.NewRequest("GET", "/remediations?since_version="+strconv.FormatInt(currentVersion, 10), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response struct {
				Status         string                   `json:"status"`
				CurrentVersion int64                    `json:"current_version"`
				Remediations   []types.RemediationTuple `json:"remediations"`
			}
			json.Unmarshal(w.Body.Bytes(), &response)

			assert.Equal(t, "ok", response.Status)
			assert.Equal(t, 1, len(response.Remediations), "Should have only the new remediation")
			assert.Equal(t, "hash5", response.Remediations[0].Hash)
		})
	})

	// Test POST to add new remediations
	t.Run("Add remediations", func(t *testing.T) {
		newRemediations := []types.RemediationTuple{
			{Pair: []string{"word9", "word10"}, Hash: "hash6"},
			{Pair: []string{"word11", "word12"}, Hash: "hash7"},
		}

		body, _ := json.Marshal(newRemediations)
		req, _ := http.NewRequest("POST", "/remediations", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)

		var response struct {
			Status   string `json:"status"`
			Enqueued int    `json:"enqueued"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)

		assert.Equal(t, "accepted", response.Status)
		assert.Equal(t, 2, response.Enqueued)

		// Sleep to allow the background flusher to process
		time.Sleep(50 * time.Millisecond)

		// Verify the store has been updated
		queueStore.mu.RLock()
		assert.Equal(t, 7, len(queueStore.store), "Should have 7 remediations in store")
		queueStore.mu.RUnlock()
	})
}

func TestRemediationDeduplicate(t *testing.T) {
	router, queueStore := setupRouter()

	// Try to add duplicates of existing data
	duplicates := []types.RemediationTuple{
		{Pair: []string{"word1", "word2"}, Hash: "hash1"}, // Exact duplicate
		{Pair: []string{"word3", "word4"}, Hash: "hash2"}, // Exact duplicate
	}

	body, _ := json.Marshal(duplicates)
	req, _ := http.NewRequest("POST", "/remediations", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	// Sleep to allow the background flusher to process
	time.Sleep(50 * time.Millisecond)

	// Verify store size hasn't changed (due to deduplication)
	queueStore.mu.RLock()
	assert.Equal(t, 4, len(queueStore.store), "Should still have 4 remediations (duplicates ignored)")
	queueStore.mu.RUnlock()
}

func TestMalformedRequest(t *testing.T) {
	router, _ := setupRouter()

	// Test invalid JSON in POST
	t.Run("Invalid JSON POST", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/remediations", bytes.NewBufferString("{invalid json}"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	// Test invalid since_version in GET
	t.Run("Invalid since_version", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/remediations?since_version=not-a-number", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
