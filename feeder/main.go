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
	}

	timeoutStr := os.Getenv("FEEDER_PROCESSING_TIMEOUT")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		timeout = 30 * time.Second // Default timeout
	}

	return &Config{
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
	}, nil
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

	// Initialize RabbitMQ client for DB queue
	var initErr error
	dbQueueClient, initErr = httpclient.NewRabbitClient[types.DBQueueItem](cfg.RabbitMQURL)
	if initErr != nil {
		log.Fatalf("Failed to create DB queue client: %v", initErr)
	}
	defer dbQueueClient.Close(ctx)

	// Declare queue to ensure it exists
	if err := dbQueueClient.DeclareQueue(ctx, cfg.DBQueueName); err != nil {
		log.Fatalf("Failed to declare DB queue: %v", err)
	}

	// Set up a channel to listen for shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Define batch processing constants
	const maxBatchSize = 1000 // Maximum number of messages to process before committing
	const batchTimeoutMs = 5  // Maximum time in milliseconds to wait before committing

	// Start the main processing loop
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Create a span for batch processing
				ctxWithSpan, span := tracer.Start(ctx, "process_db_queue_batch")

				// Pop a batch of messages from the DB queue
				log.Printf("Popping batch of up to %d messages from DB queue", cfg.BatchSize)
				messages, err := dbQueueClient.PopMessagesFromQueue(ctxWithSpan, cfg.DBQueueName, cfg.BatchSize)

				if err != nil {
					log.Printf("Error popping messages from DB queue: %v", err)
					span.End()
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

				// Set up database connection for the batch
				ctxStore, err := types.NewLibSQLContextStore(cfg.ContextDBEndpoint)
				if err != nil {
					log.Printf("Error connecting to context database: %v", err)
					// Nack all messages in batch since we couldn't process them
					batchNack(messages)
					span.End()
					time.Sleep(1 * time.Second)
					continue
				}
				defer ctxStore.Close()

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

	switch item.Type {
	case types.DBQueueItemTypeVersion:
		version, err := item.GetVersion()
		if err != nil {
			log.Printf("Error unmarshaling version: %v", err)
			return false
		}
		log.Printf("Processing version update: %d", version.Version)

		// Use transaction to update version
		_, err = tx.Exec("UPDATE version SET version = ?", version.Version)
		if err != nil {
			log.Printf("Error updating version in database: %v", err)
			return false
		}

		log.Printf("Successfully updated version to %d in batch transaction", version.Version)
		return true

	case types.DBQueueItemTypePair:
		pair, err := item.GetPair()
		if err != nil {
			log.Printf("Error unmarshaling pair: %v", err)
			return false
		}
		log.Printf("Processing pair: %s - %s", pair.Left, pair.Right)

		// Create a query to find remediations associated with this pair
		pairKey := createPairKey([]string{pair.Left, pair.Right})

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
			log.Printf("No orthos found for pair: %s - %s", pair.Left, pair.Right)
			return true
		}

		log.Printf("Found %d orthos for pair: %s - %s", len(orthoIDs), pair.Left, pair.Right)

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
			workQueueClient, err := httpclient.NewRabbitClient[types.Ortho](cfg.RabbitMQURL)
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
				// Serialize ortho to JSON
				orthoJSON, err := json.Marshal(ortho)
				if err != nil {
					log.Printf("Error marshaling ortho to JSON: %v", err)
					return false
				}

				// Push the message using the correct method name
				err = workQueueClient.PushMessage(context.Background(), cfg.WorkQueueName, orthoJSON)
				if err != nil {
					log.Printf("Error pushing ortho %s to work queue: %v", ortho.ID, err)
					return false
				} else {
					log.Printf("Successfully pushed ortho %s to work queue", ortho.ID)
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
			log.Printf("Error unmarshaling context: %v", err)
			return false
		}
		log.Printf("Processing context: %s", context.Title)

		// Save vocabulary terms using the transaction
		vocabStmt, err := tx.Prepare("INSERT OR IGNORE INTO vocabulary (term) VALUES (?)")
		if err != nil {
			log.Printf("Error preparing vocabulary statement: %v", err)
			return false
		}
		defer vocabStmt.Close()

		for _, term := range context.Vocabulary {
			if _, err := vocabStmt.Exec(term); err != nil {
				log.Printf("Error inserting vocabulary term %s: %v", term, err)
				return false
			}
		}

		// Save subphrases using the transaction
		subphraseStmt, err := tx.Prepare("INSERT OR IGNORE INTO subphrases (phrase) VALUES (?)")
		if err != nil {
			log.Printf("Error preparing subphrase statement: %v", err)
			return false
		}
		defer subphraseStmt.Close()

		for _, phrase := range context.Subphrases {
			if _, err := subphraseStmt.Exec(phrase); err != nil {
				log.Printf("Error inserting subphrase %s: %v", phrase, err)
				return false
			}
		}

		return true

	case types.DBQueueItemTypeOrtho:
		ortho, err := item.GetOrtho()
		if err != nil {
			log.Printf("Error unmarshaling ortho: %v", err)
			return false
		}
		log.Printf("Processing ortho with ID: %s", ortho.ID)

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
		workQueueClient, err := httpclient.NewRabbitClient[types.Ortho](cfg.RabbitMQURL)
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

		// Serialize ortho to JSON
		orthoJSON, err := json.Marshal(ortho)
		if err != nil {
			log.Printf("Error marshaling ortho to JSON: %v", err)
			return false
		}

		// Push to work queue
		err = workQueueClient.PushMessage(context.Background(), cfg.WorkQueueName, orthoJSON)
		if err != nil {
			log.Printf("Error pushing ortho %s to work queue: %v", ortho.ID, err)
			return false
		} else {
			log.Printf("Successfully pushed ortho %s to work queue", ortho.ID)
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
		// Crash the service by panicking
		panic(errMsg)
	}
}
