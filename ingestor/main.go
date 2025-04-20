package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"crochet/text"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
)

type Corpus struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type ContextService interface {
	SendMessage(ctx context.Context, message string) (map[string]interface{}, error)
}

type RealContextService struct {
	URL string
}

func (s *RealContextService) SendMessage(ctx context.Context, message string) (map[string]interface{}, error) {
	// Create a new request with the current context
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL+"/input", bytes.NewBuffer([]byte(message)))
	if err != nil {
		return nil, fmt.Errorf("error creating request to context service: %w", err)
	}

	// Get the current span and inject its context into the HTTP headers
	span := opentracing.SpanFromContext(ctx)
	if span != nil {
		// Inject the span context into the HTTP request headers
		err = opentracing.GlobalTracer().Inject(
			span.Context(),
			opentracing.HTTPHeaders,
			opentracing.HTTPHeadersCarrier(req.Header),
		)
		if err != nil {
			log.Printf("Error injecting span context: %v", err)
		}
	}

	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling context service: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("Response from context service: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("context service error: %s, Status Code: %d", string(body), resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		log.Printf("Error parsing context service response: %v", err)
		return nil, fmt.Errorf("invalid response from context service")
	}

	log.Printf("Parsed response: %v", response)
	return response, nil
}

type RemediationsService interface {
	FetchRemediations(ctx context.Context, subphrases [][]string) (map[string]interface{}, error)
}

type RealRemediationsService struct {
	URL string
}

func (s *RealRemediationsService) FetchRemediations(ctx context.Context, subphrases [][]string) (map[string]interface{}, error) {
	// Filter subphrases to only include pairs
	var pairs [][]string
	for _, subphrase := range subphrases {
		if len(subphrase) == 2 {
			pairs = append(pairs, subphrase)
		}
	}

	// Prepare request payload
	requestData := map[string]interface{}{
		"pairs": pairs,
	}

	requestJSON, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("error marshaling remediations request: %w", err)
	}

	// Create a new request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL+"/remediate", bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, fmt.Errorf("error creating request to remediations service: %w", err)
	}

	// Get the current span and inject its context into the HTTP headers
	span := opentracing.SpanFromContext(ctx)
	if span != nil {
		// Inject the span context into the HTTP request headers
		err = opentracing.GlobalTracer().Inject(
			span.Context(),
			opentracing.HTTPHeaders,
			opentracing.HTTPHeadersCarrier(req.Header),
		)
		if err != nil {
			log.Printf("Error injecting span context: %v", err)
		}
	}

	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling remediations service: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("Response from remediations service: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remediations service error: %s, Status Code: %d", string(body), resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		log.Printf("Error parsing remediations service response: %v", err)
		return nil, fmt.Errorf("invalid response from remediations service")
	}

	return response, nil
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

