package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"crochet/config"
	"crochet/middleware"
	"crochet/telemetry"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type RemediationRequest struct {
	Pairs [][]string `json:"pairs"`
}

// NewRemediationsError creates a new error specific to the remediations service
func NewRemediationsError(code int, message string) *telemetry.ServiceError {
	return telemetry.NewServiceError("remediations", code, message)
}

// Convert standard HTTP handlers to Gin handlers
func ginOkHandler(c *gin.Context) {
	// Get the tracer from the context
	tracer := otel.Tracer("remediations-service")
	ctx, span := tracer.Start(c.Request.Context(), "okHandler")
	defer span.End()

	// Create a new context with the span
	c.Request = c.Request.WithContext(ctx)

	// Set response
	c.JSON(http.StatusOK, gin.H{"status": "OK"})
}

func ginRemediateHandler(c *gin.Context) {
	// Get the tracer from the context
	tracer := otel.Tracer("remediations-service")
	ctx, span := tracer.Start(c.Request.Context(), "remediateHandler")
	defer span.End()

	// Create a new context with the span
	c.Request = c.Request.WithContext(ctx)

	var request RemediationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		telemetry.LogAndError(c, err, "remediations", "Invalid JSON format")
		span.SetStatus(codes.Error, "Invalid JSON format")
		span.RecordError(err)
		return
	}

	pairs_count := len(request.Pairs)
	log.Printf("Received %d pairs for remediation", pairs_count)
	span.SetAttributes(attribute.Int("pairs_count", pairs_count))

	// Log the pairs we received
	for i, pair := range request.Pairs {
		log.Printf("Pair %d: %v", i+1, pair)
		pairStr := fmt.Sprintf("%v", pair)
		span.AddEvent(fmt.Sprintf("pair.%d", i+1),
			trace.WithAttributes(attribute.String("value", pairStr)))
	}

	// Process pairs - create a child span for processing
	ctx, processSpan := tracer.Start(ctx, "processPairs")

	// Return a list of mock hashes as the response
	hashes := []string{
		"1234567890abcdef1234567890abcdef",
		"abcdef1234567890abcdef1234567890",
		"aabbccddeeff00112233445566778899",
		"99887766554433221100ffeeddccbbaa",
		"112233445566778899aabbccddeeff00",
		"00ffeeddccbbaa99887766554433221",
	}

	processSpan.SetAttributes(attribute.Int("hashes_count", len(hashes)))
	processSpan.End()

	c.JSON(http.StatusOK, gin.H{
		"status": "OK",
		"hashes": hashes,
	})
}

func main() {
	// Load configuration using the unified config package
	cfg, err := config.LoadRemediationsConfig()
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

	// Register Gin routes
	router.GET("/", ginOkHandler)
	router.POST("/remediate", ginRemediateHandler)

	address := cfg.GetAddress()
	log.Printf("Remediations service starting on %s...", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
