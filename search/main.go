package main

import (
	"context"
	"crochet/clients"
	"crochet/config"
	"crochet/health"
	"crochet/httpclient"
	"crochet/middleware"
	"crochet/types"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Global state for search processing
var searchState *State

// State represents the current state of the search process
type State struct {
	Version    int                 // Current version
	Vocabulary []string            // Available vocabulary
	Pairs      map[string]struct{} // Tracked pairs
}

// Global metrics
var processingTimeByShape metric.Float64Histogram
var searchSuccessByShape metric.Float64Counter
var itemsFoundByShape metric.Float64Counter
var searchesByShape metric.Int64Counter

// New metrics that include position
var itemsFoundByShapePosition metric.Float64Counter
var searchesByShapePosition metric.Int64Counter
var searchSuccessByShapePosition metric.Float64Counter

// Add Prometheus metrics to complement OpenTelemetry metrics
var (
	prometheusSearchSuccessByShapePosition = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "crochet_ortho_search_success_by_shape_position_total",
			Help: "Number of searches that found at least one item by input ortho shape and position",
		},
		[]string{"shape", "position"},
	)

	prometheusSearchesByShapePosition = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "crochet_ortho_searches_by_shape_position_total",
			Help: "Total number of searches processed by input ortho shape and position",
		},
		[]string{"shape", "position"},
	)
)

func init() {
	// Register prometheus metrics
	prometheus.MustRegister(prometheusSearchSuccessByShapePosition)
	prometheus.MustRegister(prometheusSearchesByShapePosition)
}

