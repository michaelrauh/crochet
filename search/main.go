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
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Global version tracking
var currentVersion int
var contextVocabulary []string
var contextLines [][]string

// Maximum number of orthos and remediations to generate for demonstration purposes
const MAX_GENERATED_ITEMS = 5

// processWorkItem handles a single work item according to the pattern in work.md
func processWorkItem(
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

	// Add work item details to the span
	span.SetAttributes(
		attribute.String("work_item.id", itemID),
		attribute.String("ortho.id", ortho.ID),
	)

	log.Printf("Processing work item with ID: %s", itemID)

	// Add a span for generating random orthos
	ctx, genOrthosSpan := tracer.Start(ctx, "generate_orthos")
	generatedOrthos := generateRandomOrthos(ortho, rand.Intn(MAX_GENERATED_ITEMS))
	genOrthosSpan.SetAttributes(attribute.Int("generated_count", len(generatedOrthos)))
	genOrthosSpan.End()
	log.Printf("Generated %d new orthos from input", len(generatedOrthos))

	// Add a span for generating random remediations
	ctx, genRemSpan := tracer.Start(ctx, "generate_remediations")
	generatedRemediations := generateRandomRemediations(rand.Intn(MAX_GENERATED_ITEMS))
	genRemSpan.SetAttributes(attribute.Int("generated_count", len(generatedRemediations)))
	genRemSpan.End()
	log.Printf("Generated %d new remediations", len(generatedRemediations))

	// Step 1: Add generated orthos to the database
	if len(generatedOrthos) > 0 {
		ctx, saveSpan := tracer.Start(ctx, "save_orthos")
		saveSpan.SetAttributes(attribute.Int("orthos_count", len(generatedOrthos)))

		orthosResp, err := orthosService.SaveOrthos(ctx, generatedOrthos)
		if err != nil {
			saveSpan.RecordError(err)
			saveSpan.End()
			log.Printf("Error saving orthos: %v", err)
			sendNack(ctx, workServerService, itemID, "Error saving orthos")
			return
		}

		saveSpan.SetAttributes(attribute.Int("saved_count", orthosResp.Count))
		saveSpan.End()
		log.Printf("Added %d new orthos to database", orthosResp.Count)
	} else {
		// Add a span to show we're skipping this step
		_, skipSpan := tracer.Start(ctx, "skip_save_orthos")
		skipSpan.SetAttributes(attribute.String("reason", "no_orthos_to_save"))
		skipSpan.End()
	}

	// Step 2: Add generated remediations to the database
	if len(generatedRemediations) > 0 {
		ctx, addRemSpan := tracer.Start(ctx, "add_remediations")
		addRemSpan.SetAttributes(attribute.Int("remediations_count", len(generatedRemediations)))

		remediationsResp, err := remediationsService.AddRemediations(ctx, generatedRemediations)
		if err != nil {
			addRemSpan.RecordError(err)
			addRemSpan.End()
			log.Printf("Error adding remediations: %v", err)
			sendNack(ctx, workServerService, itemID, "Error adding remediations")
			return
		}

		addRemSpan.SetAttributes(attribute.Int("added_count", remediationsResp.Count))
		addRemSpan.End()
		log.Printf("Added %d new remediations to database", remediationsResp.Count)
	} else {
		// Add a span to show we're skipping this step
		_, skipSpan := tracer.Start(ctx, "skip_add_remediations")
		skipSpan.SetAttributes(attribute.String("reason", "no_remediations_to_add"))
		skipSpan.End()
	}

	// Step 3: Push new orthos to the queue
	if len(generatedOrthos) > 0 {
		ctx, pushSpan := tracer.Start(ctx, "push_orthos")
		pushSpan.SetAttributes(attribute.Int("orthos_to_push", len(generatedOrthos)))

		pushResp, err := workServerService.PushOrthos(ctx, generatedOrthos)
		if err != nil {
			pushSpan.RecordError(err)
			pushSpan.End()
			log.Printf("Error pushing orthos to queue: %v", err)
			sendNack(ctx, workServerService, itemID, "Error pushing orthos to queue")
			return
		}

		pushSpan.SetAttributes(attribute.Int("pushed_count", pushResp.Count))
		pushSpan.End()
		log.Printf("Pushed %d orthos to work queue", pushResp.Count)
	} else {
		// Add a span to show we're skipping this step
		_, skipSpan := tracer.Start(ctx, "skip_push_orthos")
		skipSpan.SetAttributes(attribute.String("reason", "no_orthos_to_push"))
		skipSpan.End()
	}

	// Step 4: Check version again
	ctx, verSpan := tracer.Start(ctx, "check_version")
	versionResp, err := contextService.GetVersion(ctx)
	if err != nil {
		verSpan.RecordError(err)
		verSpan.End()
		log.Printf("Error checking context version: %v", err)
		sendNack(ctx, workServerService, itemID, "Error checking context version")
		return
	}
	verSpan.SetAttributes(
		attribute.Int("current_version", currentVersion),
		attribute.Int("service_version", versionResp.Version),
	)
	verSpan.End()

	// Step 5: If version has changed, update context and NACK, otherwise ACK
	if versionResp.Version != currentVersion {
		ctx, updateSpan := tracer.Start(ctx, "update_context")
		updateSpan.SetAttributes(
			attribute.Int("old_version", currentVersion),
			attribute.Int("new_version", versionResp.Version),
		)

		log.Printf("Context version changed from %d to %d, updating context",
			currentVersion, versionResp.Version)

		// Update our context data
		if err := updateContextData(ctx, contextService); err != nil {
			updateSpan.RecordError(err)
			updateSpan.End()
			log.Printf("Error updating context data: %v", err)
			sendNack(ctx, workServerService, itemID, "Error updating context data")
			return
		}

		updateSpan.End()

		// NACK the work item to reprocess with the new context
		ctx, nackSpan := tracer.Start(ctx, "nack_version_changed")
		nackSpan.SetAttributes(attribute.String("reason", "context_version_changed"))
		sendNackWithSpan(ctx, workServerService, itemID, "Context version changed", nackSpan)
		return
	}

	// Everything succeeded, ACK the work item
	ctx, ackSpan := tracer.Start(ctx, "ack_work_item")
	ackResp, err := workServerService.Ack(ctx, itemID)
	if err != nil {
		ackSpan.RecordError(err)
		ackSpan.End()
		log.Printf("Error sending ACK: %v", err)
		sendNack(ctx, workServerService, itemID, "Error sending ACK")
		return
	}
	ackSpan.SetAttributes(attribute.String("ack_status", ackResp.Status))
	ackSpan.End()

	log.Printf("Sent ACK for work item: %s, response: %s", itemID, ackResp.Status)
}

