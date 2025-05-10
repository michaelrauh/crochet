package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"crochet/httpclient"
	"crochet/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVersionUpdateFlow tests the version update flow described in the feeder_process.md
// This test focuses only on the first alternative in the sequence diagram:
// - DB Queue sends a version update to the Feeder
// - Feeder updates the version in the database
// - Feeder commits the transaction
func TestVersionUpdateFlow(t *testing.T) {
	// Set up test configuration with a shared database file
	cfg := &Config{
		ServiceName:       "feeder-integration-test",
		RabbitMQURL:       "amqp://guest:guest@localhost:5672/",
		DBQueueName:       "test-db-queue-version",
		WorkQueueName:     "test-work-queue-version",
		ContextDBEndpoint: "file:memdb1?mode=memory&cache=shared", // Use shared memory database
	}

	// Initialize an in-memory database with our schema
	db, err := sql.Open("sqlite3", cfg.ContextDBEndpoint)
	require.NoError(t, err, "Failed to open SQLite database")
	defer db.Close()

	// Initialize the database schema
	err = initializeDBSchema(db)
	require.NoError(t, err, "Failed to initialize database schema")

	// Create a context store that will use the same shared database
	ctxStore, err := types.NewLibSQLContextStore(cfg.ContextDBEndpoint)
	require.NoError(t, err, "Failed to create context store")
	defer ctxStore.Close()

	// Verify initial version is 0
	var initialVersion int
	err = db.QueryRow("SELECT version FROM version WHERE id = 1").Scan(&initialVersion)
	require.NoError(t, err, "Failed to get initial version")
	assert.Equal(t, 0, initialVersion, "Initial version should be 0")

	// Create RabbitMQ client for DB queue
	dbQueueClient, err := httpclient.NewRabbitClient[types.DBQueueItem](cfg.RabbitMQURL)
	require.NoError(t, err, "Failed to create RabbitMQ client")
	defer dbQueueClient.Close(context.Background())

	// Ensure the test queue exists and is empty
	err = dbQueueClient.DeclareQueue(context.Background(), cfg.DBQueueName)
	require.NoError(t, err, "Failed to declare test queue")

	// Clear the queue to ensure a clean test environment
	clearQueue(t, dbQueueClient, cfg.DBQueueName)

	// Create a version update message
	testVersion := 42
	versionInfo := types.VersionInfo{
		Version: testVersion,
	}

	versionQueueItem, err := types.CreateVersionQueueItem(versionInfo)
	require.NoError(t, err, "Failed to create version queue item")

	// Serialize the queue item to JSON
	versionItemJSON, err := json.Marshal(versionQueueItem)
	require.NoError(t, err, "Failed to marshal version queue item")

	// Push the version update message to the queue
	err = dbQueueClient.PushMessage(context.Background(), cfg.DBQueueName, versionItemJSON)
	require.NoError(t, err, "Failed to push version message to queue")

	// Pop the message from the queue to test processing
	messages, err := dbQueueClient.PopMessagesFromQueue(context.Background(), cfg.DBQueueName, 1)
	require.NoError(t, err, "Failed to pop message from queue")
	require.Equal(t, 1, len(messages), "Expected 1 message in the queue")

	// Process the message using a transaction
	tx, err := db.Begin()
	require.NoError(t, err, "Failed to start transaction")

	// Extract and verify the message data
	msg := messages[0]
	assert.Equal(t, types.DBQueueItemTypeVersion, msg.Data.Type, "Expected version message")

	version, err := msg.Data.GetVersion()
	require.NoError(t, err, "Failed to extract version from message")
	assert.Equal(t, testVersion, version.Version, "Version mismatch")

	// Update version in database (simulating feeder behavior)
	_, err = tx.Exec("UPDATE version SET version = ?", version.Version)
	require.NoError(t, err, "Failed to update version in database")

	// Commit the transaction
	err = tx.Commit()
	require.NoError(t, err, "Failed to commit transaction")

	// Acknowledge the message
	err = msg.Ack()
	require.NoError(t, err, "Failed to acknowledge message")

	// Verify the version was updated correctly
	var updatedVersion int
	err = db.QueryRow("SELECT version FROM version WHERE id = 1").Scan(&updatedVersion)
	require.NoError(t, err, "Failed to get updated version")
	assert.Equal(t, testVersion, updatedVersion, "Version was not updated correctly")

	// Test that the context store sees the updated version
	storeVersion, err := ctxStore.GetVersion()
	require.NoError(t, err, "Failed to get version from context store")
	assert.Equal(t, testVersion, storeVersion, "Context store returned wrong version")

	// Verify queue is now empty
	emptyMessages, err := dbQueueClient.PopMessagesFromQueue(context.Background(), cfg.DBQueueName, 1)
	require.NoError(t, err, "Failed to check if queue is empty")
	assert.Equal(t, 0, len(emptyMessages), "Queue should be empty")
}

