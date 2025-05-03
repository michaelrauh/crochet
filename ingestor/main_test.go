package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"crochet/telemetry"
	"crochet/text" // Used when testing text manipulation utilities
	"crochet/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/exp/slices"
)

func init() {
	// This ensures the text package is used directly in the test file
	// to avoid the "imported and not used" error
	_ = text.Vocabulary("sample text")
}

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
	mock.Mock
}

func (m *MockContextService) SendMessage(ctx context.Context, input types.ContextInput) (types.ContextResponse, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(types.ContextResponse), args.Error(1)
}

func (m *MockContextService) GetVersion(ctx context.Context) (types.VersionResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(types.VersionResponse), args.Error(1)
}

func (m *MockContextService) GetContext(ctx context.Context) (types.ContextDataResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(types.ContextDataResponse), args.Error(1)
}

// MockRemediationsService implements the updated RemediationsService interface
type MockRemediationsService struct {
	mock.Mock
}

func (m *MockRemediationsService) FetchRemediations(ctx context.Context, request types.RemediationRequest) (types.RemediationResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(types.RemediationResponse), args.Error(1)
}

func (m *MockRemediationsService) DeleteRemediations(ctx context.Context, hashes []string) (types.DeleteRemediationResponse, error) {
	args := m.Called(ctx, hashes)
	return args.Get(0).(types.DeleteRemediationResponse), args.Error(1)
}

func (m *MockRemediationsService) AddRemediations(ctx context.Context, remediations []types.RemediationTuple) (types.AddRemediationResponse, error) {
	args := m.Called(ctx, remediations)
	return args.Get(0).(types.AddRemediationResponse), args.Error(1)
}

// MockOrthosService implements the OrthosService interface
type MockOrthosService struct {
	mock.Mock
}

func (m *MockOrthosService) GetOrthosByIDs(ctx context.Context, ids []string) (types.OrthosResponse, error) {
	args := m.Called(ctx, ids)
	return args.Get(0).(types.OrthosResponse), args.Error(1)
}

func (m *MockOrthosService) SaveOrthos(ctx context.Context, orthos []types.Ortho) (types.OrthosSaveResponse, error) {
	args := m.Called(ctx, orthos)
	return args.Get(0).(types.OrthosSaveResponse), args.Error(1)
}

// MockWorkServerService implements the WorkServerService interface
type MockWorkServerService struct {
	mock.Mock
}

func (m *MockWorkServerService) PushOrthos(ctx context.Context, orthos []types.Ortho) (types.WorkServerPushResponse, error) {
	args := m.Called(ctx, orthos)
	return args.Get(0).(types.WorkServerPushResponse), args.Error(1)
}

func (m *MockWorkServerService) Pop(ctx context.Context) (types.WorkServerPopResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(types.WorkServerPopResponse), args.Error(1)
}

func (m *MockWorkServerService) Ack(ctx context.Context, id string) (types.WorkServerAckResponse, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(types.WorkServerAckResponse), args.Error(1)
}

func (m *MockWorkServerService) Nack(ctx context.Context, id string) (types.WorkServerAckResponse, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(types.WorkServerAckResponse), args.Error(1)
}

// MockRabbitMQService is a testify mock for the RabbitMQService interface
type MockRabbitMQService struct {
	mock.Mock
}

func (m *MockRabbitMQService) PushContext(ctx context.Context, contextInput types.ContextInput) error {
	args := m.Called(ctx, contextInput)
	return args.Error(0)
}

func (m *MockRabbitMQService) PushVersion(ctx context.Context, version types.VersionInfo) error {
	args := m.Called(ctx, version)
	return args.Error(0)
}

func (m *MockRabbitMQService) PushPairs(ctx context.Context, pairs []types.Pair) error {
	args := m.Called(ctx, pairs)
	return args.Error(0)
}