// ProcessWorkItem handles a single work item according to the pattern in work.md
func ProcessWorkItem(
	ctx context.Context,
	contextService types.ContextService,
	orthosService types.OrthosService,
	remediationsService types.RemediationsService,
	workServerService types.WorkServerService,
	ortho types.Ortho,
	itemID string,
) {
	// Create a span for the entire work item processing
	tracer := otel.Tracer("search-worker")
	ctx, span := tracer.Start(ctx, "processWorkItem")
	defer span.End()

	// Record the start time for processing duration metric
	startTime := time.Now()

	// Add work item details to the span
	span.SetAttributes(
		attribute.String("work_item.id", itemID),
		attribute.String("ortho.id", ortho.ID),
	)

	log.Printf("Processing work item with ID: %s", itemID)

	// Use the string representation of the shape array as the shape label for metrics, to match other services
	shapeStr := fmt.Sprintf("%v", ortho.Shape)
	// Create position string for position-based metrics
	positionStr := fmt.Sprintf("%v", ortho.Position)

	// Record that we processed a search for this shape
	searchesByShape.Add(ctx, 1, metric.WithAttributes(
		attribute.String("shape", shapeStr),
	))
	// Also record with position information
	searchesByShapePosition.Add(ctx, 1, metric.WithAttributes(
		attribute.String("shape", shapeStr),
		attribute.String("position", positionStr),
	))
	// Add it to Prometheus as well
	prometheusSearchesByShapePosition.WithLabelValues(shapeStr, positionStr).Inc()

	// Get forbidden and required values directly - no conversion needed
	forbidden, required := GetRequirements(ortho)

	// Convert forbidden to a map for filtering
	forbiddenMap := make(map[string]struct{})
	for _, word := range forbidden {
		forbiddenMap[word] = struct{}{}
	}

	// Filter the vocabulary to exclude forbidden words
	workingVocabulary := FilterVocabulary(searchState.Vocabulary, forbiddenMap)

	// Generate candidates and remediations
	candidates, remediations := GenerateCandidatesAndRemediations(workingVocabulary, required, searchState.Pairs, ortho)

	// Generate new orthos from candidates
	newOrthos := GenerateNewOrthos(candidates, ortho)

	// Add a span for saving orthos
	ctx, saveSpan := tracer.Start(ctx, "save_orthos")
	saveSpan.SetAttributes(attribute.Int("orthos_count", len(newOrthos)))

	// Record metrics for whether items were found and how many
	if len(newOrthos) > 0 {
		// Record that the search was successful (found at least one item)
		searchSuccessByShape.Add(ctx, 1.0, metric.WithAttributes(
			attribute.String("shape", shapeStr),
		))
		// Also record with position information
		searchSuccessByShapePosition.Add(ctx, 1.0, metric.WithAttributes(
			attribute.String("shape", shapeStr),
			attribute.String("position", positionStr),
		))
		// Add it to Prometheus as well
		prometheusSearchSuccessByShapePosition.WithLabelValues(shapeStr, positionStr).Inc()

		// Record the total number of items found for this search
		itemsFoundByShape.Add(ctx, float64(len(newOrthos)), metric.WithAttributes(
			attribute.String("shape", shapeStr),
		))
		// Also record with position information
		itemsFoundByShapePosition.Add(ctx, float64(len(newOrthos)), metric.WithAttributes(
			attribute.String("shape", shapeStr),
			attribute.String("position", positionStr),
		))
	} else {
		// Record a search with zero items found (still counts as a search)
		itemsFoundByShape.Add(ctx, 0.0, metric.WithAttributes(
			attribute.String("shape", shapeStr),
		))
		// Also record with position information
		itemsFoundByShapePosition.Add(ctx, 0.0, metric.WithAttributes(
			attribute.String("shape", shapeStr),
			attribute.String("position", positionStr),
		))
	}

	// Step 1: Add new orthos to the database using SaveOrthos
	if len(newOrthos) > 0 {
		saveResponse, err := orthosService.SaveOrthos(ctx, newOrthos)
		if err != nil {
			saveSpan.RecordError(err)
			saveSpan.End()
			log.Printf("Error saving orthos: %v", err)
			SendNack(ctx, workServerService, itemID, "Error saving orthos")
			return
		}
		saveSpan.SetAttributes(attribute.Int("saved_count", saveResponse.Count))
		log.Printf("Saved %d orthos", saveResponse.Count)
	}
	saveSpan.End()

	// Step 2: Add remediations to the database
	ctx, addRemSpan := tracer.Start(ctx, "add_remediations")
	addRemSpan.SetAttributes(attribute.Int("remediations_count", len(remediations)))

	if len(remediations) > 0 {
		addResponse, err := remediationsService.AddRemediations(ctx, remediations)
		if err != nil {
			addRemSpan.RecordError(err)
			addRemSpan.End()
			log.Printf("Error adding remediations: %v", err)
			SendNack(ctx, workServerService, itemID, "Error adding remediations")
			return
		}
		addRemSpan.SetAttributes(attribute.Int("added_count", addResponse.Count))
		log.Printf("Added %d remediations", addResponse.Count)
	}
	addRemSpan.End()

	// Step 3: Push new orthos to the work server
	ctx, pushSpan := tracer.Start(ctx, "push_orthos")
	pushSpan.SetAttributes(attribute.Int("orthos_to_push", len(newOrthos)))

	if len(newOrthos) > 0 {
		pushResponse, err := workServerService.PushOrthos(ctx, newOrthos)
		if err != nil {
			pushSpan.RecordError(err)
			pushSpan.End()
			log.Printf("Error pushing orthos to work server: %v", err)
			SendNack(ctx, workServerService, itemID, "Error pushing orthos to work server")
			return
		}
		pushSpan.SetAttributes(attribute.Int("pushed_count", pushResponse.Count))
		log.Printf("Pushed %d orthos to work server", pushResponse.Count)
	}
	pushSpan.End()

	// Step 4: Check version again
	ctx, verSpan := tracer.Start(ctx, "check_version")
	versionResp, err := contextService.GetVersion(ctx)
	if err != nil {
		verSpan.RecordError(err)
		verSpan.End()
		log.Printf("Error checking context version: %v", err)
		SendNack(ctx, workServerService, itemID, "Error checking context version")
		return
	}
	verSpan.SetAttributes(
		attribute.Int("current_version", searchState.Version),
		attribute.Int("service_version", versionResp.Version),
	)
	verSpan.End()

	// Step 5: If version has changed, update context and NACK, otherwise ACK
	if versionResp.Version != searchState.Version {
		ctx, updateSpan := tracer.Start(ctx, "update_context")
		updateSpan.SetAttributes(
			attribute.Int("old_version", searchState.Version),
			attribute.Int("new_version", versionResp.Version),
		)

		log.Printf("Context version changed from %d to %d, updating context",
			searchState.Version, versionResp.Version)

		// Update our context data
		if err := UpdateContextData(ctx, contextService); err != nil {
			updateSpan.RecordError(err)
			updateSpan.End()
			log.Printf("Error updating context data: %v", err)
			SendNack(ctx, workServerService, itemID, "Error updating context data")
			return
		}

		updateSpan.End()

		// NACK the work item to reprocess with the new context
		ctx, nackSpan := tracer.Start(ctx, "nack_version_changed")
		nackSpan.SetAttributes(attribute.String("reason", "context_version_changed"))
		SendNackWithSpan(ctx, workServerService, itemID, "Context version changed", nackSpan)
		return
	}

	// Everything succeeded, ACK the work item
	ctx, ackSpan := tracer.Start(ctx, "ack_work_item")
	ackResp, err := workServerService.Ack(ctx, itemID)
	if err != nil {
		ackSpan.RecordError(err)
		ackSpan.End()
		log.Printf("Error sending ACK: %v", err)
		SendNack(ctx, workServerService, itemID, "Error sending ACK")
		return
	}
	ackSpan.SetAttributes(attribute.String("ack_status", ackResp.Status))
	ackSpan.End()

	log.Printf("Sent ACK for work item: %s, response: %s", itemID, ackResp.Status)

	// Calculate and record the processing time in seconds
	processingTime := time.Since(startTime).Seconds()

	// Record the processing time with the shape as an attribute
	processingTimeByShape.Record(ctx, processingTime, metric.WithAttributes(
		attribute.String("shape", shapeStr),
	))

	log.Printf("Recorded processing time for ortho shape %s: %.3f seconds", shapeStr, processingTime)
}

