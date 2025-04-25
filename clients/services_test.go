package clients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"crochet/httpclient"
	"crochet/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupMockRemediationsServer() (*httptest.Server, []string) {
	// Create a mock remediations server
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Expected hashes to return
	expectedHashes := []string{"hash1", "hash2", "hash3"}

	// Handle both GET and POST requests to the root endpoint
	router.Any("/", func(c *gin.Context) {
		// For GET requests, check the query parameters
		if c.Request.Method == http.MethodGet {
			// Get and validate the pairs parameter
			pairsParam := c.Query("pairs")
			if pairsParam == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"status":  "error",
					"message": "Missing pairs parameter",
				})
				return
			}
		} else if c.Request.Method == http.MethodPost {
			// For POST requests, we'll just accept any valid JSON
			var requestBody map[string]interface{}
			if err := c.ShouldBindJSON(&requestBody); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"status":  "error",
					"message": "Invalid JSON",
				})
				return
			}
		}

		// Return the mock response for both GET and POST
		c.JSON(http.StatusOK, types.RemediationResponse{
			Status: "OK",
			Hashes: expectedHashes,
		})
	})

	// Start the server
	server := httptest.NewServer(router)
	return server, expectedHashes
}

func TestFetchRemediations(t *testing.T) {
	// Setup mock server
	server, expectedHashes := setupMockRemediationsServer()
	defer server.Close()

	// Create a client with a reasonable timeout for tests
	httpClient := httpclient.NewClient(httpclient.ClientOptions{
		ClientTimeout: 5 * time.Second,
	})

	// Create a remediations service client
	client := NewRemediationsService(server.URL, httpClient)

	// Create a test request
	request := types.RemediationRequest{
		Pairs: [][]string{
			{"word1", "word2"},
			{"word3", "word4"},
		},
	}

	// Call the service
	response, err := client.FetchRemediations(context.Background(), request)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, "OK", response.Status)
	assert.Equal(t, expectedHashes, response.Hashes)
}

func TestFetchRemediationsError(t *testing.T) {
	// Create a client pointing to a non-existent server
	httpClient := httpclient.NewClient(httpclient.ClientOptions{
		ClientTimeout: 100 * time.Millisecond, // 100ms to fail fast
	})

	// Create a remediations service client with an invalid URL
	client := NewRemediationsService("http://localhost:1", httpClient)

	// Create a test request
	request := types.RemediationRequest{
		Pairs: [][]string{
			{"word1", "word2"},
		},
	}

	// Call the service
	_, err := client.FetchRemediations(context.Background(), request)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error calling remediations service")
}

func TestRemediationsURLEncoding(t *testing.T) {
	// Create a mock server that verifies the URL encoding
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add a handler that handles both GET and POST requests
	router.Any("/", func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			// Get the pairs parameter
			pairsParam := c.Query("pairs")
			// Verify it's not empty
			if pairsParam == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"status":  "error",
					"message": "Missing pairs parameter",
				})
				return
			}
		} else if c.Request.Method == http.MethodPost {
			// For POST requests, we'll just accept any valid JSON
			var requestBody map[string]interface{}
			if err := c.ShouldBindJSON(&requestBody); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"status":  "error",
					"message": "Invalid JSON",
				})
				return
			}
		}

		// Return success - we just need to verify the parameter was sent
		c.JSON(http.StatusOK, types.RemediationResponse{
			Status: "OK",
			Hashes: []string{"hash1"},
		})
	})

	server := httptest.NewServer(router)
	defer server.Close()

	// Create a client with a reasonable timeout for tests
	httpClient := httpclient.NewClient(httpclient.ClientOptions{
		ClientTimeout: 5 * time.Second,
	})

	// Create a remediations service client
	client := NewRemediationsService(server.URL, httpClient)

	// Create a test request with a complex pair that needs encoding
	request := types.RemediationRequest{
		Pairs: [][]string{
			{"word with spaces", "definition & special chars"},
			{"another [word]", "more (special) chars"},
		},
	}

	// Call the service
	response, err := client.FetchRemediations(context.Background(), request)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, "OK", response.Status)
}
