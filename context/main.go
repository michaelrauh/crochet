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

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
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

func initJaeger(service string) (opentracing.Tracer, io.Closer) {
	// Use a more direct endpoint configuration for Jaeger HTTP reporter
	cfg := &config.Configuration{
		ServiceName: service,
		Sampler: &config.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1, // Sample 100% of traces
		},
		Reporter: &config.ReporterConfig{
			LogSpans:            true,
			CollectorEndpoint:   "http://jaeger:14268/api/traces", // Direct HTTP endpoint
			BufferFlushInterval: 1 * time.Second,
		},
	}

	// Initialize the tracer with the configuration
	tracer, closer, err := cfg.NewTracer(config.Logger(jaeger.StdLogger))
	if err != nil {
		log.Fatalf("Cannot initialize Jaeger Tracer: %s", err.Error())
	}

	log.Printf("Jaeger tracer initialized for service: %s, reporting directly to collector endpoint", service)

	return tracer, closer
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
	spanCtx, _ := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(r.Header),
	)

	span := opentracing.GlobalTracer().StartSpan(
		"handleInput",
		ext.RPCServerOption(spanCtx),
	)
	defer span.Finish()

	span.SetTag("http.method", r.Method)
	span.SetTag("http.url", r.URL.String())

	log.Printf("Received request: %v", r)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		span.SetTag("error", true)
		span.SetTag("http.status_code", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		span.SetTag("error", true)
		span.SetTag("http.status_code", http.StatusInternalServerError)
		span.LogKV("event", "error", "message", err.Error())
		return
	}
	defer r.Body.Close()

	var input Input
	if err := json.Unmarshal(body, &input); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		span.SetTag("error", true)
		span.SetTag("http.status_code", http.StatusBadRequest)
		span.LogKV("event", "error", "message", err.Error())
		return
	}

	vocabSpan := opentracing.StartSpan(
		"processVocabulary",
		opentracing.ChildOf(span.Context()),
	)
	vocabSpan.SetTag("vocabulary_count", len(input.Vocabulary))

	newVocabulary, err := saveVocabularyToStore(input.Vocabulary)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save vocabulary: %v", err), http.StatusInternalServerError)
		vocabSpan.SetTag("error", true)
		vocabSpan.LogKV("event", "error", "message", err.Error())
		vocabSpan.Finish()
		span.SetTag("error", true)
		span.SetTag("http.status_code", http.StatusInternalServerError)
		return
	}
	vocabSpan.SetTag("new_vocabulary_count", len(newVocabulary))
	vocabSpan.Finish()

	subphraseSpan := opentracing.StartSpan(
		"processSubphrases",
		opentracing.ChildOf(span.Context()),
	)
	subphraseSpan.SetTag("subphrases_count", len(input.Subphrases))

	newSubphrases, err := saveSubphrasesToStore(input.Subphrases)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save subphrases: %v", err), http.StatusInternalServerError)
		subphraseSpan.SetTag("error", true)
		subphraseSpan.LogKV("event", "error", "message", err.Error())
		subphraseSpan.Finish()
		span.SetTag("error", true)
		span.SetTag("http.status_code", http.StatusInternalServerError)
		return
	}
	subphraseSpan.SetTag("new_subphrases_count", len(newSubphrases))
	subphraseSpan.Finish()

	response := map[string]interface{}{
		"newVocabulary": newVocabulary,
		"newSubphrases": newSubphrases,
		"version":       versionCounter,
	}

	span.LogKV("event", "response_prepared", "version", versionCounter)
	versionCounter++
	log.Printf("Sending response to ingestor: %v", response)
	log.Println("Flushing logs to ensure visibility")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	span.SetTag("http.status_code", http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	spanCtx, _ := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(r.Header),
	)

	span := opentracing.GlobalTracer().StartSpan(
		"healthCheck",
		ext.RPCServerOption(spanCtx),
	)
	defer span.Finish()

	log.Println("Health check endpoint called")
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	span.SetTag("http.status_code", http.StatusOK)

	_, err := w.Write([]byte("OK"))
	if err != nil {
		log.Printf("Error writing health check response: %v", err)
		span.SetTag("error", true)
		span.LogKV("event", "error", "message", err.Error())
	}
}

func main() {
	log.Println("Starting context service...")

	tracer, closer := initJaeger("context")
	defer closer.Close()
	opentracing.SetGlobalTracer(tracer)

	port := os.Getenv("CONTEXT_PORT")
	if port == "" {
		panic("CONTEXT_PORT environment variable is not set")
	}

	host := os.Getenv("CONTEXT_HOST")
	if host == "" {
		panic("CONTEXT_HOST environment variable is not set")
	}

	initStore()

	http.HandleFunc("/input", handleInput)
	http.HandleFunc("/health", handleHealth)

	log.Printf("Context service starting on %s:%s...\n", host, port)
	if err := http.ListenAndServe(host+":"+port, nil); err != nil {
		log.Fatalf("Context service failed to start: %v", err)
	}

	log.Println("Context service is ready to accept requests")
}
