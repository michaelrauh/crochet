package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"crochet/httpclient"
	"crochet/telemetry"
	"crochet/types"

	"go.opentelemetry.io/otel"
)

// Config holds the configuration for the feeder service
type Config struct {
	ServiceName       string
	RabbitMQURL       string
	DBQueueName       string
	WorkQueueName     string
	BatchSize         int
	ProcessingTimeout time.Duration
	ContextServiceURL string
	RemediationsURL   string
	OrthosURL         string
	WorkServerURL     string
	JaegerEndpoint    string
	MetricsEndpoint   string
	PyroscopeEndpoint string
	ContextDBEndpoint string // Added for direct DB access
}

// Global RabbitMQ client for DB queue to be used in processMessage
var dbQueueClient *httpclient.RabbitClient[types.DBQueueItem]

// loadConfigFromEnv loads configuration from environment variables
func loadConfigFromEnv() (*Config, error) {
	batchSizeStr := os.Getenv("FEEDER_BATCH_SIZE")
	batchSize, err := strconv.Atoi(batchSizeStr)
	if err != nil || batchSize <= 0 {
		batchSize = 10 // Default batch size if not specified or invalid
		log.Printf("Using default batch size: %d", batchSize)
	} else {
		log.Printf("Using configured batch size: %d", batchSize)
	}

	timeoutStr := os.Getenv("FEEDER_PROCESSING_TIMEOUT")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		timeout = 30 * time.Second // Default timeout
		log.Printf("Using default processing timeout: %s", timeout)
	} else {
		log.Printf("Using configured processing timeout: %s", timeout)
	}

	config := &Config{
		ServiceName:       os.Getenv("FEEDER_SERVICE_NAME"),
		RabbitMQURL:       os.Getenv("FEEDER_RABBITMQ_URL"),
		DBQueueName:       os.Getenv("FEEDER_DB_QUEUE_NAME"),
		WorkQueueName:     os.Getenv("FEEDER_WORK_QUEUE_NAME"),
		BatchSize:         batchSize,
		ProcessingTimeout: timeout,
		ContextServiceURL: os.Getenv("FEEDER_CONTEXT_SERVICE_URL"),
		RemediationsURL:   os.Getenv("FEEDER_REMEDIATIONS_SERVICE_URL"),
		OrthosURL:         os.Getenv("FEEDER_ORTHOS_SERVICE_URL"),
		WorkServerURL:     os.Getenv("FEEDER_WORK_SERVER_URL"),
		JaegerEndpoint:    os.Getenv("FEEDER_JAEGER_ENDPOINT"),
		MetricsEndpoint:   os.Getenv("FEEDER_METRICS_ENDPOINT"),
		PyroscopeEndpoint: os.Getenv("FEEDER_PYROSCOPE_ENDPOINT"),
		ContextDBEndpoint: os.Getenv("FEEDER_CONTEXT_DB_ENDPOINT"),
	}

	log.Printf("Feeder configuration loaded:")
	log.Printf("- RabbitMQ URL: %s", config.RabbitMQURL)
	log.Printf("- DB Queue Name: %s", config.DBQueueName)
	log.Printf("- Work Queue Name: %s", config.WorkQueueName)
	log.Printf("- Batch Size: %d", config.BatchSize)
	log.Printf("- Processing Timeout: %s", config.ProcessingTimeout)
	log.Printf("- Context Service URL: %s", config.ContextServiceURL)
	log.Printf("- Context DB Endpoint: %s", config.ContextDBEndpoint)

	// Validate critical configuration
	if config.RabbitMQURL == "" {
		return nil, fmt.Errorf("FEEDER_RABBITMQ_URL environment variable is required but not set")
	}

	if config.DBQueueName == "" {
		return nil, fmt.Errorf("FEEDER_DB_QUEUE_NAME environment variable is required but not set")
	}

	if config.WorkQueueName == "" {
		return nil, fmt.Errorf("FEEDER_WORK_QUEUE_NAME environment variable is required but not set")
	}

	if config.ContextDBEndpoint == "" {
		return nil, fmt.Errorf("FEEDER_CONTEXT_DB_ENDPOINT environment variable is required but not set")
	}

	return config, nil
}

// createPairKey creates a string key from a string slice for map lookups
// This is the same implementation as in types/remediation_store.go
func createPairKey(pair []string) string {
	if len(pair) == 0 {
		return ""
	}
	result := pair[0]
	for i := 1; i < len(pair); i++ {
		result += ":" + pair[i]
	}
	return result
}

