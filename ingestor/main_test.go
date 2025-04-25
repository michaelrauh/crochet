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
	os.Setenv("INGESTOR_WORK_SERVER_URL", "http://workserver:8084")
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
	os.Unsetenv("INGESTOR_WORK_SERVER_URL")
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
	GetVersionFunc  func(ctx context.Context) (types.VersionResponse, error)
	GetContextFunc  func(ctx context.Context) (types.ContextDataResponse, error)
}

func (m *MockContextService) SendMessage(ctx context.Context, input types.ContextInput) (types.ContextResponse, error) {
	return m.SendMessageFunc(ctx, input)
}

func (m *MockContextService) GetVersion(ctx context.Context) (types.VersionResponse, error) {
	if m.GetVersionFunc != nil {
		return m.GetVersionFunc(ctx)
	}
	// Default implementation if not provided
	return types.VersionResponse{
		Version: 1,
	}, nil
}

func (m *MockContextService) GetContext(ctx context.Context) (types.ContextDataResponse, error) {
	if m.GetContextFunc != nil {
		return m.GetContextFunc(ctx)
	}
	// Default implementation if not provided
	return types.ContextDataResponse{
		Version:    1,
		Vocabulary: []string{"mock", "vocabulary"},
		Lines:      [][]string{{"mock", "line"}},
	}, nil
}

// MockRemediationsService implements the updated RemediationsService interface
type MockRemediationsService struct {
	FetchRemediationsFunc  func(ctx context.Context, request types.RemediationRequest) (types.RemediationResponse, error)
	DeleteRemediationsFunc func(ctx context.Context, hashes []string) (types.DeleteRemediationResponse, error)
	AddRemediationsFunc    func(ctx context.Context, remediations []types.RemediationTuple) (types.AddRemediationResponse, error)
}

func (m *MockRemediationsService) FetchRemediations(ctx context.Context, request types.RemediationRequest) (types.RemediationResponse, error) {
	return m.FetchRemediationsFunc(ctx, request)
}

func (m *MockRemediationsService) DeleteRemediations(ctx context.Context, hashes []string) (types.DeleteRemediationResponse, error) {
	if m.DeleteRemediationsFunc != nil {
		return m.DeleteRemediationsFunc(ctx, hashes)
	}
	// Default implementation if not provided
	return types.DeleteRemediationResponse{
		Status:  "OK",
		Message: "Mock remediations deleted successfully",
		Count:   len(hashes),
	}, nil
}

func (m *MockRemediationsService) AddRemediations(ctx context.Context, remediations []types.RemediationTuple) (types.AddRemediationResponse, error) {
	if m.AddRemediationsFunc != nil {
		return m.AddRemediationsFunc(ctx, remediations)
	}
	// Default implementation if not provided
	return types.AddRemediationResponse{
		Status:  "OK",
		Message: "Mock remediations added successfully",
	}, nil
}

// MockOrthosService implements the OrthosService interface
type MockOrthosService struct {
	GetOrthosByIDsFunc func(ctx context.Context, ids []string) (types.OrthosResponse, error)
	SaveOrthosFunc     func(ctx context.Context, orthos []types.Ortho) (types.OrthosSaveResponse, error)
}

func (m *MockOrthosService) GetOrthosByIDs(ctx context.Context, ids []string) (types.OrthosResponse, error) {
	return m.GetOrthosByIDsFunc(ctx, ids)
}

func (m *MockOrthosService) SaveOrthos(ctx context.Context, orthos []types.Ortho) (types.OrthosSaveResponse, error) {
	if m.SaveOrthosFunc != nil {
		return m.SaveOrthosFunc(ctx, orthos)
	}
	// Default implementation if not provided
	return types.OrthosSaveResponse{
		Status:  "success",
		Message: "Mock orthos saved successfully",
		Count:   len(orthos),
	}, nil
}

// MockWorkServerService implements the WorkServerService interface
type MockWorkServerService struct {
	PushOrthosFunc func(ctx context.Context, orthos []types.Ortho) (types.WorkServerPushResponse, error)
	PopFunc        func(ctx context.Context) (types.WorkServerPopResponse, error)
	AckFunc        func(ctx context.Context, id string) (types.WorkServerAckResponse, error)
	NackFunc       func(ctx context.Context, id string) (types.WorkServerAckResponse, error)
}

func (m *MockWorkServerService) PushOrthos(ctx context.Context, orthos []types.Ortho) (types.WorkServerPushResponse, error) {
	return m.PushOrthosFunc(ctx, orthos)
}

