package main

import (
	"context"
	"crochet/clients"
	"crochet/config"
	"crochet/health"
	"crochet/middleware"
	"crochet/types"
	"encoding/json"
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
	repositoryService types.RepositoryService,
	ortho types.Ortho,
	receipt string,
) {
	// Create a span for the entire work item processing
	tracer := otel.Tracer("search-worker")
	ctx, span := tracer.Start(ctx, "processWorkItem")
	defer span.End()

	// Record the start time for processing duration metric
	startTime := time.Now()

	// Add work item details to the span
	span.SetAttributes(
		attribute.String("ortho.id", ortho.ID),
	)

	log.Printf("Processing work for ortho ID: %s", ortho.ID)

	// Additional debug logging for ortho details
	log.Printf("DEBUG: Ortho details - Shape: %v, Position: %v, Shell: %d, Grid size: %d",
		ortho.Shape, ortho.Position, ortho.Shell, len(ortho.Grid))

	// Print first few grid entries to understand what's in there
	i := 0
	for k, v := range ortho.Grid {
		if i < 5 { // Limit to first 5 entries to avoid excessive logging
			log.Printf("DEBUG: Grid entry %d - Position: %s, Value: %s", i, k, v)
			i++
		} else {
			break
		}
	}

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

	// Log forbidden and required values
	log.Printf("DEBUG: Ortho requirements - Forbidden count: %d, Required paths: %d",
		len(forbidden), len(required))
	if len(forbidden) > 0 {
		log.Printf("DEBUG: First few forbidden values: %v", forbidden[:min(5, len(forbidden))])
	}
	for i, req := range required {
		if i < 3 { // Limit to first 3 required paths
			log.Printf("DEBUG: Required path %d: %v", i, req)
		} else {
			break
		}
	}

	// Convert forbidden to a map for filtering
	forbiddenMap := make(map[string]struct{})
	for _, word := range forbidden {
		forbiddenMap[word] = struct{}{}
	}

	// Filter the vocabulary to exclude forbidden words
	workingVocabulary := FilterVocabulary(searchState.Vocabulary, forbiddenMap)

	// Enhanced logging about vocabulary filtering
	log.Printf("DEBUG: Vocabulary filtering - Total vocab: %d, After filtering: %d",
		len(searchState.Vocabulary), len(workingVocabulary))
	if len(workingVocabulary) > 0 {
		sampleSize := min(5, len(workingVocabulary))
		log.Printf("DEBUG: Sample of filtered vocabulary: %v", workingVocabulary[:sampleSize])
	} else {
		log.Printf("WARNING: No working vocabulary after filtering - all words were forbidden!")
	}

	// Debug logging for searchState.Pairs
	log.Printf("DEBUG: Search state pairs: %d entries", len(searchState.Pairs))
	debugCount := 0
	for word := range searchState.Pairs {
		if debugCount < 5 {
			log.Printf("DEBUG: Pair entry: word=%s", word)
			debugCount++
		} else {
			break
		}
	}

	// Generate candidates and remediations
	candidates, remediations := GenerateCandidatesAndRemediations(workingVocabulary, required, searchState.Pairs, ortho)

	// Log candidates and remediations
	log.Printf("DEBUG: Generated %d candidates and %d remediations", len(candidates), len(remediations))
	if len(candidates) > 0 {
		log.Printf("DEBUG: First few candidates: %v", candidates[:min(5, len(candidates))])
	} else {
		log.Printf("INFO: No candidates were generated for this ortho")
	}

	if len(remediations) > 0 {
		log.Printf("DEBUG: First few remediations: %v", remediations[:min(3, len(remediations))])
	}

	// Generate new orthos from candidates
	newOrthos := GenerateNewOrthos(candidates, ortho)

	// Log new orthos
	log.Printf("DEBUG: Generated %d new orthos from candidates", len(newOrthos))
	if len(newOrthos) > 0 {
		for i, o := range newOrthos {
			if i < 3 { // Limit to first 3 new orthos
				log.Printf("DEBUG: New ortho %d - ID: %s, Shape: %v, Position: %v, Shell: %d",
					i, o.ID, o.Shape, o.Position, o.Shell)
			} else {
				break
			}
		}
	} else {
		log.Printf("INFO: No new orthos were generated from the candidates")
	}

	// Create a span for the results posting operation
	ctx, resultsSpan := tracer.Start(ctx, "post_results")
	resultsSpan.SetAttributes(
		attribute.Int("orthos_count", len(newOrthos)),
		attribute.Int("remediations_count", len(remediations)),
	)

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

	// Post results (both orthos and remediations) to the repository
	log.Printf("INFO: Posting results to repository - %d orthos, %d remediations",
		len(newOrthos), len(remediations))

	resultsResponse, err := repositoryService.PostResults(ctx, newOrthos, remediations, receipt)
	if err != nil {
		resultsSpan.RecordError(err)
		resultsSpan.End()
		log.Printf("Error posting results: %v", err)
		return
	}

	resultsSpan.SetAttributes(
		attribute.Int("saved_orthos_count", resultsResponse.NewOrthosCount),
		attribute.Int("saved_remediations_count", resultsResponse.RemediationsCount),
	)
	resultsSpan.End()

	log.Printf("Posted results: %d new orthos, %d remediations",
		resultsResponse.NewOrthosCount, resultsResponse.RemediationsCount)

	// Check if version has changed in the repository response
	if resultsResponse.Version != searchState.Version {
		ctx, updateSpan := tracer.Start(ctx, "update_context")
		updateSpan.SetAttributes(
			attribute.Int("old_version", searchState.Version),
			attribute.Int("new_version", resultsResponse.Version),
		)

		log.Printf("Context version changed from %d to %d, updating context",
			searchState.Version, resultsResponse.Version)

		// Update our context data
		if err := UpdateContextData(ctx, repositoryService); err != nil {
			updateSpan.RecordError(err)
			updateSpan.End()
			log.Printf("Error updating context data: %v", err)
			return
		}

		updateSpan.End()
	}

	// Calculate and record the processing time in seconds
	processingTime := time.Since(startTime).Seconds()

	// Record the processing time with the shape as an attribute
	processingTimeByShape.Record(ctx, processingTime, metric.WithAttributes(
		attribute.String("shape", shapeStr),
	))

	log.Printf("Processed ortho shape %s in %.3f seconds - generated %d candidates, %d remediations, %d new orthos",
		shapeStr, processingTime, len(candidates), len(remediations), len(newOrthos))
}

