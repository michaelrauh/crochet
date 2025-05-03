// filepath: /Users/michaelrauh/dev/crochet/repository/main.go
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"crochet/clients"
	"crochet/config"
	"crochet/health"
	"crochet/httpclient"
	"crochet/middleware"
	"crochet/telemetry"
	"crochet/text"
	"crochet/types"

	"github.com/dgraph-io/ristretto"
	"github.com/gin-gonic/gin"
	"github.com/kelseyhightower/envconfig"
	"go.opentelemetry.io/otel"
)

var repositoryMutex sync.Mutex
var ctxStore types.ContextStore  // Add a ContextStore for direct DB access
var orthosCache *ristretto.Cache // Ristretto cache for Orthos

type contextKey string

const configKey contextKey = "config"

// Define types needed for the POST /Results endpoint
type ResultsRequest struct {
	Orthos       []types.Ortho            `json:"orthos"`
	Remediations []types.RemediationTuple `json:"remediations"`
	Receipt      string                   `json:"receipt"`
}

type ResultsResponse struct {
	Status            string `json:"status"`
	Version           int    `json:"version"`
	NewOrthosCount    int    `json:"newOrthosCount"`
	RemediationsCount int    `json:"remediationsCount"`
}

// filterNewOrthos filters out orthos that we've already seen using Ristretto cache
func filterNewOrthos(orthos []types.Ortho) []types.Ortho {
	if orthosCache == nil {
		// This shouldn't happen, but as a fallback return all orthos
		log.Println("Warning: Orthos cache not initialized")
		return orthos
	}

	// Return only new orthos not in the cache
	newOrthos := make([]types.Ortho, 0)

	for _, ortho := range orthos {
		// Check if this ortho ID is in the cache
		if _, found := orthosCache.Get(ortho.ID); !found {
			// Not in cache, so it's new
			newOrthos = append(newOrthos, ortho)

			// Add to cache with TTL of 24 hours (can be configured)
			// Cost is set to 1 for simplicity
			orthosCache.SetWithTTL(ortho.ID, true, 1, 24*time.Hour)
		}
	}

	// Wait for any background processing to complete
	orthosCache.Wait()

	return newOrthos
}

// Initialize the Ristretto cache
func initOrthosCache() error {
	config := &ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M)
		MaxCost:     1 << 30, // maximum cost of cache (1GB)
		BufferItems: 64,      // number of keys per Get buffer
		Metrics:     true,    // collect metrics
	}

	var err error
	orthosCache, err = ristretto.NewCache(config)
	if err != nil {
		return fmt.Errorf("failed to initialize orthos cache: %w", err)
	}

	log.Println("Ristretto cache initialized successfully")
	return nil
}

func ginHandleTextInput(c *gin.Context, contextService types.ContextService, remediationsService types.RemediationsService, orthosService types.OrthosService, workServerService types.WorkServerService) {
	if !repositoryMutex.TryLock() {
		c.JSON(http.StatusLocked, gin.H{
			"status":  "error",
			"message": "Another repository operation is in progress. Please try again later.",
		})
		return
	}
	defer repositoryMutex.Unlock()

	corpus, err := types.ProcessIncomingCorpus(c, "repository")
	if telemetry.LogAndError(c, err, "repository", "Error processing incoming corpus") {
		return
	}

	contextResponse, err := contextService.SendMessage(c.Request.Context(), types.ContextInput{
		Title:      corpus.Title,
		Vocabulary: text.Vocabulary(corpus.Text),
		Subphrases: text.GenerateSubphrases(corpus.Text),
	})
	if telemetry.LogAndError(c, err, "repository", "Error sending message to context service") {
		return
	}

	remediationResp, err := remediationsService.FetchRemediations(c.Request.Context(), types.RemediationRequest{
		Pairs: types.ExtractPairsFromSubphrases(contextResponse.NewSubphrases),
	})
	if telemetry.LogAndError(c, err, "repository", "Error fetching remediations") {
		return
	}

	processedHashes := remediationResp.Hashes
	var orthosResp types.OrthosResponse
	orthosResp, err = orthosService.GetOrthosByIDs(c.Request.Context(), remediationResp.Hashes)
	if telemetry.LogAndError(c, err, "repository", "Error fetching orthos") {
		return
	}

	_, err = workServerService.PushOrthos(c.Request.Context(), orthosResp.Orthos)
	if telemetry.LogAndError(c, err, "repository", "Error pushing orthos to work server") {
		return
	}

	// Todo: make the minimal ortho smaller
	_, err = workServerService.PushOrthos(c.Request.Context(), []types.Ortho{{
		Grid:     make(map[string]string),
		Shape:    []int{2, 2},
		Position: []int{0, 0},
		Shell:    0,
		ID:       fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))),
	}})
	if telemetry.LogAndError(c, err, "repository", "Error pushing blank ortho to work server") {
		return
	}

	_, err = remediationsService.DeleteRemediations(c.Request.Context(), processedHashes)
	if telemetry.LogAndError(c, err, "repository", "Error deleting processed remediations") {
		return
	}

	response := gin.H{
		"status":  "success",
		"version": contextResponse.Version,
	}
	c.JSON(http.StatusOK, response)
}

