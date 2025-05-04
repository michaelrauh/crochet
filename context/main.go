package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"crochet/config"
	"crochet/health"
	"crochet/middleware"
	"crochet/telemetry"
	"crochet/types"

	"github.com/gin-gonic/gin"
)

var ctxStore types.ContextStore
var versionCounter int

func initStore(cfg config.Context) error {
	versionCounter = 1

	var err error
	connURL := cfg.LibSQLEndpoint
	ctxStore, err = types.NewLibSQLContextStore(connURL)
	if err != nil {
		return fmt.Errorf("failed to initialize SQLite store: %w", err)
	}

	return nil
}

func ginHandleInput(c *gin.Context) {
	var input types.ContextInput
	if err := c.ShouldBindJSON(&input); err != nil {
		telemetry.LogAndError(c, err, "context", "Invalid JSON format")
		return
	}

	newVocabulary := ctxStore.SaveVocabulary(input.Vocabulary)
	newSubphrases := ctxStore.SaveSubphrases(input.Subphrases)

	response := gin.H{
		"newVocabulary": newVocabulary,
		"newSubphrases": newSubphrases,
		"version":       versionCounter,
	}

	versionCounter++
	c.JSON(http.StatusOK, response)
}

func ginGetVersion(c *gin.Context) {
	response := gin.H{
		"version": versionCounter,
	}
	c.JSON(http.StatusOK, response)
}

func ginGetContext(c *gin.Context) {
	vocabularySlice := ctxStore.GetVocabulary()
	linesSlice := ctxStore.GetSubphrases()

	response := gin.H{
		"version":    versionCounter,
		"vocabulary": vocabularySlice,
		"lines":      linesSlice,
	}

	c.JSON(http.StatusOK, response)
}

func ginUpdateVersion(c *gin.Context) {
	var updateRequest types.VersionUpdateRequest
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		telemetry.LogAndError(c, err, "context", "Invalid JSON format for version update")
		return
	}

	// Update the version in the database
	if err := ctxStore.SetVersion(updateRequest.Version); err != nil {
		telemetry.LogAndError(c, err, "context", "Failed to update version in database")
		return
	}

	// Update the in-memory counter to match
	versionCounter = updateRequest.Version

	response := types.VersionUpdateResponse{
		Status:  "success",
		Message: fmt.Sprintf("Version updated successfully to %d", updateRequest.Version),
		Version: updateRequest.Version,
	}

	log.Printf("Version updated to %d", updateRequest.Version)
	c.JSON(http.StatusOK, response)
}

func main() {
	config := config.GetContext()

	router, tp, mp, pp, err := middleware.SetupCommonComponents(
		config.ServiceName,
		config.JaegerEndpoint,
		config.MetricsEndpoint,
		config.PyroscopeEndpoint,
	)

	if err != nil {
		log.Fatalf("Failed to set up application: %v", err)
	}

	defer tp.ShutdownWithTimeout(5 * time.Second)
	defer mp.ShutdownWithTimeout(5 * time.Second)
	defer pp.StopWithTimeout(5 * time.Second)

	if err := initStore(config); err != nil {
		log.Fatalf("Failed to initialize context store: %v", err)
	}

	defer func() {
		if err := ctxStore.Close(); err != nil {
			log.Printf("Error closing context store: %v", err)
		}
	}()

	router.POST("/input", ginHandleInput)
	router.GET("/version", ginGetVersion)
	router.GET("/context", ginGetContext)
	router.POST("/update-version", ginUpdateVersion)

	// Set up health check
	healthCheck := health.New(health.Options{
		ServiceName: config.ServiceName,
		Version:     "1.0.0",
		Details: map[string]string{
			"description": "Manages context data for the Crochet system",
		},
	})

	router.GET("/health", gin.WrapF(healthCheck.Handler()))

	address := config.GetContextAddr()

	if err := router.Run(address); err != nil {
		log.Fatalf("Context service failed to start: %v", err)
	}
}
