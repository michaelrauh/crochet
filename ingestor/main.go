package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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

// GenerateID creates a unique ID for an ortho
func GenerateID() string {
	// Generate 16 random bytes
	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Fall back to timestamp-based ID in case of error
		return fmt.Sprintf("id-%d", time.Now().UnixNano())
	}

	// Convert to hex string
	return hex.EncodeToString(randomBytes)
}

// createNewOrtho creates and returns a new Ortho object with default values
func createNewOrtho() types.Ortho {
	return types.Ortho{
		Grid:     make(map[string]string),
		Shape:    []int{2, 2},
		Position: []int{0, 0},
		Shell:    0,
		ID:       GenerateID(),
	}
}

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

	log.Println("Starting ingest processing...")

	corpus, err := types.ProcessIncomingCorpus(c, "ingestor")
	if telemetry.LogAndError(c, err, "ingestor", "Error processing incoming corpus") {
		return
	}
	fmt.Printf("Title: %s\nText length: %d characters\n", corpus.Title, len(corpus.Text))

	log.Println("Generating subphrases and vocabulary...")
	subphrases := text.GenerateSubphrases(corpus.Text)
	vocabulary := text.Vocabulary(corpus.Text)
	log.Printf("Generated %d subphrases and %d vocabulary items", len(subphrases), len(vocabulary))

	// Create input for context service
	contextInput := types.ContextInput{
		Title:      corpus.Title,
		Vocabulary: vocabulary,
		Subphrases: subphrases,
	}

	log.Println("Sending data to context service...")
	// Send to context service with request context to maintain trace
	contextResponse, err := contextService.SendMessage(c.Request.Context(), contextInput)
	if telemetry.LogAndError(c, err, "ingestor", "Error sending message to context service") {
		log.Printf("Context service error details: %v", err)
		return
	}
	log.Printf("Received response from context service with version: %s and %d new subphrases",
		contextResponse.Version, len(contextResponse.NewSubphrases))

	// Extract pairs for remediations
	pairs := types.ExtractPairsFromSubphrases(contextResponse.NewSubphrases)
	log.Printf("Extracted %d pairs from subphrases for remediation", len(pairs))

	remediationReq := types.RemediationRequest{
		Pairs: pairs,
	}

	// Log detailed request to remediation service
	pairsJSON, _ := json.Marshal(pairs)
	// Fix the type assertion to use a pointer type
	cfg := c.MustGet("config").(*config.IngestorConfig)
	log.Printf("Calling remediations service at URL: %s", cfg.RemediationsServiceURL)
	log.Printf("Remediation request contains %d pairs: %s", len(pairs), string(pairsJSON))

	// Send to remediations service with request context to maintain trace
	startTime := time.Now()
	remediationResp, err := remediationsService.FetchRemediations(c.Request.Context(), remediationReq)
	elapsed := time.Since(startTime)

	if err != nil {
		log.Printf("ERROR: Remediation service call failed after %v: %v", elapsed, err)
		telemetry.LogAndError(c, err, "ingestor", "Error fetching remediations")
		return
	}

	log.Printf("Successfully received response from remediations service in %v with %d hashes",
		elapsed, len(remediationResp.Hashes))

	// Store the hashes we received for later cleanup
	processedHashes := remediationResp.Hashes

	// If we have hashes from the remediation service, get corresponding orthos
	var orthosResp types.OrthosResponse
	if len(remediationResp.Hashes) > 0 {
		log.Printf("Fetching orthos for %d hashes...", len(remediationResp.Hashes))
		// Get orthos by hashes (which are ortho IDs)
		orthosResp, err = orthosService.GetOrthosByIDs(c.Request.Context(), remediationResp.Hashes)
		if telemetry.LogAndError(c, err, "ingestor", "Error fetching orthos") {
			log.Printf("Orthos service error details: %v", err)
			return
		}
		log.Printf("Retrieved %d orthos for %d hash IDs", orthosResp.Count, len(remediationResp.Hashes))
		// Push the retrieved orthos to work server
		if orthosResp.Count > 0 {
			log.Printf("Pushing %d orthos to work server...", orthosResp.Count)
			workServerResp, err := workServerService.PushOrthos(c.Request.Context(), orthosResp.Orthos)
			if telemetry.LogAndError(c, err, "ingestor", "Error pushing orthos to work server") {
				log.Printf("Work server error details: %v", err)
				return
			}
			log.Printf("Pushed %d orthos to work server, received %d IDs", workServerResp.Count, len(workServerResp.IDs))
		}
	} else {
		log.Println("No hashes returned from remediations service")
	}

	// Create a blank ortho as specified
	log.Println("Creating and pushing blank ortho to work server...")
	blankOrtho := types.Ortho{
		Grid:     make(map[string]string),
		Shape:    []int{2, 2},
		Position: []int{0, 0},
		Shell:    0,
		ID:       "",
	}
	// Push the blank ortho to work server
	blankOrthoArray := []types.Ortho{blankOrtho}
	blankWorkServerResp, err := workServerService.PushOrthos(c.Request.Context(), blankOrthoArray)
	if telemetry.LogAndError(c, err, "ingestor", "Error pushing blank ortho to work server") {
		log.Printf("Blank ortho push error details: %v", err)
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

	// Start the server
	address := cfg.GetAddress()
	log.Printf("Server starting on %s...\n", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
