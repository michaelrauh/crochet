package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
)

type RemediationRequest struct {
	Pairs [][]string `json:"pairs"`
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

func okHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the tracing context from the incoming request
	spanCtx, _ := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(r.Header),
	)

	// Create a new span for this request
	span := opentracing.GlobalTracer().StartSpan(
		"okHandler",
		ext.RPCServerOption(spanCtx),
	)
	defer span.Finish()

	span.SetTag("http.method", r.Method)
	span.SetTag("http.url", r.URL.String())

	w.Header().Set("Content-Type", "application/json")
	span.SetTag("http.status_code", http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "OK"})
}

func remediateHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the tracing context from the incoming request
	spanCtx, _ := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(r.Header),
	)

	// Create a new span for this request
	span := opentracing.GlobalTracer().StartSpan(
		"remediateHandler",
		ext.RPCServerOption(spanCtx),
	)
	defer span.Finish()

	span.SetTag("http.method", r.Method)
	span.SetTag("http.url", r.URL.String())

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		span.SetTag("error", true)
		span.SetTag("http.status_code", http.StatusMethodNotAllowed)
		return
	}

	var request RemediationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error decoding request: %v", err)
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		span.SetTag("error", true)
		span.SetTag("http.status_code", http.StatusBadRequest)
		span.LogKV("event", "error", "message", err.Error())
		return
	}

	pairs_count := len(request.Pairs)
	log.Printf("Received %d pairs for remediation", pairs_count)
	span.SetTag("pairs_count", pairs_count)

	// Log the pairs we received
	for i, pair := range request.Pairs {
		log.Printf("Pair %d: %v", i+1, pair)
		span.LogKV("pair", i+1, "value", fmt.Sprintf("%v", pair))
	}

	// Process pairs - create a child span for processing
	processSpan := opentracing.StartSpan(
		"processPairs",
		opentracing.ChildOf(span.Context()),
	)

	// Return a list of mock hashes as the response
	hashes := []string{
		"1234567890abcdef1234567890abcdef",
		"abcdef1234567890abcdef1234567890",
		"aabbccddeeff00112233445566778899",
		"99887766554433221100ffeeddccbbaa",
		"112233445566778899aabbccddeeff00",
		"00ffeeddccbbaa99887766554433221",
	}

	processSpan.SetTag("hashes_count", len(hashes))
	processSpan.Finish()

	response := map[string]interface{}{
		"status": "OK",
		"hashes": hashes,
	}

	w.Header().Set("Content-Type", "application/json")
	span.SetTag("http.status_code", http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func main() {
	// Initialize Jaeger tracer
	tracer, closer := initJaeger("remediations")
	defer closer.Close()
	opentracing.SetGlobalTracer(tracer)

	port := os.Getenv("REMEDIATIONS_PORT")
	if port == "" {
		panic("REMEDIATIONS_PORT environment variable must be set")
	}

	http.HandleFunc("/", okHandler)
	http.HandleFunc("/remediate", remediateHandler)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Remediations service starting on port %s...", port)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