// handlePostCorpus implements the flow from post_corpora.md, handling corpus submissions using RabbitMQ
func handlePostCorpus(c *gin.Context, contextService types.ContextService, rabbitMQService types.RabbitMQService) {
	if !repositoryMutex.TryLock() {
		c.JSON(http.StatusLocked, gin.H{
			"status":  "error",
			"message": "Another repository operation is in progress. Please try again later.",
		})
		return
	}
	defer repositoryMutex.Unlock()

	// Parse the incoming corpus
	corpus, err := types.ProcessIncomingCorpus(c, "repository")
	if telemetry.LogAndError(c, err, "repository", "Error processing incoming corpus") {
		return
	}

	// Create context input from corpus
	contextInput := types.ContextInput{
		Title:      corpus.Title,
		Vocabulary: text.Vocabulary(corpus.Text),
		Subphrases: text.GenerateSubphrases(corpus.Text),
	}

	// Step 1: Push Context to RabbitMQ
	err = rabbitMQService.PushContext(c.Request.Context(), contextInput)
	if telemetry.LogAndError(c, err, "repository", "Error pushing context to queue") {
		return
	}

	// Get context service response
	contextResponse, err := contextService.SendMessage(c.Request.Context(), contextInput)
	if telemetry.LogAndError(c, err, "repository", "Error sending message to context service") {
		return
	}

	// Step 2: Push Version to RabbitMQ using timestamp instead of context version
	currentTimestamp := int(time.Now().Unix())
	err = rabbitMQService.PushVersion(c.Request.Context(), types.VersionInfo{
		Version: currentTimestamp,
	})
	if telemetry.LogAndError(c, err, "repository", "Error pushing version to queue") {
		return
	}

	// Extract pairs from subphrases
	pairs := make([]types.Pair, 0)
	for _, subphrase := range contextResponse.NewSubphrases {
		if len(subphrase) >= 2 {
			pairs = append(pairs, types.Pair{
				Left:  subphrase[0],
				Right: subphrase[1],
			})
		}
	}

	// Step 3: Push Pairs to RabbitMQ
	err = rabbitMQService.PushPairs(c.Request.Context(), pairs)
	if telemetry.LogAndError(c, err, "repository", "Error pushing pairs to queue") {
		return
	}

	// Generate a seed ortho
	seedOrtho := types.Ortho{
		Grid:     make(map[string]string),
		Shape:    []int{2, 2},
		Position: []int{0, 0},
		Shell:    0,
		ID:       fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))),
	}

	// Step 4: Push Seed to RabbitMQ
	err = rabbitMQService.PushSeed(c.Request.Context(), seedOrtho)
	if telemetry.LogAndError(c, err, "repository", "Error pushing seed ortho to queue") {
		return
	}

	// Return 202 Accepted response (as per the diagram)
	c.JSON(http.StatusAccepted, gin.H{
		"status":  "success",
		"message": "Corpus processing initiated",
	})
}

