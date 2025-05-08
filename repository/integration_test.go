package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"crochet/clients"
	"crochet/config"
	"crochet/httpclient"
	"crochet/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegratedPostCorpus tests the full flow of posting a corpus and verifying
// the messages end up in the RabbitMQ queue
func TestIntegratedPostCorpus(t *testing.T) {
	// Set up test configuration
	var cfg config.RepositoryConfig
	cfg.ServiceName = "repository-test"
	cfg.RabbitMQURL = "amqp://guest:guest@localhost:5672/"
	cfg.DBQueueName = "test-db-queue"
	cfg.WorkQueueName = "test-work-queue"
	cfg.ContextDBEndpoint = ":memory:"

	// Initialize in-memory cache for orthos
	orthosCache, err := NewRistrettoOrthosCache()
	require.NoError(t, err, "Failed to initialize orthos cache")

	// Initialize RabbitMQ clients directly
	dbQueueClients, err := clients.NewRabbitMQClients(cfg.RabbitMQURL, cfg.DBQueueName)
	require.NoError(t, err, "Failed to initialize DB queue RabbitMQ clients")
	defer dbQueueClients.CloseAll(context.Background())

	// Create a new client for verifying queue content
	verificationClient, err := httpclient.NewRabbitClient[types.DBQueueItem](cfg.RabbitMQURL)
	require.NoError(t, err, "Failed to create verification RabbitMQ client")
	defer verificationClient.Close(context.Background())

	// Clear the queue before starting the test
	clearQueue(t, verificationClient, cfg.DBQueueName)

	// Initialize work queue client
	workQueueClient, err := NewRabbitWorkQueueClient(cfg.RabbitMQURL)
	require.NoError(t, err, "Failed to initialize work queue client")
	defer workQueueClient.Close(context.Background())

	// Set up context store
	err = initStore(cfg)
	require.NoError(t, err, "Failed to initialize context store")
	defer ctxStore.Close()

	// Create the handler with real dependencies
	handler := NewRepositoryHandler(
		ctxStore,
		orthosCache,
		dbQueueClients.Service,
		workQueueClient,
		cfg,
	)

	// Set up a router with the handler
	router := setupGinRouter()
	router.POST("/corpora", handler.HandlePostCorpus)

	// Prepare the test request
	body := `{"title": "Integration Test Title", "text": "Sample text for integration testing with some phrases."}`
	req, _ := http.NewRequest(http.MethodPost, "/corpora", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Send the request to the router
	router.ServeHTTP(w, req)

	// Verify the response code
	assert.Equal(t, http.StatusAccepted, w.Code)

	// Wait for messages to be processed
	time.Sleep(500 * time.Millisecond)

	// Verify messages in the queue
	messages, err := verificationClient.PopMessagesFromQueue(context.Background(), cfg.DBQueueName, 10)
	require.NoError(t, err, "Failed to pop messages from queue")

	// We should have at least 4 messages (context, version, at least one pair, and seed)
	require.GreaterOrEqual(t, len(messages), 4, "Expected at least 4 messages in the queue")

	// Track which types of messages we've seen
	var foundContext, foundVersion, foundPair, foundSeed bool

	// Analyze each message
	for _, msg := range messages {
		// First acknowledge the message
		err = msg.Ack()
		require.NoError(t, err, "Failed to acknowledge message")

		switch msg.Data.Type {
		case types.DBQueueItemTypeContext:
			foundContext = true
			// Verify context data
			var contextData types.ContextInput
			err = json.Unmarshal(msg.Data.Payload, &contextData)
			require.NoError(t, err, "Failed to unmarshal context data")
			assert.Equal(t, "Integration Test Title", contextData.Title)
			assert.ElementsMatch(t, []string{"Sample", "text", "for", "integration", "testing", "with", "some", "phrases"}, contextData.Vocabulary)
			expectedSubphrases := [][]string{
				{"Sample", "text"},
				{"Sample", "text", "for"},
				{"Sample", "text", "for", "integration"},
				{"Sample", "text", "for", "integration", "testing"},
				{"Sample", "text", "for", "integration", "testing", "with"},
				{"Sample", "text", "for", "integration", "testing", "with", "some"},
				{"Sample", "text", "for", "integration", "testing", "with", "some", "phrases"},
				{"text", "for"},
				{"text", "for", "integration"},
				{"text", "for", "integration", "testing"},
				{"text", "for", "integration", "testing", "with"},
				{"text", "for", "integration", "testing", "with", "some"},
				{"text", "for", "integration", "testing", "with", "some", "phrases"},
				{"for", "integration"},
				{"for", "integration", "testing"},
				{"for", "integration", "testing", "with"},
				{"for", "integration", "testing", "with", "some"},
				{"for", "integration", "testing", "with", "some", "phrases"},
				{"integration", "testing"},
				{"integration", "testing", "with"},
				{"integration", "testing", "with", "some"},
				{"integration", "testing", "with", "some", "phrases"},
				{"testing", "with"},
				{"testing", "with", "some"},
				{"testing", "with", "some", "phrases"},
				{"with", "some"},
				{"with", "some", "phrases"},
				{"some", "phrases"},
			}

			assert.ElementsMatch(t, expectedSubphrases, contextData.Subphrases)

		case types.DBQueueItemTypeVersion:
			foundVersion = true
			var versionData types.VersionInfo
			err = json.Unmarshal(msg.Data.Payload, &versionData)
			require.NoError(t, err, "Failed to unmarshal version data")
			assert.Greater(t, versionData.Version, 0)

		case types.DBQueueItemTypePair:
			foundPair = true
			// Verify pair data

			expectedPairs := [][]string{
				{"Sample", "text"},
				{"text", "for"},
				{"for", "integration"},
				{"integration", "testing"},
				{"testing", "with"},
				{"with", "some"},
				{"some", "phrases"},
			}
			var pairData types.Pair
			err = json.Unmarshal(msg.Data.Payload, &pairData)
			require.NoError(t, err, "Failed to unmarshal pair data")
			assert.NotEmpty(t, pairData.Left)
			assert.NotEmpty(t, pairData.Right)

			// Check if the pair exists in the expected pairs
			actualPair := []string{pairData.Left, pairData.Right}
			assert.Contains(t, expectedPairs, actualPair, "Unexpected pair found")

		case types.DBQueueItemTypeOrtho:
			foundSeed = true
			// Verify seed ortho data
			var orthoData types.Ortho
			err = json.Unmarshal(msg.Data.Payload, &orthoData)
			require.NoError(t, err, "Failed to unmarshal ortho data")
			assert.NotEmpty(t, orthoData.ID)
			assert.Equal(t, []int{2, 2}, orthoData.Shape)
			assert.Equal(t, []int{0, 0}, orthoData.Position)

		default:
			t.Errorf("Unexpected message type: %s", msg.Data.Type)
		}
	}

	// Verify that we found all expected message types
	assert.True(t, foundContext, "Context message not found in queue")
	assert.True(t, foundVersion, "Version message not found in queue")
	assert.True(t, foundPair, "Pair message not found in queue")
	assert.True(t, foundSeed, "Seed ortho message not found in queue")
}

// TestIntegratedGetWork tests the full flow of the GET /Work endpoint as described in the
// sequence diagram in get_work.md
func TestIntegratedGetWork(t *testing.T) {
	// Set up test configuration
	var cfg config.RepositoryConfig
	cfg.ServiceName = "repository-test"
	cfg.RabbitMQURL = "amqp://guest:guest@localhost:5672/"
	cfg.DBQueueName = "test-db-queue-getwork"
	cfg.WorkQueueName = "test-work-queue-getwork"
	cfg.ContextDBEndpoint = ":memory:"

	// Initialize in-memory cache for orthos
	orthosCache, err := NewRistrettoOrthosCache()
	require.NoError(t, err, "Failed to initialize orthos cache")

	// Initialize work queue client
	workQueueClient, err := NewRabbitWorkQueueClient(cfg.RabbitMQURL)
	require.NoError(t, err, "Failed to initialize work queue client")
	defer workQueueClient.Close(context.Background())

	// Create a RabbitMQ client for pushing test work items
	workItemClient, err := httpclient.NewRabbitClient[types.WorkItem](cfg.RabbitMQURL)
	require.NoError(t, err, "Failed to create work item client")
	defer workItemClient.Close(context.Background())

	// Declare work queue
	err = workItemClient.DeclareQueue(context.Background(), cfg.WorkQueueName)
	require.NoError(t, err, "Failed to declare work queue")

	// Clear any existing messages
	clearWorkQueue(t, workItemClient, cfg.WorkQueueName)

	// Initialize DB queue clients
	dbQueueClients, err := clients.NewRabbitMQClients(cfg.RabbitMQURL, cfg.DBQueueName)
	require.NoError(t, err, "Failed to initialize DB queue clients")
	defer dbQueueClients.CloseAll(context.Background())

	// Set up context store with a version
	err = initStore(cfg)
	require.NoError(t, err, "Failed to initialize context store")
	defer ctxStore.Close()

	// Set a specific version in the database that we can check in the response
	testVersion := 12345
	err = ctxStore.SetVersion(testVersion)
	require.NoError(t, err, "Failed to save test version")

	// Verify version was saved
	savedVersion, err := ctxStore.GetVersion()
	require.NoError(t, err, "Failed to get version")
	assert.Equal(t, testVersion, savedVersion, "Version not saved correctly")

	// Push a test work item to the queue
	testWorkItem := types.WorkItem{
		ID: "test-work-item-id",
		Data: map[string]interface{}{
			"test_key": "test_value",
			"ortho": map[string]interface{}{
				"id":       "test-ortho-id",
				"position": []int{1, 2},
				"shape":    []int{3, 3},
				"shell":    1,
				"grid":     map[string]interface{}{},
			},
		},
		Timestamp: time.Now().UnixNano(),
	}

	workItemJSON, err := json.Marshal(testWorkItem)
	require.NoError(t, err, "Failed to marshal test work item")

	err = workItemClient.PushMessage(context.Background(), cfg.WorkQueueName, workItemJSON)
	require.NoError(t, err, "Failed to push test work item to queue")

	// Create the handler with real dependencies
	handler := NewRepositoryHandler(
		ctxStore,
		orthosCache,
		dbQueueClients.Service,
		workQueueClient,
		cfg,
	)

	// Set up a router with the handler
	router := setupGinRouter()
	router.GET("/work", handler.HandleGetWork)

	// Make GET /work request
	req, _ := http.NewRequest(http.MethodGet, "/work", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify the response code
	assert.Equal(t, http.StatusOK, w.Code, "Expected 200 OK response")

	// Parse the response
	var response types.WorkResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Failed to unmarshal response")

	// Verify the version matches what we set in the database
	assert.Equal(t, testVersion, response.Version, "Response version doesn't match expected version")

	// Verify we got a work item back
	require.NotNil(t, response.Work, "Work item in response is nil")

	// We can't check the exact ID as it may be regenerated in the handler
	assert.NotEmpty(t, response.Work.ID, "Work item ID should not be empty")

	// Check the receipt is not empty (should be the delivery tag)
	assert.NotEmpty(t, response.Receipt, "Receipt should not be empty")

	// Verify the work item data
	assert.NotNil(t, response.Work.Data, "Work item data should not be nil")
}

// TestIntegratedPostResults tests the full flow of the POST /Results endpoint as described
// in the sequence diagram in post_results.md
// It first gets a work item via GET /Work, then posts results with the receipt
func TestIntegratedPostResults(t *testing.T) {
	// Set up test configuration
	var cfg config.RepositoryConfig
	cfg.ServiceName = "repository-test"
	cfg.RabbitMQURL = "amqp://guest:guest@localhost:5672/"
	cfg.DBQueueName = "test-db-queue-postresults"
	cfg.WorkQueueName = "test-work-queue-postresults"
	cfg.ContextDBEndpoint = ":memory:"

	// Initialize in-memory cache for orthos
	orthosCache, err := NewRistrettoOrthosCache()
	require.NoError(t, err, "Failed to initialize orthos cache")

	// Initialize work queue client
	workQueueClient, err := NewRabbitWorkQueueClient(cfg.RabbitMQURL)
	require.NoError(t, err, "Failed to initialize work queue client")
	defer workQueueClient.Close(context.Background())

	// Create a RabbitMQ client for pushing test work items and verifying DB queue
	workItemClient, err := httpclient.NewRabbitClient[types.WorkItem](cfg.RabbitMQURL)
	require.NoError(t, err, "Failed to create work item client")
	defer workItemClient.Close(context.Background())

	// DB queue verification client
	dbQueueVerificationClient, err := httpclient.NewRabbitClient[types.DBQueueItem](cfg.RabbitMQURL)
	require.NoError(t, err, "Failed to create DB queue verification client")
	defer dbQueueVerificationClient.Close(context.Background())

	// Declare and clear queues
	err = workItemClient.DeclareQueue(context.Background(), cfg.WorkQueueName)
	require.NoError(t, err, "Failed to declare work queue")
	clearWorkQueue(t, workItemClient, cfg.WorkQueueName)

	err = dbQueueVerificationClient.DeclareQueue(context.Background(), cfg.DBQueueName)
	require.NoError(t, err, "Failed to declare DB queue")
	clearQueue(t, dbQueueVerificationClient, cfg.DBQueueName)

	// Initialize DB queue clients for the repository service
	dbQueueClients, err := clients.NewRabbitMQClients(cfg.RabbitMQURL, cfg.DBQueueName)
	require.NoError(t, err, "Failed to initialize DB queue clients")
	defer dbQueueClients.CloseAll(context.Background())

	// Set up context store with a version
	err = initStore(cfg)
	require.NoError(t, err, "Failed to initialize context store")
	defer ctxStore.Close()

	// Set a specific version
	testVersion := 12345
	err = ctxStore.SetVersion(testVersion)
	require.NoError(t, err, "Failed to save test version")

	// Push a test work item to the queue
	testWorkItem := types.WorkItem{
		ID: "test-work-item-id",
		Data: map[string]interface{}{
			"test_key": "test_value",
			"ortho": map[string]interface{}{
				"id":       "test-ortho-id",
				"position": []int{1, 2},
				"shape":    []int{3, 3},
				"shell":    1,
				"grid":     map[string]interface{}{},
			},
		},
		Timestamp: time.Now().UnixNano(),
	}

	workItemJSON, err := json.Marshal(testWorkItem)
	require.NoError(t, err, "Failed to marshal test work item")

	err = workItemClient.PushMessage(context.Background(), cfg.WorkQueueName, workItemJSON)
	require.NoError(t, err, "Failed to push test work item to queue")

	// Create the handler with real dependencies
	handler := NewRepositoryHandler(
		ctxStore,
		orthosCache,
		dbQueueClients.Service,
		workQueueClient,
		cfg,
	)

	// Set up a router with both GET /work and POST /results endpoints
	router := setupGinRouter()
	router.GET("/work", handler.HandleGetWork)
	router.POST("/results", handler.HandlePostResults)

	// PART 1: Call GET /Work to get a work item and receipt
	workReq, _ := http.NewRequest(http.MethodGet, "/work", nil)
	workResp := httptest.NewRecorder()
	router.ServeHTTP(workResp, workReq)

	// Verify we got a work item successfully
	assert.Equal(t, http.StatusOK, workResp.Code, "Expected 200 OK response for GET /work")

	// Parse the response to get the receipt
	var workResponse types.WorkResponse
	err = json.Unmarshal(workResp.Body.Bytes(), &workResponse)
	require.NoError(t, err, "Failed to unmarshal work response")
	require.NotEmpty(t, workResponse.Receipt, "Receipt should not be empty")

	// PART 2: Send POST /Results with test orthos and the receipt
	// Create test orthos to submit as results
	testOrthos := []types.Ortho{
		{
			ID:       "test-result-ortho-1",
			Grid:     map[string]string{"0,0": "test1", "1,0": "test2"},
			Shape:    []int{2, 2},
			Position: []int{0, 0},
			Shell:    0,
		},
		{
			ID:       "test-result-ortho-2",
			Grid:     map[string]string{"0,0": "test3", "0,1": "test4"},
			Shape:    []int{2, 2},
			Position: []int{1, 1},
			Shell:    1,
		},
	}

	// Create test remediations
	testRemediations := []types.RemediationTuple{
		{
			Pair: []string{"word1", "word2"},
		},
	}

	// Create results request with receipt from work response
	resultsRequest := types.ResultsRequest{
		Orthos:       testOrthos,
		Remediations: testRemediations,
		Receipt:      workResponse.Receipt,
	}

	resultsJSON, err := json.Marshal(resultsRequest)
	require.NoError(t, err, "Failed to marshal results request")

	// Send POST /results request
	resultsReq, _ := http.NewRequest(http.MethodPost, "/results", bytes.NewBuffer(resultsJSON))
	resultsReq.Header.Set("Content-Type", "application/json")
	resultsResp := httptest.NewRecorder()
	router.ServeHTTP(resultsResp, resultsReq)

	// Verify response
	assert.Equal(t, http.StatusOK, resultsResp.Code, "Expected 200 OK response for POST /results")

	var resultsResponse types.ResultsResponse
	err = json.Unmarshal(resultsResp.Body.Bytes(), &resultsResponse)
	require.NoError(t, err, "Failed to unmarshal results response")
	assert.Equal(t, "success", resultsResponse.Status, "Expected success status")
	assert.Equal(t, testVersion, resultsResponse.Version, "Expected correct version")
	// Expected 2 new orthos since they were both new to the cache
	assert.Equal(t, 2, resultsResponse.NewOrthosCount, "Expected 2 new orthos count")

	// PART 3: Verify orthos were added to DB queue
	time.Sleep(500 * time.Millisecond) // Wait for async processing

	// Check the DB queue for the new orthos
	messages, err := dbQueueVerificationClient.PopMessagesFromQueue(context.Background(), cfg.DBQueueName, 10)
	require.NoError(t, err, "Failed to pop messages from DB queue")

	// We expect at least our 2 orthos in the DB queue
	assert.GreaterOrEqual(t, len(messages), 2, "Expected at least 2 messages in DB queue")

	// Track if we found our test orthos
	foundOrtho1 := false
	foundOrtho2 := false

	// Analyze each message
	for _, msg := range messages {
		// Acknowledge the message so it's removed from the queue
		err = msg.Ack()
		require.NoError(t, err, "Failed to acknowledge message")

		// Check if this is one of our ortho messages
		if msg.Data.Type == types.DBQueueItemTypeOrtho {
			var orthoData types.Ortho
			err = json.Unmarshal(msg.Data.Payload, &orthoData)
			require.NoError(t, err, "Failed to unmarshal ortho data")

			if orthoData.ID == "test-result-ortho-1" {
				foundOrtho1 = true
				assert.Equal(t, map[string]string{"0,0": "test1", "1,0": "test2"}, orthoData.Grid)
				assert.Equal(t, []int{2, 2}, orthoData.Shape)
				assert.Equal(t, []int{0, 0}, orthoData.Position)
			} else if orthoData.ID == "test-result-ortho-2" {
				foundOrtho2 = true
				assert.Equal(t, map[string]string{"0,0": "test3", "0,1": "test4"}, orthoData.Grid)
				assert.Equal(t, []int{2, 2}, orthoData.Shape)
				assert.Equal(t, []int{1, 1}, orthoData.Position)
			}
		}
	}

	assert.True(t, foundOrtho1, "First test ortho not found in DB queue")
	assert.True(t, foundOrtho2, "Second test ortho not found in DB queue")

	// PART 4: Verify the receipt was acknowledged
	// We know it was acknowledged if the test completes successfully, as the handler
	// would have returned an error if the acknowledgment failed.
	// Additionally, we can try to get another work item and verify it's not the same one

	secondWorkReq, _ := http.NewRequest(http.MethodGet, "/work", nil)
	secondWorkResp := httptest.NewRecorder()
	router.ServeHTTP(secondWorkResp, secondWorkReq)

	// Should get a 200 but with an empty work item since we popped the only one
	assert.Equal(t, http.StatusOK, secondWorkResp.Code)

	var secondWorkResponse types.WorkResponse
	err = json.Unmarshal(secondWorkResp.Body.Bytes(), &secondWorkResponse)
	require.NoError(t, err)

	// The work response should have an empty receipt as there are no more work items
	assert.Empty(t, secondWorkResponse.Receipt, "Expected empty receipt as the work queue should be empty")
}

// TestIntegratedGetContext tests the GET /Context endpoint functionality as described
// in the sequence diagram in get_context.md
func TestIntegratedGetContext(t *testing.T) {
	// Set up test configuration
	var cfg config.RepositoryConfig
	cfg.ServiceName = "repository-test"
	cfg.RabbitMQURL = "amqp://guest:guest@localhost:5672/"
	cfg.DBQueueName = "test-db-queue-getcontext"
	cfg.WorkQueueName = "test-work-queue-getcontext"
	cfg.ContextDBEndpoint = ":memory:"

	// Initialize in-memory cache for orthos
	orthosCache, err := NewRistrettoOrthosCache()
	require.NoError(t, err, "Failed to initialize orthos cache")

	// Initialize DB queue client
	dbQueueClients, err := clients.NewRabbitMQClients(cfg.RabbitMQURL, cfg.DBQueueName)
	require.NoError(t, err, "Failed to initialize DB queue clients")
	defer dbQueueClients.CloseAll(context.Background())

	// Initialize work queue client
	workQueueClient, err := NewRabbitWorkQueueClient(cfg.RabbitMQURL)
	require.NoError(t, err, "Failed to initialize work queue client")
	defer workQueueClient.Close(context.Background())

	// Set up context store
	err = initStore(cfg)
	require.NoError(t, err, "Failed to initialize context store")
	defer ctxStore.Close()

	// Pre-populate the database with test data
	testVersion := 9876
	testVocabulary := []string{"apple", "banana", "cherry", "date", "elderberry"}
	testSubphrases := [][]string{
		{"apple", "banana"},
		{"banana", "cherry"},
		{"cherry", "date"},
		{"date", "elderberry"},
	}

	// Set version
	err = ctxStore.SetVersion(testVersion)
	require.NoError(t, err, "Failed to set test version")

	// Save vocabulary and subphrases
	newVocab := ctxStore.SaveVocabulary(testVocabulary)
	assert.ElementsMatch(t, testVocabulary, newVocab, "All vocabulary should be saved as new")

	newSubphrases := ctxStore.SaveSubphrases(testSubphrases)
	assert.ElementsMatch(t, testSubphrases, newSubphrases, "All subphrases should be saved as new")

	// Create the handler with real dependencies
	handler := NewRepositoryHandler(
		ctxStore,
		orthosCache,
		dbQueueClients.Service,
		workQueueClient,
		cfg,
	)

	// Set up a router with the handler
	router := setupGinRouter()
	router.GET("/context", handler.HandleGetContext)

	// Make GET /context request
	req, _ := http.NewRequest(http.MethodGet, "/context", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify the response code
	assert.Equal(t, http.StatusOK, w.Code, "Expected 200 OK response")

	// Parse the response
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Failed to unmarshal response")

	// Verify the version matches what we set in the database
	assert.Equal(t, float64(testVersion), response["version"], "Response version doesn't match expected version")

	// Verify vocabulary
	vocabulary, ok := response["vocabulary"].([]interface{})
	require.True(t, ok, "Expected vocabulary in response")
	assert.Len(t, vocabulary, len(testVocabulary), "Vocabulary length mismatch")

	// Convert []interface{} to []string for comparison
	vocabStrings := make([]string, len(vocabulary))
	for i, v := range vocabulary {
		vocabStrings[i] = v.(string)
	}
	assert.ElementsMatch(t, testVocabulary, vocabStrings, "Vocabulary content mismatch")

	// Verify subphrases (called "lines" in the API response)
	lines, ok := response["lines"].([]interface{})
	require.True(t, ok, "Expected lines in response")
	assert.Len(t, lines, len(testSubphrases), "Subphrases/lines length mismatch")

	// Convert []interface{} to [][]string for comparison
	linesStrings := make([][]string, len(lines))
	for i, line := range lines {
		lineArray := line.([]interface{})
		lineStrings := make([]string, len(lineArray))
		for j, word := range lineArray {
			lineStrings[j] = word.(string)
		}
		linesStrings[i] = lineStrings
	}
	assert.ElementsMatch(t, testSubphrases, linesStrings, "Subphrases/lines content mismatch")
}

// clearQueue removes all messages from a queue
func clearQueue(t *testing.T, client *httpclient.RabbitClient[types.DBQueueItem], queueName string) {
	// Ensure the queue exists
	err := client.DeclareQueue(context.Background(), queueName)
	require.NoError(t, err, "Failed to declare queue for clearing")

	// Pop and acknowledge all messages until the queue is empty
	for {
		msgs, err := client.PopMessagesFromQueue(context.Background(), queueName, 10)
		if err != nil || len(msgs) == 0 {
			break
		}
		for _, msg := range msgs {
			err = msg.Ack()
			if err != nil {
				t.Logf("Warning: Failed to acknowledge message during queue clearing: %v", err)
			}
		}
	}
}

// clearWorkQueue removes all messages from a work queue
func clearWorkQueue(t *testing.T, client *httpclient.RabbitClient[types.WorkItem], queueName string) {
	// Ensure the queue exists
	err := client.DeclareQueue(context.Background(), queueName)
	require.NoError(t, err, "Failed to declare queue for clearing")

	// Pop and acknowledge all messages until the queue is empty
	for {
		msgs, err := client.PopMessagesFromQueue(context.Background(), queueName, 10)
		if err != nil || len(msgs) == 0 {
			break
		}
		for _, msg := range msgs {
			err = msg.Ack()
			if err != nil {
				t.Logf("Warning: Failed to acknowledge message during queue clearing: %v", err)
			}
		}
	}
}