// SendNack is a helper function to send a NACK with error logging
func SendNack(_ context.Context, workServerService types.WorkServerService, itemID, reason string) {
	log.Printf("Sending NACK for work item %s. Reason: %s", itemID, reason)
	ackResp, err := workServerService.Nack(context.Background(), itemID)
	if err != nil {
		log.Printf("Error sending NACK: %v", err)
		return
	}
	log.Printf("Sent NACK for work item: %s, response: %s", itemID, ackResp.Status)
}

// SendNackWithSpan is a helper function to send a NACK with error logging and tracing
func SendNackWithSpan(_ context.Context, workServerService types.WorkServerService, itemID, reason string, span trace.Span) {
	log.Printf("Sending NACK for work item %s. Reason: %s", itemID, reason)
	ackResp, err := workServerService.Nack(context.Background(), itemID)
	if err != nil {
		log.Printf("Error sending NACK: %v", err)
		span.RecordError(err)
		span.End()
		return
	}
	span.SetAttributes(attribute.String("nack_status", ackResp.Status))
	span.End()
	log.Printf("Sent NACK for work item: %s, response: %s", itemID, ackResp.Status)
}

// UpdateContextData fetches the latest context data from context service
func UpdateContextData(ctx context.Context, contextService types.ContextService) error {
	versionResp, err := contextService.GetVersion(ctx)
	if err != nil {
		return err
	}

	contextResp, err := contextService.GetContext(ctx)
	if err != nil {
		return err
	}

	// Initialize the state if it doesn't exist
	if searchState == nil {
		searchState = &State{
			Pairs: make(map[string]struct{}),
		}
	}

	// Update search state
	searchState.Version = versionResp.Version
	searchState.Vocabulary = contextResp.Vocabulary

	// Update pairs from context lines
	for _, line := range contextResp.Lines {
		key := strings.Join(line, ",")
		searchState.Pairs[key] = struct{}{}
	}

	log.Printf("Updated context to version %d with %d vocabulary terms and %d lines",
		versionResp.Version, len(searchState.Vocabulary), len(contextResp.Lines))

	return nil
}

