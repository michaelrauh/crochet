package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Response types to parse JSON responses
type IngestResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type OrthoCountsResponse struct {
	Status     string                    `json:"status"`
	TotalCount int                       `json:"totalCount"`
	Counts     map[string]map[string]int `json:"counts"`
}

// Helper function to check if services are ready
func waitForServices() {
	fmt.Println("Waiting for services to start...")
	// Increase wait time to 30 seconds to allow more time for services to initialize properly
	time.Sleep(30 * time.Second)

	// Try to verify repository service is available by calling its health endpoint
	for i := 0; i < 5; i++ {
		resp, err := http.Get("http://localhost:8080/health")
		if err == nil && resp.StatusCode == 200 {
			fmt.Println("Repository service is ready")
			resp.Body.Close()
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		fmt.Printf("Waiting for repository service (attempt %d/5)...\n", i+1)
		time.Sleep(5 * time.Second)
	}

	fmt.Println("Services should be ready. Running tests...")
}

// Helper function to make HTTP requests
func makeRequest(method, url string, body []byte) ([]byte, int, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return nil, 0, fmt.Errorf("error creating request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("error reading response body: %v", err)
	}

	return respBody, resp.StatusCode, nil
}

// Test: Submit corpus and check for 202 Accepted response
func testPostCorpora() error {
	fmt.Println("Test: Posting corpus to repository service...")

	// Simple but effective test corpus that should generate orthos
	testCorpus := `{
		"title": "Test Corpus",
		"text": "a b. c d. a c. b d."
	}`

	payload := []byte(testCorpus)
	respBody, statusCode, err := makeRequest("POST", "http://localhost:8080/corpora", payload)

	if err != nil {
		return fmt.Errorf("POST request failed: %v", err)
	}

	fmt.Printf("Response status code: %d\n", statusCode)
	fmt.Printf("Response body: %s\n", string(respBody))

	if statusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected status code: got %d, want %d", statusCode, http.StatusAccepted)
	}

	var response IngestResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return fmt.Errorf("error parsing response: %v", err)
	}

	if response.Status != "success" {
		return fmt.Errorf("expected status 'success', got '%s'", response.Status)
	}

	fmt.Println("Corpus submission test passed - received 202 Accepted response as expected.")
	return nil
}

// Test: Get ortho counts and verify they're being generated
func testGetOrthoCounts() error {
	fmt.Println("Test: Getting ortho counts from repository service...")

	// Wait for the system to process the corpus and generate some orthos
	fmt.Println("Waiting for system to process corpus and generate orthos...")

	// Increase max retries and wait time to give the system more time to process
	maxRetries := 10
	waitTime := 3 * time.Second

	var response OrthoCountsResponse

	// Poll the /results endpoint until we get some orthos or timeout
	for i := 0; i < maxRetries; i++ {
		time.Sleep(waitTime)

		respBody, statusCode, err := makeRequest("GET", "http://localhost:8080/results", nil)
		if err != nil {
			return fmt.Errorf("GET request failed: %v", err)
		}

		fmt.Printf("Attempt %d - Response status code: %d\n", i+1, statusCode)

		if statusCode != http.StatusOK {
			fmt.Printf("Unexpected status code: got %d, want %d\n", statusCode, http.StatusOK)
			continue
		}

		if err := json.Unmarshal(respBody, &response); err != nil {
			return fmt.Errorf("error parsing response: %v", err)
		}

		fmt.Printf("Total ortho count: %d\n", response.TotalCount)

		// If we have orthos, break out of the loop
		if response.TotalCount > 0 {
			fmt.Printf("Found %d orthos after %d attempts\n", response.TotalCount, i+1)
			break
		}

		fmt.Println("No orthos found yet, retrying...")
	}

	// Verify we have some orthos in the system
	if response.TotalCount == 0 {
		return fmt.Errorf("no orthos found after maximum retries")
	}

	// Print the counts by shape and position
	fmt.Println("Ortho counts by shape and position:")
	for shape, positions := range response.Counts {
		fmt.Printf("  Shape %s:\n", shape)
		for position, count := range positions {
			fmt.Printf("    Position %s: %d\n", position, count)
		}
	}

	fmt.Println("Ortho counts test passed - found orthos in the system.")
	return nil
}

func main() {
	fmt.Println("Starting e2e tests...")

	// Wait for services to be available
	waitForServices()

	// Run test for POST /corpora endpoint
	if err := testPostCorpora(); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	// Run test for GET /results endpoint
	if err := testGetOrthoCounts(); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("SUCCESS: End-to-end tests passed!")
}
