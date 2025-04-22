package main

import (
	"crochet/config"
	"crochet/health"
	"crochet/middleware"
	"crochet/telemetry"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// OrthosConfig holds configuration specific to the orthos server
type OrthosConfig struct {
	config.BaseConfig
}

// Ortho represents the structure for orthogonal data
type Ortho struct {
	Grid     map[string]interface{} `json:"grid"`
	Shape    []int                  `json:"shape"`
	Position []int                  `json:"position"`
	Shell    int                    `json:"shell"`
	ID       string                 `json:"id"`
}

// OrthosRequest represents a request containing multiple Ortho objects
type OrthosRequest struct {
	Orthos []Ortho `json:"orthos"`
}

// OrthosGetRequest represents a request to fetch orthos by IDs
type OrthosGetRequest struct {
	IDs []string `json:"ids"`
}

// OrthosStorage provides thread-safe storage for orthos
type OrthosStorage struct {
	orthos map[string]Ortho
	mutex  sync.RWMutex
}

// NewOrthosStorage creates a new orthos storage
func NewOrthosStorage() *OrthosStorage {
	return &OrthosStorage{
		orthos: make(map[string]Ortho),
	}
}

// AddOrthos adds multiple orthos to storage with thread safety and returns new IDs
func (s *OrthosStorage) AddOrthos(orthos []Ortho) []string {
	if len(orthos) == 0 {
		return []string{}
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	newIDs := make([]string, 0, len(orthos))

	for _, ortho := range orthos {
		// Check if the ortho with this ID already exists
		if _, exists := s.orthos[ortho.ID]; !exists {
			// Only add to newIDs if it's a new ortho
			newIDs = append(newIDs, ortho.ID)
		}
		// Add or update the ortho in the map
		s.orthos[ortho.ID] = ortho
	}

	log.Printf("Added %d orthos to storage. %d are new. Total orthos: %d",
		len(orthos), len(newIDs), len(s.orthos))

	return newIDs
}

// GetOrthosByIDs returns orthos matching the provided IDs
func (s *OrthosStorage) GetOrthosByIDs(ids []string) []Ortho {
	if len(ids) == 0 {
		return []Ortho{}
	}

	// No lock needed as per requirements for reads

	result := make([]Ortho, 0, len(ids))
	for _, id := range ids {
		if ortho, exists := s.orthos[id]; exists {
			result = append(result, ortho)
		}
	}

	return result
}

// Global storage instance
var orthosStorage *OrthosStorage

func handleRoot(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Hello World",
	})
}

func handleOrthos(c *gin.Context) {
	var request OrthosRequest

	// Bind JSON to struct
	if err := c.ShouldBindJSON(&request); err != nil {
		telemetry.LogAndError(c, err, "orthos", "Error parsing orthos request")
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request format",
		})
		return
	}

	// Log the received orthos
	log.Printf("Received %d orthos", len(request.Orthos))
	for i, ortho := range request.Orthos {
		log.Printf("Ortho %d: ID=%s, Shell=%d, Position=%v, Shape=%v",
			i, ortho.ID, ortho.Shell, ortho.Position, ortho.Shape)
	}

	// Store orthos in memory and get new IDs
	newIDs := orthosStorage.AddOrthos(request.Orthos)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Received orthos successfully",
		"count":   len(request.Orthos),
		"total":   len(orthosStorage.orthos),
		"newIDs":  newIDs,
	})
}

func handleGetOrthosByIDs(c *gin.Context) {
	var request OrthosGetRequest

	// Bind JSON to struct
	if err := c.ShouldBindJSON(&request); err != nil {
		telemetry.LogAndError(c, err, "orthos", "Error parsing orthos get request")
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request format",
		})
		return
	}

	// Get orthos by IDs
	matchedOrthos := orthosStorage.GetOrthosByIDs(request.IDs)
	log.Printf("Found %d orthos matching %d requested IDs", len(matchedOrthos), len(request.IDs))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Retrieved orthos successfully",
		"count":   len(matchedOrthos),
		"orthos":  matchedOrthos,
	})
}

func main() {
	// Initialize orthos storage
	orthosStorage = NewOrthosStorage()

	// Load configuration
	var cfg OrthosConfig
	// Set service name before loading config
	cfg.ServiceName = "orthos"
	// Process environment variables with the appropriate prefix
	if err := config.LoadConfig("ORTHOS", &cfg); err != nil {
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

	// Create health check service with appropriate options
	healthOptions := health.Options{
		ServiceName: cfg.ServiceName,
		Version:     "0.1.0", // Set a version
		Details: map[string]string{
			"environment": "development",
		},
	}
	// Create the health service
	healthService := health.New(healthOptions)

	// Register health check endpoint
	router.GET("/health", gin.WrapF(healthService.Handler()))

	// Register routes
	router.GET("/", handleRoot)
	router.POST("/orthos", handleOrthos)
	router.POST("/orthos/get", handleGetOrthosByIDs)

	// Start the server
	address := cfg.GetAddress()
	log.Printf("Orthos server starting on %s...\n", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
