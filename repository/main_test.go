package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"crochet/config"
	"crochet/telemetry"
	"crochet/text"
	"crochet/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func init() {
	_ = text.Vocabulary("sample text")
}

func setupTestEnvironment() {
	os.Setenv("REPOSITORY_SERVICE_NAME", "repository")
	os.Setenv("REPOSITORY_HOST", "0.0.0.0")
	os.Setenv("REPOSITORY_PORT", "8080")
	os.Setenv("REPOSITORY_JAEGER_ENDPOINT", "jaeger:4317")
	os.Setenv("REPOSITORY_RABBIT_MQ_URL", "amqp://guest:guest@localhost:5672/")
	os.Setenv("REPOSITORY_DB_QUEUE_NAME", "db_queue")
	os.Setenv("REPOSITORY_WORK_QUEUE_NAME", "work_queue")
	os.Setenv("REPOSITORY_CONTEXT_DB_ENDPOINT", ":memory:")
}

func teardownTestEnvironment() {
	os.Unsetenv("REPOSITORY_SERVICE_NAME")
	os.Unsetenv("REPOSITORY_HOST")
	os.Unsetenv("REPOSITORY_PORT")
	os.Unsetenv("REPOSITORY_JAEGER_ENDPOINT")
	os.Unsetenv("REPOSITORY_RABBIT_MQ_URL")
	os.Unsetenv("REPOSITORY_DB_QUEUE_NAME")
	os.Unsetenv("REPOSITORY_WORK_QUEUE_NAME")
	os.Unsetenv("REPOSITORY_CONTEXT_DB_ENDPOINT")
}

func TestMain(m *testing.M) {
	setupTestEnvironment()
	code := m.Run()
	teardownTestEnvironment()
	os.Exit(code)
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
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(telemetry.GinErrorHandler())
	return r
}

// MockContextStore implements the types.ContextStore interface for testing
type MockContextStore struct {
	mock.Mock
}

func (m *MockContextStore) SaveVocabulary(words []string) []string {
	args := m.Called(words)
	return args.Get(0).([]string)
}

func (m *MockContextStore) SaveSubphrases(phrases [][]string) [][]string {
	args := m.Called(phrases)
	return args.Get(0).([][]string)
}

func (m *MockContextStore) GetVocabulary() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *MockContextStore) GetSubphrases() [][]string {
	args := m.Called()
	return args.Get(0).([][]string)
}

func (m *MockContextStore) GetVersion() (int, error) {
	args := m.Called()
	return args.Int(0), args.Error(1)
}

func (m *MockContextStore) SetVersion(version int) error {
	args := m.Called(version)
	return args.Error(0)
}

func (m *MockContextStore) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockContextStore) GetOrthoCount() (int, error) {
	args := m.Called()
	return args.Int(0), args.Error(1)
}

func (m *MockContextStore) GetOrthoCountByShapePosition() (map[string]map[string]int, error) {
	args := m.Called()
	return args.Get(0).(map[string]map[string]int), args.Error(1)
}

func (m *MockContextStore) SaveOrthos(orthos []types.Ortho) error {
	args := m.Called(orthos)
	return args.Error(0)
}

// MockWorkQueueClient implements the WorkQueueClient interface for testing
type MockWorkQueueClient struct {
	mock.Mock
}

func (m *MockWorkQueueClient) PushMessage(ctx context.Context, queueName string, message []byte) error {
	args := m.Called(ctx, queueName, message)
	return args.Error(0)
}

func (m *MockWorkQueueClient) Close(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockWorkQueueClient) PopMessagesFromQueue(ctx context.Context, queueName string, count int) ([]RabbitMessage, error) {
	args := m.Called(ctx, queueName, count)
	return args.Get(0).([]RabbitMessage), args.Error(1)
}

func (m *MockWorkQueueClient) AckByDeliveryTag(ctx context.Context, tag uint64) error {
	args := m.Called(ctx, tag)
	return args.Error(0)
}

// MockOrthosCache implements the OrthosCache interface for testing
type MockOrthosCache struct {
	mock.Mock
}

func (m *MockOrthosCache) FilterNewOrthos(orthos []types.Ortho) []types.Ortho {
	args := m.Called(orthos)
	return args.Get(0).([]types.Ortho)
}

