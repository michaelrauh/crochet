package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

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
	os.Setenv("REPOSITORY_SERVICE_NAME", "repository")
	os.Setenv("REPOSITORY_HOST", "0.0.0.0")
	os.Setenv("REPOSITORY_PORT", "8080")
	os.Setenv("REPOSITORY_JAEGER_ENDPOINT", "jaeger:4317")
	os.Setenv("REPOSITORY_CONTEXT_SERVICE_URL", "http://context:8081")
	os.Setenv("REPOSITORY_REMEDIATIONS_SERVICE_URL", "http://remediations:8082")
	os.Setenv("REPOSITORY_ORTHOS_SERVICE_URL", "http://orthos:8083")
	os.Setenv("REPOSITORY_WORK_SERVER_URL", "http://workserver:8084")
	os.Setenv("REPOSITORY_CONTEXT_DB_ENDPOINT", ":memory:")
}

func teardownTestEnvironment() {
	// Clear environment variables after tests
	os.Unsetenv("REPOSITORY_SERVICE_NAME")
	os.Unsetenv("REPOSITORY_HOST")
	os.Unsetenv("REPOSITORY_PORT")
	os.Unsetenv("REPOSITORY_JAEGER_ENDPOINT")
	os.Unsetenv("REPOSITORY_CONTEXT_SERVICE_URL")
	os.Unsetenv("REPOSITORY_REMEDIATIONS_SERVICE_URL")
	os.Unsetenv("REPOSITORY_ORTHOS_SERVICE_URL")
	os.Unsetenv("REPOSITORY_WORK_SERVER_URL")
	os.Unsetenv("REPOSITORY_CONTEXT_DB_ENDPOINT")
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
func setupGinRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New() // Use New instead of Default to avoid default middleware
	r.Use(gin.Recovery())
	r.Use(telemetry.GinErrorHandler()) // Use the shared error handler middleware

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

	router := setupGinRouter()

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

// Test cases for handlePostCorpus, handleGetContext, and other functions would follow
// Using the same patterns as in the original ingestor test file
// ...

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

	// Create a test request
	req, _ := http.NewRequest(http.MethodGet, "/Context", nil)
	w := httptest.NewRecorder()

	// Perform the request
	router.ServeHTTP(w, req)

	// Check the response status code (should be 200 OK)
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