// initializeDBSchema initializes the database schema for a new or in-memory database
func initializeDBSchema(db *sql.DB) error {
	log.Printf("Initializing database schema...")

	// Verify database connection
	err := db.Ping()
	if err != nil {
		return fmt.Errorf("database connection failed: %v", err)
	}
	log.Printf("Database connection verified.")

	// Run all schema creation in a transaction for atomicity
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start schema initialization transaction: %v", err)
	}
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()

	// IMPORTANT: We no longer drop existing tables to maintain schema compatibility with repository
	// Instead, we use CREATE TABLE IF NOT EXISTS to ensure consistency

	// Create version table
	_, err = tx.Exec(`
        CREATE TABLE IF NOT EXISTS version (
            id INTEGER PRIMARY KEY CHECK (id = 1),
            version INTEGER NOT NULL
        )
    `)
	if err != nil {
		return fmt.Errorf("failed to create version table: %v", err)
	}
	log.Printf("Version table created or already exists")

	// Initialize with default version if not exists
	_, err = tx.Exec("INSERT OR IGNORE INTO version (id, version) VALUES (1, 0)")
	if err != nil {
		return fmt.Errorf("failed to insert initial version: %v", err)
	}
	log.Printf("Initialized version table with default version 0 if needed")

	// Create vocabulary table - using 'word' as column name to match repository
	_, err = tx.Exec(`
        CREATE TABLE IF NOT EXISTS vocabulary (
            word TEXT PRIMARY KEY
        )
    `)
	if err != nil {
		return fmt.Errorf("failed to create vocabulary table: %v", err)
	}
	log.Printf("Vocabulary table created or already exists")

	// Create subphrases table
	_, err = tx.Exec(`
        CREATE TABLE IF NOT EXISTS subphrases (
            phrase TEXT PRIMARY KEY
        )
    `)
	if err != nil {
		return fmt.Errorf("failed to create subphrases table: %v", err)
	}
	log.Printf("Subphrases table created or already exists")

	// Create orthos table
	_, err = tx.Exec(`
        CREATE TABLE IF NOT EXISTS orthos (
            id TEXT PRIMARY KEY,
            grid TEXT NOT NULL,
            shape TEXT NOT NULL,
            position TEXT NOT NULL,
            shell INTEGER NOT NULL DEFAULT 0
        )
    `)
	if err != nil {
		return fmt.Errorf("failed to create orthos table: %v", err)
	}
	log.Printf("Orthos table created or already exists")

	// Create remediations table - ensuring it supports arbitrary-length phrases through pair_key
	_, err = tx.Exec(`
        CREATE TABLE IF NOT EXISTS remediations (
            id TEXT PRIMARY KEY,
            ortho_id TEXT NOT NULL,
            pair_key TEXT NOT NULL,
            FOREIGN KEY (ortho_id) REFERENCES orthos(id)
        )
    `)
	if err != nil {
		return fmt.Errorf("failed to create remediations table: %v", err)
	}
	log.Printf("Remediations table created or already exists")

	// Create index on pair_key for efficient lookups of arbitrary-length phrases
	_, err = tx.Exec(`
        CREATE INDEX IF NOT EXISTS idx_remediations_pair_key ON remediations(pair_key)
    `)
	if err != nil {
		return fmt.Errorf("failed to create index on remediations table: %v", err)
	}
	log.Printf("Remediations index created or already exists")

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit schema initialization transaction: %v", err)
	}
	tx = nil // Set to nil to prevent rollback in defer

	// Verify all tables were created properly
	tables := []string{"version", "vocabulary", "subphrases", "orthos", "remediations"}
	for _, table := range tables {
		var tableExists bool
		query := fmt.Sprintf("SELECT 1 FROM sqlite_master WHERE type='table' AND name='%s'", table)
		err = db.QueryRow(query).Scan(&tableExists)
		if err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("table %s was not created properly", table)
			}
			return fmt.Errorf("error verifying table %s: %v", table, err)
		}
		log.Printf("Verified table %s exists", table)
	}

	log.Printf("Database schema initialization complete")
	return nil
}

