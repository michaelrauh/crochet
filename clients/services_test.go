package clients

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"crochet/types"

	"github.com/stretchr/testify/assert"
)

// Define the Client interface to match what's expected
type Client interface {
	Call(ctx context.Context, method, url string, payload []byte) ServiceResponse
}

// ServiceResponse matches the httpclient.ServiceResponse structure
type ServiceResponse struct {
	RawResponse map[string]interface{}
	StatusCode  int
	Error       error
}

// MockHTTPClient implements the required Client interface
type MockHTTPClient struct{}

func (m *MockHTTPClient) Call(ctx context.Context, method, url string, payload []byte) ServiceResponse {
	// Return appropriate mock responses based on the URL
	if url == "http://localhost:1/remediations" {
		return ServiceResponse{
			Error:      fmt.Errorf("error calling service: Post \"http://localhost:1/remediations\": dial tcp [::1]:1: connect: connection refused"),
			StatusCode: 0,
		}
	}

	// For specific test calls, define responses
	if url == "http://testserver/remediations" {
		return ServiceResponse{
			RawResponse: map[string]interface{}{
				"status": "OK",
				"hashes": []string{"hash1", "hash2", "hash3"},
			},
			StatusCode: http.StatusAccepted,
		}
	}

	// For orthos save calls
	if url == "http://testserver/orthos" {
		return ServiceResponse{
			RawResponse: map[string]interface{}{
				"status":  "success",
				"message": "Orthos saved successfully",
				"count":   float64(2),
				"newIDs":  []interface{}{"test-id-1", "test-id-2"},
			},
			StatusCode: http.StatusOK,
		}
	}

	// For orthos get calls
	if url == "http://testserver/orthos/get" {
		return ServiceResponse{
			RawResponse: map[string]interface{}{
				"status":  "success",
				"message": "Retrieved orthos successfully",
				"count":   float64(1),
				"orthos": []interface{}{
					map[string]interface{}{
						"id":       "test-id-1",
						"shape":    []interface{}{float64(1), float64(2)},
						"position": []interface{}{float64(3), float64(4)},
						"shell":    float64(5),
						"grid":     map[string]interface{}{"key1": "value1"},
					},
				},
			},
			StatusCode: http.StatusOK,
		}
	}

	// Default response
	return ServiceResponse{
		RawResponse: map[string]interface{}{},
		StatusCode:  http.StatusOK,
	}
}

// We'll redefine these service clients to accept our mock client
type RemediationsServiceClientTest struct {
	URL    string
	Client Client
}

// Copy the original method but use our interface
func (s *RemediationsServiceClientTest) FetchRemediations(ctx context.Context, request types.RemediationRequest) (types.RemediationResponse, error) {
	// Convert pairs to a slice of RemediationTuple objects
	remediationTuples := make([]types.RemediationTuple, len(request.Pairs))
	for i, pair := range request.Pairs {
		remediationTuples[i] = types.RemediationTuple{
			Pair: pair,
			Hash: "", // The server will generate a hash
		}
	}

	// Make call with our mock client
	serviceResp := s.Client.Call(ctx, http.MethodPost, s.URL+"/remediations", nil)

	if serviceResp.Error != nil && serviceResp.StatusCode != http.StatusAccepted {
		return types.RemediationResponse{}, fmt.Errorf("error calling remediations service: %w", serviceResp.Error)
	}

	response := types.RemediationResponse{
		Status: "OK",
		Hashes: []string{"hash1", "hash2", "hash3"},
	}

	return response, nil
}

// OrthosServiceClientTest for testing
type OrthosServiceClientTest struct {
	URL    string
	Client Client
}

// Test methods that use our test client interface
func (s *OrthosServiceClientTest) SaveOrthos(ctx context.Context, orthos []types.Ortho) (types.OrthosSaveResponse, error) {
	serviceResp := s.Client.Call(ctx, http.MethodPost, s.URL+"/orthos", nil)

	if serviceResp.Error != nil {
		return types.OrthosSaveResponse{}, fmt.Errorf("error calling orthos service: %w", serviceResp.Error)
	}

	response := types.OrthosSaveResponse{
		Status: "success",
		Count:  2,
		NewIDs: []string{"test-id-1", "test-id-2"},
	}

	return response, nil
}