// TestVersionUpdateWithFeederProcessor tests the version update using the actual
// feeder's processBatchMessage function to verify real component behavior
func TestVersionUpdateWithFeederProcessor(t *testing.T) {
	// Set up test configuration with a shared database file
	cfg := &Config{
		ServiceName:       "feeder-integration-test",
		RabbitMQURL:       "amqp://guest:guest@localhost:5672/",
		DBQueueName:       "test-db-queue-version-2",
		WorkQueueName:     "test-work-queue-version-2",
		ContextDBEndpoint: "file:memdb1?mode=memory&cache=shared", // Use shared memory database
	}

	// Initialize an in-memory database with our schema
	db, err := sql.Open("sqlite3", cfg.ContextDBEndpoint)
	require.NoError(t, err, "Failed to open SQLite database")
	defer db.Close()

	// Initialize the database schema
	err = initializeDBSchema(db)
	require.NoError(t, err, "Failed to initialize database schema")

	// Create a context store for the feeder processor
	ctxStore, err := types.NewLibSQLContextStore(cfg.ContextDBEndpoint)
	require.NoError(t, err, "Failed to create context store")
	defer ctxStore.Close()

	// Verify initial version is 0
	var initialVersion int
	err = db.QueryRow("SELECT version FROM version WHERE id = 1").Scan(&initialVersion)
	require.NoError(t, err, "Failed to get initial version")
	assert.Equal(t, 0, initialVersion, "Initial version should be 0")

	// Create RabbitMQ client for testing
	dbQueueClient, err := httpclient.NewRabbitClient[types.DBQueueItem](cfg.RabbitMQURL)
	require.NoError(t, err, "Failed to create RabbitMQ client")
	defer dbQueueClient.Close(context.Background())

	// Initialize global RabbitMQ client (used in processBatchMessage)
	dbQueueClient, err = httpclient.NewRabbitClient[types.DBQueueItem](cfg.RabbitMQURL)
	require.NoError(t, err, "Failed to create global RabbitMQ client")
	defer dbQueueClient.Close(context.Background())

	// Ensure the test queue exists and is empty
	err = dbQueueClient.DeclareQueue(context.Background(), cfg.DBQueueName)
	require.NoError(t, err, "Failed to declare test queue")

	// Clear the queue to ensure a clean test environment
	clearQueue(t, dbQueueClient, cfg.DBQueueName)

	// Create a version update message
	testVersion := 99
	versionInfo := types.VersionInfo{
		Version: testVersion,
	}

	versionQueueItem, err := types.CreateVersionQueueItem(versionInfo)
	require.NoError(t, err, "Failed to create version queue item")

	// Serialize the queue item to JSON
	versionItemJSON, err := json.Marshal(versionQueueItem)
	require.NoError(t, err, "Failed to marshal version queue item")

	// Push the version update message to the queue
	err = dbQueueClient.PushMessage(context.Background(), cfg.DBQueueName, versionItemJSON)
	require.NoError(t, err, "Failed to push version message to queue")

	// Pop the message from the queue
	messages, err := dbQueueClient.PopMessagesFromQueue(context.Background(), cfg.DBQueueName, 1)
	require.NoError(t, err, "Failed to pop message from queue")
	require.Equal(t, 1, len(messages), "Expected 1 message in the queue")

	// Start a new transaction
	tx, err := db.Begin()
	require.NoError(t, err, "Failed to start transaction")
	defer tx.Rollback() // Will be ignored if committed

	// Process the message using the actual feeder processBatchMessage function
	processingCtx := context.Background()
	success := processBatchMessage(processingCtx, messages[0], cfg, tx, ctxStore)
	require.True(t, success, "Message processing should succeed")

	// Commit the transaction
	err = tx.Commit()
	require.NoError(t, err, "Failed to commit transaction")

	// Acknowledge the message
	err = messages[0].Ack()
	require.NoError(t, err, "Failed to acknowledge message")

	// Verify the version was updated correctly
	var updatedVersion int
	err = db.QueryRow("SELECT version FROM version WHERE id = 1").Scan(&updatedVersion)
	require.NoError(t, err, "Failed to get updated version")
	assert.Equal(t, testVersion, updatedVersion, "Version was not updated correctly")

	// Verify the version in context store matches
	dbVersion, err := ctxStore.GetVersion()
	require.NoError(t, err, "Failed to get version from context store")
	assert.Equal(t, testVersion, dbVersion, "Context store returned wrong version")
}

