package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"crochet/config"
	"crochet/telemetry"
	"crochet/text"
	"crochet/types"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
)

// RepositoryHandler encapsulates the dependencies needed for repository endpoints
type RepositoryHandler struct {
	Store           types.ContextStore
	OrthosCache     OrthosCache
	RabbitMQService types.RabbitMQService // For pushing context, version, pairs, and seed to DB queue
	WorkQueueClient WorkQueueClient       // For popping work and acknowledging receipts
	Config          config.RepositoryConfig
}

// NewRepositoryHandler creates a new repository handler with the given dependencies
func NewRepositoryHandler(
	store types.ContextStore,
	orthosCache OrthosCache,
	rabbitMQService types.RabbitMQService,
	workQueueClient WorkQueueClient,
	cfg config.RepositoryConfig,
) *RepositoryHandler {
	log.Printf("Creating repository handler with RabbitMQ service at %T", rabbitMQService)
	return &RepositoryHandler{
		Store:           store,
		OrthosCache:     orthosCache,
		RabbitMQService: rabbitMQService,
		WorkQueueClient: workQueueClient,
		Config:          cfg,
	}
}

// HandlePostCorpus handles the POST /corpora endpoint
func (h *RepositoryHandler) HandlePostCorpus(c *gin.Context) {
	tracer := otel.Tracer("crochet/repository")
	ctx, span := tracer.Start(c.Request.Context(), "repository.handlePostCorpus")
	defer span.End()
	requestID := ctx.Value("request_id")
	log.Printf("[%s] Received POST /corpora request", requestID)

	// Log RabbitMQ service information
	log.Printf("[%s] Using RabbitMQ service: %T", requestID, h.RabbitMQService)

	corpus, err := types.ProcessIncomingCorpus(c, "repository")
	if telemetry.LogAndError(c, err, "repository", "Error processing incoming corpus") {
		log.Printf("[%s] Failed to process incoming corpus: %v", requestID, err)
		return
	}

	log.Printf("[%s] Successfully processed corpus with title: %s and text: %s", requestID, corpus.Title, corpus.Text)

	subphrases := text.GenerateSubphrases(corpus.Text)
	log.Printf("[%s] Generated %d subphrases from corpus text: %v", requestID, len(subphrases), subphrases)

	vocabulary := text.Vocabulary(corpus.Text)
	log.Printf("[%s] Generated vocabulary with %d words: %v", requestID, len(vocabulary), vocabulary)

	contextInput := types.ContextInput{
		Title:      corpus.Title,
		Vocabulary: vocabulary,
		Subphrases: subphrases,
	}

	log.Printf("[%s] Preparing to push context with %d vocabulary items to DB queue: %v",
		requestID, len(contextInput.Vocabulary), contextInput.Vocabulary)

	// Start pushing context
	log.Printf("[%s] Starting RabbitMQService.PushContext call", requestID)
	err = h.RabbitMQService.PushContext(ctx, contextInput)
	if telemetry.LogAndError(c, err, "repository", "Error pushing context to queue") {
		log.Printf("[%s] Failed to push context to DB queue: %v", requestID, err)
		return
	}
	log.Printf("[%s] Successfully pushed context to DB queue", requestID)

	currentTimestamp := int(time.Now().Unix())
	versionInfo := types.VersionInfo{
		Version: currentTimestamp,
	}
	log.Printf("[%s] Preparing to push version %d to DB queue", requestID, versionInfo.Version)

	// Start pushing version
	log.Printf("[%s] Starting RabbitMQService.PushVersion call", requestID)
	err = h.RabbitMQService.PushVersion(ctx, versionInfo)
	if telemetry.LogAndError(c, err, "repository", "Error pushing version to queue") {
		log.Printf("[%s] Failed to push version to DB queue: %v", requestID, err)
		return
	}
	log.Printf("[%s] Successfully pushed version to DB queue", requestID)

	pairs := make([]types.Pair, 0)
	for _, subphrase := range subphrases {
		if len(subphrase) == 2 {
			pairs = append(pairs, types.Pair{
				Left:  subphrase[0],
				Right: subphrase[1],
			})
		}
	}
	log.Printf("[%s] Preparing to push %d pairs to DB queue: %v", requestID, len(pairs), pairs)

	// Start pushing pairs
	log.Printf("[%s] Starting RabbitMQService.PushPairs call with %d pairs", requestID, len(pairs))
	err = h.RabbitMQService.PushPairs(ctx, pairs)
	if telemetry.LogAndError(c, err, "repository", "Error pushing pairs to queue") {
		log.Printf("[%s] Failed to push pairs to DB queue: %v", requestID, err)
		return
	}
	log.Printf("[%s] Successfully pushed pairs to DB queue", requestID)

	seed := types.Ortho{
		Grid:     make(map[string]string),
		Shape:    []int{2, 2},
		Position: []int{0, 0},
		Shell:    0,
		ID:       fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))),
	}
	log.Printf("[%s] Preparing to push seed ortho with ID %s to DB queue", requestID, seed.ID)

	// Start pushing seed
	log.Printf("[%s] Starting RabbitMQService.PushSeed call", requestID)
	err = h.RabbitMQService.PushSeed(ctx, seed)
	if telemetry.LogAndError(c, err, "repository", "Error pushing seed ortho to queue") {
		log.Printf("[%s] Failed to push seed ortho to DB queue: %v", requestID, err)
		return
	}
	log.Printf("[%s] Successfully pushed seed ortho to DB queue", requestID)

	c.JSON(http.StatusAccepted, gin.H{
		"status":  "success",
		"message": "Corpus processing initiated",
	})
	log.Printf("[%s] Corpus processing initiated successfully, POST /corpora request complete", requestID)
}