func (m *MockWorkServerService) Pop(ctx context.Context) (types.WorkServerPopResponse, error) {
	if m.PopFunc != nil {
		return m.PopFunc(ctx)
	}
	// Default implementation if not provided
	return types.WorkServerPopResponse{
		Status:  "success",
		Message: "Mock item popped from queue",
		Ortho:   nil,
		ID:      "",
	}, nil
}

func (m *MockWorkServerService) Ack(ctx context.Context, id string) (types.WorkServerAckResponse, error) {
	if m.AckFunc != nil {
		return m.AckFunc(ctx, id)
	}
	// Default implementation if not provided
	return types.WorkServerAckResponse{
		Status:  "success",
		Message: "Mock work item acknowledged",
	}, nil
}

func (m *MockWorkServerService) Nack(ctx context.Context, id string) (types.WorkServerAckResponse, error) {
	if m.NackFunc != nil {
		return m.NackFunc(ctx, id)
	}
	// Default implementation if not provided
	return types.WorkServerAckResponse{
		Status:  "success",
		Message: "Mock work item returned to queue",
	}, nil
}

// setupGinRouter creates a test Gin router with the specified handlers
func setupGinRouter(contextService types.ContextService, remediationsService types.RemediationsService, orthosService types.OrthosService, workServerService types.WorkServerService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New() // Use New instead of Default to avoid default middleware
	r.Use(gin.Recovery())
	r.Use(telemetry.GinErrorHandler()) // Use the shared error handler middleware
	r.POST("/ingest", func(c *gin.Context) {
		ginHandleTextInput(c, contextService, remediationsService, orthosService, workServerService)
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
						Grid:     map[string]string{"0,0": "test1", "0,1": "test2"},
						Shape:    []int{3, 4},
						Position: []int{5, 6},
						Shell:    7,
					},
				},
			}, nil
		},
	}

	mockWorkServerService := &MockWorkServerService{
		PushOrthosFunc: func(ctx context.Context, orthos []types.Ortho) (types.WorkServerPushResponse, error) {
			return types.WorkServerPushResponse{
				Status:  "success",
				Message: "Items pushed to work queue successfully",
				Count:   len(orthos),
				IDs:     []string{"1234567890abcdef1234567890abcdef"},
			}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService, mockOrthosService, mockWorkServerService)
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

	mockWorkServerService := &MockWorkServerService{
		PushOrthosFunc: func(ctx context.Context, orthos []types.Ortho) (types.WorkServerPushResponse, error) {
			return types.WorkServerPushResponse{}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService, mockOrthosService, mockWorkServerService)
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

	mockWorkServerService := &MockWorkServerService{
		PushOrthosFunc: func(ctx context.Context, orthos []types.Ortho) (types.WorkServerPushResponse, error) {
			return types.WorkServerPushResponse{}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService, mockOrthosService, mockWorkServerService)
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

	mockWorkServerService := &MockWorkServerService{
		PushOrthosFunc: func(ctx context.Context, orthos []types.Ortho) (types.WorkServerPushResponse, error) {
			return types.WorkServerPushResponse{}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService, mockOrthosService, mockWorkServerService)
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

	mockWorkServerService := &MockWorkServerService{
		PushOrthosFunc: func(ctx context.Context, orthos []types.Ortho) (types.WorkServerPushResponse, error) {
			return types.WorkServerPushResponse{}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService, mockOrthosService, mockWorkServerService)
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

	mockWorkServerService := &MockWorkServerService{
		PushOrthosFunc: func(ctx context.Context, orthos []types.Ortho) (types.WorkServerPushResponse, error) {
			return types.WorkServerPushResponse{}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService, mockOrthosService, mockWorkServerService)
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
				Status:  "success",
				Message: "Orthos retrieved successfully",
				Count:   1,
				Orthos: []types.Ortho{
					{
						Grid:     map[string]string{"0,0": "test1", "0,1": "test2"},
						Shape:    []int{2, 2},
						Position: []int{0, 0},
						Shell:    0,
						ID:       "test-ortho-id",
					},
				},
			}, nil
		},
	}

	mockWorkServerService := &MockWorkServerService{
		PushOrthosFunc: func(ctx context.Context, orthos []types.Ortho) (types.WorkServerPushResponse, error) {
			return types.WorkServerPushResponse{
				Status:  "success",
				Message: "Items pushed to work queue successfully",
				Count:   len(orthos),
				IDs:     []string{"1234567890abcdef1234567890abcdef"},
			}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService, mockOrthosService, mockWorkServerService)

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
