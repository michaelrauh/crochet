package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"crochet/httpclient"
	"crochet/types"
)

// ContextServiceClient implements the types.ContextService interface
type ContextServiceClient struct {
	URL    string
	Client *httpclient.Client
}

// SendMessage sends data to the context service and returns the response
func (s *ContextServiceClient) SendMessage(input types.ContextInput) (types.ContextResponse, error) {
	ctx := context.Background()

	// Marshal to JSON for the HTTP request
	requestJSON, err := json.Marshal(input)
	if err != nil {
		return types.ContextResponse{}, fmt.Errorf("error marshaling context request: %w", err)
	}

	serviceResp := s.Client.Call(ctx, http.MethodPost, s.URL+"/input", requestJSON)
	if serviceResp.Error != nil {
		return types.ContextResponse{}, fmt.Errorf("error calling context service: %w", serviceResp.Error)
	}

	log.Printf("Received context service raw response: %v", serviceResp.RawResponse)

	var response types.ContextResponse
	if err := mapResponseToStruct(serviceResp.RawResponse, &response); err != nil {
		return types.ContextResponse{}, fmt.Errorf("error parsing context response: %w", err)
	}

	return response, nil
}

// RemediationsServiceClient implements the types.RemediationsService interface
type RemediationsServiceClient struct {
	URL    string
	Client *httpclient.Client
}

// FetchRemediations sends request to the remediations service and returns the response
func (s *RemediationsServiceClient) FetchRemediations(request types.RemediationRequest) (types.RemediationResponse, error) {
	ctx := context.Background()

	// Marshal to JSON for the HTTP request
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return types.RemediationResponse{}, fmt.Errorf("error marshaling remediations request: %w", err)
	}

	serviceResp := s.Client.Call(ctx, http.MethodPost, s.URL+"/remediate", requestJSON)
	if serviceResp.Error != nil {
		return types.RemediationResponse{}, fmt.Errorf("error calling remediations service: %w", serviceResp.Error)
	}

	var response types.RemediationResponse
	if err := mapResponseToStruct(serviceResp.RawResponse, &response); err != nil {
		return types.RemediationResponse{}, fmt.Errorf("error parsing remediations response: %w", err)
	}

	return response, nil
}

// NewContextService creates a new context service client
func NewContextService(url string, client *httpclient.Client) types.ContextService {
	return &ContextServiceClient{
		URL:    url,
		Client: client,
	}
}

// NewRemediationsService creates a new remediations service client
func NewRemediationsService(url string, client *httpclient.Client) types.RemediationsService {
	return &RemediationsServiceClient{
		URL:    url,
		Client: client,
	}
}

// Helper function to map a raw response to a struct
func mapResponseToStruct(rawResponse map[string]interface{}, target interface{}) error {
	jsonData, err := json.Marshal(rawResponse)
	if err != nil {
		return fmt.Errorf("error re-marshaling response: %w", err)
	}

	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("error unmarshaling response: %w", err)
	}

	return nil
}