func (m *MockRabbitMQService) PushSeed(ctx context.Context, seed types.Ortho) error {
	args := m.Called(ctx, seed)
	return args.Error(0)
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
	mockContextService := new(MockContextService)
	mockContextService.On("SendMessage", mock.Anything, mock.AnythingOfType("types.ContextInput")).Return(types.ContextResponse{
		Version: 1,
		NewSubphrases: [][]string{
			{"test", "content"},
		},
	}, nil)

	mockRemediationsService := new(MockRemediationsService)
	mockRemediationsService.On("FetchRemediations", mock.Anything, mock.AnythingOfType("types.RemediationRequest")).Return(types.RemediationResponse{
		Status: "OK",
		Hashes: []string{"1234567890abcdef1234567890abcdef"},
	}, nil)
	mockRemediationsService.On("DeleteRemediations", mock.Anything, mock.Anything).Return(types.DeleteRemediationResponse{
		Status:  "success",
		Message: "Remediations deleted successfully",
		Count:   1,
	}, nil)

	mockOrthosService := new(MockOrthosService)
	mockOrthosService.On("GetOrthosByIDs", mock.Anything, mock.Anything).Return(types.OrthosResponse{
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
	}, nil)

	mockWorkServerService := new(MockWorkServerService)
	mockWorkServerService.On("PushOrthos", mock.Anything, mock.Anything).Return(types.WorkServerPushResponse{
		Status:  "success",
		Message: "Items pushed to work queue successfully",
		Count:   1,
		IDs:     []string{"1234567890abcdef1234567890abcdef"},
	}, nil)

	router := setupGinRouter(mockContextService, mockRemediationsService, mockOrthosService, mockWorkServerService)
	body := `{"title": "Test Title", "text": "Test Content"}`
	req, _ := http.NewRequest(http.MethodPost, "/ingest", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", w.Code, http.StatusOK)
	}

	var response map[string]any
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("json.Unmarshal failed for actual response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("unexpected response: got %v want %v", response["status"], "success")
	}
}

func TestHandleTextInputInvalidJSON(t *testing.T) {
	mockContextService := new(MockContextService)
	mockRemediationsService := new(MockRemediationsService)
	mockOrthosService := new(MockOrthosService)
	mockWorkServerService := new(MockWorkServerService)

	router := setupGinRouter(mockContextService, mockRemediationsService, mockOrthosService, mockWorkServerService)
	invalidJSON := `{title:"Invalid JSON"}`
	req, _ := http.NewRequest(http.MethodPost, "/ingest", bytes.NewBufferString(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for invalid JSON: got %v want %v", w.Code, http.StatusBadRequest)
	}

	var response map[string]any
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("json.Unmarshal failed for error response: %v", err)
	}

	if _, ok := response["error"]; !ok {
		t.Errorf("response does not contain error field: %v", response)
	}
}

func TestHandleTextInputEmptyBody(t *testing.T) {
	mockContextService := new(MockContextService)
	mockRemediationsService := new(MockRemediationsService)
	mockOrthosService := new(MockOrthosService)
	mockWorkServerService := new(MockWorkServerService)

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
	mockContextService := new(MockContextService)
	mockRemediationsService := new(MockRemediationsService)
	mockOrthosService := new(MockOrthosService)
	mockWorkServerService := new(MockWorkServerService)

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
	mockContextService := new(MockContextService)
	mockContextService.On("SendMessage", mock.Anything, mock.AnythingOfType("types.ContextInput")).Return(types.ContextResponse{
		Version: 1,
	}, nil)

	mockRemediationsService := new(MockRemediationsService)
	mockRemediationsService.On("FetchRemediations", mock.Anything, mock.AnythingOfType("types.RemediationRequest")).Return(types.RemediationResponse{
		Status: "OK",
		Hashes: []string{"1234567890abcdef1234567890abcdef"},
	}, nil)
	mockRemediationsService.On("DeleteRemediations", mock.Anything, mock.Anything).Return(types.DeleteRemediationResponse{
		Status:  "success",
		Message: "Remediations deleted successfully",
		Count:   1,
	}, nil)

	mockOrthosService := new(MockOrthosService)
	mockOrthosService.On("GetOrthosByIDs", mock.Anything, mock.Anything).Return(types.OrthosResponse{
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
	}, nil)

	mockWorkServerService := new(MockWorkServerService)
	mockWorkServerService.On("PushOrthos", mock.Anything, mock.Anything).Return(types.WorkServerPushResponse{
		Status:  "success",
		Message: "Items pushed to work queue successfully",
		Count:   1,
		IDs:     []string{"1234567890abcdef1234567890abcdef"},
	}, nil)

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

	var response map[string]any
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("json.Unmarshal failed for actual response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("unexpected response: got %v want %v", response["status"], "success")
	}
}