// FilterVocabulary filters out forbidden words from the vocabulary.
func FilterVocabulary(vocabulary []string, forbidden map[string]struct{}) []string {
	filtered := []string{}
	for _, word := range vocabulary {
		if _, exists := forbidden[word]; !exists {
			filtered = append(filtered, word)
		}
	}
	return filtered
}

// GenerateCandidatesAndRemediations generates candidates and remediations based on the vocabulary and requirements.
func GenerateCandidatesAndRemediations(
	workingVocabulary []string,
	required [][]string,
	pairs map[string]struct{},
	top types.Ortho,
) ([]string, []types.RemediationTuple) {
	candidates := []string{}
	remediations := []types.RemediationTuple{}

	for _, word := range workingVocabulary {
		missingRequired := FindMissingRequired(required, pairs, word)
		if missingRequired != nil {
			remediations = append(remediations, types.RemediationTuple{
				Pair: append(missingRequired, word),
				Hash: GenerateUniqueHash(append(missingRequired, word)),
			})
		} else {
			candidates = append(candidates, word)
		}
	}

	return candidates, remediations
}

// FindMissingRequired finds the first missing required pair for a given word.
func FindMissingRequired(required [][]string, pairs map[string]struct{}, word string) []string {
	for _, req := range required {
		combined := append([]string{}, req...)
		combined = append(combined, word)
		key := strings.Join(combined, ",")
		if _, exists := pairs[key]; !exists {
			return req
		}
	}
	return nil
}

// GenerateNewOrthos generates new orthos from the candidates.
func GenerateNewOrthos(candidates []string, parent types.Ortho) []types.Ortho {
	// Create a counter for generating new orthos
	counter := NewCounter()

	// Generate new orthos using the Add function directly - no conversion needed
	var result []types.Ortho

	for _, word := range candidates {
		// Generate internal orthos using Add function
		newOrthos := Add(parent, word, counter)
		result = append(result, newOrthos...)
	}

	return result
}

// GenerateUniqueHash generates a unique hash for a pair.
func GenerateUniqueHash(pair []string) string {
	return "hash-" + strings.Join(pair, "-")
}