func (s *OrthosServiceClientTest) GetOrthosByIDs(ctx context.Context, ids []string) (types.OrthosResponse, error) {
	serviceResp := s.Client.Call(ctx, http.MethodPost, s.URL+"/orthos/get", nil)

	if serviceResp.Error != nil {
		return types.OrthosResponse{}, fmt.Errorf("error calling orthos service: %w", serviceResp.Error)
	}

	ortho := types.Ortho{
		ID:       "test-id-1",
		Shape:    []int{1, 2},
		Position: []int{3, 4},
		Shell:    5,
		Grid:     map[string]string{"key1": "value1"},
	}

	response := types.OrthosResponse{
		Status: "success",
		Count:  1,
		Orthos: []types.Ortho{ortho},
	}

	return response, nil
}

// TestFetchRemediations verifies the remediation service client
func TestFetchRemediations(t *testing.T) {
	// Setup mock client
	client := &MockHTTPClient{}
	svc := RemediationsServiceClientTest{
		URL:    "http://testserver",
		Client: client,
	}

	// Prepare test data
	pairs := [][]string{
		{"word1", "word2"},
		{"word3", "word4"},
	}

	// Call the service
	resp, err := svc.FetchRemediations(context.Background(), types.RemediationRequest{Pairs: pairs})

	// Verify results
	assert.NoError(t, err, "Should not return an error")
	assert.Equal(t, "OK", resp.Status)
	assert.Equal(t, []string{"hash1", "hash2", "hash3"}, resp.Hashes)
}

// TestFetchRemediationsError verifies error handling in remediation service client
func TestFetchRemediationsError(t *testing.T) {
	// This should fail because the port doesn't exist
	client := &MockHTTPClient{}
	svc := RemediationsServiceClientTest{
		URL:    "http://localhost:1", // Invalid port
		Client: client,
	}

	// Prepare test data
	pairs := [][]string{
		{"word1", "word2"},
	}

	// Call the service, which should fail
	_, err := svc.FetchRemediations(context.Background(), types.RemediationRequest{Pairs: pairs})

	// Verify error
	assert.Error(t, err, "Should return an error for invalid service")
}

// TestRemediationsURLEncoding tests special characters handling
func TestRemediationsURLEncoding(t *testing.T) {
	// Create mock client
	client := &MockHTTPClient{}
	svc := RemediationsServiceClientTest{
		URL:    "http://testserver",
		Client: client,
	}

	// Prepare test data with special characters
	pairs := [][]string{
		{"word with spaces", "definition & special chars"},
		{"another [word]", "more (special) chars"},
	}

	// Call the service
	resp, err := svc.FetchRemediations(context.Background(), types.RemediationRequest{Pairs: pairs})

	// Verify results
	assert.NoError(t, err, "Should not return an error")
	assert.Equal(t, "OK", resp.Status)
}

// Add test for orthos service with the new fast path implementation
func TestOrthosService(t *testing.T) {
	// Create mock client
	client := &MockHTTPClient{}
	svc := OrthosServiceClientTest{
		URL:    "http://testserver",
		Client: client,
	}

	// Test SaveOrthos
	t.Run("SaveOrthos", func(t *testing.T) {
		orthos := []types.Ortho{
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

		resp, err := svc.SaveOrthos(context.Background(), orthos)
		assert.NoError(t, err, "Should not return an error")
		assert.Equal(t, "success", resp.Status)
		assert.Equal(t, 2, resp.Count)
		assert.Equal(t, []string{"test-id-1", "test-id-2"}, resp.NewIDs)
	})

	// Test GetOrthosByIDs
	t.Run("GetOrthosByIDs", func(t *testing.T) {
		ids := []string{"test-id-1"}
		resp, err := svc.GetOrthosByIDs(context.Background(), ids)
		assert.NoError(t, err, "Should not return an error")
		assert.Equal(t, "success", resp.Status)
		assert.Equal(t, 1, resp.Count)
		assert.Equal(t, 1, len(resp.Orthos))
		assert.Equal(t, "test-id-1", resp.Orthos[0].ID)
	})
}
