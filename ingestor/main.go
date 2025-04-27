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
)

var ingestMutex sync.Mutex

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
	_, err = workServerService.PushOrthos(c.Request.Context(), []types.Ortho{types.Ortho{
		Grid:     make(map[string]string),
		Shape:    []int{2, 2},
		Position: []int{0, 0},
		Shell:    0,
		ID:       fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))),
	}})
	if telemetry.LogAndError(c, err, "ingestor", "Error pushing blank ortho to work server") {
		return
	}

	if len(processedHashes) > 0 {
		log.Printf("Cleaning up %d processed remediation hashes", len(processedHashes))
		deleteResp, err := remediationsService.DeleteRemediations(c.Request.Context(), processedHashes)
		if err != nil {
			// Log the error but continue - this is cleanup so we don't want to fail the request
			log.Printf("Warning: Failed to delete processed remediations: %v", err)
		} else {
			log.Printf("Successfully deleted %d remediations", deleteResp.Count)
		}
	}

	// Return all data to the client
	response := gin.H{
		"status":  "success",
		"version": contextResponse.Version,
		"hashes":  remediationResp.Hashes,
	}
	// Add orthos to the response if we have any
	if orthosResp.Count > 0 {
		response["orthos"] = orthosResp.Orthos
		response["orthosCount"] = orthosResp.Count
	}

	log.Println("Ingest operation completed successfully")
	c.JSON(http.StatusOK, response)
}

func main() {
	// Load configuration
	cfg, err := config.LoadIngestorConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Log configuration details
	log.Printf("Using unified configuration management: %+v", cfg)
	config.LogConfig(cfg.BaseConfig)
	log.Printf("Remediations service URL: %s", cfg.RemediationsServiceURL)
	log.Printf("Context service URL: %s", cfg.ContextServiceURL)
	log.Printf("Orthos service URL: %s", cfg.OrthosServiceURL)
	log.Printf("Work server URL: %s", cfg.WorkServerURL)

	// Set up common components using the updated shared helper with profiling
	router, tp, mp, pp, err := middleware.SetupCommonComponents(
		cfg.ServiceName,
		cfg.JaegerEndpoint,
		cfg.MetricsEndpoint,
		cfg.PyroscopeEndpoint,
	)
	if err != nil {
		log.Fatalf("Failed to set up application: %v", err)
	}
	// Ensure resources are properly cleaned up
	defer tp.ShutdownWithTimeout(5 * time.Second)
	if mp != nil {
		defer mp.ShutdownWithTimeout(5 * time.Second)
	}
	if pp != nil {
		defer pp.StopWithTimeout(5 * time.Second)
	}

	// Set up HTTP client (specific to ingestor service)
	httpClientOptions := httpclient.ClientOptions{
		DialTimeout:   cfg.DialTimeout,
		DialKeepAlive: cfg.DialKeepAlive,
		MaxIdleConns:  cfg.MaxIdleConns,
		ClientTimeout: cfg.ClientTimeout,
	}
	log.Printf("HTTP client options: DialTimeout=%v, DialKeepAlive=%v, MaxIdleConns=%d, ClientTimeout=%v",
		cfg.DialTimeout, cfg.DialKeepAlive, cfg.MaxIdleConns, cfg.ClientTimeout)

	httpClient := httpclient.NewClient(httpClientOptions)

	// Initialize services using the new clients package
	contextService := clients.NewContextService(cfg.ContextServiceURL, httpClient)
	remediationsService := clients.NewRemediationsService(cfg.RemediationsServiceURL, httpClient)
	orthosService := clients.NewOrthosService(cfg.OrthosServiceURL, httpClient)
	workServerService := clients.NewWorkServerService(cfg.WorkServerURL, httpClient)

	// Register routes
	router.POST("/ingest", func(c *gin.Context) {
		c.Set("config", cfg) // Store config in context for logging
		ctxWithConfig := context.WithValue(c.Request.Context(), configKey, cfg)
		c.Request = c.Request.WithContext(ctxWithConfig)
		ginHandleTextInput(c, contextService, remediationsService, orthosService, workServerService)
	})

	// Set up health check
	healthCheck := health.New(health.Options{
		ServiceName: cfg.ServiceName,
		Version:     "1.0.0",
		Details: map[string]string{
			"description": "Ingests text data into the Crochet system",
		},
	})

	// Register health check handler in the gin router
	router.GET("/health", gin.WrapF(healthCheck.Handler()))

	// Start the server
	address := cfg.GetAddress()
	log.Printf("Server starting on %s...\n", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