// HandlePostResults handles the POST /Results endpoint
func (h *RepositoryHandler) HandlePostResults(c *gin.Context) {
	tracer := otel.Tracer("crochet/repository")
	ctx, span := tracer.Start(c.Request.Context(), "repository.handlePostResults")
	defer span.End()
	requestID := ctx.Value("request_id")

	var request types.ResultsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		telemetry.LogAndError(c, err, "repository", "Invalid JSON format for results")
		return
	}

	version, err := h.Store.GetVersion()
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Error getting version from database")
		return
	}

	// Filter new orthos using the cache but don't try to push them to any service
	newOrthos := h.OrthosCache.FilterNewOrthos(request.Orthos)
	log.Printf("[%s] Received %d orthos, %d are new", requestID, len(request.Orthos), len(newOrthos))

	// Save the new orthos to the database
	if len(newOrthos) > 0 {
		log.Printf("[%s] Saving %d new orthos to database", requestID, len(newOrthos))
		if err := h.Store.SaveOrthos(newOrthos); err != nil {
			telemetry.LogAndError(c, err, "repository", "Failed to save orthos to database")
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to save orthos to database",
			})
			return
		}
		log.Printf("[%s] Successfully saved %d new orthos to database", requestID, len(newOrthos))
	}

	// Parse receipt tag and acknowledge it on the work queue
	receiptTag, err := strconv.ParseUint(request.Receipt, 10, 64)
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Invalid receipt format")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Invalid receipt format",
		})
		return
	}

	// Acknowledge the receipt on the work queue
	if err := h.WorkQueueClient.AckByDeliveryTag(ctx, receiptTag); err != nil {
		telemetry.LogAndError(c, err, "repository", "Failed to acknowledge work receipt")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to acknowledge work receipt",
		})
		return
	}

	log.Printf("[%s] Successfully acknowledged receipt: %s", requestID, request.Receipt)

	response := types.ResultsResponse{
		Status:         "success",
		Version:        version,
		NewOrthosCount: len(newOrthos),
	}
	c.JSON(http.StatusOK, response)
}

// HandleGetContext handles the GET /Context endpoint
func (h *RepositoryHandler) HandleGetContext(c *gin.Context) {
	tracer := otel.Tracer("crochet/repository")
	ctx, span := tracer.Start(c.Request.Context(), "repository.handleGetContext")
	defer span.End()

	requestID := ctx.Value("request_id")
	log.Printf("[%s] Received GET /context request", requestID)

	vocabularySlice := h.Store.GetVocabulary()
	log.Printf("[%s] Retrieved vocabulary from store: %v", requestID, vocabularySlice)

	linesSlice := h.Store.GetSubphrases()
	log.Printf("[%s] Retrieved subphrases from store: %v", requestID, linesSlice)

	version, err := h.Store.GetVersion()
	if err != nil {
		log.Printf("[%s] Error getting version from store: %v", requestID, err)
		telemetry.LogAndError(c, err, "repository", "Error getting context version")
		return
	}
	log.Printf("[%s] Retrieved version from store: %d", requestID, version)

	response := gin.H{
		"version":    version,
		"vocabulary": vocabularySlice,
		"lines":      linesSlice,
	}

	log.Printf("[%s] Retrieved context data: %d vocabulary items, %d lines",
		ctx.Value("request_id"), len(vocabularySlice), len(linesSlice))
	c.JSON(http.StatusOK, response)
}