// TestPairProcessingFlow tests the pair processing flow described in the feeder_process.md
// This test focuses on the "Pair" alternative in the sequence diagram:
// - DB Queue sends a pair to the Feeder
// - Feeder looks up orthos from the pair (via remediations)
// - Feeder pushes ortho to Work Queue
// - Feeder pushes remediation delete to DB Queue
func TestPairProcessingFlow(t *testing.T) {
	// Set up test configuration with a shared database file
	cfg := &Config{
		ServiceName:       "feeder-integration-test",
		RabbitMQURL:       "amqp://guest:guest@localhost:5672/",
		DBQueueName:       "test-db-queue-pair",
		WorkQueueName:     "test-work-queue-pair",
		ContextDBEndpoint: "file:memdb2?mode=memory&cache=shared", // Use shared memory database
	}

	// Initialize an in-memory database with our schema
	db, err := sql.Open("sqlite3", cfg.ContextDBEndpoint)
	require.NoError(t, err, "Failed to open SQLite database")
	defer db.Close()

	// Initialize the database schema
	err = initializeDBSchema(db)
	require.NoError(t, err, "Failed to initialize database schema")

	// Create a context store that will use the same shared database
	ctxStore, err := types.NewLibSQLContextStore(cfg.ContextDBEndpoint)
	require.NoError(t, err, "Failed to create context store")
	defer ctxStore.Close()

	// Create RabbitMQ clients for DB and Work queues
	var initErr error
	dbQueueClient, initErr = httpclient.NewRabbitClient[types.DBQueueItem](cfg.RabbitMQURL)
	require.NoError(t, initErr, "Failed to create DB Queue RabbitMQ client")
	defer dbQueueClient.Close(context.Background())

	workQueueClient, err := httpclient.NewRabbitClient[types.WorkItem](cfg.RabbitMQURL)
	require.NoError(t, err, "Failed to create Work Queue RabbitMQ client")
	defer workQueueClient.Close(context.Background())

	// Ensure test queues exist and are empty
	err = dbQueueClient.DeclareQueue(context.Background(), cfg.DBQueueName)
	require.NoError(t, err, "Failed to declare DB test queue")

	err = workQueueClient.DeclareQueue(context.Background(), cfg.WorkQueueName)
	require.NoError(t, err, "Failed to declare Work test queue")

	// Clear the queues to ensure a clean test environment
	clearQueue(t, dbQueueClient, cfg.DBQueueName)
	clearWorkQueue(t, workQueueClient, cfg.WorkQueueName)

	// Set up test data
	orthoID := "test-ortho-123"
	remediationID := "remediation-456"

	// Create a pair that will be used in the test
	testPair := types.Pair{"term1", "term2"}

	// Insert necessary test data into the database
	// 1. Create a proper ortho object with all required fields
	testGrid := map[string]string{
		"0,0": "A",
		"0,1": "B",
		"1,0": "C",
		"1,1": "D",
	}
	testShape := []int{2, 2}    // 2x2 grid
	testPosition := []int{0, 0} // Origin position
	testShell := 0              // Shell value

	// Serialize grid to JSON for storage
	gridJSON, err := json.Marshal(testGrid)
	require.NoError(t, err, "Failed to marshal grid to JSON")

	// Serialize shape to JSON for storage
	shapeJSON, err := json.Marshal(testShape)
	require.NoError(t, err, "Failed to marshal shape to JSON")

	// Serialize position to JSON for storage
	positionJSON, err := json.Marshal(testPosition)
	require.NoError(t, err, "Failed to marshal position to JSON")

	// Insert the complete ortho object into the database
	_, err = db.Exec(
		"INSERT INTO orthos (id, grid, shape, position, shell) VALUES (?, ?, ?, ?, ?)",
		orthoID, gridJSON, shapeJSON, positionJSON, testShell,
	)
	require.NoError(t, err, "Failed to insert test ortho with complete data")

	// 2. Add remediation entry that associates the pair with the ortho
	// Create the pair_key in the same format as createPairKey() in main.go
	pairKey := createPairKey(testPair)

	// Insert into remediations table (matches the table created in initializeDBSchema)
	_, err = db.Exec(
		"INSERT INTO remediations (id, ortho_id, pair_key) VALUES (?, ?, ?)",
		remediationID, orthoID, pairKey,
	)
	require.NoError(t, err, "Failed to insert test remediation")

	// Create a DBQueueItem for the pair and push it to the queue
	pairQueueItem, err := types.CreatePairQueueItem(testPair)
	require.NoError(t, err, "Failed to create pair queue item")
	pairItemJSON, err := json.Marshal(pairQueueItem)
	require.NoError(t, err, "Failed to marshal pair queue item")
	err = dbQueueClient.PushMessage(context.Background(), cfg.DBQueueName, pairItemJSON)
	require.NoError(t, err, "Failed to push pair message to queue")

	// Pop the message from the queue
	messages, err := dbQueueClient.PopMessagesFromQueue(context.Background(), cfg.DBQueueName, 1)
	require.NoError(t, err, "Failed to pop message from queue")
	require.Equal(t, 1, len(messages), "Expected 1 message in the queue")

	// Verify the message is of the expected type
	msg := messages[0]
	assert.Equal(t, types.DBQueueItemTypePair, msg.Data.Type, "Expected pair message")

	// Start a new transaction
	tx, err := db.Begin()
	require.NoError(t, err, "Failed to start transaction")
	defer tx.Rollback() // Will be ignored if committed

	// Process the message using the actual feeder processBatchMessage function
	processingCtx := context.Background()
	success := processBatchMessage(processingCtx, messages[0], cfg, tx, ctxStore)
	require.True(t, success, "Message processing should succeed")

	// Commit the transaction
	err = tx.Commit()
	require.NoError(t, err, "Failed to commit transaction")

	// Acknowledge the message
	err = msg.Ack()
	require.NoError(t, err, "Failed to acknowledge message")

	// Verify an ortho was pushed to the Work Queue
	workMessages, err := workQueueClient.PopMessagesFromQueue(context.Background(), cfg.WorkQueueName, 1)
	require.NoError(t, err, "Failed to pop message from Work Queue")
	require.Equal(t, 1, len(workMessages), "Expected 1 message in the Work Queue")

	// Verify the ortho message content
	workMsg := workMessages[0]
	// Work items have a dynamically generated ID that starts with "work-"
	assert.Contains(t, workMsg.Data.ID, "work-", "Work Queue message ID format is incorrect")

	// When we pushed our ortho to the work queue, it was wrapped in a WorkItem structure
	// We need to access the Data field to get the actual ortho
	orthoData, ok := workMsg.Data.Data.(map[string]interface{})
	require.True(t, ok, "Failed to convert work item data to map")

	// Check that the ortho ID in the work item is correct
	assert.Equal(t, orthoID, orthoData["id"], "Work Queue ortho ID mismatch")

	// Acknowledge the work message
	err = workMsg.Ack()
	require.NoError(t, err, "Failed to acknowledge work message")

	// Verify a remediation delete was pushed to the DB Queue
	dbMessages, err := dbQueueClient.PopMessagesFromQueue(context.Background(), cfg.DBQueueName, 1)
	require.NoError(t, err, "Failed to pop message from DB Queue")
	require.Equal(t, 1, len(dbMessages), "Expected 1 message in the DB Queue")

	// Verify the remediation delete message content
	dbMsg := dbMessages[0]
	assert.Equal(t, types.DBQueueItemTypeRemediationDelete, dbMsg.Data.Type, "Expected remediation delete message")

	remediationDeleteID, err := dbMsg.Data.GetRemediationDeleteID()
	require.NoError(t, err, "Failed to extract remediation delete ID from message")
	// The main.go code uses the orthoID as the remediation delete ID, not the remediationID
	assert.Equal(t, orthoID, remediationDeleteID, "Remediation delete ID mismatch")

	// Acknowledge the remediation delete message
	err = dbMsg.Ack()
	require.NoError(t, err, "Failed to acknowledge remediation delete message")
}