// handlePostResults implements the flow from post_results.md
func handlePostResults(c *gin.Context) {
	// Create a span using OpenTelemetry
	tracer := otel.Tracer("crochet/repository")
	ctx, span := tracer.Start(c.Request.Context(), "repository.handlePostResults")
	defer span.End()

	// Get the config from the context
	cfg, ok := ctx.Value(configKey).(config.RepositoryConfig)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Configuration not available",
		})
		return
	}

	// Parse the request body
	var request ResultsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		telemetry.LogAndError(c, err, "repository", "Invalid JSON format for results")
		return
	}

	// Get the current version from the database
	version, err := ctxStore.GetVersion()
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Error getting version from database")
		return
	}

	// Filter out orthos that we've already seen using Ristretto cache
	newOrthos := filterNewOrthos(request.Orthos)
	log.Printf("[%s] Received %d orthos, %d are new", ctx.Value("request_id"), len(request.Orthos), len(newOrthos))

	// Create RabbitMQ clients for DB operations
	orthosClient, err := httpclient.NewRabbitClient[types.Ortho](cfg.RabbitMQURL)
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Failed to create orthos RabbitMQ client")
		return
	}
	defer orthosClient.Close(ctx)

	remediationsClient, err := httpclient.NewRabbitClient[types.RemediationTuple](cfg.RabbitMQURL)
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Failed to create remediations RabbitMQ client")
		return
	}
	defer remediationsClient.Close(ctx)

	// Push new orthos to DB queue
	for _, ortho := range newOrthos {
		orthoJSON, err := json.Marshal(ortho)
		if err != nil {
			telemetry.LogAndError(c, err, "repository", "Failed to marshal ortho")
			continue
		}

		if err := orthosClient.PushMessage(ctx, cfg.DBQueueName, orthoJSON); err != nil {
			telemetry.LogAndError(c, err, "repository", "Failed to push ortho to DB queue")
			continue
		}
	}

	// Push remediations to DB queue
	for _, remediation := range request.Remediations {
		remediationJSON, err := json.Marshal(remediation)
		if err != nil {
			telemetry.LogAndError(c, err, "repository", "Failed to marshal remediation")
			continue
		}

		if err := remediationsClient.PushMessage(ctx, cfg.DBQueueName, remediationJSON); err != nil {
			telemetry.LogAndError(c, err, "repository", "Failed to push remediation to DB queue")
			continue
		}
	}

	// Now acknowledge the receipt in the Work queue if provided
	if request.Receipt != "" {
		// Convert receipt string back to delivery tag
		receiptTag, err := strconv.ParseUint(request.Receipt, 10, 64)
		if err != nil {
			telemetry.LogAndError(c, err, "repository", "Invalid receipt format")
			// Continue anyway, still returning success for the data we processed
		} else {
			// Create a client for the work queue
			workClient, err := httpclient.NewRabbitClient[WorkItem](cfg.RabbitMQURL)
			if err != nil {
				telemetry.LogAndError(c, err, "repository", "Failed to create work queue client")
				// Continue anyway
			} else {
				defer workClient.Close(ctx)

				// Use the new AckByDeliveryTag method
				if err := workClient.AckByDeliveryTag(ctx, receiptTag); err != nil {
					telemetry.LogAndError(c, err, "repository", "Failed to acknowledge work receipt")
					// Continue anyway
				} else {
					log.Printf("[%s] Successfully acknowledged receipt: %s", ctx.Value("request_id"), request.Receipt)
				}
			}
		}
	}

	// Return response
	response := ResultsResponse{
		Status:            "success",
		Version:           version,
		NewOrthosCount:    len(newOrthos),
		RemediationsCount: len(request.Remediations),
	}

	c.JSON(http.StatusOK, response)
}

// AckRabbitMessage acknowledges a message with the given delivery tag
func AckRabbitMessage(ctx context.Context, client *httpclient.RabbitClient[WorkItem], deliveryTag uint64) error {
	// Create a dummy MessageWithAck to use its Ack method
	// This is a workaround since we only have the delivery tag, not the full message
	tempCh, err := client.SetupConsumer(ctx, "", 1) // Use empty queue name as we won't actually consume
	if err != nil {
		return fmt.Errorf("failed to create temporary channel: %w", err)
	}

	// Pop at least one message to get a valid message with Ack capability
	// This is just to get a valid MessageWithAck instance
	msgs, err := client.PopMessages(ctx, tempCh, 1)
	if err != nil || len(msgs) == 0 {
		// If we couldn't get a real message, we can't ack directly
		// Return a more helpful error
		return fmt.Errorf("cannot ack message with tag %d: no direct access to channel", deliveryTag)
	}

	// Replace the delivery tag with the one we want to ack
	msg := msgs[0]
	msg.DeliveryTag = deliveryTag

	// Now ack the message
	return msg.Ack()
}

// initStore initializes the context store for direct DB access
func initStore(cfg config.RepositoryConfig) error {
	var err error
	connURL := cfg.ContextDBEndpoint
	ctxStore, err = types.NewLibSQLContextStore(connURL)
	if err != nil {
		return fmt.Errorf("failed to initialize context store: %w", err)
	}
	return nil
}

