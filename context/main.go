package main

import (
	"log"
	"net/http"
	"strings"
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

type Input struct {
	Vocabulary []string   `json:"vocabulary"`
	Subphrases [][]string `json:"subphrases"`
}

type MemoryStore struct {
	Vocabulary map[string]struct{}
	Subphrases map[string]struct{}
}

var store MemoryStore
var versionCounter int

// NewContextError creates a new error specific to the context service
func NewContextError(code int, message string) *telemetry.ServiceError {
	return telemetry.NewServiceError("context", code, message)
}

func initStore() {
	store = MemoryStore{
		Vocabulary: make(map[string]struct{}),
		Subphrases: make(map[string]struct{}),
	}
	versionCounter = 1
	log.Println("In-memory store initialized successfully")
}

func saveVocabularyToStore(vocabulary []string) ([]string, error) {
	var newlyAdded []string
	for _, word := range vocabulary {
		if _, exists := store.Vocabulary[word]; !exists {
			store.Vocabulary[word] = struct{}{}
			newlyAdded = append(newlyAdded, word)
		}
	}
	return newlyAdded, nil
}

func saveSubphrasesToStore(subphrases [][]string) ([][]string, error) {
	var newlyAdded [][]string
	for _, subphrase := range subphrases {
		joinedSubphrase := strings.Join(subphrase, " ")
		if _, exists := store.Subphrases[joinedSubphrase]; !exists {
			store.Subphrases[joinedSubphrase] = struct{}{}
			newlyAdded = append(newlyAdded, subphrase)
		}
	}
	return newlyAdded, nil
}

// Convert standard HTTP handlers to Gin handlers
func ginHandleInput(c *gin.Context) {
	// Get the tracer from the context
	tracer := otel.Tracer("context-service")
	ctx, span := tracer.Start(c.Request.Context(), "handleInput")
	defer span.End()

	// Create a new context with the span
	c.Request = c.Request.WithContext(ctx)

	log.Printf("Received request: %v", c.Request)

	var input Input
	if err := c.ShouldBindJSON(&input); err != nil {
		telemetry.LogAndError(c, err, "context", "Invalid JSON format")
		span.SetStatus(codes.Error, "Invalid JSON format")
		span.RecordError(err)
		return
	}

	// Create a child span for processing vocabulary
	ctx, vocabSpan := tracer.Start(ctx, "processVocabulary")
	vocabSpan.SetAttributes(attribute.Int("vocabulary_count", len(input.Vocabulary)))
	newVocabulary, err := saveVocabularyToStore(input.Vocabulary)
	if err != nil {
		telemetry.LogAndError(c, err, "context", "Failed to save vocabulary")
		vocabSpan.SetStatus(codes.Error, "Failed to save vocabulary")
		vocabSpan.RecordError(err)
		vocabSpan.End()
		span.SetStatus(codes.Error, "Failed to save vocabulary")
		return
	}
	vocabSpan.SetAttributes(attribute.Int("new_vocabulary_count", len(newVocabulary)))
	vocabSpan.End()

	// Create a child span for processing subphrases
	ctx, subphraseSpan := tracer.Start(ctx, "processSubphrases")
	subphraseSpan.SetAttributes(attribute.Int("subphrases_count", len(input.Subphrases)))
	newSubphrases, err := saveSubphrasesToStore(input.Subphrases)
	if err != nil {
		telemetry.LogAndError(c, err, "context", "Failed to save subphrases")
		subphraseSpan.SetStatus(codes.Error, "Failed to save subphrases")
		subphraseSpan.RecordError(err)
		subphraseSpan.End()
		span.SetStatus(codes.Error, "Failed to save subphrases")
		return
	}
	subphraseSpan.SetAttributes(attribute.Int("new_subphrases_count", len(newSubphrases)))
	subphraseSpan.End()

	response := gin.H{
		"newVocabulary": newVocabulary,
		"newSubphrases": newSubphrases,
		"version":       versionCounter,
	}

	span.AddEvent("response_prepared", trace.WithAttributes(
		attribute.Bool("success", true),
		attribute.Int("version", versionCounter),
	))
	versionCounter++

	log.Printf("Sending response to ingestor: %v", response)
	log.Println("Flushing logs to ensure visibility")

	c.JSON(http.StatusOK, response)
}

func ginHandleHealth(c *gin.Context) {
	// Get the tracer from the context
	tracer := otel.Tracer("context-service")
	ctx, span := tracer.Start(c.Request.Context(), "healthCheck")
	defer span.End()

	// Create a new context with the span
	c.Request = c.Request.WithContext(ctx)

	log.Println("Health check endpoint called")
	c.String(http.StatusOK, "OK")
}

func main() {
	log.Println("Starting context service...")

	// Load configuration using the unified config package
	cfg, err := config.LoadContextConfig()
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

	initStore()

	// Register Gin routes
	router.POST("/input", ginHandleInput)
	router.GET("/health", ginHandleHealth)

	address := cfg.GetAddress()
	log.Printf("Context service starting on %s...\n", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Context service failed to start: %v", err)
	}
}
