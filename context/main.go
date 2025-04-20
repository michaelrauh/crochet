package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"crochet/telemetry"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
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

func initStore() {
	store = MemoryStore{
		Vocabulary: make(map[string]struct{}),
		Subphrases: make(map[string]struct{}),
	}
	versionCounter = 1
	fmt.Println("In-memory store initialized successfully")
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

func handleInput(w http.ResponseWriter, r *http.Request) {
	// Extract trace context from incoming request
	ctx := r.Context()
	propagator := otel.GetTextMapPropagator()
	ctx = propagator.Extract(ctx, propagation.HeaderCarrier(r.Header))

	// Create a span for this handler
	tracer := otel.Tracer("context-service")
	ctx, span := tracer.Start(ctx, "handleInput")
	defer span.End()

	log.Printf("Received request: %v", r)

	if r.Method != http.MethodPost {
		telemetry.WriteJSONError(w, "context", http.StatusMethodNotAllowed, "Method not allowed")
		span.SetStatus(codes.Error, "Method not allowed")
		span.SetAttributes(attribute.Int("http.status_code", http.StatusMethodNotAllowed))
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		telemetry.WriteJSONError(w, "context", http.StatusInternalServerError, "Error reading request body")
		span.SetStatus(codes.Error, "Error reading request body")
		span.SetAttributes(attribute.Int("http.status_code", http.StatusInternalServerError))
		span.RecordError(err)
		return
	}
	defer r.Body.Close()

	var input Input
	if err := json.Unmarshal(body, &input); err != nil {
		telemetry.WriteJSONError(w, "context", http.StatusBadRequest, "Invalid JSON format")
		span.SetStatus(codes.Error, "Invalid JSON format")
		span.SetAttributes(attribute.Int("http.status_code", http.StatusBadRequest))
		span.RecordError(err)
		return
	}

	// Create a child span for processing vocabulary
	ctx, vocabSpan := tracer.Start(ctx, "processVocabulary")
	vocabSpan.SetAttributes(attribute.Int("vocabulary_count", len(input.Vocabulary)))
	newVocabulary, err := saveVocabularyToStore(input.Vocabulary)
	if err != nil {
		telemetry.WriteJSONError(w, "context", http.StatusInternalServerError, fmt.Sprintf("Failed to save vocabulary: %v", err))
		vocabSpan.SetStatus(codes.Error, "Failed to save vocabulary")
		vocabSpan.RecordError(err)
		vocabSpan.End()
		span.SetStatus(codes.Error, "Failed to save vocabulary")
		span.SetAttributes(attribute.Int("http.status_code", http.StatusInternalServerError))
		return
	}
	vocabSpan.SetAttributes(attribute.Int("new_vocabulary_count", len(newVocabulary)))
	vocabSpan.End()

	// Create a child span for processing subphrases
	ctx, subphraseSpan := tracer.Start(ctx, "processSubphrases")
	subphraseSpan.SetAttributes(attribute.Int("subphrases_count", len(input.Subphrases)))
	newSubphrases, err := saveSubphrasesToStore(input.Subphrases)
	if err != nil {
		telemetry.WriteJSONError(w, "context", http.StatusInternalServerError, fmt.Sprintf("Failed to save subphrases: %v", err))
		subphraseSpan.SetStatus(codes.Error, "Failed to save subphrases")
		subphraseSpan.RecordError(err)
		subphraseSpan.End()
		span.SetStatus(codes.Error, "Failed to save subphrases")
		span.SetAttributes(attribute.Int("http.status_code", http.StatusInternalServerError))
		return
	}
	subphraseSpan.SetAttributes(attribute.Int("new_subphrases_count", len(newSubphrases)))
	subphraseSpan.End()

	response := map[string]interface{}{
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	span.SetAttributes(attribute.Int("http.status_code", http.StatusOK))
	json.NewEncoder(w).Encode(response)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	// Extract trace context from incoming request
	ctx := r.Context()
	propagator := otel.GetTextMapPropagator()
	ctx = propagator.Extract(ctx, propagation.HeaderCarrier(r.Header))

	// Create a span for the health check
	tracer := otel.Tracer("context-service")
	ctx, span := tracer.Start(ctx, "healthCheck")
	defer span.End()

	log.Println("Health check endpoint called")
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	span.SetAttributes(attribute.Int("http.status_code", http.StatusOK))

	_, err := w.Write([]byte("OK"))
	if err != nil {
		log.Printf("Error writing health check response: %v", err)
		span.SetStatus(codes.Error, "Error writing response")
		span.RecordError(err)
	}
}

func main() {
	log.Println("Starting context service...")

	port := os.Getenv("CONTEXT_PORT")
	if port == "" {
		panic("CONTEXT_PORT environment variable is not set")
	}

	host := os.Getenv("CONTEXT_HOST")
	if host == "" {
		panic("CONTEXT_HOST environment variable is not set")
	}

	jaegerEndpoint := os.Getenv("JAEGER_ENDPOINT")
	if jaegerEndpoint == "" {
		panic("JAEGER_ENDPOINT environment variable must be set")
	}

	// Initialize OpenTelemetry with the shared telemetry package
	tp, err := telemetry.InitTracer("context-service", jaegerEndpoint)
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}
	defer tp.ShutdownWithTimeout(5 * time.Second)

	initStore()

	// Register HTTP handlers
	http.HandleFunc("/input", handleInput)
	http.HandleFunc("/health", handleHealth)

	log.Printf("Context service starting on %s:%s...\n", host, port)
	if err := http.ListenAndServe(host+":"+port, nil); err != nil {
		log.Fatalf("Context service failed to start: %v", err)
	}

	log.Println("Context service is ready to accept requests")
}
