// filepath: /Users/michaelrauh/dev/crochet/remediations/main_test.go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"crochet/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	initStore() // Initialize the store for testing

	// Add some test data
	testRemediations := []types.RemediationTuple{
		{Pair: []string{"word1", "word2"}, Hash: "hash1"},
		{Pair: []string{"word3", "word4"}, Hash: "hash2"},
		{Pair: []string{"word5", "word6"}, Hash: "hash3"},
		{Pair: []string{"word1", "word2"}, Hash: "hash4"}, // Same pair, different hash
	}

	types.SaveRemediationsToStore(store, testRemediations)

	router.GET("/", ginGetRemediationsHandler)
	router.POST("/add", ginAddRemediationHandler)

	return router
}

func TestGetRemediationsHandler(t *testing.T) {
	router := setupRouter()

	// Test cases
	testCases := []struct {
		name           string
		pairs          [][]string
		expectedStatus int
		expectedHashes []string
	}{
		{
			name:           "Find single hash",
			pairs:          [][]string{{"word3", "word4"}},
			expectedStatus: http.StatusOK,
			expectedHashes: []string{"hash2"},
		},
		{
			name:           "Find multiple hashes for the same pair",
			pairs:          [][]string{{"word1", "word2"}},
			expectedStatus: http.StatusOK,
			expectedHashes: []string{"hash1", "hash4"},
		},
		{
			name:           "Find hashes for multiple pairs",
			pairs:          [][]string{{"word1", "word2"}, {"word3", "word4"}},
			expectedStatus: http.StatusOK,
			expectedHashes: []string{"hash1", "hash2", "hash4"},
		},
		{
			name:           "No matches found",
			pairs:          [][]string{{"nonexistent", "pair"}},
			expectedStatus: http.StatusOK,
			expectedHashes: []string{},
		},
		{
			name:           "Empty pairs array",
			pairs:          [][]string{},
			expectedStatus: http.StatusOK,
			expectedHashes: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Convert pairs to JSON
			pairsJSON, _ := json.Marshal(tc.pairs)

			// URL encode the JSON
			encodedPairs := url.QueryEscape(string(pairsJSON))

			// Create request
			req, _ := http.NewRequest("GET", "/?pairs="+encodedPairs, nil)
			w := httptest.NewRecorder()

			// Serve the request
			router.ServeHTTP(w, req)

			// Assert status code
			assert.Equal(t, tc.expectedStatus, w.Code)

			// Parse response
			var response types.RemediationResponse
			json.Unmarshal(w.Body.Bytes(), &response)

			// Check that all expected hashes are in the response (order doesn't matter)
			assert.Equal(t, len(tc.expectedHashes), len(response.Hashes),
				"Expected %d hashes but got %d", len(tc.expectedHashes), len(response.Hashes))

			// Convert hashes to a map for easier comparison
			expectedHashMap := make(map[string]struct{})
			for _, hash := range tc.expectedHashes {
				expectedHashMap[hash] = struct{}{}
			}

			// Check that all hashes in the response are expected
			for _, hash := range response.Hashes {
				_, exists := expectedHashMap[hash]
				assert.True(t, exists, "Unexpected hash in response: %s", hash)
			}
		})
	}
}

func TestGetRemediationsHandlerMissingPairsParam(t *testing.T) {
	router := setupRouter()

	// Create request with missing pairs parameter
	req, _ := http.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(w, req)

	// Assert status code
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Parse response
	var response gin.H
	json.Unmarshal(w.Body.Bytes(), &response)

	// Check error message
	assert.Equal(t, "error", response["status"])
	assert.Equal(t, "Missing 'pairs' parameter", response["message"])
}

func TestGetRemediationsHandlerInvalidPairsFormat(t *testing.T) {
	router := setupRouter()

	// Create request with invalid pairs format
	req, _ := http.NewRequest("GET", "/?pairs=invalid-json", nil)
	w := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(w, req)

	// Assert status code
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGinAddRemediationHandler(t *testing.T) {
	router := setupRouter()

	// Create a sample add remediation request with tuples
	request := types.AddRemediationRequest{
		Remediations: []types.RemediationTuple{
			{
				Pair: []string{"term1", "definition1"},
				Hash: "hash5",
			},
			{
				Pair: []string{"term2", "definition2"},
				Hash: "hash6",
			},
		},
	}

	jsonData, _ := json.Marshal(request)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/add", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	// Check the response
	assert.Equal(t, http.StatusOK, w.Code)

	var response types.AddRemediationResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)

	assert.Nil(t, err)
	assert.Equal(t, "OK", response.Status)
	assert.Equal(t, "Remediations added successfully", response.Message)

	// Verify that the new items were added to the store (4 existing + 2 new)
	assert.Equal(t, 6, len(store.Remediations))
}
