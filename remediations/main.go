package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"crochet/telemetry"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type RemediationRequest struct {
	Pairs [][]string `json:"pairs"`
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	// Extract trace context from incoming request
	ctx := r.Context()
	propagator := otel.GetTextMapPropagator()
	ctx = propagator.Extract(ctx, propagation.HeaderCarrier(r.Header))

	// Create a span for this handler
	tracer := otel.Tracer("remediations-service")
	ctx, span := tracer.Start(ctx, "okHandler")
	defer span.End()

	// Set attributes for the HTTP request
	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.url", r.URL.String()),
	)

	w.Header().Set("Content-Type", "application/json")
	span.SetAttributes(attribute.Int("http.status_code", http.StatusOK))
	json.NewEncoder(w).Encode(map[string]string{"status": "OK"})
}

func remediateHandler(w http.ResponseWriter, r *http.Request) {
	// Extract trace context from incoming request
	ctx := r.Context()
	propagator := otel.GetTextMapPropagator()
	ctx = propagator.Extract(ctx, propagation.HeaderCarrier(r.Header))

	// Create a span for this handler
	tracer := otel.Tracer("remediations-service")
	ctx, span := tracer.Start(ctx, "remediateHandler")
	defer span.End()

	// Set attributes for the HTTP request
	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.url", r.URL.String()),
	)

	if r.Method != http.MethodPost {
		telemetry.WriteJSONError(w, "remediations", http.StatusMethodNotAllowed, "Method not allowed")
		span.SetStatus(codes.Error, "Method not allowed")
		span.SetAttributes(attribute.Int("http.status_code", http.StatusMethodNotAllowed))
		return
	}

	var request RemediationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error decoding request: %v", err)
		telemetry.WriteJSONError(w, "remediations", http.StatusBadRequest, "Invalid JSON format")
		span.SetStatus(codes.Error, "Invalid JSON format")
		span.SetAttributes(attribute.Int("http.status_code", http.StatusBadRequest))
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

	response := map[string]interface{}{
		"status": "OK",
		"hashes": hashes,
	}

	w.Header().Set("Content-Type", "application/json")
	span.SetAttributes(attribute.Int("http.status_code", http.StatusOK))
	json.NewEncoder(w).Encode(response)
}

func main() {
	port := os.Getenv("REMEDIATIONS_PORT")
	if port == "" {
		panic("REMEDIATIONS_PORT environment variable must be set")
	}

	host := os.Getenv("REMEDIATIONS_HOST")
	if host == "" {
		panic("REMEDIATIONS_HOST environment variable must be set")
	}

	jaegerEndpoint := os.Getenv("JAEGER_ENDPOINT")
	if jaegerEndpoint == "" {
		panic("JAEGER_ENDPOINT environment variable must be set")
	}

	// Initialize OpenTelemetry with the shared telemetry package
	tp, err := telemetry.InitTracer("remediations-service", jaegerEndpoint)
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}
	defer tp.ShutdownWithTimeout(5 * time.Second)

	// Register HTTP handlers
	http.HandleFunc("/", okHandler)
	http.HandleFunc("/remediate", remediateHandler)

	addr := fmt.Sprintf("%s:%s", host, port)
	log.Printf("Remediations service starting on %s:%s...", host, port)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
