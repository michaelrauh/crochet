package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"crochet/config"
	"crochet/httpclient"
	"crochet/telemetry"
	"crochet/text"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// For backwards compatibility - use the shared error type under the hood
func NewIngestorError(code int, message string) *telemetry.ServiceError {
	return telemetry.NewServiceError("ingestor", code, message)
}

// Keeping these type definitions local to ingestor for now
// Later they can be moved to a shared types package (improvement #8)

type Corpus struct {
	Title string `mapstructure:"title"`
	Text  string `mapstructure:"text"`
}

// ContextInput represents the data sent to the context service
type ContextInput struct {
	Title      string     `mapstructure:"title"`
	Vocabulary []string   `mapstructure:"vocabulary"`
	Subphrases [][]string `mapstructure:"subphrases"`
}

// ContextResponse represents the response from the context service
type ContextResponse struct {
	Version       int        `mapstructure:"version"`
	NewSubphrases [][]string `mapstructure:"newSubphrases"`
}

// RemediationRequest represents the request data for the remediation service
type RemediationRequest struct {
	Pairs [][]string `mapstructure:"pairs"`
}

// RemediationResponse represents the response from the remediation service
type RemediationResponse struct {
	Status string   `mapstructure:"status"`
	Hashes []string `mapstructure:"hashes"`
}

type ContextService interface {
	SendMessage(ctx context.Context, message string) (map[string]interface{}, error)
}

type RealContextService struct {
	URL    string
	Client *httpclient.Client
}

func (s *RealContextService) SendMessage(ctx context.Context, message string) (map[string]interface{}, error) {
	serviceResp := s.Client.Call(ctx, http.MethodPost, s.URL+"/input", []byte(message))
	if serviceResp.Error != nil {
		return nil, fmt.Errorf("error calling context service: %w", serviceResp.Error)
	}

	log.Printf("Parsed context service response: %v", serviceResp.RawResponse)
	return serviceResp.RawResponse, nil
}

type RemediationsService interface {
	FetchRemediations(ctx context.Context, subphrases [][]string) (map[string]interface{}, error)
}

type RealRemediationsService struct {
	URL    string
	Client *httpclient.Client
}

func (s *RealRemediationsService) FetchRemediations(ctx context.Context, subphrases [][]string) (map[string]interface{}, error) {
	var pairs [][]string
	for _, subphrase := range subphrases {
		if len(subphrase) == 2 {
			pairs = append(pairs, subphrase)
		}
	}

	// Use mapstructure to encode the request data
	requestData := RemediationRequest{
		Pairs: pairs,
	}

	// We still need to marshal to JSON for the HTTP request
	requestJSON, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("error marshaling remediations request: %w", err)
	}

	serviceResp := s.Client.Call(ctx, http.MethodPost, s.URL+"/remediate", requestJSON)
	if serviceResp.Error != nil {
		return nil, fmt.Errorf("error calling remediations service: %w", serviceResp.Error)
	}

	return serviceResp.RawResponse, nil
}

// Helper functions for ginHandleTextInput
func processIncomingCorpus(c *gin.Context) (*Corpus, error) {
	var rawData map[string]interface{}
	if err := c.ShouldBindJSON(&rawData); err != nil {
		return nil, NewIngestorError(http.StatusBadRequest, "Invalid JSON format")
	}

	// Convert raw data to Corpus struct using mapstructure
	var corpus Corpus
	if err := mapstructure.Decode(rawData, &corpus); err != nil {
		return nil, NewIngestorError(http.StatusBadRequest, "Invalid data format")
	}

	return &corpus, nil
}

func prepareContextServiceInput(corpus *Corpus) ([]byte, error) {
	subphrases := text.GenerateSubphrases(corpus.Text)
	vocabulary := text.Vocabulary(corpus.Text)

	// Create a structured object for context input
	contextInput := ContextInput{
		Title:      corpus.Title,
		Vocabulary: vocabulary,
		Subphrases: subphrases,
	}

	// Marshal to JSON for HTTP request
	return json.Marshal(contextInput)
}

func processContextResponse(rawResponse map[string]interface{}) (*ContextResponse, error) {
	var response ContextResponse
	if err := mapstructure.Decode(rawResponse, &response); err != nil {
		return nil, fmt.Errorf("error decoding context service response: %w", err)
	}

	// If newSubphrases is nil, initialize it to an empty slice
	if response.NewSubphrases == nil {
		response.NewSubphrases = [][]string{}
	}

	return &response, nil
}