func handleTextInput(w http.ResponseWriter, r *http.Request, contextService ContextService, remediationsService RemediationsService) {
	// Extract the tracing context from the incoming request
	spanCtx, _ := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(r.Header),
	)

	// Create a new span for this request
	span := opentracing.GlobalTracer().StartSpan(
		"handleTextInput",
		ext.RPCServerOption(spanCtx),
	)
	defer span.Finish()

	// Set operation-specific tags
	span.SetTag("http.method", r.Method)
	span.SetTag("http.url", r.URL.String())

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

	var corpus Corpus
	if err := json.Unmarshal(body, &corpus); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		span.SetTag("error", true)
		span.SetTag("http.status_code", http.StatusBadRequest)
		span.LogKV("event", "error", "message", err.Error())
		return
	}

	span.LogKV("event", "received corpus", "title", corpus.Title)
	fmt.Printf("Title: %s\nText: %s\n", corpus.Title, corpus.Text)

	// Create child span for processing
	processSpan := opentracing.StartSpan(
		"processText",
		opentracing.ChildOf(span.Context()),
	)

	// Process the text using methods from crochet/text
	subphrases := text.GenerateSubphrases(corpus.Text) // Generate subphrases
	vocabulary := text.Vocabulary(corpus.Text)         // Generate vocabulary

	processSpan.LogKV(
		"event", "processed text",
		"subphrases_count", len(subphrases),
		"vocabulary_size", len(vocabulary),
	)
	processSpan.Finish()

	// Prepare the data to send to the context service
	contextInput := map[string]interface{}{
		"title":      corpus.Title,
		"vocabulary": vocabulary,
		"subphrases": subphrases,
	}

	contextInputJSON, err := json.Marshal(contextInput)
	if err != nil {
		log.Printf("Error preparing data for context service: %v", err)
		http.Error(w, "Error preparing data for context service", http.StatusInternalServerError)
		span.SetTag("error", true)
		span.SetTag("http.status_code", http.StatusInternalServerError)
		span.LogKV("event", "error", "message", err.Error())
		return
	}

	// Create child span for context service call
	contextSpan := opentracing.StartSpan(
		"callContextService",
		opentracing.ChildOf(span.Context()),
	)
	contextSpan.SetTag("service", "context")

	// Create a new context containing the span
	ctx := opentracing.ContextWithSpan(context.Background(), contextSpan)

	// Forward the processed data to the context service
	contextResponse, err := contextService.SendMessage(ctx, string(contextInputJSON))

	if err != nil {
		log.Printf("Error sending message to context service: %v", err)
		http.Error(w, "Error calling context service", http.StatusInternalServerError)
		contextSpan.SetTag("error", true)
		contextSpan.LogKV("event", "error", "message", err.Error())
		contextSpan.Finish()
		span.SetTag("error", true)
		span.SetTag("http.status_code", http.StatusInternalServerError)
		return
	}

	contextSpan.LogKV("event", "context service response received")
	contextSpan.Finish()

	// Extract newSubphrases from the context service response
	newSubphrases, ok := contextResponse["newSubphrases"]
	if !ok || newSubphrases == nil {
		log.Printf("No new subphrases returned from context service or field is nil")
		// Initialize empty array and continue instead of returning an error
		newSubphrases = []interface{}{}
	}

	// Initialize subphrasesForRemediations outside of conditional blocks
	var subphrasesForRemediations [][]string

	// Convert newSubphrases to the expected format for remediations service
	if subphrasesArray, ok := newSubphrases.([]interface{}); ok {
		for _, subphrase := range subphrasesArray {
			if subphraseArray, ok := subphrase.([]interface{}); ok {
				var stringArray []string
				for _, word := range subphraseArray {
					if str, ok := word.(string); ok {
						stringArray = append(stringArray, str)
					}
				}
				subphrasesForRemediations = append(subphrasesForRemediations, stringArray)
			}
		}
	} else {
		log.Printf("newSubphrases has unexpected type: %T", newSubphrases)
	}

	// Create child span for remediations service call
	remediationsSpan := opentracing.StartSpan(
		"callRemediationsService",
		opentracing.ChildOf(span.Context()),
	)
	remediationsSpan.SetTag("service", "remediations")
	remediationsSpan.LogKV("event", "calling remediations service", "pairs_count", len(subphrasesForRemediations))

	// Create a new context containing the span
	ctx = opentracing.ContextWithSpan(context.Background(), remediationsSpan)

	// Call remediations service with the filtered subphrases
	remediationsResponse, err := remediationsService.FetchRemediations(ctx, subphrasesForRemediations)
	if err != nil {
		log.Printf("Error fetching remediations: %v", err)
		remediationsSpan.SetTag("error", true)
		remediationsSpan.LogKV("event", "error", "message", err)
		// Continue without remediations for now
	} else {
		log.Printf("Remediations response: %v", remediationsResponse)
		remediationsSpan.LogKV("event", "remediations received")
		// For now, we'll just log the response, not returning it to the client
	}
	remediationsSpan.Finish()

	// Respond to the client with the version from the context service
	response := map[string]interface{}{
		"status":  "success",
		"version": contextResponse["version"],
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	span.SetTag("http.status_code", http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func main() {
	// Initialize Jaeger tracer
	tracer, closer := initJaeger("ingestor")
	defer closer.Close()
	opentracing.SetGlobalTracer(tracer)

	port := os.Getenv("INGESTOR_PORT")
	if port == "" {
		panic("INGESTOR_PORT environment variable is not set")
	}

	contextServiceURL := os.Getenv("CONTEXT_SERVICE_URL")
	if contextServiceURL == "" {
		log.Fatal("CONTEXT_SERVICE_URL environment variable is not set")
	}
	contextService := &RealContextService{URL: contextServiceURL}

	remediationsServiceURL := os.Getenv("REMEDIATIONS_SERVICE_URL")
	if remediationsServiceURL == "" {
		log.Fatal("REMEDIATIONS_SERVICE_URL environment variable is not set")
	}
	remediationsService := &RealRemediationsService{URL: remediationsServiceURL}

	http.HandleFunc("/ingest", func(w http.ResponseWriter, r *http.Request) {
		handleTextInput(w, r, contextService, remediationsService)
	})

	log.Printf("Server starting on port %s...\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