// TestContextProcessingFlow tests the context processing flow described in the feeder_process.md
// This test focuses on the "Context" alternative in the sequence diagram:
// - DB Queue sends a context to the Feeder
// - Feeder upserts the context in the database
func TestContextProcessingFlow(t *testing.T) {
	// Set up test configuration with a shared database file
	cfg := &Config{
		ServiceName:       "feeder-integration-test",
		RabbitMQURL:       "amqp://guest:guest@localhost:5672/",
		DBQueueName:       "test-db-queue-context",
		WorkQueueName:     "test-work-queue-context",
		ContextDBEndpoint: "file:memdb3?mode=memory&cache=shared", // Use shared memory database
	}

	// Initialize an in-memory database with our schema
	db, err := sql.Open("sqlite3", cfg.ContextDBEndpoint)
	require.NoError(t, err, "Failed to open SQLite database")
	defer db.Close()

	// Initialize the database schema
	err = initializeDBSchema(db)
	require.NoError(t, err, "Failed to initialize database schema")

	// Create a context store that will use the same shared database
	ctxStore, err := types.NewLibSQLContextStore(cfg.ContextDBEndpoint)
	require.NoError(t, err, "Failed to create context store")
	defer ctxStore.Close()

	// Create RabbitMQ client for DB queue
	dbQueueClient, err := httpclient.NewRabbitClient[types.DBQueueItem](cfg.RabbitMQURL)
	require.NoError(t, err, "Failed to create RabbitMQ client")
	defer dbQueueClient.Close(context.Background())

	// Ensure the test queue exists and is empty
	err = dbQueueClient.DeclareQueue(context.Background(), cfg.DBQueueName)
	require.NoError(t, err, "Failed to declare test queue")

	// Clear the queue to ensure a clean test environment
	clearQueue(t, dbQueueClient, cfg.DBQueueName)

	// Create a context input for testing
	testContext := types.ContextInput{
		Title:      "Test Context Title",
		Vocabulary: []string{"word1", "word2", "word3"},
		Subphrases: [][]string{{"word1", "word2"}, {"word2", "word3"}},
	}

	// Create a DBQueueItem for the context and push it to the queue
	contextQueueItem, err := types.CreateContextQueueItem(testContext)
	require.NoError(t, err, "Failed to create context queue item")
	contextItemJSON, err := json.Marshal(contextQueueItem)
	require.NoError(t, err, "Failed to marshal context queue item")
	err = dbQueueClient.PushMessage(context.Background(), cfg.DBQueueName, contextItemJSON)
	require.NoError(t, err, "Failed to push context message to queue")

	// Pop the message from the queue
	messages, err := dbQueueClient.PopMessagesFromQueue(context.Background(), cfg.DBQueueName, 1)
	require.NoError(t, err, "Failed to pop message from queue")
	require.Equal(t, 1, len(messages), "Expected 1 message in the queue")

	// Verify the message is of the expected type
	msg := messages[0]
	assert.Equal(t, types.DBQueueItemTypeContext, msg.Data.Type, "Expected context message")

	// Extract context from the message to verify contents
	contextData, err := msg.Data.GetContext()
	require.NoError(t, err, "Failed to extract context from message")
	assert.Equal(t, testContext.Title, contextData.Title, "Context title mismatch")
	assert.Equal(t, testContext.Vocabulary, contextData.Vocabulary, "Context vocabulary mismatch")
	assert.Equal(t, testContext.Subphrases, contextData.Subphrases, "Context subphrases mismatch")

	// Start a new transaction
	tx, err := db.Begin()
	require.NoError(t, err, "Failed to start transaction")
	defer tx.Rollback() // Will be ignored if committed

	// Process the message using the actual feeder processBatchMessage function
	processingCtx := context.Background()
	success := processBatchMessage(processingCtx, messages[0], cfg, tx, ctxStore)
	require.True(t, success, "Message processing should succeed")

	// Commit the transaction
	err = tx.Commit()
	require.NoError(t, err, "Failed to commit transaction")

	// Acknowledge the message
	err = msg.Ack()
	require.NoError(t, err, "Failed to acknowledge message")

	// Verify the context was stored in the database by retrieving it
	// Unlike other contexts that might use a "context_titles" table, here we need to check
	// the vocabulary and subphrases tables directly as that's how the 'processBatchMessage'
	// function in main.go handles context items.

	// Check if vocabulary words were stored in the vocabulary table
	for _, word := range testContext.Vocabulary {
		var existingWord string
		err = db.QueryRow("SELECT word FROM vocabulary WHERE word = ?", word).Scan(&existingWord)
		require.NoError(t, err, "Failed to get stored vocabulary word "+word)
		assert.Equal(t, word, existingWord, "Stored vocabulary word mismatch for "+word)
	}

	// Check if subphrases were stored in the subphrases table
	for _, subphrase := range testContext.Subphrases {
		// Join the subphrase array elements with a space, as done in processBatchMessage
		subphraseStr := strings.Join(subphrase, " ")
		var existingSubphrase string
		err = db.QueryRow("SELECT phrase FROM subphrases WHERE phrase = ?", subphraseStr).Scan(&existingSubphrase)
		require.NoError(t, err, "Failed to get stored subphrase "+subphraseStr)
		assert.Equal(t, subphraseStr, existingSubphrase, "Stored subphrase mismatch for "+subphraseStr)
	}

	// Verify queue is now empty
	emptyMessages, err := dbQueueClient.PopMessagesFromQueue(context.Background(), cfg.DBQueueName, 1)
	require.NoError(t, err, "Failed to check if queue is empty")
	assert.Equal(t, 0, len(emptyMessages), "Queue should be empty")
}