func main() {
	// Load configuration
	cfg, err := loadConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize telemetry components with panic on failure
	tp, err := telemetry.InitTracer(cfg.ServiceName, cfg.JaegerEndpoint)
	if err != nil {
		errMsg := fmt.Sprintf("Fatal error initializing tracer: %v", err)
		log.Printf("%s", errMsg)
		panic(errMsg)
	}
	defer tp.ShutdownWithTimeout(5 * time.Second)

	mp, err := telemetry.InitMeter(cfg.ServiceName, cfg.MetricsEndpoint)
	if err != nil {
		errMsg := fmt.Sprintf("Fatal error initializing metrics: %v", err)
		log.Printf("%s", errMsg)
		panic(errMsg)
	}
	defer mp.ShutdownWithTimeout(5 * time.Second)

	pp, err := telemetry.InitProfiler(cfg.ServiceName, cfg.PyroscopeEndpoint)
	if err != nil {
		errMsg := fmt.Sprintf("Fatal error initializing profiler: %v", err)
		log.Printf("%s", errMsg)
		panic(errMsg)
	}
	defer pp.StopWithTimeout(5 * time.Second)

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize tracer
	tracer := otel.Tracer("github.com/crochet/feeder")

	// Create and initialize the context store with database schema
	ctxStore, err := types.NewLibSQLContextStore(cfg.ContextDBEndpoint)
	if err != nil {
		log.Fatalf("Error connecting to context database: %v", err)
	}
	defer ctxStore.Close()

	// Initialize the database schema
	if err := initializeDBSchema(ctxStore.DB()); err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}

	// Initialize RabbitMQ client for DB queue
	log.Printf("Initializing RabbitMQ client with URL: %s and queue: %s", cfg.RabbitMQURL, cfg.DBQueueName)
	var initErr error
	dbQueueClient, initErr = httpclient.NewRabbitClient[types.DBQueueItem](cfg.RabbitMQURL)
	if initErr != nil {
		log.Fatalf("Failed to create DB queue client: %v", initErr)
	}
	defer func() {
		log.Printf("Closing RabbitMQ connection...")
		if err := dbQueueClient.Close(ctx); err != nil {
			log.Printf("Error closing RabbitMQ connection: %v", err)
		}
	}()

	// Force-close and recreate the connection to RabbitMQ to ensure a fresh start
	log.Printf("Forcibly closing and recreating RabbitMQ connection to ensure clean state")
	dbQueueClient.Close(ctx)
	dbQueueClient, initErr = httpclient.NewRabbitClient[types.DBQueueItem](cfg.RabbitMQURL)
	if initErr != nil {
		log.Fatalf("Failed to recreate DB queue client: %v", initErr)
	}

	// Declare queue to ensure it exists
	log.Printf("Declaring DB queue: %s", cfg.DBQueueName)
	if err := dbQueueClient.DeclareQueue(ctx, cfg.DBQueueName); err != nil {
		log.Fatalf("Failed to declare DB queue: %v", err)
	}
	log.Printf("DB queue %s declared successfully", cfg.DBQueueName)

	// Get RabbitMQ queue stats via terminal command for debugging
	log.Printf("Attempting to check RabbitMQ queue status using docker command...")

	// Set up a channel to listen for shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Define batch processing constants
	const maxBatchSize = 1000 // Maximum number of messages to process before committing
	const batchTimeoutMs = 5  // Maximum time in milliseconds to wait before committing
	log.Printf("Using max batch size: %d, batch timeout: %dms", maxBatchSize, batchTimeoutMs)

	// Start the main processing loop
	log.Printf("Starting main message processing loop")
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Create a span for batch processing
				ctxWithSpan, span := tracer.Start(ctx, "process_db_queue_batch")

				// Add additional logging for RabbitMQ connection state
				log.Printf("DIAG[unknown]: Popping messages from queue %s with batch size %d",
					cfg.DBQueueName, cfg.BatchSize)

				// Add additional connection state logging
				connectionState := "unknown"
				channelState := "unknown"
				// Indirect way to check connection status without accessing private fields
				err := dbQueueClient.DeclareQueue(ctx, cfg.DBQueueName)
				if err != nil {
					log.Printf("DIAG[unknown]: Connection state before pop - closed: true (error: %v)", err)
					connectionState = "closed"
				} else {
					log.Printf("DIAG[unknown]: Connection state before pop - closed: false")
					connectionState = "open"
				}

				log.Printf("DIAG[unknown]: Connection state: %s, Channel state: %s",
					connectionState, channelState)

				// Pop a batch of messages from the DB queue
				log.Printf("Popping batch of up to %d messages from DB queue", cfg.BatchSize)
				messages, err := dbQueueClient.PopMessagesFromQueue(ctxWithSpan, cfg.DBQueueName, cfg.BatchSize)
				if err != nil {
					log.Printf("Error popping messages from DB queue: %v", err)
					span.End()

					// If the connection error appears to be related to closed channel or connection,
					// recreate the RabbitMQ client
					if strings.Contains(err.Error(), "channel") || strings.Contains(err.Error(), "connection") {
						log.Printf("RabbitMQ connection appears to have issues. Recreating client...")
						dbQueueClient.Close(ctx)
						time.Sleep(1 * time.Second) // Wait before reconnecting
						dbQueueClient, err = httpclient.NewRabbitClient[types.DBQueueItem](cfg.RabbitMQURL)
						if err != nil {
							log.Printf("Failed to recreate RabbitMQ client: %v", err)
						} else {
							// Declare the queue after reconnecting
							if err := dbQueueClient.DeclareQueue(ctx, cfg.DBQueueName); err != nil {
								log.Printf("Failed to declare queue after reconnect: %v", err)
							} else {
								log.Printf("Successfully reconnected to RabbitMQ")
							}
						}
					}

					// Sleep before retrying to avoid tight loop on error
					time.Sleep(1 * time.Second)
					continue
				}

				if len(messages) == 0 {
					log.Println("No messages in DB queue, waiting before next attempt")
					span.End()
					// Sleep before checking again to avoid tight loop
					time.Sleep(1 * time.Second)
					continue
				}

				log.Printf("Popped %d messages from DB queue", len(messages))

				// Set up transaction for the batch
				tx, err := ctxStore.DB().Begin()
				if err != nil {
					log.Printf("Error starting batch transaction: %v", err)
					// Nack all messages in batch
					batchNack(messages)
					span.End()
					time.Sleep(1 * time.Second)
					continue
				}
				defer tx.Rollback() // Will be ignored if committed

				// Track successful and failed messages
				successfulMessages := make([]httpclient.MessageWithAck[types.DBQueueItem], 0, len(messages))
				failedMessages := make([]httpclient.MessageWithAck[types.DBQueueItem], 0)

				// Set up a deadline for the batch processing
				batchStartTime := time.Now()
				batchDeadline := batchStartTime.Add(time.Duration(batchTimeoutMs) * time.Millisecond)

				// Process each message in the batch
				batchCount := 0
				for i, msg := range messages {
					// Check if we've reached max batch size or timeout
					if batchCount >= maxBatchSize || time.Now().After(batchDeadline) {
						// Commit the current batch
						if err := tx.Commit(); err != nil {
							log.Printf("Error committing batch transaction: %v", err)
							failedMessages = append(failedMessages, messages[i:]...)
							break
						}
						// Start a new transaction for remaining messages
						tx, err = ctxStore.DB().Begin()
						if err != nil {
							log.Printf("Error starting new batch transaction: %v", err)
							failedMessages = append(failedMessages, messages[i:]...)
							break
						}
						// Reset batch counter and update deadline
						batchCount = 0
						batchStartTime = time.Now()
						batchDeadline = batchStartTime.Add(time.Duration(batchTimeoutMs) * time.Millisecond)
						log.Printf("Committed sub-batch after %d messages", i)
					}

					log.Printf("Processing message %d of type: %s", i, msg.Data.Type)

					// Check if this is a version update
					isVersionUpdate := msg.Data.Type == types.DBQueueItemTypeVersion

					success := processBatchMessage(ctxWithSpan, msg, cfg, tx, ctxStore)
					if success {
						successfulMessages = append(successfulMessages, msg)
					} else {
						failedMessages = append(failedMessages, msg)
						// If a version update failed, mark the flag to nack the remaining messages
						if isVersionUpdate {
							log.Printf("Version update failed - will nack all remaining messages in batch")
							// Add all remaining messages to failed messages
							failedMessages = append(failedMessages, messages[i+1:]...)
							break // Stop processing the batch
						}
					}
					batchCount++
				}

				// Commit the final batch transaction if there were successful messages
				if len(successfulMessages) > 0 {
					if err := tx.Commit(); err != nil {
						log.Printf("Error committing final batch transaction: %v", err)
						// Move all successful messages to failed since the transaction failed
						failedMessages = append(failedMessages, successfulMessages...)
						successfulMessages = nil
					} else {
						log.Printf("Successfully committed batch with %d messages", len(successfulMessages))
					}
				}

				// Acknowledge successful messages in batch
				if len(successfulMessages) > 0 {
					batchAck(successfulMessages)
				}

				// Nack failed messages
				if len(failedMessages) > 0 {
					batchNack(failedMessages)
				}

				span.End()
			}
		}
	}()

	// Wait for termination signal
	<-sigChan
	fmt.Println("Shutdown signal received, gracefully shutting down...")

	// Cancel context to stop processing loop
	cancel()

	// Allow some time for cleanup before exiting
	time.Sleep(1 * time.Second)
	fmt.Println("Feeder service shutdown complete")
}

