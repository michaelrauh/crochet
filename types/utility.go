package types

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
)

// Todo fix
func ProcessIncomingCorpus(c *gin.Context, serviceName string) (Corpus, error) {
	var corpus Corpus
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("[ERROR] %s: Failed to read request body: %v", serviceName, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Failed to read request body",
		})
		return corpus, err
	}
	c.Request.Body.Close()

	err = json.Unmarshal(body, &corpus)
	if err != nil {
		log.Printf("[ERROR] %s: Invalid JSON in request body: %v", serviceName, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid JSON in request body",
		})
		return corpus, err
	}

	// Validate the corpus - now we only validate if title is missing, not text
	if corpus.Title == "" {
		log.Printf("[ERROR] %s: Missing required title field in request", serviceName)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Title is required",
		})
		return corpus, fmt.Errorf("missing title field")
	}

	return corpus, nil
}

// PrepareContextServiceInput creates a ContextInput from a Corpus and marshals it to JSON
func PrepareContextServiceInput(corpus *Corpus, vocabulary []string, subphrases [][]string) ([]byte, error) {
	// Create a structured object for context input
	contextInput := ContextInput{
		Title:      corpus.Title,
		Vocabulary: vocabulary,
		Subphrases: subphrases,
	}

	// Marshal to JSON for HTTP request
	return json.Marshal(contextInput)
}

// ProcessContextResponse converts the raw response map into a ContextResponse struct
func ProcessContextResponse(rawResponse map[string]interface{}) (*ContextResponse, error) {
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

// Todo fix
func ExtractPairsFromSubphrases(subphrases [][]string) [][]string {
	var pairs [][]string
	for _, subphrase := range subphrases {
		if len(subphrase) >= 2 {
			pair := []string{subphrase[0], subphrase[1]}
			pairs = append(pairs, pair)
		}
	}
	return pairs
}

// ProcessRemediationsResponse converts the raw response map into a RemediationResponse struct
func ProcessRemediationsResponse(rawResponse map[string]interface{}) (*RemediationResponse, error) {
	var response RemediationResponse
	if err := mapstructure.Decode(rawResponse, &response); err != nil {
		return nil, fmt.Errorf("error decoding remediations response: %w", err)
	}
	return &response, nil
}

// ProcessRemediationPairs extracts pairs from a query parameter, decodes and validates them
func ProcessRemediationPairs(c *gin.Context, serviceName string) ([][]string, error) {
	// Get the pairs parameter from the query string
	pairsParam := c.Query("pairs")
	if pairsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Missing 'pairs' parameter",
		})
		return nil, fmt.Errorf("missing pairs parameter")
	}

	// URL decode the parameter
	decodedPairs, err := url.QueryUnescape(pairsParam)
	if err != nil {
		log.Printf("[ERROR] %s: Error decoding pairs parameter: %v", serviceName, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Error decoding pairs parameter",
		})
		return nil, err
	}

	// Parse the JSON
	var pairs [][]string
	if err := json.Unmarshal([]byte(decodedPairs), &pairs); err != nil {
		log.Printf("[ERROR] %s: Invalid JSON format in pairs parameter: %v", serviceName, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Invalid JSON format in pairs parameter",
		})
		return nil, err
	}

	return pairs, nil
}

// ProcessAddRemediationRequest extracts and validates AddRemediationRequest from a request
func ProcessAddRemediationRequest(c *gin.Context, serviceName string) (AddRemediationRequest, error) {
	var request AddRemediationRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		log.Printf("[ERROR] %s: Invalid JSON in add remediation request: %v", serviceName, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid JSON in add remediation request",
		})
		return request, err
	}

	return request, nil
}

// ProcessDeleteRemediationRequest extracts and processes delete remediation request from an HTTP request
// Returns the request and an error if any occurred
func ProcessDeleteRemediationRequest(c *gin.Context, serviceName string) (DeleteRemediationRequest, error) {
	var request DeleteRemediationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Invalid request format: %v", err),
		})
		return DeleteRemediationRequest{}, fmt.Errorf("invalid request format: %w", err)
	}

	// Validate the request
	if len(request.Hashes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "No hashes provided for deletion",
		})
		return DeleteRemediationRequest{}, fmt.Errorf("no hashes provided for deletion")
	}

	return request, nil
}

// DeleteRemediationsFromStore removes remediations with matching hashes from the store
// Returns the count of deleted remediations
func DeleteRemediationsFromStore(store *RemediationMemoryStore, hashes []string) int {
	if len(hashes) == 0 {
		return 0
	}

	// Create a map for faster hash lookups
	hashMap := make(map[string]struct{})
	for _, hash := range hashes {
		hashMap[hash] = struct{}{}
	}

	// Create a new slice to hold remediations that aren't being deleted
	// Since we no longer have hashes in the RemediationTuple, we'll need to modify
	// the deletion logic. For backward compatibility, we'll treat each pair's string
	// representation as a potential match for the hashes.
	newRemediations := make([]RemediationTuple, 0, len(store.Remediations))
	deletedCount := 0

	// Add only remediations that don't match any hash
	for _, remediation := range store.Remediations {
		// Create a string from the pair that could be used as a hash equivalent
		pairKey := createPairKey(remediation.Pair)
		if _, shouldDelete := hashMap[pairKey]; !shouldDelete {
			newRemediations = append(newRemediations, remediation)
		} else {
			deletedCount++
		}
	}

	// Replace the old remediations slice with the filtered one
	store.Remediations = newRemediations
	return deletedCount
}