func TestErrorHandlerMiddleware(t *testing.T) {
	mockContextService := new(MockContextService)
	mockContextService.On("SendMessage", mock.Anything, mock.AnythingOfType("types.ContextInput")).Return(types.ContextResponse{}, telemetry.NewServiceError("ingestor", http.StatusBadGateway, "Test middleware error"))

	mockRemediationsService := new(MockRemediationsService)
	mockOrthosService := new(MockOrthosService)
	mockWorkServerService := new(MockWorkServerService)

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

	var response map[string]any
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

	mockContextService := new(MockContextService)
	mockContextService.On("SendMessage", mock.Anything, mock.AnythingOfType("types.ContextInput")).Return(types.ContextResponse{
		Version: 1,
		NewSubphrases: [][]string{
			{"test", "content"},
		},
	}, nil).Run(func(args mock.Arguments) {
		// Wait for the signal before proceeding - this simulates a long-running operation
		select {
		case <-proceed:
			// Then return a successful response
			return
		case <-time.After(5 * time.Second):
			// Timeout to prevent test hanging indefinitely
			t.Error("Test timed out waiting for proceed signal")
			return
		}
	})

	mockRemediationsService := new(MockRemediationsService)
	mockRemediationsService.On("FetchRemediations", mock.Anything, mock.Anything).Return(types.RemediationResponse{
		Status: "OK",
		Hashes: []string{"1234567890abcdef1234567890abcdef"},
	}, nil)
	mockRemediationsService.On("DeleteRemediations", mock.Anything, mock.Anything).Return(types.DeleteRemediationResponse{
		Status:  "success",
		Message: "Remediations deleted successfully",
		Count:   1,
	}, nil)

	mockOrthosService := new(MockOrthosService)
	mockOrthosService.On("GetOrthosByIDs", mock.Anything, mock.Anything).Return(types.OrthosResponse{
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
	}, nil)

	mockWorkServerService := new(MockWorkServerService)
	mockWorkServerService.On("PushOrthos", mock.Anything, mock.Anything).Return(types.WorkServerPushResponse{
		Status:  "success",
		Message: "Items pushed to work queue successfully",
		Count:   1,
		IDs:     []string{"1234567890abcdef1234567890abcdef"},
	}, nil)

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

	var response map[string]any
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

func TestPostCorpus(t *testing.T) {
	// Create test input
	testTitle := "Test Corpus"
	testText := "This is a test corpus. It contains multiple sentences for testing."

	// Step 1: Setup expectations with argument captures
	var capturedContextInput types.ContextInput
	var capturedVersion types.VersionInfo
	var capturedPairs []types.Pair
	var capturedSeed types.Ortho

	// Create mock context service that captures and validates the input
	mockContextService := new(MockContextService)
	// Use Run to capture the input before returning
	mockContextService.On("SendMessage", mock.Anything, mock.AnythingOfType("types.ContextInput")).
		Run(func(args mock.Arguments) {
			// Capture the actual input for later validation
			capturedContextInput = args.Get(1).(types.ContextInput)
		}).
		Return(types.ContextResponse{
			Version: 42, // This value shouldn't be used for the pushed version
			NewSubphrases: [][]string{
				{"this", "is"},
				{"a", "test"},
				{"corpus", "it"},
				{"contains", "multiple"},
			},
		}, nil)

	// Create testify mock for RabbitMQ service that captures arguments
	mockRabbitMQService := new(MockRabbitMQService)

	// 1. Expect PushContext and capture the input
	mockRabbitMQService.On("PushContext", mock.Anything, mock.AnythingOfType("types.ContextInput")).
		Run(func(args mock.Arguments) {
			// No need to capture here as we've already captured it from SendMessage
		}).
		Return(nil)

	// 2. Expect PushVersion and capture the version
	mockRabbitMQService.On("PushVersion", mock.Anything, mock.AnythingOfType("types.VersionInfo")).
		Run(func(args mock.Arguments) {
			capturedVersion = args.Get(1).(types.VersionInfo)
		}).
		Return(nil)

	// 3. Expect PushPairs and capture the pairs
	mockRabbitMQService.On("PushPairs", mock.Anything, mock.AnythingOfType("[]types.Pair")).
		Run(func(args mock.Arguments) {
			capturedPairs = args.Get(1).([]types.Pair)
		}).
		Return(nil)

	// 4. Expect PushSeed and capture the seed ortho
	mockRabbitMQService.On("PushSeed", mock.Anything, mock.AnythingOfType("types.Ortho")).
		Run(func(args mock.Arguments) {
			capturedSeed = args.Get(1).(types.Ortho)
		}).
		Return(nil)

	// Set up a router with the handlePostCorpus function from main.go
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/corpora", func(c *gin.Context) {
		handlePostCorpus(c, mockContextService, mockRabbitMQService)
	})

	// Create the request with our test input
	requestBody := fmt.Sprintf(`{"title": "%s", "text": "%s"}`, testTitle, testText)
	req, _ := http.NewRequest(http.MethodPost, "/corpora", bytes.NewBufferString(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute the request
	router.ServeHTTP(w, req)

	// Step 2: Verify the response
	// Check the response status code (should be 202 Accepted as per the diagram)
	if w.Code != http.StatusAccepted {
		t.Errorf("handler returned wrong status code: got %v want %v", w.Code, http.StatusAccepted)
	}

	// Check the response body
	var response map[string]any
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("json.Unmarshal failed for response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("unexpected response status: got %v want %v", response["status"], "success")
	}

	// Step 3: Verify that all the expected RabbitMQ calls were made
	mockRabbitMQService.AssertExpectations(t)
	mockContextService.AssertExpectations(t)

	// Step 4: Now validate the captured arguments to ensure they're derived correctly from the input

	// Validate Context Input
	if capturedContextInput.Title != testTitle {
		t.Errorf("Context title incorrect: expected '%s', got '%s'", testTitle, capturedContextInput.Title)
	}

	// Vocabulary should contain words from the test text (case-insensitive check)
	expectedVocabSubset := []string{"this", "is", "a", "test", "corpus"}
	for _, word := range expectedVocabSubset {
		found := false
		for _, vocabWord := range capturedContextInput.Vocabulary {
			// Case-insensitive comparison
			if strings.EqualFold(vocabWord, word) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected vocabulary to contain '%s' (case-insensitive), but it wasn't found", word)
		}
	}

	// Subphrases should be derived from the text
	if len(capturedContextInput.Subphrases) == 0 {
		t.Errorf("Expected subphrases to be non-empty")
	}

	// Validate Version Info - it should be a timestamp, not the context service version
	if capturedVersion.Version <= 0 {
		t.Errorf("Expected version to be a positive timestamp, got %d", capturedVersion.Version)
	}

	// The version should be a unix timestamp (within a reasonable range)
	currentTime := time.Now().Unix()
	fiveMinutesAgo := currentTime - 300
	if int64(capturedVersion.Version) < fiveMinutesAgo || int64(capturedVersion.Version) > currentTime {
		t.Errorf("Version doesn't appear to be a recent timestamp. Got: %d, expected a value between %d and %d",
			capturedVersion.Version, fiveMinutesAgo, currentTime)
	}

	// Validate Pairs
	if len(capturedPairs) == 0 {
		t.Errorf("Expected pairs to be non-empty")
	}

	// Verify pairs match the subphrases format from the mock response
	expectedPairs := []types.Pair{
		{Left: "this", Right: "is"},
		{Left: "a", Right: "test"},
		{Left: "corpus", Right: "it"},
		{Left: "contains", Right: "multiple"},
	}

	for i, expectedPair := range expectedPairs {
		pairFound := false
		for _, pair := range capturedPairs {
			if pair.Left == expectedPair.Left && pair.Right == expectedPair.Right {
				pairFound = true
				break
			}
		}
		if !pairFound {
			t.Errorf("Expected pair %d (%s, %s) not found in captured pairs",
				i, expectedPair.Left, expectedPair.Right)
		}
	}

	// Validate Seed Ortho
	if capturedSeed.Shape == nil || len(capturedSeed.Shape) != 2 {
		t.Errorf("Expected seed ortho to have shape of length 2, got %v", capturedSeed.Shape)
	}

	if capturedSeed.Position == nil || len(capturedSeed.Position) != 2 {
		t.Errorf("Expected seed ortho to have position of length 2, got %v", capturedSeed.Position)
	}

	if capturedSeed.ID == "" {
		t.Errorf("Expected seed ortho to have non-empty ID")
	}
}

// TestHandleGetContext tests the GET /Context endpoint that implements the flow from get_context.md
// In this test, the ingestor acts as the repository and retrieves context data directly from the database
func TestHandleGetContext(t *testing.T) {
	// Set up a mock context store
	mockStore := &MockContextStore{
		vocabulary: []string{"test", "vocabulary", "words"},
		subphrases: [][]string{
			{"test", "phrase"},
			{"another", "line"},
		},
	}

	// Replace the global ctxStore with our mock
	originalStore := ctxStore
	ctxStore = mockStore
	defer func() { ctxStore = originalStore }()

	// Setup the Gin router for testing
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/Context", handleGetContext)

	// Create a test request
	req, _ := http.NewRequest(http.MethodGet, "/Context", nil)
	w := httptest.NewRecorder()

	// Perform the request
	router.ServeHTTP(w, req)

	// Check the response status code (should be 200 OK as per the diagram)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// Parse the response
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	// Verify the version
	assert.Contains(t, response, "version")

	// Verify the vocabulary matches what we set in the mock
	vocabulary, ok := response["vocabulary"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, vocabulary, 3)
	assert.Contains(t, vocabulary, "test")
	assert.Contains(t, vocabulary, "vocabulary")
	assert.Contains(t, vocabulary, "words")

	// Verify the lines match what we set in the mock
	lines, ok := response["lines"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, lines, 2)

	// Verify the first line
	firstLine, ok := lines[0].([]interface{})
	assert.True(t, ok)
	assert.Equal(t, "test", firstLine[0])
	assert.Equal(t, "phrase", firstLine[1])

	// Verify the second line
	secondLine, ok := lines[1].([]interface{})
	assert.True(t, ok)
	assert.Equal(t, "another", secondLine[0])
	assert.Equal(t, "line", secondLine[1])
}

// MockContextStore implements the types.ContextStore interface for testing
type MockContextStore struct {
	vocabulary []string
	subphrases [][]string
	version    int
}

func (m *MockContextStore) SaveVocabulary(words []string) []string {
	newWords := make([]string, 0)
	for _, word := range words {
		if !slices.Contains(m.vocabulary, word) {
			m.vocabulary = append(m.vocabulary, word)
			newWords = append(newWords, word)
		}
	}
	return newWords
}

func (m *MockContextStore) SaveSubphrases(phrases [][]string) [][]string {
	newPhrases := make([][]string, 0)
	for _, phrase := range phrases {
		found := false
		for _, existing := range m.subphrases {
			if reflect.DeepEqual(existing, phrase) {
				found = true
				break
			}
		}
		if !found {
			m.subphrases = append(m.subphrases, phrase)
			newPhrases = append(newPhrases, phrase)
		}
	}
	return newPhrases
}

func (m *MockContextStore) GetVocabulary() []string {
	return m.vocabulary
}

func (m *MockContextStore) GetSubphrases() [][]string {
	return m.subphrases
}

// GetVersion retrieves the current version from the mock store
func (m *MockContextStore) GetVersion() (int, error) {
	if m.version == 0 {
		// Default version is 1
		return 1, nil
	}
	return m.version, nil
}

// SetVersion updates the version in the mock store
func (m *MockContextStore) SetVersion(version int) error {
	m.version = version
	return nil
}

func (m *MockContextStore) Close() error {
	return nil
}

// TestHandleGetContextError tests error handling in the GET /Context endpoint
func TestHandleGetContextError(t *testing.T) {
	// Create a mock store that returns empty data
	mockStore := &MockContextStore{
		vocabulary: []string{},
		subphrases: [][]string{},
	}

	// Replace the global ctxStore with our mock
	originalStore := ctxStore
	ctxStore = mockStore
	defer func() { ctxStore = originalStore }()

	// Setup the Gin router with error handling middleware
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(telemetry.GinErrorHandler())
	router.GET("/Context", handleGetContext)

	// Create a test request
	req, _ := http.NewRequest(http.MethodGet, "/Context", nil)
	w := httptest.NewRecorder()

	// Perform the request
	router.ServeHTTP(w, req)

	// Should return success status
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse the response
	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.Nil(t, err)

	// Verify that the response has the expected empty fields
	assert.Contains(t, response, "vocabulary")
	assert.Contains(t, response, "lines")
	assert.Contains(t, response, "version")

	// Vocabulary should be empty
	vocabulary, ok := response["vocabulary"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, vocabulary, 0)

	// Lines should be empty
	lines, ok := response["lines"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, lines, 0)
}