func TestHandleGetContext(t *testing.T) {
	mockStore := new(MockContextStore)
	mockStore.On("GetVocabulary").Return([]string{"test", "vocabulary", "words"})
	mockStore.On("GetSubphrases").Return([][]string{
		{"test", "phrase"},
		{"another", "line"},
	})
	mockStore.On("GetVersion").Return(1, nil)
	mockRabbitMQ := new(MockRabbitMQService)
	mockWorkQueueClient := new(MockWorkQueueClient)
	mockOrthosCache := new(MockOrthosCache)

	handler := NewRepositoryHandler(
		mockStore,
		mockOrthosCache,
		mockRabbitMQ,
		mockWorkQueueClient,
		config.RepositoryConfig{},
	)

	router := setupGinRouter()
	router.GET("/context", handler.HandleGetContext)

	req, _ := http.NewRequest(http.MethodGet, "/context", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "version")
	assert.Equal(t, float64(1), response["version"])

	vocabulary, ok := response["vocabulary"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, vocabulary, 3)
	assert.Contains(t, vocabulary, "test")
	assert.Contains(t, vocabulary, "vocabulary")
	assert.Contains(t, vocabulary, "words")

	lines, ok := response["lines"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, lines, 2)
	mockStore.AssertExpectations(t)
}

func TestHandlePostCorpus(t *testing.T) {
	mockStore := new(MockContextStore)
	mockRabbitMQ := new(MockRabbitMQService)
	mockRabbitMQ.On("PushContext", mock.Anything, mock.AnythingOfType("types.ContextInput")).Return(nil)
	mockRabbitMQ.On("PushVersion", mock.Anything, mock.AnythingOfType("types.VersionInfo")).Return(nil)
	mockRabbitMQ.On("PushPairs", mock.Anything, mock.AnythingOfType("[]types.Pair")).Return(nil)
	mockRabbitMQ.On("PushSeed", mock.Anything, mock.AnythingOfType("types.Ortho")).Return(nil)

	mockWorkQueueClient := new(MockWorkQueueClient)
	mockOrthosCache := new(MockOrthosCache)

	handler := NewRepositoryHandler(
		mockStore,
		mockOrthosCache,
		mockRabbitMQ,
		mockWorkQueueClient,
		config.RepositoryConfig{},
	)

	router := setupGinRouter()
	router.POST("/corpora", handler.HandlePostCorpus)

	body := `{"title": "Test Title", "text": "Test Content"}`
	req, _ := http.NewRequest(http.MethodPost, "/corpora", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, "Corpus processing initiated", response["message"])
	mockRabbitMQ.AssertExpectations(t)
}

func TestHandleGetWork(t *testing.T) {
	mockStore := new(MockContextStore)
	mockStore.On("GetVersion").Return(1, nil)
	mockRabbitMQ := new(MockRabbitMQService)
	mockWorkQueueClient := new(MockWorkQueueClient)
	mockOrthosCache := new(MockOrthosCache)

	timestamp := time.Now().Unix()
	workItem := types.WorkItem{
		ID:        "test-id-12345",
		Data:      map[string]interface{}{"key": "value"},
		Timestamp: timestamp,
	}

	mockMessages := []RabbitMessage{
		{
			DeliveryTag: 12345,
			Data:        workItem,
		},
	}

	mockWorkQueueClient.On("PopMessagesFromQueue", mock.Anything, "test_work_queue", 1).Return(mockMessages, nil)

	handler := NewRepositoryHandler(
		mockStore,
		mockOrthosCache,
		mockRabbitMQ,
		mockWorkQueueClient,
		config.RepositoryConfig{
			WorkQueueName: "test_work_queue",
		},
	)

	router := setupGinRouter()
	router.GET("/work", handler.HandleGetWork)

	req, _ := http.NewRequest(http.MethodGet, "/work", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response types.WorkResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 1, response.Version)
	assert.NotNil(t, response.Work)

	// We're not checking the exact ID anymore since it's dynamically generated
	// Just make sure it starts with "work-" and is not empty
	assert.NotEmpty(t, response.Work.ID)
	assert.True(t, strings.HasPrefix(response.Work.ID, "work-"), "Work ID should start with 'work-'")

	assert.Equal(t, "12345", response.Receipt)

	mockStore.AssertExpectations(t)
	mockWorkQueueClient.AssertExpectations(t)
}

