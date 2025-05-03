package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
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

	"github.com/gin-gonic/gin"
	"github.com/kelseyhightower/envconfig"
	"go.opentelemetry.io/otel"
)

var ingestMutex sync.Mutex
var ctxStore types.ContextStore // Add a ContextStore for direct DB access

type contextKey string

const configKey contextKey = "config"

func ginHandleTextInput(c *gin.Context, contextService types.ContextService, remediationsService types.RemediationsService, orthosService types.OrthosService, workServerService types.WorkServerService) {
	if !ingestMutex.TryLock() {
		c.JSON(http.StatusLocked, gin.H{
			"status":  "error",
			"message": "Another ingest operation is in progress. Please try again later.",
		})
		return
	}
	defer ingestMutex.Unlock()

	corpus, err := types.ProcessIncomingCorpus(c, "ingestor")
	if telemetry.LogAndError(c, err, "ingestor", "Error processing incoming corpus") {
		return
	}

	contextResponse, err := contextService.SendMessage(c.Request.Context(), types.ContextInput{
		Title:      corpus.Title,
		Vocabulary: text.Vocabulary(corpus.Text),
		Subphrases: text.GenerateSubphrases(corpus.Text),
	})
	if telemetry.LogAndError(c, err, "ingestor", "Error sending message to context service") {
		return
	}

	remediationResp, err := remediationsService.FetchRemediations(c.Request.Context(), types.RemediationRequest{
		Pairs: types.ExtractPairsFromSubphrases(contextResponse.NewSubphrases),
	})

	if telemetry.LogAndError(c, err, "ingestor", "Error fetching remediations") {
		return
	}

	processedHashes := remediationResp.Hashes

	var orthosResp types.OrthosResponse

	orthosResp, err = orthosService.GetOrthosByIDs(c.Request.Context(), remediationResp.Hashes)
	if telemetry.LogAndError(c, err, "ingestor", "Error fetching orthos") {
		return
	}

	_, err = workServerService.PushOrthos(c.Request.Context(), orthosResp.Orthos)
	if telemetry.LogAndError(c, err, "ingestor", "Error pushing orthos to work server") {
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
	if telemetry.LogAndError(c, err, "ingestor", "Error pushing blank ortho to work server") {
		return
	}

	_, err = remediationsService.DeleteRemediations(c.Request.Context(), processedHashes)
	if telemetry.LogAndError(c, err, "ingestor", "Error deleting processed remediations") {
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
	if !ingestMutex.TryLock() {
		c.JSON(http.StatusLocked, gin.H{
			"status":  "error",
			"message": "Another ingest operation is in progress. Please try again later.",
		})
		return
	}
	defer ingestMutex.Unlock()

	// Parse the incoming corpus
	corpus, err := types.ProcessIncomingCorpus(c, "ingestor")
	if telemetry.LogAndError(c, err, "ingestor", "Error processing incoming corpus") {
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
	if telemetry.LogAndError(c, err, "ingestor", "Error pushing context to queue") {
		return
	}

	// Get context service response
	contextResponse, err := contextService.SendMessage(c.Request.Context(), contextInput)
	if telemetry.LogAndError(c, err, "ingestor", "Error sending message to context service") {
		return
	}

	// Step 2: Push Version to RabbitMQ using timestamp instead of context version
	currentTimestamp := int(time.Now().Unix())
	err = rabbitMQService.PushVersion(c.Request.Context(), types.VersionInfo{
		Version: currentTimestamp,
	})
	if telemetry.LogAndError(c, err, "ingestor", "Error pushing version to queue") {
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
	if telemetry.LogAndError(c, err, "ingestor", "Error pushing pairs to queue") {
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
	if telemetry.LogAndError(c, err, "ingestor", "Error pushing seed ortho to queue") {
		return
	}

	// Return 202 Accepted response (as per the diagram)
	c.JSON(http.StatusAccepted, gin.H{
		"status":  "success",
		"message": "Corpus processing initiated",
	})
}

// initStore initializes the context store for direct DB access
func initStore(cfg config.IngestorConfig) error {
	var err error
	connURL := cfg.ContextDBEndpoint
	ctxStore, err = types.NewLibSQLContextStore(connURL)
	if err != nil {
		return fmt.Errorf("failed to initialize context store: %w", err)
	}
	return nil
}

// handleGetContext implements the flow from get_context.md
// The ingestor acts as the repository in this case, fetching context data directly from the DB
func handleGetContext(c *gin.Context) {
	// Create a span using OpenTelemetry
	tracer := otel.Tracer("crochet/ingestor")
	ctx, span := tracer.Start(c.Request.Context(), "ingestor.handleGetContext")
	defer span.End()

	// Read context data directly from DB
	vocabularySlice := ctxStore.GetVocabulary()
	linesSlice := ctxStore.GetSubphrases()

	// Get the version from the database using the new GetVersion method
	version, err := ctxStore.GetVersion()
	if err != nil {
		telemetry.LogAndError(c, err, "ingestor", "Error getting context version")
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

func main() {
	var cfg config.IngestorConfig
	if err := envconfig.Process("INGESTOR", &cfg); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
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
	contextRabbitClient, err := httpclient.NewRabbitClient[types.ContextInput]("amqp://guest:guest@rabbitmq:5672/")
	if err != nil {
		log.Fatalf("Failed to create context RabbitMQ client: %v", err)
	}
	defer contextRabbitClient.Close(context.Background())

	versionRabbitClient, err := httpclient.NewRabbitClient[types.VersionInfo]("amqp://guest:guest@rabbitmq:5672/")
	if err != nil {
		log.Fatalf("Failed to create version RabbitMQ client: %v", err)
	}
	defer versionRabbitClient.Close(context.Background())

	pairsRabbitClient, err := httpclient.NewRabbitClient[types.Pair]("amqp://guest:guest@rabbitmq:5672/")
	if err != nil {
		log.Fatalf("Failed to create pairs RabbitMQ client: %v", err)
	}
	defer pairsRabbitClient.Close(context.Background())

	seedRabbitClient, err := httpclient.NewRabbitClient[types.Ortho]("amqp://guest:guest@rabbitmq:5672/")
	if err != nil {
		log.Fatalf("Failed to create seed RabbitMQ client: %v", err)
	}
	defer seedRabbitClient.Close(context.Background())

	// Create the RabbitMQ service
	rabbitMQService := clients.NewRabbitMQService(
		"amqp://guest:guest@rabbitmq:5672/",
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

	// Add the GET /Context endpoint as per get_context.md diagram
	// This implementation reads directly from the database
	router.GET("/Context", handleGetContext)

	healthCheck := health.New(health.Options{
		ServiceName: cfg.ServiceName,
		Version:     "1.0.0",
		Details: map[string]string{
			"description": "Ingests text data into the Crochet system",
		},
	})

	router.GET("/health", gin.WrapF(healthCheck.Handler()))

	address := cfg.GetAddress()
	log.Printf("Server starting on %s...\n", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