// handleGetContext implements the flow from get_context.md
// The repository acts as the repository in this case, fetching context data directly from the DB
func handleGetContext(c *gin.Context) {
	// Create a span using OpenTelemetry
	tracer := otel.Tracer("crochet/repository")
	ctx, span := tracer.Start(c.Request.Context(), "repository.handleGetContext")
	defer span.End()

	// Read context data directly from DB
	vocabularySlice := ctxStore.GetVocabulary()
	linesSlice := ctxStore.GetSubphrases()

	// Get the version from the database using the new GetVersion method
	version, err := ctxStore.GetVersion()
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Error getting context version")
		return
	}

	// Construct response with the proper version from the DB
	response := gin.H{
		"version":    version,
		"vocabulary": vocabularySlice,
		"lines":      linesSlice,
	}

	// Log the operation with the created context for tracing
	log.Printf("[%s] Retrieved context data: %d vocabulary items, %d lines",
		ctx.Value("request_id"), len(vocabularySlice), len(linesSlice))

	// Return the context data with a 200 OK response as per the diagram
	c.JSON(http.StatusOK, response)
}

// Define WorkItem and WorkResponse types
type WorkItem struct {
	ID        string      `json:"id"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

type WorkResponse struct {
	Version int       `json:"version"`
	Work    *WorkItem `json:"work"`
	Receipt string    `json:"receipt,omitempty"`
}

// handleGetWork implements the flow from get_work.md
func handleGetWork(c *gin.Context) {
	// Create a span using OpenTelemetry
	tracer := otel.Tracer("crochet/repository")
	ctx, span := tracer.Start(c.Request.Context(), "repository.handleGetWork")
	defer span.End()

	// Get the config from the context
	cfg, ok := ctx.Value(configKey).(config.RepositoryConfig)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Configuration not available",
		})
		return
	}

	// Read the current version from the database
	version, err := ctxStore.GetVersion()
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Error getting version from database")
		return
	}

	// Create a RabbitMQ client for the work queue
	workClient, err := httpclient.NewRabbitClient[WorkItem](cfg.RabbitMQURL)
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Failed to create RabbitMQ client for work queue")
		return
	}
	defer workClient.Close(ctx)

	// Pop a message from the work queue
	messages, err := workClient.PopMessagesFromQueue(ctx, cfg.WorkQueueName, 1)
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Error popping message from work queue")
		return
	}

	// No work available
	if len(messages) == 0 {
		c.JSON(http.StatusOK, WorkResponse{
			Version: version,
			Work:    nil,
		})
		return
	}

	// Process the work item
	message := messages[0]
	workItem := message.Data

	// Generate a receipt (using the delivery tag as the receipt)
	receipt := fmt.Sprintf("%d", message.DeliveryTag)

	// Return the work item to the worker
	response := WorkResponse{
		Version: version,
		Work:    &workItem,
		Receipt: receipt,
	}

	c.JSON(http.StatusOK, response)
	log.Printf("[%s] Provided work item with ID %s to worker, receipt: %s", ctx.Value("request_id"), workItem.ID, receipt)
}

func main() {
	var cfg config.RepositoryConfig
	if err := envconfig.Process("REPOSITORY", &cfg); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize Ristretto cache
	if err := initOrthosCache(); err != nil {
		log.Fatalf("Failed to initialize Ristretto cache: %v", err)
	}

	router, tp, mp, pp, err := middleware.SetupCommonComponents(
		cfg.ServiceName,
		cfg.JaegerEndpoint,
		cfg.MetricsEndpoint,
		cfg.PyroscopeEndpoint,
	)
	if err != nil {
		log.Fatalf("Failed to set up application: %v", err)
	}
	defer tp.ShutdownWithTimeout(5 * time.Second)
	defer mp.ShutdownWithTimeout(5 * time.Second)
	defer pp.StopWithTimeout(5 * time.Second)

	log.Printf("HTTP client options: DialTimeout=%v, DialKeepAlive=%v, MaxIdleConns=%d, ClientTimeout=%v",
		cfg.DialTimeout, cfg.DialKeepAlive, cfg.MaxIdleConns, cfg.ClientTimeout)

	contextClient := httpclient.NewDefaultGenericClient[types.ContextResponse]()
	versionClient := httpclient.NewDefaultGenericClient[types.VersionResponse]()
	dataClient := httpclient.NewDefaultGenericClient[types.ContextDataResponse]()
	remediationClient := httpclient.NewDefaultGenericClient[types.RemediationResponse]()
	addRemediationClient := httpclient.NewDefaultGenericClient[types.AddRemediationResponse]()
	deleteRemediationClient := httpclient.NewDefaultGenericClient[types.DeleteRemediationResponse]()
	getOrthosClient := httpclient.NewDefaultGenericClient[types.OrthosResponse]()
	saveOrthosClient := httpclient.NewDefaultGenericClient[types.OrthosSaveResponse]()
	pushClient := httpclient.NewDefaultGenericClient[types.WorkServerPushResponse]()
	popClient := httpclient.NewDefaultGenericClient[types.WorkServerPopResponse]()
	ackClient := httpclient.NewDefaultGenericClient[types.WorkServerAckResponse]()

	contextService := clients.NewContextService(cfg.ContextServiceURL, contextClient, versionClient, dataClient)
	remediationsService := clients.NewRemediationsService(cfg.RemediationsServiceURL, remediationClient, deleteRemediationClient, addRemediationClient)
	orthosService := clients.NewOrthosService(cfg.OrthosServiceURL, getOrthosClient, saveOrthosClient)
	workServerService := clients.NewWorkServerService(cfg.WorkServerURL, pushClient, popClient, ackClient)

	// Create RabbitMQ clients for the different types
	contextRabbitClient, err := httpclient.NewRabbitClient[types.ContextInput](cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to create context RabbitMQ client: %v", err)
	}
	defer contextRabbitClient.Close(context.Background())

	versionRabbitClient, err := httpclient.NewRabbitClient[types.VersionInfo](cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to create version RabbitMQ client: %v", err)
	}
	defer versionRabbitClient.Close(context.Background())

	pairsRabbitClient, err := httpclient.NewRabbitClient[types.Pair](cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to create pairs RabbitMQ client: %v", err)
	}
	defer pairsRabbitClient.Close(context.Background())

	seedRabbitClient, err := httpclient.NewRabbitClient[types.Ortho](cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to create seed RabbitMQ client: %v", err)
	}
	defer seedRabbitClient.Close(context.Background())

	// Create the RabbitMQ service
	rabbitMQService := clients.NewRabbitMQService(
		cfg.RabbitMQURL,
		cfg.DBQueueName,
		contextRabbitClient,
		versionRabbitClient,
		pairsRabbitClient,
		seedRabbitClient,
	)

	// Initialize the store for direct DB access
	if err := initStore(cfg); err != nil {
		log.Fatalf("Failed to initialize context store: %v", err)
	}
	defer func() {
		if err := ctxStore.Close(); err != nil {
			log.Printf("Error closing context store: %v", err)
		}
	}()

	router.POST("/ingest", func(c *gin.Context) {
		c.Set("config", cfg)
		ctxWithConfig := context.WithValue(c.Request.Context(), configKey, cfg)
		c.Request = c.Request.WithContext(ctxWithConfig)
		ginHandleTextInput(c, contextService, remediationsService, orthosService, workServerService)
	})

	// Add the new corpora handler that uses RabbitMQ
	router.POST("/corpora", func(c *gin.Context) {
		handlePostCorpus(c, contextService, rabbitMQService)
	})

	// Add the POST /Results endpoint as per post_results.md diagram
	router.POST("/Results", func(c *gin.Context) {
		ctxWithConfig := context.WithValue(c.Request.Context(), configKey, cfg)
		c.Request = c.Request.WithContext(ctxWithConfig)
		handlePostResults(c)
	})

	// Add the GET /Context endpoint as per get_context.md diagram
	// This implementation reads directly from the database
	router.GET("/Context", handleGetContext)

	// Add the GET /Work endpoint as per get_work.md diagram
	router.GET("/Work", func(c *gin.Context) {
		ctxWithConfig := context.WithValue(c.Request.Context(), configKey, cfg)
		c.Request = c.Request.WithContext(ctxWithConfig)
		handleGetWork(c)
	})

	healthCheck := health.New(health.Options{
		ServiceName: cfg.ServiceName,
		Version:     "1.0.0",
		Details: map[string]string{
			"description": "Provides repository functions for the Crochet system",
		},
	})
	router.GET("/health", gin.WrapF(healthCheck.Handler()))

	address := cfg.GetAddress()
	log.Printf("Server starting on %s...\n", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