// min is a helper function to find the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// UpdateContextDataWithRetry fetches the latest context data from repository service with retries
func UpdateContextDataWithRetry(ctx context.Context, repositoryService types.RepositoryService, maxRetries int) error {
	var lastErr error
	// Start with 1 second delay, then exponential backoff
	retryDelay := 1 * time.Second

	for i := 0; i < maxRetries; i++ {
		err := UpdateContextData(ctx, repositoryService)
		if err == nil {
			// Success!
			if i > 0 {
				log.Printf("Successfully initialized context data after %d retries", i)
			}
			return nil
		}

		lastErr = err
		log.Printf("Failed to initialize context data (attempt %d/%d): %v", i+1, maxRetries, err)

		// Sleep before retrying
		log.Printf("Retrying in %v...", retryDelay)
		time.Sleep(retryDelay)

		// Exponential backoff with jitter
		jitter := time.Duration(rand.Intn(500)) * time.Millisecond
		retryDelay = retryDelay*2 + jitter

		// Cap the retry delay at 30 seconds
		if retryDelay > 30*time.Second {
			retryDelay = 30*time.Second + jitter
		}
	}

	return fmt.Errorf("failed to initialize context after %d attempts: %w", maxRetries, lastErr)
}

// UpdateContextData fetches the latest context data from repository service
func UpdateContextData(ctx context.Context, repositoryService types.RepositoryService) error {
	contextResp, err := repositoryService.GetContext(ctx)
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
	searchState.Version = contextResp.Version
	searchState.Vocabulary = contextResp.Vocabulary

	// Update pairs from context lines
	for _, line := range contextResp.Lines {
		key := strings.Join(line, ",")
		searchState.Pairs[key] = struct{}{}
	}

	log.Printf("Updated context to version %d with %d vocabulary terms and %d lines",
		contextResp.Version, len(searchState.Vocabulary), len(contextResp.Lines))

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

// GenerateCandidatesAndRemediations generates ortho candidates and remediations from the given workingVocabulary and required paths
func GenerateCandidatesAndRemediations(workingVocabulary []string, required [][]string, pairsMap map[string]struct{}, ortho types.Ortho) ([]string, []types.RemediationTuple) {
	// Enhanced debug logging
	log.Printf("DEBUG: GenerateCandidatesAndRemediations called with: %d vocabulary items, %d required paths, %d pairs",
		len(workingVocabulary), len(required), len(pairsMap))

	// Log sample of working vocabulary
	if len(workingVocabulary) > 0 {
		sampleSize := min(10, len(workingVocabulary))
		log.Printf("DEBUG: Working vocabulary sample: %v", workingVocabulary[:sampleSize])
	} else {
		log.Printf("WARNING: Empty working vocabulary!")
	}

	// Log sample of required paths
	if len(required) > 0 {
		sampleSize := min(5, len(required))
		log.Printf("DEBUG: Required paths sample: %v", required[:sampleSize])
	}

	// Log sample of pairs
	pairCount := 0
	log.Printf("DEBUG: Checking pairs map...")
	for key := range pairsMap {
		if pairCount < 10 {
			log.Printf("DEBUG: Pair entry: %s", key)
			pairCount++
		} else {
			break
		}
	}

	// Keep track of candidates that meet all requirements
	var candidates []string
	// Keep track of pairs that need to be remediated
	var remediations []types.RemediationTuple

	// Iterate through each vocabulary word (matches the Elixir implementation)
	for _, word := range workingVocabulary {
		log.Printf("DEBUG: Processing vocabulary word: %s", word)

		// For each word, check if any required path is missing when combined with this word
		missingRequiredFound := false
		var missingRequired []string

		for _, req := range required {
			// Check if this pair exists in the map (creates the pair key as req + word)
			pairKey := strings.Join(append(req, word), ",")
			_, exists := pairsMap[pairKey]

			log.Printf("DEBUG: Checking if pair '%s' exists: %t", pairKey, exists)

			if !exists {
				// Found a missing requirement for this word
				missingRequiredFound = true
				missingRequired = req
				break // Stop at the first missing requirement (matches Elixir implementation)
			}
		}

		if missingRequiredFound {
			// This word fails at least one requirement, add it to remediations
			log.Printf("DEBUG: Word fails requirement, adding to remediations: %s", word)
			remTuple := types.RemediationTuple{
				Pair: append(missingRequired, word),
			}
			remediations = append(remediations, remTuple)
		} else {
			// This word passes all requirements, add it to candidates
			log.Printf("DEBUG: Word passes all requirements, adding to candidates: %s", word)
			candidates = append(candidates, word)
		}
	}

	// Log the final results
	log.Printf("DEBUG: Generated %d candidates and %d remediations", len(candidates), len(remediations))
	return candidates, remediations
}

// GenerateNewOrthos generates new orthos from the candidates.
func GenerateNewOrthos(candidates []string, parent types.Ortho) []types.Ortho {
	// Create a counter for generating new orthos
	counter := NewCounter()
	// Generate new orthos using the Add function directly
	var result []types.Ortho

	// Log candidates for debugging
	log.Printf("DEBUG: Generating orthos from %d candidates", len(candidates))

	for _, word := range candidates {
		// For debugging
		log.Printf("DEBUG: Processing candidate word: %s", word)

		// Generate internal orthos using Add function directly with the word
		newOrthos := Add(parent, word, counter)
		result = append(result, newOrthos...)
	}

	return result
}

// StartWorker begins the worker loop that processes items from the queue
func StartWorker(
	ctx context.Context,
	repositoryService types.RepositoryService,
) {
	// Get a separate tracer for the worker
	tracer := otel.Tracer("search-worker")
	// Get the raw worker loop context
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	// Initialize the context on startup with a proper span
	initCtx, initSpan := tracer.Start(workerCtx, "initialize_context")
	// Try to initialize context data with retries
	if err := UpdateContextDataWithRetry(initCtx, repositoryService, 15); err != nil {
		log.Printf("Failed to initialize context data after maximum retries: %v", err)
		initSpan.RecordError(err)
		initSpan.End()
		// Don't return - we'll keep trying in the worker loop
		log.Println("Continuing with worker loop despite initialization failure")
	} else {
		log.Println("Successfully initialized context data")
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
		// If we don't have context data yet, keep trying to initialize
		if searchState == nil || len(searchState.Vocabulary) == 0 {
			if err := UpdateContextData(workerCtx, repositoryService); err != nil {
				log.Printf("Still unable to initialize context data: %v", err)
				time.Sleep(5 * time.Second) // Wait before retrying
				continue
			}
			log.Println("Successfully initialized context data in worker loop")
		}
		// Create a root span for this work cycle
		rootCtx, rootSpan := tracer.Start(context.Background(), "worker_cycle")
		rootSpan.SetAttributes(attribute.String("service.name", "search"))
		// Add a waiting span to show we're actively polling
		_, waitSpan := tracer.Start(rootCtx, "waiting_for_work")
		// Call the repository to get a work item with a dedicated span
		workCtx, workSpan := tracer.Start(rootCtx, "get_work_item")
		workResponse, err := repositoryService.GetWork(workCtx)
		if err != nil {
			log.Printf("Error getting work item: %v", err)
			workSpan.RecordError(err)
			workSpan.End()
			waitSpan.End()
			rootSpan.End()
			// Apply cooldown after error
			time.Sleep(5 * time.Second) // Wait a bit before trying again
			continue
		}
		// End the work span now
		workSpan.End()
		// Check if the Work field is nil - WorkResponse is a value type, not a pointer
		hasWork := workResponse.Work != nil
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

		// Get the receipt from the work response
		receipt := workResponse.Receipt
		log.Printf("Received work item with receipt: %s", receipt)

		// Get the work item from the response
		workItem := workResponse.Work

		// Enhanced logging: Log detailed work information including version
		log.Printf("WORKER RECEIVED WORK: ID=%s, Timestamp=%d, Version=%d",
			workItem.ID, workItem.Timestamp, workResponse.Version)

		// Log work data details if available
		if workItem.Data != nil {
			dataJSON, _ := json.Marshal(workItem.Data)
			log.Printf("Work item data: %s", string(dataJSON))
		}

		// Convert the work item data to an Ortho
		var ortho types.Ortho

		// First convert the work item's Data field to bytes
		dataBytes, err := json.Marshal(workItem.Data)
		if err != nil {
			log.Printf("Error marshaling work item data: %v", err)
			rootSpan.RecordError(err)
			rootSpan.End()
			time.Sleep(1 * time.Second)
			continue
		}

		// Log the raw data before unmarshaling
		log.Printf("RAW WORK DATA: %s", string(dataBytes))

		// Enhanced validation before unmarshaling
		if len(dataBytes) <= 2 { // Empty JSON object "{}" or less
			log.Printf("WARNING: Work item data is empty or nearly empty: '%s'", string(dataBytes))
			rootSpan.RecordError(fmt.Errorf("empty work item data"))
			rootSpan.End()
			time.Sleep(1 * time.Second)
			continue
		}

		// Check if this is a nested structure with a data field
		var nestedStructure struct {
			Data *types.Ortho `json:"data"`
		}

		if err := json.Unmarshal(dataBytes, &nestedStructure); err == nil && nestedStructure.Data != nil {
			// Successfully detected and extracted the nested ortho
			ortho = *nestedStructure.Data
			log.Printf("Detected and extracted nested ortho structure with ID: %s", ortho.ID)
		} else {
			// Try direct unmarshaling as fallback (backward compatibility)
			if err := json.Unmarshal(dataBytes, &ortho); err != nil {
				log.Printf("Error unmarshaling work item data to Ortho: %v", err)
				log.Printf("UNMARSHAL ERROR DETAILS: Data='%s', Error='%v'", string(dataBytes), err)
				rootSpan.RecordError(err)
				rootSpan.End()
				time.Sleep(1 * time.Second)
				continue
			}
		}

		// Validate the unmarshaled ortho object
		if ortho.ID == "" {
			log.Printf("WARNING: Unmarshaled ortho has empty ID, generating a fallback ID")
			ortho.ID = fmt.Sprintf("fallback_ortho_%d", time.Now().UnixNano())
		}

		if ortho.Grid == nil {
			log.Printf("WARNING: Unmarshaled ortho has nil Grid, initializing empty map")
			ortho.Grid = make(map[string]string)
		}

		log.Printf("WORKER PROCESSING ORTHO: ID=%s, Shape=%v, Position=%v, Shell=%d, GridSize=%d",
			ortho.ID, ortho.Shape, ortho.Position, ortho.Shell, len(ortho.Grid))

		// Process the work item inside a span
		processCtx, processSpan := tracer.Start(rootCtx, "process_work_item")
		processSpan.SetAttributes(
			attribute.String("work_item.id", workItem.ID),
			attribute.String("ortho.id", ortho.ID),
			attribute.String("receipt", receipt),
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
					// Mark that we had an error
					hadProcessingError = true
				}
				processSpan.End()
			}()
			// Process the work item
			ProcessWorkItem(
				processCtx,
				repositoryService,
				ortho,
				receipt,
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
	cfg, err := config.LoadSearchConfig()
	if err != nil {
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

	if err2 != nil {
		log.Fatalf("Failed to create repository GetContext duration metric: %v", err2)
	}

	// Set the global TextMapPropagator for OpenTelemetry
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Initialize the repository service client
	repositoryService := clients.NewRepositoryService(cfg.RepositoryServiceURL)

	// Initialize searchState
	searchState = &State{
		Version:    0,
		Vocabulary: []string{},
		Pairs:      make(map[string]struct{}),
	}

	// Log the initialization of service
	log.Printf("Search service initialized with Repository service URL: %s", cfg.RepositoryServiceURL)

	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())

	// Start the worker in a goroutine
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker() // Ensure we stop the worker when the main function exits
	go StartWorker(workerCtx, repositoryService)

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
