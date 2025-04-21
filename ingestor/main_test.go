package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

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
}

func teardownTestEnvironment() {
	// Clear environment variables after tests
	os.Unsetenv("INGESTOR_SERVICE_NAME")
	os.Unsetenv("INGESTOR_HOST")
	os.Unsetenv("INGESTOR_PORT")
	os.Unsetenv("INGESTOR_JAEGER_ENDPOINT")
	os.Unsetenv("INGESTOR_CONTEXT_SERVICE_URL")
	os.Unsetenv("INGESTOR_REMEDIATIONS_SERVICE_URL")
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
	SendMessageFunc func(input types.ContextInput) (types.ContextResponse, error)
}

func (m *MockContextService) SendMessage(input types.ContextInput) (types.ContextResponse, error) {
	return m.SendMessageFunc(input)
}

// MockRemediationsService implements the updated RemediationsService interface
type MockRemediationsService struct {
	FetchRemediationsFunc func(request types.RemediationRequest) (types.RemediationResponse, error)
}

func (m *MockRemediationsService) FetchRemediations(request types.RemediationRequest) (types.RemediationResponse, error) {
	return m.FetchRemediationsFunc(request)
}

// setupGinRouter creates a test Gin router with the specified handlers
func setupGinRouter(contextService types.ContextService, remediationsService types.RemediationsService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New() // Use New instead of Default to avoid default middleware
	r.Use(gin.Recovery())
	r.Use(telemetry.GinErrorHandler()) // Use the shared error handler middleware
	r.POST("/ingest", func(c *gin.Context) {
		ginHandleTextInput(c, contextService, remediationsService)
	})
	return r
}

func TestHandleTextInputValidJSON(t *testing.T) {
	mockContextService := &MockContextService{
		SendMessageFunc: func(input types.ContextInput) (types.ContextResponse, error) {
			return types.ContextResponse{
				Version: 1,
				NewSubphrases: [][]string{
					{"test", "content"},
				},
			}, nil
		},
	}

	mockRemediationsService := &MockRemediationsService{
		FetchRemediationsFunc: func(request types.RemediationRequest) (types.RemediationResponse, error) {
			return types.RemediationResponse{
				Status: "OK",
				Hashes: []string{"1234567890abcdef1234567890abcdef"},
			}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService)
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
		SendMessageFunc: func(input types.ContextInput) (types.ContextResponse, error) {
			return types.ContextResponse{}, nil
		},
	}

	mockRemediationsService := &MockRemediationsService{
		FetchRemediationsFunc: func(request types.RemediationRequest) (types.RemediationResponse, error) {
			return types.RemediationResponse{}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService)
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
		SendMessageFunc: func(input types.ContextInput) (types.ContextResponse, error) {
			return types.ContextResponse{}, nil
		},
	}

	mockRemediationsService := &MockRemediationsService{
		FetchRemediationsFunc: func(request types.RemediationRequest) (types.RemediationResponse, error) {
			return types.RemediationResponse{}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService)
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
		SendMessageFunc: func(input types.ContextInput) (types.ContextResponse, error) {
			return types.ContextResponse{}, nil
		},
	}

	mockRemediationsService := &MockRemediationsService{
		FetchRemediationsFunc: func(request types.RemediationRequest) (types.RemediationResponse, error) {
			return types.RemediationResponse{}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService)
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
		SendMessageFunc: func(input types.ContextInput) (types.ContextResponse, error) {
			return types.ContextResponse{
				Version: 1,
			}, nil
		},
	}

	mockRemediationsService := &MockRemediationsService{
		FetchRemediationsFunc: func(request types.RemediationRequest) (types.RemediationResponse, error) {
			return types.RemediationResponse{
				Status: "OK",
			}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService)
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
		SendMessageFunc: func(input types.ContextInput) (types.ContextResponse, error) {
			// Return our custom error directly - this simulates what will happen in the real service
			return types.ContextResponse{}, telemetry.NewServiceError("ingestor", http.StatusBadGateway, "Test middleware error")
		},
	}

	mockRemediationsService := &MockRemediationsService{
		FetchRemediationsFunc: func(request types.RemediationRequest) (types.RemediationResponse, error) {
			return types.RemediationResponse{}, nil
		},
	}

	router := setupGinRouter(mockContextService, mockRemediationsService)
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