// sendNack is a helper function to send a NACK with error logging
func sendNack(_ context.Context, workServerService types.WorkServerService, itemID, reason string) {
	log.Printf("Sending NACK for work item %s. Reason: %s", itemID, reason)
	ackResp, err := workServerService.Nack(context.Background(), itemID)
	if err != nil {
		log.Printf("Error sending NACK: %v", err)
		return
	}
	log.Printf("Sent NACK for work item: %s, response: %s", itemID, ackResp.Status)
}

// sendNackWithSpan is a helper function to send a NACK with error logging and tracing
func sendNackWithSpan(_ context.Context, workServerService types.WorkServerService, itemID, reason string, span trace.Span) {
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

// updateContextData fetches the latest context data from context service
func updateContextData(ctx context.Context, contextService types.ContextService) error {
	versionResp, err := contextService.GetVersion(ctx)
	if err != nil {
		return err
	}

	contextResp, err := contextService.GetContext(ctx)
	if err != nil {
		return err
	}

	// Update global state
	currentVersion = versionResp.Version
	contextVocabulary = contextResp.Vocabulary
	contextLines = contextResp.Lines

	log.Printf("Updated context to version %d with %d vocabulary terms and %d lines",
		currentVersion, len(contextVocabulary), len(contextLines))

	return nil
}

// generateRandomOrthos creates random orthos for demonstration
func generateRandomOrthos(baseOrtho types.Ortho, count int) []types.Ortho {
	orthos := make([]types.Ortho, count)
	for i := 0; i < count; i++ {
		orthos[i] = types.Ortho{
			ID:       generateRandomID("ortho"), // Always generate a random ID
			Shape:    []int{rand.Intn(5) + 1, rand.Intn(5) + 1},
			Position: []int{rand.Intn(10), rand.Intn(10)},
			Shell:    rand.Intn(3) + 1,
			Grid:     map[string]interface{}{"x": rand.Intn(100), "y": rand.Intn(100)},
		}
	}
	return orthos
}

// generateRandomRemediations creates random remediations for demonstration
func generateRandomRemediations(count int) []types.RemediationTuple {
	remediations := make([]types.RemediationTuple, count)
	for i := 0; i < count; i++ {
		remediations[i] = types.RemediationTuple{
			Pair: []string{
				generateRandomWord(5 + rand.Intn(5)),
				generateRandomWord(5 + rand.Intn(5)),
			},
			Hash: generateRandomID("hash"),
		}
	}
	return remediations
}

// generateRandomID creates a random ID with a prefix
func generateRandomID(prefix string) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, 8)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return prefix + "-" + string(result)
}

// generateRandomWord creates a random word of the given length
func generateRandomWord(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

// startWorker begins the worker loop that processes items from the queue
func startWorker(
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
	if err := updateContextData(initCtx, contextService); err != nil {
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

		// Use a panic handler to ensure trace completion
		func() {
			defer func() {
				if r := recover(); r != nil {
					errMsg := fmt.Sprintf("PANIC during processing: %v", r)
					log.Printf(errMsg)
					processSpan.RecordError(fmt.Errorf(errMsg))

					// Try to NACK the item after a panic
					nackCtx, nackSpan := tracer.Start(rootCtx, "nack_after_panic")
					_, nackErr := workServerService.Nack(nackCtx, popResp.ID)
					if nackErr != nil {
						nackSpan.RecordError(nackErr)
					}
					nackSpan.End()
				}
				processSpan.End()
			}()

			// Process the work item
			processWorkItem(
				processCtx,
				contextService,
				orthosService,
				remediationsService,
				workServerService,
				*popResp.Ortho,
				popResp.ID,
			)
		}()

		// Add a small cooldown span to visualize the intentional delay
		_, cooldownSpan := tracer.Start(rootCtx, "cooldown")
		time.Sleep(500 * time.Millisecond) // Small cooldown between work items
		cooldownSpan.End()

		// End the root span for this cycle
		rootSpan.End()
	}
}

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

	go startWorker(
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

	// Register health check endpoint only
	router.GET("/health", gin.WrapF(healthService.Handler()))

	// Start the server
	address := cfg.GetAddress()
	log.Printf("Search server starting on %s...\n", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