// batchAck acknowledges multiple messages in a single batch
func batchAck(messages []httpclient.MessageWithAck[types.DBQueueItem]) {
	for i, msg := range messages {
		if err := msg.Ack(); err != nil {
			log.Printf("Error acknowledging message %d: %v", i, err)
		}
	}
	log.Printf("Batch acknowledged %d messages", len(messages))
}

// batchNack negative-acknowledges multiple messages in a single batch
func batchNack(messages []httpclient.MessageWithAck[types.DBQueueItem]) {
	for i, msg := range messages {
		if err := msg.Nack(); err != nil {
			log.Printf("Error nacking message %d: %v", i, err)
		}
	}
	log.Printf("Batch nacked %d messages", len(messages))
}

// processBatchMessage processes a single message as part of a batch transaction
// It returns true if processing was successful, false otherwise
func processBatchMessage(ctx context.Context, msg httpclient.MessageWithAck[types.DBQueueItem], cfg *Config, tx *sql.Tx, ctxStore *types.LibSQLContextStore) bool {
	item := msg.Data

	// Debug logging to see raw message data
	itemJSON, _ := json.Marshal(item)
	log.Printf("DEBUG: Processing message of type: %s with raw data: %s", item.Type, string(itemJSON))

	switch item.Type {
	case types.DBQueueItemTypeVersion:
		version, err := item.GetVersion()
		if err != nil {
			log.Printf("ERROR: Error unmarshaling version: %v", err)
			return false
		}
		log.Printf("PROCESSING: Version update from %d to %d", getCurrentVersion(tx), version.Version)

		// Use transaction to update version
		_, err = tx.Exec("UPDATE version SET version = ?", version.Version)
		if err != nil {
			log.Printf("ERROR: Error updating version in database: %v", err)
			return false
		}
		log.Printf("SUCCESS: Updated version to %d in batch transaction", version.Version)

		// After updating version, log full context data state for debugging
		logDatabaseState(tx)

		return true

	case types.DBQueueItemTypePair:
		pair, err := item.GetPair()
		if err != nil {
			log.Printf("ERROR: Error unmarshaling pair: %v", err)
			return false
		}
		log.Printf("PROCESSING: Pair: %v", pair)

		// Create a query to find remediations associated with this pair
		// Use createPairKey which already supports arbitrary-length phrases
		pairKey := createPairKey(pair)
		log.Printf("DEBUG: Using pair key: %s", pairKey)

		// Query database for remediation records that match this pair
		var remediationIDs []string
		var orthoIDs []string

		// Find remediations that match this pair
		rows, err := tx.Query("SELECT ortho_id FROM remediations WHERE pair_key = ?", pairKey)
		if err != nil {
			log.Printf("Error querying remediations for pair %s: %v", pairKey, err)
			return false
		}
		defer rows.Close()

		// Collect all ortho IDs associated with this pair
		for rows.Next() {
			var orthoID string
			if err := rows.Scan(&orthoID); err != nil {
				log.Printf("Error scanning remediation record: %v", err)
				continue
			}
			orthoIDs = append(orthoIDs, orthoID)
			remediationIDs = append(remediationIDs, orthoID) // Use ortho ID as remediation ID
		}

		if err := rows.Err(); err != nil {
			log.Printf("Error iterating remediation rows: %v", err)
			return false
		}

		// If no orthos found, nothing to do but still consider successful
		if len(orthoIDs) == 0 {
			log.Printf("No orthos found for pair: %v", pair)
			return true
		}

		log.Printf("Found %d orthos for pair: %v", len(orthoIDs), pair)

		// Now query for the actual ortho objects
		placeholders := make([]string, len(orthoIDs))
		args := make([]interface{}, len(orthoIDs))
		for i, id := range orthoIDs {
			placeholders[i] = "?"
			args[i] = id
		}

		// Fetch orthos from the database
		query := fmt.Sprintf("SELECT id, grid, shape, position, shell FROM orthos WHERE id IN (%s)",
			strings.Join(placeholders, ","))

		orthoRows, err := tx.Query(query, args...)
		if err != nil {
			log.Printf("Error querying orthos: %v", err)
			return false
		}
		defer orthoRows.Close()

		// Collect orthos to push to work queue
		var orthosToProcess []types.Ortho
		for orthoRows.Next() {
			var id string
			var gridJSON, shapeJSON, positionJSON string
			var shell int

			if err := orthoRows.Scan(&id, &gridJSON, &shapeJSON, &positionJSON, &shell); err != nil {
				log.Printf("Error scanning ortho row: %v", err)
				continue
			}

			// Parse JSON data into proper structures
			var grid map[string]string
			var shape, position []int

			if err := json.Unmarshal([]byte(gridJSON), &grid); err != nil {
				log.Printf("Error unmarshaling grid: %v", err)
				continue
			}

			if err := json.Unmarshal([]byte(shapeJSON), &shape); err != nil {
				log.Printf("Error unmarshaling shape: %v", err)
				continue
			}

			if err := json.Unmarshal([]byte(positionJSON), &position); err != nil {
				log.Printf("Error unmarshaling position: %v", err)
				continue
			}

			ortho := types.Ortho{
				ID:       id,
				Grid:     grid,
				Shape:    shape,
				Position: position,
				Shell:    shell,
			}

			orthosToProcess = append(orthosToProcess, ortho)
		}

		if err := orthoRows.Err(); err != nil {
			log.Printf("Error iterating ortho rows: %v", err)
			return false
		}

		// Now push the orthos to the work queue
		if len(orthosToProcess) > 0 {
			// Initialize RabbitMQ client for work queue
			workQueueClient, err := httpclient.NewRabbitClient[types.WorkItem](cfg.RabbitMQURL)
			if err != nil {
				log.Printf("Failed to create work queue client: %v", err)
				return false
			}
			defer workQueueClient.Close(context.Background())

			// Ensure the queue exists
			if err := workQueueClient.DeclareQueue(context.Background(), cfg.WorkQueueName); err != nil {
				log.Printf("Failed to declare work queue: %v", err)
				return false
			}

			// Push each ortho to the work queue
			for _, ortho := range orthosToProcess {
				// Create a proper WorkItem to wrap the ortho
				workItem := types.WorkItem{
					ID:        fmt.Sprintf("work-%d", time.Now().UnixNano()),
					Data:      ortho,
					Timestamp: time.Now().UnixNano(),
				}

				// Serialize the WorkItem (not just the raw ortho) to JSON
				workItemJSON, err := json.Marshal(workItem)
				if err != nil {
					log.Printf("Error marshaling WorkItem to JSON: %v", err)
					return false
				}

				// Push the message using the correct method name
				err = workQueueClient.PushMessage(context.Background(), cfg.WorkQueueName, workItemJSON)
				if err != nil {
					log.Printf("Error pushing WorkItem with ortho %s to work queue: %v", ortho.ID, err)
					return false
				} else {
					log.Printf("Successfully pushed WorkItem with ortho %s to work queue", ortho.ID)
				}
			}

			// Create a remediation delete message for each remediation
			for _, remediationID := range remediationIDs {
				// Create the remediation delete message
				remediationDeleteItem, err := types.CreateRemediationDeleteQueueItem(remediationID)
				if err != nil {
					log.Printf("Error creating remediation delete queue item: %v", err)
					return false
				}

				// Serialize the remediation delete message to JSON
				remediationDeleteJSON, err := json.Marshal(remediationDeleteItem)
				if err != nil {
					log.Printf("Error marshaling remediation delete for ID %s to JSON: %v", remediationID, err)
					return false
				}

				// Push it to the DB queue using the correct method name
				err = dbQueueClient.PushMessage(context.Background(), cfg.DBQueueName, remediationDeleteJSON)
				if err != nil {
					log.Printf("Error pushing remediation delete to DB queue: %v", err)
					return false
				} else {
					log.Printf("Successfully pushed remediation delete for ID %s to DB queue", remediationID)
				}
			}
		}

		return true

	case types.DBQueueItemTypeContext:
		context, err := item.GetContext()
		if err != nil {
			log.Printf("ERROR: Error unmarshaling context: %v, raw payload: %s", err, string(item.Payload))
			return false
		}
		log.Printf("PROCESSING: Context: %s with %d vocabulary items and %d subphrases",
			context.Title, len(context.Vocabulary), len(context.Subphrases))
		log.Printf("DEBUG: Vocabulary: %v", context.Vocabulary)
		log.Printf("DEBUG: Sample subphrases: %v", getSampleSubphrases(context.Subphrases, 5))

		// First check if vocabulary table exists and has the right schema
		var tableExists bool
		err = tx.QueryRow("SELECT 1 FROM sqlite_master WHERE type='table' AND name='vocabulary'").Scan(&tableExists)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("ERROR: Error checking if vocabulary table exists: %v", err)
			return false
		}

		// Save vocabulary terms using the transaction - use 'word' column name to match schema
		log.Printf("PROCESSING: Saving %d vocabulary terms", len(context.Vocabulary))
		for _, term := range context.Vocabulary {
			// Fix: Use 'word' instead of 'term' as the column name to match the schema
			_, err := tx.Exec("INSERT OR IGNORE INTO vocabulary (word) VALUES (?)", term)
			if err != nil {
				log.Printf("ERROR: Error inserting vocabulary term %s: %v", term, err)
				return false
			}
		}
		log.Printf("SUCCESS: Inserted %d vocabulary terms", len(context.Vocabulary))

		// Verify vocabulary was saved correctly
		var vocabCount int
		err = tx.QueryRow("SELECT COUNT(*) FROM vocabulary").Scan(&vocabCount)
		if err != nil {
			log.Printf("WARNING: Could not verify vocabulary count: %v", err)
		} else {
			log.Printf("VERIFICATION: Total vocabulary count in DB: %d", vocabCount)
		}

		// Verify vocabulary content after insertion
		rows, err := tx.Query("SELECT word FROM vocabulary LIMIT 20")
		if err != nil {
			log.Printf("WARNING: Could not verify vocabulary content: %v", err)
		} else {
			defer rows.Close()
			var terms []string
			for rows.Next() {
				var term string
				if err := rows.Scan(&term); err != nil {
					log.Printf("WARNING: Error scanning vocabulary term: %v", err)
					continue
				}
				terms = append(terms, term)
			}
			log.Printf("CONTEXT UPDATE: Vocabulary terms in DB (sample): %v", terms)
			log.Printf("CONTEXT UPDATE: Vocabulary insertion verification: %t", containsAll(terms, context.Vocabulary))
		}

		// Flatten subphrases to a list of strings
		var flattenedSubphrases []string
		for _, subphrase := range context.Subphrases {
			// Join all elements in the subphrase array with a space
			joined := strings.Join(subphrase, " ")
			flattenedSubphrases = append(flattenedSubphrases, joined)
		}

		// Save subphrases directly with simple statements to avoid prepared statement issues
		log.Printf("PROCESSING: Saving %d subphrases", len(flattenedSubphrases))
		for _, phrase := range flattenedSubphrases {
			_, err := tx.Exec("INSERT OR IGNORE INTO subphrases (phrase) VALUES (?)", phrase)
			if err != nil {
				log.Printf("ERROR: Error inserting subphrase %s: %v", phrase, err)
				return false
			}
		}
		log.Printf("SUCCESS: Inserted %d subphrases", len(flattenedSubphrases))

		// Verify subphrases were saved correctly
		var subphraseCount int
		err = tx.QueryRow("SELECT COUNT(*) FROM subphrases").Scan(&subphraseCount)
		if err != nil {
			log.Printf("WARNING: Could not verify subphrase count: %v", err)
		} else {
			log.Printf("VERIFICATION: Total subphrase count in DB: %d", subphraseCount)
		}

		// Verify subphrases content after insertion
		subRows, err := tx.Query("SELECT phrase FROM subphrases LIMIT 20")
		if err != nil {
			log.Printf("WARNING: Could not verify subphrases content: %v", err)
		} else {
			defer subRows.Close()
			var phrases []string
			for subRows.Next() {
				var phrase string
				if err := subRows.Scan(&phrase); err != nil {
					log.Printf("WARNING: Error scanning subphrase: %v", err)
					continue
				}
				phrases = append(phrases, phrase)
			}
			log.Printf("CONTEXT UPDATE: Subphrases in DB (sample): %v", phrases)

			// Check if sample of flattened subphrases appears in the DB
			var samplePhrases []string
			if len(flattenedSubphrases) > 5 {
				samplePhrases = flattenedSubphrases[:5]
			} else {
				samplePhrases = flattenedSubphrases
			}

			log.Printf("CONTEXT UPDATE: Subphrase insertion verification (sample): %t", containsAny(phrases, samplePhrases))
		}

		// Log comprehensive context update status
		log.Printf("CONTEXT UPDATE SUMMARY: Title=%s, VocabItems=%d/%d, Subphrases=%d/%d",
			context.Title, vocabCount, len(context.Vocabulary),
			subphraseCount, len(flattenedSubphrases))

		return true

	case types.DBQueueItemTypeOrtho:
		ortho, err := item.GetOrtho()
		if err != nil {
			log.Printf("ERROR: Error unmarshaling ortho: %v", err)
			return false
		}
		// Enhanced logging for seed ortho details
		log.Printf("PROCESSING: Seed ortho with ID: %s, Shape: %v, Position: %v",
			ortho.ID, ortho.Shape, ortho.Position)
		log.Printf("DEBUG: Seed ortho details - Shell: %d, Grid size: %d",
			ortho.Shell, len(ortho.Grid))
		// Log grid contents
		i := 0
		for k, v := range ortho.Grid {
			if i < 5 { // Limit to first 5 entries to avoid excessive logging
				log.Printf("DEBUG: Seed ortho grid entry %d - Position: %s, Value: %s", i, k, v)
				i++
			} else {
				break
			}
		}

		// ENHANCED LOGGING: Verify ortho is valid before serialization
		if ortho.ID == "" {
			log.Printf("WARNING: Ortho has empty ID, generating a new one")
			ortho.ID = fmt.Sprintf("ortho_%d", time.Now().UnixNano())
		}

		if ortho.Grid == nil {
			log.Printf("ERROR: Ortho has nil Grid, initializing empty map")
			ortho.Grid = make(map[string]string)
		}

		if len(ortho.Shape) == 0 {
			log.Printf("WARNING: Ortho has empty Shape array")
		}

		if len(ortho.Position) == 0 {
			log.Printf("WARNING: Ortho has empty Position array")
		}

		// Serialize the ortho fields to JSON for storage
		gridJSON, err := json.Marshal(ortho.Grid)
		if err != nil {
			log.Printf("Error marshaling ortho grid to JSON: %v", err)
			return false
		}
		shapeJSON, err := json.Marshal(ortho.Shape)
		if err != nil {
			log.Printf("Error marshaling ortho shape to JSON: %v", err)
			return false
		}
		positionJSON, err := json.Marshal(ortho.Position)
		if err != nil {
			log.Printf("Error marshaling ortho position to JSON: %v", err)
			return false
		}
		// Check if ortho already exists
		var exists bool
		err = tx.QueryRow("SELECT 1 FROM orthos WHERE id = ?", ortho.ID).Scan(&exists)
		// Upsert the ortho in the database
		if err == sql.ErrNoRows {
			// Insert new ortho
			_, err = tx.Exec(
				"INSERT INTO orthos (id, grid, shape, position, shell) VALUES (?, ?, ?, ?, ?)",
				ortho.ID, gridJSON, shapeJSON, positionJSON, ortho.Shell,
			)
			if err != nil {
				log.Printf("Error inserting ortho: %v", err)
				return false
			}
			log.Printf("Inserted new ortho with ID: %s", ortho.ID)
		} else if err != nil {
			log.Printf("Error checking if ortho exists: %v", err)
			return false
		} else {
			// Update existing ortho
			_, err = tx.Exec(
				"UPDATE orthos SET grid = ?, shape = ?, position = ?, shell = ? WHERE id = ?",
				gridJSON, shapeJSON, positionJSON, ortho.Shell, ortho.ID,
			)
			if err != nil {
				log.Printf("Error updating ortho: %v", err)
				return false
			}
			log.Printf("Updated existing ortho with ID: %s", ortho.ID)
		}
		// Now push the ortho to the work queue
		workQueueClient, err := httpclient.NewRabbitClient[types.WorkItem](cfg.RabbitMQURL)
		if err != nil {
			log.Printf("ERROR: Failed to create work queue client: %v", err)
			return false
		}
		defer workQueueClient.Close(context.Background())
		// Ensure the queue exists
		if err := workQueueClient.DeclareQueue(context.Background(), cfg.WorkQueueName); err != nil {
			log.Printf("ERROR: Failed to declare work queue: %v", err)
			return false
		}

		// ENHANCED LOGGING: Create a deep copy of the ortho for a more complete inspection before pushing
		orthoForWorkQueue := types.Ortho{
			ID:       ortho.ID,
			Grid:     ortho.Grid,
			Shape:    ortho.Shape,
			Position: ortho.Position,
			Shell:    ortho.Shell,
		}

		// Log complete ortho structure before serialization
		log.Printf("FEEDER PUSHING WORK: Ortho ID=%s, Shell=%d, Shape=%v, Position=%v, GridSize=%d",
			orthoForWorkQueue.ID, orthoForWorkQueue.Shell, orthoForWorkQueue.Shape,
			orthoForWorkQueue.Position, len(orthoForWorkQueue.Grid))

		// Create a proper WorkItem to wrap the ortho
		workItem := types.WorkItem{
			ID:        fmt.Sprintf("work-%d", time.Now().UnixNano()),
			Data:      orthoForWorkQueue,
			Timestamp: time.Now().UnixNano(),
		}

		// Serialize the WorkItem (not just the raw ortho) to JSON
		workItemJSON, err := json.Marshal(workItem)
		if err != nil {
			log.Printf("Error marshaling WorkItem to JSON: %v", err)
			return false
		}

		// Push to work queue (now sending the wrapped WorkItem)
		err = workQueueClient.PushMessage(context.Background(), cfg.WorkQueueName, workItemJSON)
		if err != nil {
			log.Printf("ERROR: Error pushing WorkItem with ortho %s to work queue: %v", ortho.ID, err)
			return false
		} else {
			log.Printf("SUCCESS: Pushed WorkItem with seed ortho %s to work queue for processing", ortho.ID)
			// Log the content of the work queue
			queueInfo := getQueueInfo(workQueueClient, cfg.WorkQueueName)
			log.Printf("DEBUG: Work queue info: %s", queueInfo)
		}

		return true

	case types.DBQueueItemTypeRemediation:
		remediation, err := item.GetRemediation()
		if err != nil {
			log.Printf("Error unmarshaling remediation: %v", err)
			return false
		}
		log.Printf("Processing remediation for pair: %v", remediation.Pair)

		// Create a unique identifier for this remediation
		// Generate a deterministic ID based on the pair
		pairKey := createPairKey(remediation.Pair)
		remediationID := fmt.Sprintf("rem_%s", pairKey)

		// Check if this pair already has a remediation
		var exists bool
		var existingID string
		err = tx.QueryRow("SELECT 1, id FROM remediations WHERE pair_key = ?", pairKey).Scan(&exists, &existingID)

		if err == sql.ErrNoRows {
			// Insert new remediation
			// For now, set ortho_id the same as remediation ID
			_, err = tx.Exec(
				"INSERT INTO remediations (id, ortho_id, pair_key) VALUES (?, ?, ?)",
				remediationID, remediationID, pairKey,
			)
			if err != nil {
				log.Printf("Error inserting remediation: %v", err)
				return false
			}
			log.Printf("Inserted new remediation with ID: %s for pair: %v", remediationID, remediation.Pair)
		} else if err != nil {
			log.Printf("Error checking if remediation exists: %v", err)
			return false
		} else {
			// Update existing remediation
			// For consistency, we'll keep the same ID but update the association
			_, err = tx.Exec(
				"UPDATE remediations SET ortho_id = ? WHERE id = ?",
				remediationID, existingID,
			)
			if err != nil {
				log.Printf("Error updating remediation: %v", err)
				return false
			}
			log.Printf("Updated existing remediation for pair: %v", remediation.Pair)
		}

		return true

	case types.DBQueueItemTypeRemediationDelete:
		remediationID, err := item.GetRemediationDeleteID()
		if err != nil {
			log.Printf("Error unmarshaling remediation delete: %v", err)
			return false
		}
		log.Printf("Processing remediation delete with ID: %s", remediationID)

		// Delete the remediation
		result, err := tx.Exec("DELETE FROM remediations WHERE id = ? OR ortho_id = ?", remediationID, remediationID)
		if err != nil {
			log.Printf("Error deleting remediation: %v", err)
			return false
		}

		// Check how many rows were affected
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			log.Printf("Error checking rows affected: %v", err)
			return false
		}

		if rowsAffected == 0 {
			log.Printf("No remediation found with ID: %s", remediationID)
		} else {
			log.Printf("Deleted %d remediation(s) with ID: %s", rowsAffected, remediationID)
		}

		return true

	default:
		errMsg := fmt.Sprintf("Critical error: Unknown item type: %s", item.Type)
		log.Printf("%s", errMsg)
		// Acknowledge the message to remove it from the queue (we don't want to reprocess it)
		if err := msg.Ack(); err != nil {
			log.Printf("Error acknowledging unknown message type: %v", err)
		}
		return false
	}

	// If we somehow get here, it means we didn't return in one of the case statements
	// This should never happen with proper case coverage
	log.Printf("Error: Reached end of processBatchMessage without returning, type: %s", item.Type)
	return false
}