// TestOrthoProcessingFlow tests the ortho processing flow described in the feeder_process.md
// This test focuses on the "Ortho" alternative in the sequence diagram:
// - DB Queue sends an ortho to the Feeder
// - Feeder upserts the ortho in the database
// - Feeder pushes the ortho to the Work Queue
func TestOrthoProcessingFlow(t *testing.T) {
	// Set up test configuration with a shared database file
	cfg := &Config{
		ServiceName:       "feeder-integration-test",
		RabbitMQURL:       "amqp://guest:guest@localhost:5672/",
		DBQueueName:       "test-db-queue-ortho",
		WorkQueueName:     "test-work-queue-ortho",
		ContextDBEndpoint: "file:memdb4?mode=memory&cache=shared", // Use shared memory database
	}

	// Initialize an in-memory database with our schema
	db, err := sql.Open("sqlite3", cfg.ContextDBEndpoint)
	require.NoError(t, err, "Failed to open SQLite database")
	defer db.Close()

	// Initialize the database schema
	err = initializeDBSchema(db)
	require.NoError(t, err, "Failed to initialize database schema")

	// Create a context store that will use the same shared database
	ctxStore, err := types.NewLibSQLContextStore(cfg.ContextDBEndpoint)
	require.NoError(t, err, "Failed to create context store")
	defer ctxStore.Close()

	// Create RabbitMQ clients for DB and Work queues
	dbQueueClient, err := httpclient.NewRabbitClient[types.DBQueueItem](cfg.RabbitMQURL)
	require.NoError(t, err, "Failed to create DB Queue RabbitMQ client")
	defer dbQueueClient.Close(context.Background())

	workQueueClient, err := httpclient.NewRabbitClient[types.WorkItem](cfg.RabbitMQURL)
	require.NoError(t, err, "Failed to create Work Queue RabbitMQ client")
	defer workQueueClient.Close(context.Background())

	// Ensure test queues exist and are empty
	err = dbQueueClient.DeclareQueue(context.Background(), cfg.DBQueueName)
	require.NoError(t, err, "Failed to declare DB test queue")

	err = workQueueClient.DeclareQueue(context.Background(), cfg.WorkQueueName)
	require.NoError(t, err, "Failed to declare Work test queue")

	// Clear the queues to ensure a clean test environment
	clearQueue(t, dbQueueClient, cfg.DBQueueName)
	clearWorkQueue(t, workQueueClient, cfg.WorkQueueName)

	// Create a test ortho
	orthoID := "test-ortho-789"
	testGrid := map[string]string{
		"0,0": "X",
		"0,1": "Y",
		"1,0": "Z",
		"1,1": "W",
	}
	testShape := []int{2, 2}    // 2x2 grid
	testPosition := []int{1, 1} // Position (1,1)
	testShell := 1              // Shell value

	testOrtho := types.Ortho{
		ID:       orthoID,
		Grid:     testGrid,
		Shape:    testShape,
		Position: testPosition,
		Shell:    testShell,
	}

	// Create a DBQueueItem for the ortho and push it to the queue
	orthoQueueItem, err := types.CreateOrthoQueueItem(testOrtho)
	require.NoError(t, err, "Failed to create ortho queue item")
	orthoItemJSON, err := json.Marshal(orthoQueueItem)
	require.NoError(t, err, "Failed to marshal ortho queue item")
	err = dbQueueClient.PushMessage(context.Background(), cfg.DBQueueName, orthoItemJSON)
	require.NoError(t, err, "Failed to push ortho message to queue")

	// Pop the message from the queue
	messages, err := dbQueueClient.PopMessagesFromQueue(context.Background(), cfg.DBQueueName, 1)
	require.NoError(t, err, "Failed to pop message from queue")
	require.Equal(t, 1, len(messages), "Expected 1 message in the queue")

	// Verify the message is of the expected type
	msg := messages[0]
	assert.Equal(t, types.DBQueueItemTypeOrtho, msg.Data.Type, "Expected ortho message")

	// Extract ortho from the message to verify contents
	orthoData, err := msg.Data.GetOrtho()
	require.NoError(t, err, "Failed to extract ortho from message")
	assert.Equal(t, testOrtho.ID, orthoData.ID, "Ortho ID mismatch")
	assert.Equal(t, testOrtho.Grid, orthoData.Grid, "Ortho grid mismatch")
	assert.Equal(t, testOrtho.Shape, orthoData.Shape, "Ortho shape mismatch")
	assert.Equal(t, testOrtho.Position, orthoData.Position, "Ortho position mismatch")
	assert.Equal(t, testOrtho.Shell, orthoData.Shell, "Ortho shell mismatch")

	// Start a new transaction
	tx, err := db.Begin()
	require.NoError(t, err, "Failed to start transaction")
	defer tx.Rollback() // Will be ignored if committed

	// Process the message using the actual feeder processBatchMessage function
	processingCtx := context.Background()
	success := processBatchMessage(processingCtx, messages[0], cfg, tx, ctxStore)
	require.True(t, success, "Message processing should succeed")

	// Commit the transaction
	err = tx.Commit()
	require.NoError(t, err, "Failed to commit transaction")

	// Acknowledge the message
	err = msg.Ack()
	require.NoError(t, err, "Failed to acknowledge message")

	// Verify the ortho was stored in the database
	var storedID string
	var storedGridJSON, storedShapeJSON, storedPositionJSON []byte
	var storedShell int
	err = db.QueryRow(
		"SELECT id, grid, shape, position, shell FROM orthos WHERE id = ?",
		orthoID,
	).Scan(&storedID, &storedGridJSON, &storedShapeJSON, &storedPositionJSON, &storedShell)
	require.NoError(t, err, "Failed to get stored ortho")

	// Verify stored data matches original data
	assert.Equal(t, orthoID, storedID, "Stored ortho ID mismatch")
	assert.Equal(t, testOrtho.Shell, storedShell, "Stored ortho shell mismatch")

	// Verify JSON fields by unmarshaling them
	var storedGrid map[string]string
	err = json.Unmarshal(storedGridJSON, &storedGrid)
	require.NoError(t, err, "Failed to unmarshal stored grid JSON")
	assert.Equal(t, testOrtho.Grid, storedGrid, "Stored ortho grid mismatch")

	var storedShape []int
	err = json.Unmarshal(storedShapeJSON, &storedShape)
	require.NoError(t, err, "Failed to unmarshal stored shape JSON")
	assert.Equal(t, testOrtho.Shape, storedShape, "Stored ortho shape mismatch")

	var storedPosition []int
	err = json.Unmarshal(storedPositionJSON, &storedPosition)
	require.NoError(t, err, "Failed to unmarshal stored position JSON")
	assert.Equal(t, testOrtho.Position, storedPosition, "Stored ortho position mismatch")

	// Verify an ortho was pushed to the Work Queue
	workMessages, err := workQueueClient.PopMessagesFromQueue(context.Background(), cfg.WorkQueueName, 1)
	require.NoError(t, err, "Failed to pop message from Work Queue")
	require.Equal(t, 1, len(workMessages), "Expected 1 message in the Work Queue")

	// Verify the ortho message content
	workMsg := workMessages[0]
	// Work items have a dynamically generated ID that starts with "work-"
	assert.Contains(t, workMsg.Data.ID, "work-", "Work Queue message ID format is incorrect")

	// When we pushed our ortho to the work queue, it was wrapped in a WorkItem structure
	// We need to access the Data field to get the actual ortho
	orthoDataFromWork, ok := workMsg.Data.Data.(map[string]interface{})
	require.True(t, ok, "Failed to convert work item data to map")

	// Check that the ortho ID in the work item is correct
	assert.Equal(t, orthoID, orthoDataFromWork["id"], "Work Queue ortho ID mismatch")

	// Acknowledge the work message
	err = workMsg.Ack()
	require.NoError(t, err, "Failed to acknowledge work message")

	// Verify queues are now empty
	emptyDBMessages, err := dbQueueClient.PopMessagesFromQueue(context.Background(), cfg.DBQueueName, 1)
	require.NoError(t, err, "Failed to check if DB Queue is empty")
	assert.Equal(t, 0, len(emptyDBMessages), "DB Queue should be empty")

	emptyWorkMessages, err := workQueueClient.PopMessagesFromQueue(context.Background(), cfg.WorkQueueName, 1)
	require.NoError(t, err, "Failed to check if Work Queue is empty")
	assert.Equal(t, 0, len(emptyWorkMessages), "Work Queue should be empty")
}

