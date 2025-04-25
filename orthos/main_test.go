package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Initialize the orthos storage for testing
	orthosStorage = NewOrthosStorage()

	// Register the routes
	router.POST("/orthos", handleOrthos)
	router.POST("/orthos/get", handleGetOrthosByIDs)

	return router
}

func TestHandleOrthos(t *testing.T) {
	// Setup
	router := setupTestRouter()

	// Test cases
	testCases := []struct {
		name           string
		orthos         []Ortho
		expectedStatus int
		checkNewIDs    bool
	}{
		{
			name: "Add new orthos",
			orthos: []Ortho{
				{
					Grid:     map[string]string{"a": "1", "b": "2"},
					Shape:    []int{3, 4},
					Position: []int{5, 6},
					Shell:    7,
					ID:       "new-id-1",
				},
				{
					Grid:     map[string]string{"c": "3", "d": "4"},
					Shape:    []int{7, 8},
					Position: []int{9, 10},
					Shell:    11,
					ID:       "new-id-2",
				},
			},
			expectedStatus: http.StatusOK,
			checkNewIDs:    true,
		},
		{
			name: "Add duplicate orthos",
			orthos: []Ortho{
				{
					Grid:     map[string]string{"a": "1", "b": "2"},
					Shape:    []int{3, 4},
					Position: []int{5, 6},
					Shell:    7,
					ID:       "new-id-1", // Same ID as previous test case
				},
				{
					Grid:     map[string]string{"e": "5", "f": "6"},
					Shape:    []int{11, 12},
					Position: []int{13, 14},
					Shell:    15,
					ID:       "new-id-3", // New ID
				},
			},
			expectedStatus: http.StatusOK,
			checkNewIDs:    true,
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			orthosRequest := OrthosRequest{
				Orthos: tc.orthos,
			}
			reqBody, _ := json.Marshal(orthosRequest)
			req, _ := http.NewRequest("POST", "/orthos", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Verify response
			assert.Equal(t, tc.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			assert.Equal(t, "success", response["status"])
			assert.Equal(t, float64(len(tc.orthos)), response["count"])

			// Check newIDs if required
			if tc.checkNewIDs {
				newIDs, ok := response["newIDs"].([]interface{})
				assert.True(t, ok)

				// For the first test case, both IDs should be new
				if tc.name == "Add new orthos" {
					assert.Equal(t, 2, len(newIDs))
					assert.Contains(t, newIDs, "new-id-1")
					assert.Contains(t, newIDs, "new-id-2")
				}

				// For the second test case, only one ID should be new
				if tc.name == "Add duplicate orthos" {
					assert.Equal(t, 1, len(newIDs))
					assert.Contains(t, newIDs, "new-id-3")
					assert.NotContains(t, newIDs, "new-id-1") // This ID already exists
				}
			}
		})
	}
}

func TestHandleGetOrthosByIDs(t *testing.T) {
	// Setup
	router := setupTestRouter()

	// Create some test orthos
	testOrthos := []Ortho{
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
		{
			Grid:     map[string]string{"e": "5", "f": "6"},
			Shape:    []int{11, 12},
			Position: []int{13, 14},
			Shell:    15,
			ID:       "test-id-3",
		},
	}

	// Add orthos to storage
	orthosRequest := OrthosRequest{
		Orthos: testOrthos,
	}
	reqBody, _ := json.Marshal(orthosRequest)
	req, _ := http.NewRequest("POST", "/orthos", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify orthos were added
	assert.Equal(t, http.StatusOK, w.Code)

	// Test cases
	testCases := []struct {
		name           string
		requestIDs     []string
		expectedCount  int
		expectedStatus int
	}{
		{
			name:           "Get single ortho",
			requestIDs:     []string{"test-id-1"},
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Get multiple orthos",
			requestIDs:     []string{"test-id-1", "test-id-3"},
			expectedCount:  2,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Get non-existent ortho",
			requestIDs:     []string{"non-existent-id"},
			expectedCount:  0,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Get mixed existing and non-existing orthos",
			requestIDs:     []string{"test-id-2", "non-existent-id"},
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Empty ID list",
			requestIDs:     []string{},
			expectedCount:  0,
			expectedStatus: http.StatusOK,
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			getRequest := OrthosGetRequest{
				IDs: tc.requestIDs,
			}
			reqBody, _ := json.Marshal(getRequest)
			req, _ := http.NewRequest("POST", "/orthos/get", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Verify response
			assert.Equal(t, tc.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			assert.Equal(t, "success", response["status"])
			assert.Equal(t, float64(tc.expectedCount), response["count"])

			// Verify returned orthos
			if tc.expectedCount > 0 {
				orthos, ok := response["orthos"].([]interface{})
				assert.True(t, ok)
				assert.Equal(t, tc.expectedCount, len(orthos))

				// Verify each returned ortho is in the requested IDs
				for _, ortho := range orthos {
					orthoMap, ok := ortho.(map[string]interface{})
					assert.True(t, ok)

					id := orthoMap["id"].(string)
					found := false
					for _, requestedID := range tc.requestIDs {
						if id == requestedID {
							found = true
							break
						}
					}
					assert.True(t, found, "Returned ortho ID not in requested IDs")
				}
			}
		})
	}
}

func TestHandleGetOrthosByIDs_InvalidJSON(t *testing.T) {
	// Setup
	router := setupTestRouter()

	// Test with invalid JSON
	req, _ := http.NewRequest("POST", "/orthos/get", bytes.NewBuffer([]byte(`{invalid json}`)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, "error", response["status"])
	assert.Equal(t, "Invalid request format", response["message"])
}
