package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"crochet/telemetry"
	"crochet/types"

	"github.com/gin-gonic/gin"
)

func setupTestEnvironment() {
	// Set required environment variables for tests using the new config format
	os.Setenv("INGESTOR_SERVICE_NAME", "ingestor")
	os.Setenv("INGESTOR_HOST", "0.0.0.0")
	os.Setenv("INGESTOR_PORT", "8080")
	os.Setenv("INGESTOR_JAEGER_ENDPOINT", "jaeger:4317")
	os.Setenv("INGESTOR_CONTEXT_SERVICE_URL", "http://context:8081")
	os.Setenv("INGESTOR_REMEDIATIONS_SERVICE_URL", "http://remediations:8082")
	os.Setenv("INGESTOR_ORTHOS_SERVICE_URL", "http://orthos:8083")
}

func teardownTestEnvironment() {
	// Clear environment variables after tests
	os.Unsetenv("INGESTOR_SERVICE_NAME")
	os.Unsetenv("INGESTOR_HOST")
	os.Unsetenv("INGESTOR_PORT")
	os.Unsetenv("INGESTOR_JAEGER_ENDPOINT")
	os.Unsetenv("INGESTOR_CONTEXT_SERVICE_URL")
	os.Unsetenv("INGESTOR_REMEDIATIONS_SERVICE_URL")
	os.Unsetenv("INGESTOR_ORTHOS_SERVICE_URL")
}

// TestMain handles setup and teardown for all tests
func TestMain(m *testing.M) {
	// Setup test environment
	setupTestEnvironment()
	// Run tests
	code := m.Run()
	// Cleanup
	teardownTestEnvironment()
	os.Exit(code)
}

// MockContextService implements the updated ContextService interface
type MockContextService struct {
	SendMessageFunc func(ctx context.Context, input types.ContextInput) (types.ContextResponse, error)
}

func (m *MockContextService) SendMessage(ctx context.Context, input types.ContextInput) (types.ContextResponse, error) {
	return m.SendMessageFunc(ctx, input)
}

// MockRemediationsService implements the updated RemediationsService interface
type MockRemediationsService struct {
	FetchRemediationsFunc func(ctx context.Context, request types.RemediationRequest) (types.RemediationResponse, error)
}

func (m *MockRemediationsService) FetchRemediations(ctx context.Context, request types.RemediationRequest) (types.RemediationResponse, error) {
	return m.FetchRemediationsFunc(ctx, request)
}

// MockOrthosService implements the OrthosService interface
type MockOrthosService struct {
	GetOrthosByIDsFunc func(ctx context.Context, ids []string) (types.OrthosResponse, error)
}

func (m *MockOrthosService) GetOrthosByIDs(ctx context.Context, ids []string) (types.OrthosResponse, error) {
	return m.GetOrthosByIDsFunc(ctx, ids)
}

// setupGinRouter creates a test Gin router with the specified handlers
func setupGinRouter(contextService types.ContextService, remediationsService types.RemediationsService, orthosService types.OrthosService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New() // Use New instead of Default to avoid default middleware
	r.Use(gin.Recovery())
	r.Use(telemetry.GinErrorHandler()) // Use the shared error handler middleware
	r.POST("/ingest", func(c *gin.Context) {
		ginHandleTextInput(c, contextService, remediationsService, orthosService)
	})
	return r
}

