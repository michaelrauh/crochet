package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"crochet/config"
	"crochet/httpclient"
	"crochet/middleware"
	"crochet/telemetry"
	"crochet/text"
	"crochet/types"

	"github.com/gin-gonic/gin"
)

func ginHandleTextInput(c *gin.Context, contextService types.ContextService, remediationsService types.RemediationsService) {
	corpus, err := types.ProcessIncomingCorpus(c, "ingestor")
	if telemetry.LogAndError(c, err, "ingestor", "Error processing incoming corpus") {
		return
	}

	fmt.Printf("Title: %s\nText: %s\n", corpus.Title, corpus.Text)

	subphrases := text.GenerateSubphrases(corpus.Text)
	vocabulary := text.Vocabulary(corpus.Text)

	contextInputJSON, err := types.PrepareContextServiceInput(corpus, vocabulary, subphrases)
	if telemetry.LogAndError(c, err, "ingestor", "Error preparing data for context service") {
		return
	}

	ctx := c.Request.Context()
	contextResponseRaw, err := contextService.SendMessage(ctx, string(contextInputJSON))
	if telemetry.LogAndError(c, err, "ingestor", "Error sending message to context service") {
		return
	}

	contextResponse, err := types.ProcessContextResponse(contextResponseRaw)
	if telemetry.LogAndError(c, err, "ingestor", "Invalid response from context service") {
		return
	}

	subphrasesForRemediations := types.ExtractPairsFromSubphrases(contextResponse.NewSubphrases)
	remediationsResponseRaw, err := remediationsService.FetchRemediations(ctx, subphrasesForRemediations)
	if telemetry.LogAndError(c, err, "ingestor", "Error fetching remediations") {
		return
	}

	_, err = types.ProcessRemediationsResponse(remediationsResponseRaw)
	if telemetry.LogAndError(c, err, "ingestor", "Error processing remediations response") {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"version": contextResponse.Version,
	})
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

	// Set up common components using the shared helper
	router, tp, err := middleware.SetupCommonComponents(cfg.ServiceName, cfg.JaegerEndpoint)
	if err != nil {
		log.Fatalf("Failed to set up application: %v", err)
	}
	defer tp.ShutdownWithTimeout(5 * time.Second)

	// Set up HTTP client (specific to ingestor service)
	httpClientOptions := httpclient.ClientOptions{
		DialTimeout:   cfg.DialTimeout,
		DialKeepAlive: cfg.DialKeepAlive,
		MaxIdleConns:  cfg.MaxIdleConns,
		ClientTimeout: cfg.ClientTimeout,
	}
	httpClient := httpclient.NewClient(httpClientOptions)

	// Initialize services (specific to ingestor service)
	contextService := &types.RealContextService{
		URL:    cfg.ContextServiceURL,
		Client: httpClient,
	}
	remediationsService := &types.RealRemediationsService{
		URL:    cfg.RemediationsServiceURL,
		Client: httpClient,
	}

	// Register routes
	router.POST("/ingest", func(c *gin.Context) {
		ctxWithConfig := context.WithValue(c.Request.Context(), "config", cfg)
		c.Request = c.Request.WithContext(ctxWithConfig)
		ginHandleTextInput(c, contextService, remediationsService)
	})

	// Start the server
	address := cfg.GetAddress()
	log.Printf("Server starting on %s...\n", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
