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
	"crochet/httpclient"
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
	RabbitMQService types.RabbitMQService
	Config          config.RepositoryConfig
}

// NewRepositoryHandler creates a new repository handler with the given dependencies
func NewRepositoryHandler(
	store types.ContextStore,
	orthosCache OrthosCache,
	rabbitMQService types.RabbitMQService,
	cfg config.RepositoryConfig,
) *RepositoryHandler {
	return &RepositoryHandler{
		Store:           store,
		OrthosCache:     orthosCache,
		RabbitMQService: rabbitMQService,
		Config:          cfg,
	}
}

// HandlePostCorpus handles the POST /corpora endpoint
func (h *RepositoryHandler) HandlePostCorpus(c *gin.Context) {
	tracer := otel.Tracer("crochet/repository")
	ctx, span := tracer.Start(c.Request.Context(), "repository.handlePostCorpus")
	defer span.End()

	corpus, err := types.ProcessIncomingCorpus(c, "repository")
	if telemetry.LogAndError(c, err, "repository", "Error processing incoming corpus") {
		return
	}

	subphrases := text.GenerateSubphrases(corpus.Text)

	err = h.RabbitMQService.PushContext(ctx, types.ContextInput{
		Title:      corpus.Title,
		Vocabulary: text.Vocabulary(corpus.Text),
		Subphrases: subphrases,
	})
	if telemetry.LogAndError(c, err, "repository", "Error pushing context to queue") {
		return
	}

	currentTimestamp := int(time.Now().Unix())
	err = h.RabbitMQService.PushVersion(ctx, types.VersionInfo{
		Version: currentTimestamp,
	})
	if telemetry.LogAndError(c, err, "repository", "Error pushing version to queue") {
		return
	}

	pairs := make([]types.Pair, 0)
	for _, subphrase := range subphrases {
		if len(subphrase) == 2 {
			pairs = append(pairs, types.Pair{
				Left:  subphrase[0],
				Right: subphrase[1],
			})
		}
	}

	err = h.RabbitMQService.PushPairs(ctx, pairs)
	if telemetry.LogAndError(c, err, "repository", "Error pushing pairs to queue") {
		return
	}

	err = h.RabbitMQService.PushSeed(ctx, types.Ortho{
		Grid:     make(map[string]string),
		Shape:    []int{2, 2},
		Position: []int{0, 0},
		Shell:    0,
		ID:       fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))),
	})
	if telemetry.LogAndError(c, err, "repository", "Error pushing seed ortho to queue") {
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"status":  "success",
		"message": "Corpus processing initiated",
	})
}

// HandlePostResults handles the POST /Results endpoint
func (h *RepositoryHandler) HandlePostResults(c *gin.Context) {
	tracer := otel.Tracer("crochet/repository")
	ctx, span := tracer.Start(c.Request.Context(), "repository.handlePostResults")
	defer span.End()

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

	newOrthos := h.OrthosCache.FilterNewOrthos(request.Orthos)
	log.Printf("[%s] Received %d orthos, %d are new", ctx.Value("request_id"), len(request.Orthos), len(newOrthos))

	orthosClient, err := httpclient.NewRabbitClient[types.Ortho](h.Config.RabbitMQURL)
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Failed to create orthos RabbitMQ client")
		return
	}
	defer orthosClient.Close(ctx)

	remediationsClient, err := httpclient.NewRabbitClient[types.RemediationTuple](h.Config.RabbitMQURL)
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Failed to create remediations RabbitMQ client")
		return
	}
	defer remediationsClient.Close(ctx)

	for _, ortho := range newOrthos {
		orthoJSON, err := json.Marshal(ortho)
		if err != nil {
			telemetry.LogAndError(c, err, "repository", "Failed to marshal ortho")
			continue
		}

		if err := orthosClient.PushMessage(ctx, h.Config.DBQueueName, orthoJSON); err != nil {
			telemetry.LogAndError(c, err, "repository", "Failed to push ortho to DB queue")
			continue
		}
	}

	for _, remediation := range request.Remediations {
		remediationJSON, err := json.Marshal(remediation)
		if err != nil {
			telemetry.LogAndError(c, err, "repository", "Failed to marshal remediation")
			continue
		}

		if err := remediationsClient.PushMessage(ctx, h.Config.DBQueueName, remediationJSON); err != nil {
			telemetry.LogAndError(c, err, "repository", "Failed to push remediation to DB queue")
			continue
		}
	}

	receiptTag, err := strconv.ParseUint(request.Receipt, 10, 64)
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Invalid receipt format")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Invalid receipt format",
		})
		return
	}

	workClient, err := httpclient.NewRabbitClient[types.WorkItem](h.Config.RabbitMQURL)
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Failed to create work queue client")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to create work queue client",
		})
		return
	}
	defer workClient.Close(ctx)

	if err := workClient.AckByDeliveryTag(ctx, receiptTag); err != nil {
		telemetry.LogAndError(c, err, "repository", "Failed to acknowledge work receipt")
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to acknowledge work receipt",
		})
		return
	}

	log.Printf("[%s] Successfully acknowledged receipt: %s", ctx.Value("request_id"), request.Receipt)

	response := types.ResultsResponse{
		Status:            "success",
		Version:           version,
		NewOrthosCount:    len(newOrthos),
		RemediationsCount: len(request.Remediations),
	}

	c.JSON(http.StatusOK, response)
}

// HandleGetContext handles the GET /Context endpoint
func (h *RepositoryHandler) HandleGetContext(c *gin.Context) {
	tracer := otel.Tracer("crochet/repository")
	ctx, span := tracer.Start(c.Request.Context(), "repository.handleGetContext")
	defer span.End()

	vocabularySlice := h.Store.GetVocabulary()
	linesSlice := h.Store.GetSubphrases()

	version, err := h.Store.GetVersion()
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Error getting context version")
		return
	}

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

	version, err := h.Store.GetVersion()
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Error getting version from database")
		return
	}

	workClient, err := httpclient.NewRabbitClient[types.WorkItem](h.Config.RabbitMQURL)
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Failed to create RabbitMQ client for work queue")
		return
	}
	defer workClient.Close(ctx)

	messages, err := workClient.PopMessagesFromQueue(ctx, h.Config.WorkQueueName, 1)
	if err != nil {
		telemetry.LogAndError(c, err, "repository", "Error popping message from work queue")
		return
	}

	if len(messages) == 0 {
		c.JSON(http.StatusOK, types.WorkResponse{
			Version: version,
			Work:    nil,
		})
		return
	}

	message := messages[0]
	workItem := message.Data

	receipt := fmt.Sprintf("%d", message.DeliveryTag)

	response := types.WorkResponse{
		Version: version,
		Work:    &workItem,
		Receipt: receipt,
	}

	c.JSON(http.StatusOK, response)
	log.Printf("[%s] Provided work item with ID %s to worker, receipt: %s", ctx.Value("request_id"), workItem.ID, receipt)
}