// TestRemediationDeleteFlow tests the remediation delete flow described in the feeder_process.md
// This test focuses on the "Remediation Delete" alternative in the sequence diagram:
// - DB Queue sends a remediation delete to the Feeder
// - Feeder deletes the remediation from the database
func TestRemediationDeleteFlow(t *testing.T) {
	// Set up test configuration with a shared database file
	cfg := &Config{
		ServiceName:       "feeder-integration-test",
		RabbitMQURL:       "amqp://guest:guest@localhost:5672/",
		DBQueueName:       "test-db-queue-remediation-delete",
		WorkQueueName:     "test-work-queue-remediation-delete",
		ContextDBEndpoint: "file:memdb5?mode=memory&cache=shared", // Use shared memory database
	}

	// Initialize an in-memory database with our schema
	db, err := sql.Open("sqlite3", cfg.ContextDBEndpoint)
	require.NoError(t, err, "Failed to open SQLite database")
	defer db.Close()

	// Initialize the database schema
	err = initializeDBSchema(db)
	require.NoError(t, err, "Failed to initialize database schema")

	// Create a context store that will use the same shared database
	ctxStore, err := types.NewLibSQLContextStore(cfg.ContextDBEndpoint)
	require.NoError(t, err, "Failed to create context store")
	defer ctxStore.Close()

	// Create RabbitMQ client for DB queue
	dbQueueClient, err := httpclient.NewRabbitClient[types.DBQueueItem](cfg.RabbitMQURL)
	require.NoError(t, err, "Failed to create RabbitMQ client")
	defer dbQueueClient.Close(context.Background())

	// Ensure the test queue exists and is empty
	err = dbQueueClient.DeclareQueue(context.Background(), cfg.DBQueueName)
	require.NoError(t, err, "Failed to declare test queue")

	// Clear the queue to ensure a clean test environment
	clearQueue(t, dbQueueClient, cfg.DBQueueName)

	// Set up test data
	orthoID := "test-ortho-remediation-delete"
	remediationID := "remediation-to-delete"
	pairKey := "term1:term2"

	// Insert a test remediation entry into the database
	_, err = db.Exec(
		"INSERT INTO remediations (id, ortho_id, pair_key) VALUES (?, ?, ?)",
		remediationID, orthoID, pairKey,
	)
	require.NoError(t, err, "Failed to insert test remediation")

	// Verify the remediation was inserted correctly
	var existingRemediationID string
	err = db.QueryRow("SELECT id FROM remediations WHERE id = ?", remediationID).Scan(&existingRemediationID)
	require.NoError(t, err, "Failed to verify remediation was inserted")
	assert.Equal(t, remediationID, existingRemediationID, "Remediation ID mismatch after insertion")

	// Create a DBQueueItem for the remediation delete and push it to the queue
	// Note: Based on TestPairProcessingFlow, the remediation delete ID is the ortho ID
	remediationDeleteQueueItem, err := types.CreateRemediationDeleteQueueItem(orthoID)
	require.NoError(t, err, "Failed to create remediation delete queue item")
	remediationDeleteItemJSON, err := json.Marshal(remediationDeleteQueueItem)
	require.NoError(t, err, "Failed to marshal remediation delete queue item")
	err = dbQueueClient.PushMessage(context.Background(), cfg.DBQueueName, remediationDeleteItemJSON)
	require.NoError(t, err, "Failed to push remediation delete message to queue")

	// Pop the message from the queue
	messages, err := dbQueueClient.PopMessagesFromQueue(context.Background(), cfg.DBQueueName, 1)
	require.NoError(t, err, "Failed to pop message from queue")
	require.Equal(t, 1, len(messages), "Expected 1 message in the queue")

	// Verify the message is of the expected type
	msg := messages[0]
	assert.Equal(t, types.DBQueueItemTypeRemediationDelete, msg.Data.Type, "Expected remediation delete message")

	// Extract remediation delete ID from the message to verify contents
	remediationDeleteID, err := msg.Data.GetRemediationDeleteID()
	require.NoError(t, err, "Failed to extract remediation delete ID from message")
	assert.Equal(t, orthoID, remediationDeleteID, "Remediation delete ID mismatch")

	// Start a new transaction
	tx, err := db.Begin()
	require.NoError(t, err, "Failed to start transaction")
	defer tx.Rollback() // Will be ignored if committed

	// Process the message using the actual feeder processBatchMessage function
	processingCtx := context.Background()
	success := processBatchMessage(processingCtx, messages[0], cfg, tx, ctxStore)
	require.True(t, success, "Message processing should succeed")

	// Commit the transaction
	err = tx.Commit()
	require.NoError(t, err, "Failed to commit transaction")

	// Acknowledge the message
	err = msg.Ack()
	require.NoError(t, err, "Failed to acknowledge message")

	// Verify the remediation was deleted from the database
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM remediations WHERE ortho_id = ?", orthoID).Scan(&count)
	require.NoError(t, err, "Failed to count remediations after delete")
	assert.Equal(t, 0, count, "Remediation was not deleted")

	// Verify queue is now empty
	emptyMessages, err := dbQueueClient.PopMessagesFromQueue(context.Background(), cfg.DBQueueName, 1)
	require.NoError(t, err, "Failed to check if queue is empty")
	assert.Equal(t, 0, len(emptyMessages), "Queue should be empty")
}