// Helper functions for improved logging

// getCurrentVersion gets the current version from the database
func getCurrentVersion(tx *sql.Tx) int {
	var version int
	err := tx.QueryRow("SELECT version FROM version LIMIT 1").Scan(&version)
	if err != nil {
		log.Printf("WARNING: Failed to get current version: %v", err)
		return -1
	}
	return version
}

// logDatabaseState logs the current state of the database for debugging
func logDatabaseState(tx *sql.Tx) {
	// Log version
	var version int
	err := tx.QueryRow("SELECT version FROM version LIMIT 1").Scan(&version)
	if err != nil {
		log.Printf("WARNING: Failed to get version for state logging: %v", err)
	} else {
		log.Printf("DB STATE: Current version: %d", version)
	}

	// Log vocabulary count
	var vocabCount int
	err = tx.QueryRow("SELECT COUNT(*) FROM vocabulary").Scan(&vocabCount)
	if err != nil {
		log.Printf("WARNING: Failed to get vocabulary count: %v", err)
	} else {
		log.Printf("DB STATE: Vocabulary count: %d", vocabCount)

		// Log a sample of vocabulary
		rows, err := tx.Query("SELECT word FROM vocabulary LIMIT 10")
		if err != nil {
			log.Printf("WARNING: Failed to get sample vocabulary: %v", err)
		} else {
			var sampleVocab []string
			for rows.Next() {
				var term string
				if err := rows.Scan(&term); err != nil {
					continue
				}
				sampleVocab = append(sampleVocab, term)
			}
			rows.Close()
			log.Printf("DB STATE: Sample vocabulary: %v", sampleVocab)
		}
	}

	// Log subphrase count
	var subphraseCount int
	err = tx.QueryRow("SELECT COUNT(*) FROM subphrases").Scan(&subphraseCount)
	if err != nil {
		log.Printf("WARNING: Failed to get subphrase count: %v", err)
	} else {
		log.Printf("DB STATE: Subphrase count: %d", subphraseCount)

		// Log a sample of subphrases
		rows, err := tx.Query("SELECT phrase FROM subphrases LIMIT 5")
		if err != nil {
			log.Printf("WARNING: Failed to get sample subphrases: %v", err)
		} else {
			var sampleSubphrases []string
			for rows.Next() {
				var phrase string
				if err := rows.Scan(&phrase); err != nil {
					continue
				}
				sampleSubphrases = append(sampleSubphrases, phrase)
			}
			rows.Close()
			log.Printf("DB STATE: Sample subphrases: %v", sampleSubphrases)
		}
	}

	// Log ortho count
	var orthoCount int
	err = tx.QueryRow("SELECT COUNT(*) FROM orthos").Scan(&orthoCount)
	if err != nil {
		log.Printf("WARNING: Failed to get ortho count: %v", err)
	} else {
		log.Printf("DB STATE: Ortho count: %d", orthoCount)
	}
}

