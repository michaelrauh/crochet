package main

import (
	"bytes"
	"crochet/types"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Initialize the work queue for testing with a short timeout for testing requeue
	workQueue = NewWorkQueue(1) // 1 second timeout for testing

	// Register the routes
	router.POST("/push", handlePush)
	router.POST("/pop", handlePop)
	router.POST("/ack", handleAck)
	router.POST("/nack", handleNack)

	return router
}

func TestHandlePush(t *testing.T) {
	// Setup
	router := setupTestRouter()

	// Test cases
	testCases := []struct {
		name           string
		orthos         []types.Ortho
		expectedStatus int
		expectedCount  int
	}{
		{
			name: "Push multiple orthos",
			orthos: []types.Ortho{
				{
					Grid:     map[string]string{"a": "1", "b": "2"},
					Shape:    []int{3, 4},
					Position: []int{5, 6},
					Shell:    7,
					ID:       "test-id-1",
				},
				{
					Grid:     map[string]string{"c": "3", "d": "4"},
					Shape:    []int{7, 8},
					Position: []int{9, 10},
					Shell:    11,
					ID:       "test-id-2",
				},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "Push empty orthos list",
			orthos:         []types.Ortho{},
			expectedStatus: http.StatusOK,
			expectedCount:  0,
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			pushRequest := PushRequest{
				Orthos: tc.orthos,
			}
			reqBody, _ := json.Marshal(pushRequest)
			req, _ := http.NewRequest("POST", "/push", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Verify response
			assert.Equal(t, tc.expectedStatus, w.Code)

			var response PushResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			assert.Equal(t, "success", response.Status)
			assert.Equal(t, tc.expectedCount, response.Count)

			// Verify IDs are returned correctly
			if tc.expectedCount > 0 {
				assert.Equal(t, tc.expectedCount, len(response.IDs))
				for i, ortho := range tc.orthos {
					assert.Equal(t, ortho.ID, response.IDs[i])
				}
			}
		})
	}
}

func TestPopAckNack(t *testing.T) {
	// Setup
	router := setupTestRouter()

	// Create test orthos
	testOrthos := []types.Ortho{
		{
			Grid:     map[string]string{"a": "1", "b": "2"},
			Shape:    []int{3, 4},
			Position: []int{5, 6},
			Shell:    7,
			ID:       "pop-test-id-1",
		},
		{
			Grid:     map[string]string{"c": "3", "d": "4"},
			Shape:    []int{7, 8},
			Position: []int{9, 10},
			Shell:    11,
			ID:       "pop-test-id-2",
		},
	}

	// Push orthos to queue
	pushRequest := PushRequest{
		Orthos: testOrthos,
	}
	reqBody, _ := json.Marshal(pushRequest)
	req, _ := http.NewRequest("POST", "/push", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test Pop
	t.Run("Pop item from queue", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/pop", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response PopResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, "success", response.Status)
		assert.NotNil(t, response.Ortho)
		assert.Equal(t, "pop-test-id-1", response.ID)
	})

	// Test Ack
	t.Run("Acknowledge item", func(t *testing.T) {
		ackRequest := AckRequest{
			ID: "pop-test-id-1",
		}
		reqBody, _ := json.Marshal(ackRequest)
		req, _ := http.NewRequest("POST", "/ack", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response AckResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, "success", response.Status)
	})

	// Pop second item
	t.Run("Pop second item from queue", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/pop", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response PopResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, "success", response.Status)
		assert.NotNil(t, response.Ortho)
		assert.Equal(t, "pop-test-id-2", response.ID)
	})

	// Test Nack
	t.Run("Negative acknowledge item", func(t *testing.T) {
		nackRequest := AckRequest{
			ID: "pop-test-id-2",
		}
		reqBody, _ := json.Marshal(nackRequest)
		req, _ := http.NewRequest("POST", "/nack", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response AckResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, "success", response.Status)
	})

	// Pop the same item again after nack
	t.Run("Pop same item after nack", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/pop", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response PopResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, "success", response.Status)
		assert.NotNil(t, response.Ortho)
		assert.Equal(t, "pop-test-id-2", response.ID)
	})
}

func TestRequeueTimeout(t *testing.T) {
	// Setup with shorter timeout for testing
	workQueue = NewWorkQueue(1) // 1 second timeout
	router := setupTestRouter()

	// Create test ortho
	testOrthos := []types.Ortho{
		{
			Grid:     map[string]string{"a": "1", "b": "2"},
			Shape:    []int{3, 4},
			Position: []int{5, 6},
			Shell:    7,
			ID:       "timeout-test-id",
		},
	}

	// Push ortho to queue
	pushRequest := PushRequest{
		Orthos: testOrthos,
	}
	reqBody, _ := json.Marshal(pushRequest)
	req, _ := http.NewRequest("POST", "/push", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Pop the item
	t.Run("Pop item from queue", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/pop", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response PopResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, "success", response.Status)
		assert.NotNil(t, response.Ortho)
		assert.Equal(t, "timeout-test-id", response.ID)
	})

	// Wait for the timeout
	time.Sleep(1100 * time.Millisecond) // Slightly more than 1 second

	// Pop the item again - it should be requeued automatically
	t.Run("Pop requeued item after timeout", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/pop", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response PopResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, "success", response.Status)
		assert.NotNil(t, response.Ortho)
		assert.Equal(t, "timeout-test-id", response.ID)
	})
}

func TestPopEmptyQueue(t *testing.T) {
	// Setup with empty queue
	workQueue = NewWorkQueue(300)
	router := setupTestRouter()

	// Test pop on empty queue
	req, _ := http.NewRequest("POST", "/pop", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response PopResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, "success", response.Status)
	assert.Nil(t, response.Ortho)
	assert.Equal(t, "", response.ID)
}