func extractPairsFromSubphrases(subphrases [][]string) [][]string {
	var pairs [][]string
	for _, subphrase := range subphrases {
		if len(subphrase) == 2 {
			pairs = append(pairs, subphrase)
		}
	}
	return pairs
}

func processRemediationsResponse(rawResponse map[string]interface{}) (*RemediationResponse, error) {
	var response RemediationResponse
	if err := mapstructure.Decode(rawResponse, &response); err != nil {
		return nil, fmt.Errorf("error decoding remediations response: %w", err)
	}
	return &response, nil
}

func ginHandleTextInput(c *gin.Context, contextService ContextService, remediationsService RemediationsService) {
	// Process the incoming request
	corpus, err := processIncomingCorpus(c)
	if err != nil {
		c.Error(err) // Use Gin's error handling
		return
	}

	fmt.Printf("Title: %s\nText: %s\n", corpus.Title, corpus.Text)

	// Prepare and send request to the context service
	contextInputJSON, err := prepareContextServiceInput(corpus)
	if err != nil {
		log.Printf("Error preparing data for context service: %v", err)
		c.Error(NewIngestorError(http.StatusInternalServerError, "Error preparing data for context service"))
		return
	}

	ctx := c.Request.Context()
	contextResponseRaw, err := contextService.SendMessage(ctx, string(contextInputJSON))
	if err != nil {
		log.Printf("Error sending message to context service: %v", err)
		// Check if this is already a custom error, if not, wrap it
		if serviceErr, ok := err.(*telemetry.ServiceError); ok {
			c.Error(serviceErr)
		} else {
			c.Error(NewIngestorError(http.StatusInternalServerError, "Error calling context service"))
		}
		return
	}

	// Process the context service response
	contextResponse, err := processContextResponse(contextResponseRaw)
	if err != nil {
		log.Printf("%v", err)
		c.Error(NewIngestorError(http.StatusInternalServerError, "Invalid response from context service"))
		return
	}

	// Extract pairs and call remediations service
	subphrasesForRemediations := extractPairsFromSubphrases(contextResponse.NewSubphrases)
	remediationsResponseRaw, err := remediationsService.FetchRemediations(ctx, subphrasesForRemediations)

	// Process remediations service response if available
	if err != nil {
		log.Printf("Error fetching remediations: %v", err)
		// We continue even if remediations service fails
	} else {
		remediationsResponse, err := processRemediationsResponse(remediationsResponseRaw)
		if err != nil {
			log.Printf("%v", err)
			// We continue even if processing fails
		} else {
			log.Printf("Remediations response: status=%s, hashes=%v",
				remediationsResponse.Status, remediationsResponse.Hashes)
		}
	}

	// Return response to client
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"version": contextResponse.Version,
	})
}

func main() {
	// Load configuration using the shared config package
	cfg, err := config.LoadIngestorConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Log the config for debugging - this shows we're using the new shared config package
	log.Printf("[SHARED CONFIG] Using unified configuration management: %+v", cfg)
	config.LogConfig(cfg)

	// Initialize OpenTelemetry with the shared telemetry package
	tp, err := telemetry.InitTracer(cfg.ServiceName, cfg.JaegerEndpoint)
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}
	defer tp.ShutdownWithTimeout(5 * time.Second)

	// Create a shared HTTP client with options from config
	httpClientOptions := httpclient.ClientOptions{
		DialTimeout:   cfg.DialTimeout,
		DialKeepAlive: cfg.DialKeepAlive,
		MaxIdleConns:  cfg.MaxIdleConns,
		ClientTimeout: cfg.ClientTimeout,
	}

	// If no custom options set, use defaults
	httpClient := httpclient.NewClient(httpClientOptions)

	// Initialize services with the shared HTTP client
	contextService := &RealContextService{
		URL:    cfg.ContextServiceURL,
		Client: httpClient,
	}
	remediationsService := &RealRemediationsService{
		URL:    cfg.RemediationsServiceURL,
		Client: httpClient,
	}

	// Replace gin.Default() with explicit configuration
	router := gin.New()

	// Add explicitly chosen middleware instead of using defaults
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// Add custom middleware - use the shared error handler
	router.Use(telemetry.GinErrorHandler())
	router.Use(otelgin.Middleware(cfg.ServiceName))

	router.POST("/ingest", func(c *gin.Context) {
		// Add the config to the request context
		ctxWithConfig := context.WithValue(c.Request.Context(), "config", cfg)
		c.Request = c.Request.WithContext(ctxWithConfig)
		ginHandleTextInput(c, contextService, remediationsService)
	})

	address := cfg.GetAddress()
	log.Printf("Server starting on %s...\n", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