func TestHandlePostResults(t *testing.T) {
	mockStore := new(MockContextStore)
	mockStore.On("GetVersion").Return(1, nil)
	mockRabbitMQ := new(MockRabbitMQService)
	mockWorkQueueClient := new(MockWorkQueueClient)
	mockWorkQueueClient.On("AckByDeliveryTag", mock.Anything, uint64(12345)).Return(nil)
	mockOrthosCache := new(MockOrthosCache)

	newOrthos := []types.Ortho{
		{
			ID:       "test-ortho-id",
			Grid:     map[string]string{"0,0": "test1"},
			Shape:    []int{1, 1},
			Position: []int{0, 0},
			Shell:    0,
		},
	}

	// Create request orthos that will be filtered to newOrthos
	requestOrthos := []types.Ortho{
		{
			ID:       "test-ortho-id",
			Grid:     map[string]string{"0,0": "test1"},
			Shape:    []int{1, 1},
			Position: []int{0, 0},
			Shell:    0,
		},
	}

	mockOrthosCache.On("FilterNewOrthos", requestOrthos).Return(newOrthos)
	mockStore.On("SaveOrthos", newOrthos).Return(nil)

	handler := NewRepositoryHandler(
		mockStore,
		mockOrthosCache,
		mockRabbitMQ,
		mockWorkQueueClient,
		config.RepositoryConfig{},
	)

	router := setupGinRouter()
	router.POST("/results", handler.HandlePostResults)

	requestBody := types.ResultsRequest{
		Receipt: "12345",
		Orthos:  requestOrthos,
	}
	jsonBody, _ := json.Marshal(requestBody)
	req, _ := http.NewRequest(http.MethodPost, "/results", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var response types.ResultsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "success", response.Status)
	assert.Equal(t, 1, response.Version)
	assert.Equal(t, 1, response.NewOrthosCount)

	mockStore.AssertExpectations(t)
	mockWorkQueueClient.AssertExpectations(t)
	mockOrthosCache.AssertExpectations(t)
}

// TestHandleGetResults tests the GET /results endpoint
func TestHandleGetResults(t *testing.T) {
	mockStore := new(MockContextStore)

	// Setup mock for GetOrthoCount
	mockStore.On("GetOrthoCount").Return(42, nil)

	// Setup mock for GetOrthoCountByShapePosition
	countsByShapePosition := map[string]map[string]int{
		"[2,2]": {
			"[0,0]": 10,
			"[1,0]": 8,
		},
		"[3,2]": {
			"[0,0]": 12,
			"[1,0]": 6,
			"[2,0]": 6,
		},
	}
	mockStore.On("GetOrthoCountByShapePosition").Return(countsByShapePosition, nil)

	mockRabbitMQ := new(MockRabbitMQService)
	mockWorkQueueClient := new(MockWorkQueueClient)
	mockOrthosCache := new(MockOrthosCache)

	handler := NewRepositoryHandler(
		mockStore,
		mockOrthosCache,
		mockRabbitMQ,
		mockWorkQueueClient,
		config.RepositoryConfig{},
	)

	router := setupGinRouter()
	router.GET("/results", handler.HandleGetResults)

	req, _ := http.NewRequest(http.MethodGet, "/results", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, "success", response["status"])
	assert.Equal(t, float64(42), response["totalCount"])

	counts, ok := response["counts"].(map[string]interface{})
	assert.True(t, ok)
	assert.Len(t, counts, 2)

	// Check counts for shape [2,2]
	shape1, ok := counts["[2,2]"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(10), shape1["[0,0]"])
	assert.Equal(t, float64(8), shape1["[1,0]"])

	// Check counts for shape [3,2]
	shape2, ok := counts["[3,2]"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(12), shape2["[0,0]"])
	assert.Equal(t, float64(6), shape2["[1,0]"])
	assert.Equal(t, float64(6), shape2["[2,0]"])

	mockStore.AssertExpectations(t)
}