// HandleGetWork handles the GET /Work endpoint
func (h *RepositoryHandler) HandleGetWork(c *gin.Context) {
	tracer := otel.Tracer("crochet/repository")
	ctx, span := tracer.Start(c.Request.Context(), "repository.handleGetWork")
	defer span.End()

	requestID := ctx.Value("request_id")
	log.Printf("[%s] Received GET /work request", requestID)

	version, err := h.Store.GetVersion()
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Error getting version from database")
		return
	}

	log.Printf("[%s] Current version: %d", requestID, version)
	log.Printf("[%s] Attempting to pop messages from work queue: %s", requestID, h.Config.WorkQueueName)

	// Get work items from work queue
	messages, err := h.WorkQueueClient.PopMessagesFromQueue(ctx, h.Config.WorkQueueName, 1)
	if err != nil {
		log.Printf("[%s] Error popping message from work queue: %v", requestID, err)
		telemetry.LogAndError(c, err, "repository", "Error popping message from work queue")
		return
	}

	if len(messages) == 0 {
		log.Printf("[%s] No work items available in queue", requestID)
		c.JSON(http.StatusOK, types.WorkResponse{
			Version: version,
			Work:    nil,
		})
		return
	}

	message := messages[0]
	log.Printf("[%s] Popped message from work queue with delivery tag: %d", requestID, message.DeliveryTag)

	// Enhanced logging to see the raw message data
	if messageJSON, err := json.Marshal(message.Data); err == nil {
		log.Printf("[%s] Raw message data from work queue: %s", requestID, string(messageJSON))
	} else {
		log.Printf("[%s] Error marshaling message data: %v", requestID, err)
	}

	// Validate the message data before creating work item
	if message.Data == nil {
		log.Printf("[%s] WARNING: Message data is nil, creating empty work item", requestID)
	}

	// Create a new WorkItem with the Ortho as its Data field
	// This is necessary because the feeder pushes Ortho objects directly
	workItem := types.WorkItem{
		ID:        fmt.Sprintf("work-%d", time.Now().UnixNano()),
		Data:      message.Data,
		Timestamp: time.Now().UnixNano(),
	}

	// Additional validation to ensure workItem.Data is properly set
	if workItemJSON, err := json.Marshal(workItem); err == nil {
		log.Printf("[%s] Work item being sent to worker: %s", requestID, string(workItemJSON))
	} else {
		log.Printf("[%s] Error marshaling work item: %v", requestID, err)
	}

	receipt := fmt.Sprintf("%d", message.DeliveryTag)
	response := types.WorkResponse{
		Version: version,
		Work:    &workItem,
		Receipt: receipt,
	}

	log.Printf("[%s] Providing work item with ID %s to worker, receipt: %s",
		requestID, workItem.ID, receipt)

	c.JSON(http.StatusOK, response)
}

// HandleGetResults handles the GET /results endpoint to retrieve ortho counts
func (h *RepositoryHandler) HandleGetResults(c *gin.Context) {
	tracer := otel.Tracer("crochet/repository")
	ctx, span := tracer.Start(c.Request.Context(), "repository.handleGetResults")
	defer span.End()

	// Get total count of orthos
	totalCount, err := h.Store.GetOrthoCount()
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Error getting ortho count")
		return
	}

	// Get counts by shape and position
	countsByShapePosition, err := h.Store.GetOrthoCountByShapePosition()
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Error getting ortho counts by shape and position")
		return
	}

	response := gin.H{
		"status":     "success",
		"totalCount": totalCount,
		"counts":     countsByShapePosition,
	}

	log.Printf("[%s] Retrieved ortho counts: total=%d, shapes=%d",
		ctx.Value("request_id"), totalCount, len(countsByShapePosition))

	c.JSON(http.StatusOK, response)
}