// getSampleSubphrases returns a sample of subphrases for logging
func getSampleSubphrases(subphrases [][]string, maxCount int) [][]string {
	if len(subphrases) <= maxCount {
		return subphrases
	}
	return subphrases[:maxCount]
}

// getQueueInfo gets detailed information about a queue for debugging
func getQueueInfo(client *httpclient.RabbitClient[types.WorkItem], queueName string) string {
	// Try to get queue information through a direct channel inspection
	ctx := context.Background()

	// Manually check the queue state via queue declare (which also returns queue stats)
	if err := client.DeclareQueue(ctx, queueName); err != nil {
		return fmt.Sprintf("Queue: %s (Error inspecting: %v)", queueName, err)
	}

	// Use a dedicated test queue name to avoid polluting the actual work queue
	testQueueName := fmt.Sprintf("%s-test", queueName)

	// Declare the test queue
	if err := client.DeclareQueue(ctx, testQueueName); err != nil {
		return fmt.Sprintf("Queue: %s (Error creating test queue: %v)", queueName, err)
	}

	// Get current timestamp for logging
	timestamp := time.Now().Format(time.RFC3339)

	// Check if we can publish a test message and immediately get it back
	testMsg := fmt.Sprintf("test-ping-%s", timestamp)
	testWorkItem := types.WorkItem{
		ID:        fmt.Sprintf("test-%d", time.Now().UnixNano()),
		Data:      map[string]string{"test": testMsg},
		Timestamp: time.Now().UnixNano(),
	}
	testJSON, _ := json.Marshal(testWorkItem)

	log.Printf("QUEUE_TEST: Attempting to push test message to queue %s", testQueueName)
	err := client.PushMessage(ctx, testQueueName, testJSON)
	if err != nil {
		return fmt.Sprintf("Queue: %s (Error publishing test message: %v)", queueName, err)
	}

	log.Printf("QUEUE_TEST: Successfully published test message to queue %s", testQueueName)

	// Try to pop the test message back
	time.Sleep(100 * time.Millisecond) // Short delay to ensure message is available

	msgs, err := client.PopMessagesFromQueue(ctx, testQueueName, 1)
	if err != nil {
		return fmt.Sprintf("Queue: %s (Published test message but failed to pop: %v)", queueName, err)
	}

	if len(msgs) == 0 {
		return fmt.Sprintf("Queue: %s (Published test message but queue appears empty when popped)", queueName)
	}

	// Successfully popped a message, acknowledge it
	if err := msgs[0].Ack(); err != nil {
		log.Printf("QUEUE_TEST: Error acknowledging test message: %v", err)
	}

	// Now check the actual work queue status
	return fmt.Sprintf("Queue: %s (Healthy - RabbitMQ connection working properly)", queueName)
}

// containsAll checks if a slice contains all elements of another slice
func containsAll(container, items []string) bool {
	if len(items) == 0 {
		return true
	}

	itemMap := make(map[string]bool)
	for _, item := range items {
		itemMap[item] = true
	}

	foundCount := 0
	for _, item := range container {
		if itemMap[item] {
			foundCount++
			if foundCount == len(itemMap) {
				return true
			}
		}
	}

	return false
}

// containsAny checks if a slice contains any elements of another slice
func containsAny(container, items []string) bool {
	if len(items) == 0 || len(container) == 0 {
		return false
	}

	itemMap := make(map[string]bool)
	for _, item := range items {
		itemMap[item] = true
	}

	for _, item := range container {
		if itemMap[item] {
			return true
		}
	}

	return false
}