// TestRemediationFlow tests the remediation flow described in the feeder_process.md
// This test focuses on the "Remediation" alternative in the sequence diagram:
// - DB Queue sends a remediation to the Feeder
// - Feeder upserts the remediation in the database
func TestRemediationFlow(t *testing.T) {
	// Set up test configuration with a shared database file
	cfg := &Config{
		ServiceName:       "feeder-integration-test",
		RabbitMQURL:       "amqp://guest:guest@localhost:5672/",
		DBQueueName:       "test-db-queue-remediation",
		WorkQueueName:     "test-work-queue-remediation",
		ContextDBEndpoint: "file:memdb6?mode=memory&cache=shared", // Use shared memory database
	}

	// Initialize an in-memory database with our schema
	db, err := sql.Open("sqlite3", cfg.ContextDBEndpoint)
	require.NoError(t, err, "Failed to open SQLite database")
	defer db.Close()

	// Initialize the database schema
	err = initializeDBSchema(db)
	require.NoError(t, err, "Failed to initialize database schema")

	// Create a context store that will use the same shared database
	ctxStore, err := types.NewLibSQLContextStore(cfg.ContextDBEndpoint)
	require.NoError(t, err, "Failed to create context store")
	defer ctxStore.Close()

	// Create RabbitMQ client for DB queue
	dbQueueClient, err := httpclient.NewRabbitClient[types.DBQueueItem](cfg.RabbitMQURL)
	require.NoError(t, err, "Failed to create RabbitMQ client")
	defer dbQueueClient.Close(context.Background())

	// Ensure the test queue exists and is empty
	err = dbQueueClient.DeclareQueue(context.Background(), cfg.DBQueueName)
	require.NoError(t, err, "Failed to declare test queue")

	// Clear the queue to ensure a clean test environment
	clearQueue(t, dbQueueClient, cfg.DBQueueName)

	// Set up test data
	testPair := []string{"term3", "term4"}
	pairKey := "term3:term4"

	// Verify remediation doesn't exist yet
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM remediations WHERE pair_key = ?", pairKey).Scan(&count)
	require.NoError(t, err, "Failed to count remediations before insertion")
	assert.Equal(t, 0, count, "No remediation should exist yet")

	// Create a RemediationTuple object
	testRemediation := types.RemediationTuple{
		Pair: testPair,
	}

	// Create a DBQueueItem for the remediation and push it to the queue
	remediationQueueItem, err := types.CreateRemediationQueueItem(testRemediation)
	require.NoError(t, err, "Failed to create remediation queue item")
	remediationItemJSON, err := json.Marshal(remediationQueueItem)
	require.NoError(t, err, "Failed to marshal remediation queue item")
	err = dbQueueClient.PushMessage(context.Background(), cfg.DBQueueName, remediationItemJSON)
	require.NoError(t, err, "Failed to push remediation message to queue")

	// Pop the message from the queue
	messages, err := dbQueueClient.PopMessagesFromQueue(context.Background(), cfg.DBQueueName, 1)
	require.NoError(t, err, "Failed to pop message from queue")
	require.Equal(t, 1, len(messages), "Expected 1 message in the queue")

	// Verify the message is of the expected type
	msg := messages[0]
	assert.Equal(t, types.DBQueueItemTypeRemediation, msg.Data.Type, "Expected remediation message")

	// Extract remediation from the message to verify contents
	remediation, err := msg.Data.GetRemediation()
	require.NoError(t, err, "Failed to extract remediation from message")
	assert.Equal(t, testPair, remediation.Pair, "Remediation pair mismatch")

	// Start a new transaction
	tx, err := db.Begin()
	require.NoError(t, err, "Failed to start transaction")
	defer tx.Rollback() // Will be ignored if committed

	// Process the message using the actual feeder processBatchMessage function
	processingCtx := context.Background()
	success := processBatchMessage(processingCtx, messages[0], cfg, tx, ctxStore)
	require.True(t, success, "Message processing should succeed")

	// Commit the transaction
	err = tx.Commit()
	require.NoError(t, err, "Failed to commit transaction")

	// Acknowledge the message
	err = msg.Ack()
	require.NoError(t, err, "Failed to acknowledge message")

	// Calculate what the remediation ID should be based on the implementation in processBatchMessage
	expectedRemediationID := fmt.Sprintf("rem_%s", pairKey)

	// Verify the remediation was stored in the database
	var storedID, storedOrthoID, storedPairKey string
	err = db.QueryRow(
		"SELECT id, ortho_id, pair_key FROM remediations WHERE pair_key = ?",
		pairKey,
	).Scan(&storedID, &storedOrthoID, &storedPairKey)
	require.NoError(t, err, "Failed to get stored remediation")

	// Verify stored data matches expected values
	assert.Equal(t, expectedRemediationID, storedID, "Stored remediation ID mismatch")
	assert.Equal(t, expectedRemediationID, storedOrthoID, "Stored remediation ortho_id mismatch")
	assert.Equal(t, pairKey, storedPairKey, "Stored remediation pair_key mismatch")

	// Verify queue is now empty
	emptyMessages, err := dbQueueClient.PopMessagesFromQueue(context.Background(), cfg.DBQueueName, 1)
	require.NoError(t, err, "Failed to check if queue is empty")
	assert.Equal(t, 0, len(emptyMessages), "Queue should be empty")
}