func TestHandleTextInputValidJSON(t *testing.T) {
	mockContextService := &MockContextService{
		SendMessageFunc: func(ctx context.Context, input types.ContextInput) (types.ContextResponse, error) {
			return types.ContextResponse{
				Version: 1,
				NewSubphrases: [][]string{
					{"test", "content"},
				},
			}, nil
		},
	}

	mockRemediationsService := &MockRemediationsService{
		FetchRemediationsFunc: func(ctx context.Context, request types.RemediationRequest) (types.RemediationResponse, error) {
			return types.RemediationResponse{
				Status: "OK",
				Hashes: []string{"1234567890abcdef1234567890abcdef"},
			}, nil
		},
	}

	mockOrthosService := &MockOrthosService{
		GetOrthosByIDsFunc: func(ctx context.Context, ids []string) (types.OrthosResponse, error) {
			return types.OrthosResponse{
				Status: "success",
				Count:  1,
				Orthos: []types.Ortho{
					{
						ID:       "1234567890abcdef1234567890abcdef",
						Grid:     map[string]interface{}{"a": 1, "b": 2},
						Shape:    []int{3, 4},
						Position: []int{5, 6},
						Shell:    7,
					},
				},
			}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService, mockOrthosService)
	body := `{"title": "Test Title", "text": "Test Content"}`
	req, _ := http.NewRequest(http.MethodPost, "/ingest", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("json.Unmarshal failed for actual response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("unexpected response: got %v want %v", response["status"], "success")
	}
}

func TestHandleTextInputInvalidJSON(t *testing.T) {
	mockContextService := &MockContextService{
		SendMessageFunc: func(ctx context.Context, input types.ContextInput) (types.ContextResponse, error) {
			return types.ContextResponse{}, nil
		},
	}

	mockRemediationsService := &MockRemediationsService{
		FetchRemediationsFunc: func(ctx context.Context, request types.RemediationRequest) (types.RemediationResponse, error) {
			return types.RemediationResponse{}, nil
		},
	}

	mockOrthosService := &MockOrthosService{
		GetOrthosByIDsFunc: func(ctx context.Context, ids []string) (types.OrthosResponse, error) {
			return types.OrthosResponse{}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService, mockOrthosService)
	invalidJSON := `{title:"Invalid JSON"}`
	req, _ := http.NewRequest(http.MethodPost, "/ingest", bytes.NewBufferString(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for invalid JSON: got %v want %v", w.Code, http.StatusBadRequest)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("json.Unmarshal failed for error response: %v", err)
	}

	if _, ok := response["error"]; !ok {
		t.Errorf("response does not contain error field: %v", response)
	}
}

func TestHandleTextInputEmptyBody(t *testing.T) {
	mockContextService := &MockContextService{
		SendMessageFunc: func(ctx context.Context, input types.ContextInput) (types.ContextResponse, error) {
			return types.ContextResponse{}, nil
		},
	}

	mockRemediationsService := &MockRemediationsService{
		FetchRemediationsFunc: func(ctx context.Context, request types.RemediationRequest) (types.RemediationResponse, error) {
			return types.RemediationResponse{}, nil
		},
	}

	mockOrthosService := &MockOrthosService{
		GetOrthosByIDsFunc: func(ctx context.Context, ids []string) (types.OrthosResponse, error) {
			return types.OrthosResponse{}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService, mockOrthosService)
	req, _ := http.NewRequest("POST", "/ingest", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for empty body: got %v want %v", w.Code, http.StatusBadRequest)
	}
}

func TestHandleTextInputWrongMethod(t *testing.T) {
	mockContextService := &MockContextService{
		SendMessageFunc: func(ctx context.Context, input types.ContextInput) (types.ContextResponse, error) {
			return types.ContextResponse{}, nil
		},
	}

	mockRemediationsService := &MockRemediationsService{
		FetchRemediationsFunc: func(ctx context.Context, request types.RemediationRequest) (types.RemediationResponse, error) {
			return types.RemediationResponse{}, nil
		},
	}

	mockOrthosService := &MockOrthosService{
		GetOrthosByIDsFunc: func(ctx context.Context, ids []string) (types.OrthosResponse, error) {
			return types.OrthosResponse{}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService, mockOrthosService)
	req, _ := http.NewRequest("GET", "/ingest", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Gin returns 404 Not Found for methods that don't match
	if w.Code != http.StatusNotFound {
		t.Errorf("handler returned wrong status code for wrong method: got %v want %v",
			w.Code, http.StatusNotFound)
	}
}

func TestHandleTextInputWithMissingFields(t *testing.T) {
	mockContextService := &MockContextService{
		SendMessageFunc: func(ctx context.Context, input types.ContextInput) (types.ContextResponse, error) {
			return types.ContextResponse{
				Version: 1,
			}, nil
		},
	}

	mockRemediationsService := &MockRemediationsService{
		FetchRemediationsFunc: func(ctx context.Context, request types.RemediationRequest) (types.RemediationResponse, error) {
			return types.RemediationResponse{
				Status: "OK",
			}, nil
		},
	}

	mockOrthosService := &MockOrthosService{
		GetOrthosByIDsFunc: func(ctx context.Context, ids []string) (types.OrthosResponse, error) {
			return types.OrthosResponse{}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService, mockOrthosService)
	// Missing the 'text' field
	body := `{"title": "Test Title"}`
	req, _ := http.NewRequest(http.MethodPost, "/ingest", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// The request should still succeed because mapstructure will set the text field to ""
	if w.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("json.Unmarshal failed for actual response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("unexpected response: got %v want %v", response["status"], "success")
	}
}

func TestErrorHandlerMiddleware(t *testing.T) {
	mockContextService := &MockContextService{
		SendMessageFunc: func(ctx context.Context, input types.ContextInput) (types.ContextResponse, error) {
			// Return our custom error directly - this simulates what will happen in the real service
			return types.ContextResponse{}, telemetry.NewServiceError("ingestor", http.StatusBadGateway, "Test middleware error")
		},
	}

	mockRemediationsService := &MockRemediationsService{
		FetchRemediationsFunc: func(ctx context.Context, request types.RemediationRequest) (types.RemediationResponse, error) {
			return types.RemediationResponse{}, nil
		},
	}

	mockOrthosService := &MockOrthosService{
		GetOrthosByIDsFunc: func(ctx context.Context, ids []string) (types.OrthosResponse, error) {
			return types.OrthosResponse{}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService, mockOrthosService)
	body := `{"title": "Test Title", "text": "Test Content"}`
	req, _ := http.NewRequest(http.MethodPost, "/ingest", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return the status code from our custom error
	if w.Code != http.StatusBadGateway {
		t.Errorf("middleware didn't handle custom error correctly: got %v want %v", w.Code, http.StatusBadGateway)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("json.Unmarshal failed for error response: %v", err)
	}

	// Should include our error message
	if response["error"] != "Test middleware error" {
		t.Errorf("unexpected error message: got %v want %v", response["error"], "Test middleware error")
	}
}

func TestConcurrentRequests(t *testing.T) {
	// Reset the mutex state before the test
	ingestMutex = sync.Mutex{}

	// Create a channel to control the timing of our mock service
	proceed := make(chan struct{})
	completed := make(chan struct{})

	mockContextService := &MockContextService{
		SendMessageFunc: func(ctx context.Context, input types.ContextInput) (types.ContextResponse, error) {
			// Signal when this function is called (first request acquired the lock)
			select {
			case <-proceed:
				// Wait for the signal before proceeding - this simulates a long-running operation
				// Then return a successful response
				return types.ContextResponse{
					Version: 1,
					NewSubphrases: [][]string{
						{"test", "content"},
					},
				}, nil
			case <-time.After(5 * time.Second):
				// Timeout to prevent test hanging indefinitely
				t.Error("Test timed out waiting for proceed signal")
				return types.ContextResponse{}, nil
			}
		},
	}

	mockRemediationsService := &MockRemediationsService{
		FetchRemediationsFunc: func(ctx context.Context, request types.RemediationRequest) (types.RemediationResponse, error) {
			return types.RemediationResponse{
				Status: "OK",
				Hashes: []string{"1234567890abcdef1234567890abcdef"},
			}, nil
		},
	}

	mockOrthosService := &MockOrthosService{
		GetOrthosByIDsFunc: func(ctx context.Context, ids []string) (types.OrthosResponse, error) {
			return types.OrthosResponse{
				Status: "success",
				Count:  1,
				Orthos: []types.Ortho{
					{
						ID:       "1234567890abcdef1234567890abcdef",
						Grid:     map[string]interface{}{"a": 1, "b": 2},
						Shape:    []int{3, 4},
						Position: []int{5, 6},
						Shell:    7,
					},
				},
			}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService, mockOrthosService)

	// Create the first request
	body := `{"title": "Test Title", "text": "Test Content"}`
	req1, _ := http.NewRequest(http.MethodPost, "/ingest", bytes.NewBufferString(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()

	// Start the first request in a goroutine
	go func() {
		router.ServeHTTP(w1, req1)
		close(completed)
	}()

	// Give the first request time to acquire the lock
	time.Sleep(200 * time.Millisecond)

	// Now try a second request while the first one is still processing
	req2, _ := http.NewRequest(http.MethodPost, "/ingest", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	// Second request should get locked status immediately
	if w2.Code != http.StatusLocked {
		t.Errorf("concurrent request handler returned wrong status code: got %v want %v",
			w2.Code, http.StatusLocked)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w2.Body).Decode(&response); err != nil {
		t.Errorf("json.Unmarshal failed for concurrent response: %v", err)
	}

	if response["status"] != "error" {
		t.Errorf("unexpected response status for concurrent request: got %v want %v",
			response["status"], "error")
	}

	// Now allow the first request to complete
	close(proceed)

	// Wait for the first request to complete (with timeout)
	select {
	case <-completed:
		// Request completed successfully
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out waiting for first request to complete")
	}

	// Verify the first request completed successfully
	if w1.Code != http.StatusOK {
		t.Errorf("first request handler returned wrong status code: got %v want %v",
			w1.Code, http.StatusOK)
	}

	// After the first request completes, new requests should succeed
	req3, _ := http.NewRequest(http.MethodPost, "/ingest", bytes.NewBufferString(body))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("subsequent request handler returned wrong status code: got %v want %v",
			w3.Code, http.StatusOK)
	}
}