// StartWorker begins the worker loop that processes items from the queue
func StartWorker(
	ctx context.Context,
	contextService types.ContextService,
	orthosService types.OrthosService,
	remediationsService types.RemediationsService,
	workServerService types.WorkServerService,
) {
	// Get a separate tracer for the worker
	tracer := otel.Tracer("search-worker")

	// Get the raw worker loop context
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Initialize the context on startup with a proper span
	initCtx, initSpan := tracer.Start(workerCtx, "initialize_context")
	if err := UpdateContextData(initCtx, contextService); err != nil {
		log.Printf("Failed to initialize context data: %v", err)
		initSpan.RecordError(err)
		initSpan.End()
		return
	}
	initSpan.End()

	// Worker loop
	for {
		// Check if context is done (service shutting down)
		select {
		case <-workerCtx.Done():
			log.Println("Worker shutting down")
			return
		default:
			// Continue with the worker loop
		}

		// Create a root span for this work cycle
		rootCtx, rootSpan := tracer.Start(context.Background(), "worker_cycle")
		rootSpan.SetAttributes(attribute.String("service.name", "search"))

		// Add a waiting span to show we're actively polling
		_, waitSpan := tracer.Start(rootCtx, "waiting_for_work")

		// Call the work server to get a work item with a dedicated span
		popCtx, popSpan := tracer.Start(rootCtx, "pop_work_item")
		popResp, err := workServerService.Pop(popCtx)

		if err != nil {
			log.Printf("Error popping work item: %v", err)
			popSpan.RecordError(err)
			popSpan.End()
			waitSpan.End()
			rootSpan.End()

			// Apply cooldown after error
			time.Sleep(5 * time.Second) // Wait a bit before trying again
			continue
		}

		// End the pop span now
		popSpan.End()

		// Record whether we got an item
		hasWork := popResp.Ortho != nil && popResp.ID != ""
		rootSpan.SetAttributes(attribute.Bool("has_work", hasWork))

		// If no item is available, wait and try again
		if !hasWork {
			log.Println("No work items available, waiting...")
			waitSpan.SetAttributes(attribute.Bool("found_work", false))
			waitSpan.End()
			rootSpan.End()

			// Apply cooldown when no work is available
			time.Sleep(5 * time.Second)
			continue
		}

		// We found work, end the waiting span
		waitSpan.SetAttributes(attribute.Bool("found_work", true))
		waitSpan.End()

		// Process the work item inside a span
		processCtx, processSpan := tracer.Start(rootCtx, "process_work_item")
		processSpan.SetAttributes(
			attribute.String("work_item.id", popResp.ID),
			attribute.String("ortho.id", popResp.Ortho.ID),
		)

		// Track if we had an error during processing
		hadProcessingError := false

		// Use a panic handler to ensure trace completion
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Fix the non-constant format string error
					errMsg := fmt.Sprintf("PANIC during processing: %v", r)
					log.Println(errMsg)
					// Fix the non-constant format string error in fmt.Errorf
					processSpan.RecordError(fmt.Errorf("%s", errMsg))

					// Try to NACK the item after a panic
					nackCtx, nackSpan := tracer.Start(rootCtx, "nack_after_panic")
					_, nackErr := workServerService.Nack(nackCtx, popResp.ID)
					if nackErr != nil {
						nackSpan.RecordError(nackErr)
					}
					nackSpan.End()

					// Mark that we had an error
					hadProcessingError = true
				}
				processSpan.End()
			}()

			// Process the work item
			ProcessWorkItem(
				processCtx,
				contextService,
				orthosService,
				remediationsService,
				workServerService,
				*popResp.Ortho,
				popResp.ID,
			)
		}()

		// Add a cooldown span and delay only if there was an error during processing
		if hadProcessingError {
			_, cooldownSpan := tracer.Start(rootCtx, "error_cooldown")
			log.Println("Applying cooldown after processing error")
			time.Sleep(500 * time.Millisecond) // Cooldown after error
			cooldownSpan.End()
		}

		// End the root span for this cycle
		rootSpan.End()
	}
}

