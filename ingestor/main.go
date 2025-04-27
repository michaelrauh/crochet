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

	httpClient := httpclient.NewDefaultClient()

	contextService := clients.NewContextService(cfg.ContextServiceURL, httpClient)
	remediationsService := clients.NewRemediationsService(cfg.RemediationsServiceURL, httpClient)
	orthosService := clients.NewOrthosService(cfg.OrthosServiceURL, httpClient)
	workServerService := clients.NewWorkServerService(cfg.WorkServerURL, httpClient)

	router.POST("/ingest", func(c *gin.Context) {
		c.Set("config", cfg)
		ctxWithConfig := context.WithValue(c.Request.Context(), configKey, cfg)
		c.Request = c.Request.WithContext(ctxWithConfig)
		ginHandleTextInput(c, contextService, remediationsService, orthosService, workServerService)
	})

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
