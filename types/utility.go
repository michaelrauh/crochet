package types

import (
	"encoding/json"
	"fmt"
	"net/http"

	"crochet/telemetry"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
)

// ProcessIncomingCorpus processes the JSON payload from the request and returns a Corpus
func ProcessIncomingCorpus(c *gin.Context, serviceName string) (*Corpus, error) {
	var rawData map[string]interface{}
	if err := c.ShouldBindJSON(&rawData); err != nil {
		return nil, telemetry.NewServiceError(serviceName, http.StatusBadRequest, "Invalid JSON format")
	}

	// Convert raw data to Corpus struct using mapstructure
	var corpus Corpus
	if err := mapstructure.Decode(rawData, &corpus); err != nil {
		return nil, telemetry.NewServiceError(serviceName, http.StatusBadRequest, "Invalid data format")
	}

	return &corpus, nil
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

// ExtractPairsFromSubphrases extracts pairs (subphrases of length 2) from a list of subphrases
func ExtractPairsFromSubphrases(subphrases [][]string) [][]string {
	var pairs [][]string
	for _, subphrase := range subphrases {
		if len(subphrase) == 2 {
			pairs = append(pairs, subphrase)
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
