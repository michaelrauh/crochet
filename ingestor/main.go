package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"crochet/clients"
	"crochet/config"
	"crochet/httpclient"
	"crochet/middleware"
	"crochet/telemetry"
	"crochet/text"
	"crochet/types"

	"github.com/gin-gonic/gin"
)

// ingestMutex ensures only one ingest operation runs at a time
var ingestMutex sync.Mutex

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// config key constant
const configKey contextKey = "config"

func ginHandleTextInput(c *gin.Context, contextService types.ContextService, remediationsService types.RemediationsService, orthosService types.OrthosService, workServerService types.WorkServerService) {
	// Try to acquire the mutex, return busy status if we can't
	if !ingestMutex.TryLock() {
		c.JSON(http.StatusLocked, gin.H{
			"status":  "error",
			"message": "Another ingest operation is in progress. Please try again later.",
		})
		return
	}
	// Ensure we release the mutex when done
	defer ingestMutex.Unlock()

	corpus, err := types.ProcessIncomingCorpus(c, "ingestor")
	if telemetry.LogAndError(c, err, "ingestor", "Error processing incoming corpus") {
		return
	}

	fmt.Printf("Title: %s\nText: %s\n", corpus.Title, corpus.Text)

	subphrases := text.GenerateSubphrases(corpus.Text)
	vocabulary := text.Vocabulary(corpus.Text)

	// Create input for context service
	contextInput := types.ContextInput{
		Title:      corpus.Title,
		Vocabulary: vocabulary,
		Subphrases: subphrases,
	}

	// Send to context service with request context to maintain trace
	contextResponse, err := contextService.SendMessage(c.Request.Context(), contextInput)
	if telemetry.LogAndError(c, err, "ingestor", "Error sending message to context service") {
		return
	}

	// Extract pairs for remediations
	remediationReq := types.RemediationRequest{
		Pairs: types.ExtractPairsFromSubphrases(contextResponse.NewSubphrases),
	}

	// Send to remediations service with request context to maintain trace
	remediationResp, err := remediationsService.FetchRemediations(c.Request.Context(), remediationReq)
	if telemetry.LogAndError(c, err, "ingestor", "Error fetching remediations") {
		return
	}

	// Store the hashes we received for later cleanup
	processedHashes := remediationResp.Hashes

	// If we have hashes from the remediation service, get corresponding orthos
	var orthosResp types.OrthosResponse
	if len(remediationResp.Hashes) > 0 {
		// Get orthos by hashes (which are ortho IDs)
		orthosResp, err = orthosService.GetOrthosByIDs(c.Request.Context(), remediationResp.Hashes)
		if telemetry.LogAndError(c, err, "ingestor", "Error fetching orthos") {
			return
		}
		log.Printf("Retrieved %d orthos for %d hash IDs", orthosResp.Count, len(remediationResp.Hashes))

		// Push the retrieved orthos to work server
		if orthosResp.Count > 0 {
			workServerResp, err := workServerService.PushOrthos(c.Request.Context(), orthosResp.Orthos)
			if telemetry.LogAndError(c, err, "ingestor", "Error pushing orthos to work server") {
				return
			}
			log.Printf("Pushed %d orthos to work server, received %d IDs", workServerResp.Count, len(workServerResp.IDs))
		}
	}

	// Create a blank ortho as specified
	blankOrtho := types.Ortho{
		Grid:     make(map[string]interface{}),
		Shape:    []int{2, 2},
		Position: []int{0, 0},
		Shell:    0,
		ID:       "",
	}

	// Push the blank ortho to work server
	blankOrthoArray := []types.Ortho{blankOrtho}
	blankWorkServerResp, err := workServerService.PushOrthos(c.Request.Context(), blankOrthoArray)
	if telemetry.LogAndError(c, err, "ingestor", "Error pushing blank ortho to work server") {
		return
	}
	log.Printf("Pushed blank ortho to work server, received %d IDs", len(blankWorkServerResp.IDs))

	// Clean up processed remediations - only delete the ones we've handled
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
	httpClient := httpclient.NewClient(httpClientOptions)

	// Initialize services using the new clients package
	contextService := clients.NewContextService(cfg.ContextServiceURL, httpClient)
	remediationsService := clients.NewRemediationsService(cfg.RemediationsServiceURL, httpClient)
	orthosService := clients.NewOrthosService(cfg.OrthosServiceURL, httpClient)
	workServerService := clients.NewWorkServerService(cfg.WorkServerURL, httpClient)

	// Register routes
	router.POST("/ingest", func(c *gin.Context) {
		ctxWithConfig := context.WithValue(c.Request.Context(), configKey, cfg)
		c.Request = c.Request.WithContext(ctxWithConfig)
		ginHandleTextInput(c, contextService, remediationsService, orthosService, workServerService)
	})

	// Start the server
	address := cfg.GetAddress()
	log.Printf("Server starting on %s...\n", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