// clearQueue removes all messages from a queue
func clearQueue(t *testing.T, client *httpclient.RabbitClient[types.DBQueueItem], queueName string) {
	ctx := context.Background()

	// Ensure the queue exists
	err := client.DeclareQueue(ctx, queueName)
	require.NoError(t, err, "Failed to declare queue for clearing")

	// Pop and acknowledge all messages until the queue is empty
	for {
		messages, err := client.PopMessagesFromQueue(ctx, queueName, 10)
		if err != nil {
			t.Logf("Error while clearing queue: %v", err)
			break
		}
		if len(messages) == 0 {
			break
		}

		for _, msg := range messages {
			err := msg.Ack()
			if err != nil {
				t.Logf("Warning: Failed to acknowledge message while clearing queue: %v", err)
			}
		}

		// Small delay to prevent tight loop
		time.Sleep(50 * time.Millisecond)
	}
}

// Helper function to clear work queue messages
func clearWorkQueue(t *testing.T, client *httpclient.RabbitClient[types.WorkItem], queueName string) {
	ctx := context.Background()

	// Ensure the queue exists
	err := client.DeclareQueue(ctx, queueName)
	require.NoError(t, err, "Failed to declare queue for clearing")

	// Pop and acknowledge all messages until the queue is empty
	for {
		messages, err := client.PopMessagesFromQueue(ctx, queueName, 10)
		if err != nil {
			t.Logf("Error while clearing queue: %v", err)
			break
		}
		if len(messages) == 0 {
			break
		}

		for _, msg := range messages {
			err := msg.Ack()
			if err != nil {
				t.Logf("Warning: Failed to acknowledge message while clearing queue: %v", err)
			}
		}

		// Small delay to prevent tight loop
		time.Sleep(50 * time.Millisecond)
	}
}