// main function starts the service
func main() {
	// Load configuration
	var cfg config.SearchConfig

	// Set service name before loading config
	cfg.ServiceName = "search"

	// Process environment variables with the appropriate prefix
	if err := config.LoadConfig("SEARCH", &cfg); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Log configuration details
	log.Printf("Using configuration: %+v", cfg)
	config.LogConfig(cfg.BaseConfig)

	// Set up common components using the shared helper
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

	// Initialize custom metrics
	meter := mp.Meter(cfg.ServiceName)
	var err2 error
	processingTimeByShape, err2 = meter.Float64Histogram(
		"ortho_processing_duration_seconds_by_shape",
		metric.WithDescription("Duration of ortho processing in seconds by shape"),
		metric.WithUnit("s"),
		// Add custom explicit bucket boundaries for more precise percentile calculations
		metric.WithExplicitBucketBoundaries(
			0.001, 0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1.0,
			1.5, 2.0, 2.5, 3.0, 4.0, 5.0, 7.5, 10.0, 15.0, 20.0, 30.0,
		),
	)
	if err2 != nil {
		log.Fatalf("Failed to create processing time metric: %v", err2)
	}

	// Initialize metrics for search counts and success rates by input ortho shape
	searchesByShape, err2 = meter.Int64Counter(
		"ortho_searches_by_shape",
		metric.WithDescription("Total number of searches processed by input ortho shape"),
		metric.WithUnit("{search}"),
	)
	if err2 != nil {
		log.Fatalf("Failed to create searches by shape metric: %v", err2)
	}

	searchSuccessByShape, err2 = meter.Float64Counter(
		"ortho_search_success_by_shape",
		metric.WithDescription("Number of searches that found at least one item by input ortho shape"),
		metric.WithUnit("{success}"),
	)
	if err2 != nil {
		log.Fatalf("Failed to create search success metric: %v", err2)
	}

	itemsFoundByShape, err2 = meter.Float64Counter(
		"ortho_items_found_by_shape",
		metric.WithDescription("Total count of items found in searches by input ortho shape"),
		metric.WithUnit("{item}"),
	)
	if err2 != nil {
		log.Fatalf("Failed to create items found metric: %v", err2)
	}

	// Initialize new metrics for shape and position
	searchesByShapePosition, err2 = meter.Int64Counter(
		"ortho_searches_by_shape_position",
		metric.WithDescription("Total number of searches processed by input ortho shape and position"),
		metric.WithUnit("{search}"),
	)
	if err2 != nil {
		log.Fatalf("Failed to create searches by shape and position metric: %v", err2)
	}

	searchSuccessByShapePosition, err2 = meter.Float64Counter(
		"ortho_search_success_by_shape_position",
		metric.WithDescription("Number of searches that found at least one item by input ortho shape and position"),
		metric.WithUnit("{success}"),
	)
	if err2 != nil {
		log.Fatalf("Failed to create search success by shape and position metric: %v", err2)
	}

	itemsFoundByShapePosition, err2 = meter.Float64Counter(
		"ortho_items_found_by_shape_position",
		metric.WithDescription("Total count of items found in searches by input ortho shape and position"),
		metric.WithUnit("{item}"),
	)
	if err2 != nil {
		log.Fatalf("Failed to create items found by shape and position metric: %v", err2)
	}

	// Set the global TextMapPropagator for OpenTelemetry
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Initialize HTTP client for service communication with tracing
	httpClient := httpclient.NewDefaultClient()

	// Initialize service clients
	contextService := clients.NewContextService(cfg.ContextServiceURL, httpClient)
	orthosService := clients.NewOrthosService(cfg.OrthosServiceURL, httpClient)
	remediationsService := clients.NewRemediationsService(cfg.RemediationsServiceURL, httpClient)
	workServerService := clients.NewWorkServerService(cfg.WorkServerURL, httpClient)

	// Initialize searchState
	searchState = &State{
		Version:    0,
		Vocabulary: []string{},
		Pairs:      make(map[string]struct{}),
	}

	// Log the initialization of service
	log.Printf("Search service initialized with the following service URLs:")
	log.Printf("  Orthos: %s", cfg.OrthosServiceURL)
	log.Printf("  Remediations: %s", cfg.RemediationsServiceURL)
	log.Printf("  Context: %s", cfg.ContextServiceURL)
	log.Printf("  WorkServer: %s", cfg.WorkServerURL)

	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())

	// Start the worker in a goroutine
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker() // Ensure we stop the worker when the main function exits

	go StartWorker(
		workerCtx,
		contextService,
		orthosService,
		remediationsService,
		workServerService,
	)

	// Create health check service with appropriate options
	healthOptions := health.Options{
		ServiceName: cfg.ServiceName,
		Version:     "0.1.0",
		Details: map[string]string{
			"environment": "development",
		},
	}

	// Create the health service
	healthService := health.New(healthOptions)

	// Register health check endpoint
	router.GET("/health", gin.WrapF(healthService.Handler()))

	// Register metrics endpoint with proper handler from the Prometheus client
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Start the server
	address := cfg.GetAddress()
	log.Printf("Search server starting on %s...\n", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
