package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"crochet/httpclient"
	"crochet/types"
)

// ContextServiceClient implements the types.ContextService interface
type ContextServiceClient struct {
	URL    string
	Client *httpclient.Client
}

// SendMessage sends data to the context service and returns the response
func (s *ContextServiceClient) SendMessage(ctx context.Context, input types.ContextInput) (types.ContextResponse, error) {
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
func (s *RemediationsServiceClient) FetchRemediations(ctx context.Context, request types.RemediationRequest) (types.RemediationResponse, error) {
	// Marshal pairs to JSON
	pairsJSON, err := json.Marshal(request.Pairs)
	if err != nil {
		return types.RemediationResponse{}, fmt.Errorf("error marshaling pairs: %w", err)
	}

	// URL encode the JSON for use in a query parameter
	encodedPairs := url.QueryEscape(string(pairsJSON))

	// Build the URL with the query parameter
	requestURL := fmt.Sprintf("%s/?pairs=%s", s.URL, encodedPairs)

	// Make GET request to the remediations service
	serviceResp := s.Client.Call(ctx, http.MethodGet, requestURL, nil)
	if serviceResp.Error != nil {
		return types.RemediationResponse{}, fmt.Errorf("error calling remediations service: %w", serviceResp.Error)
	}

	log.Printf("Received remediations service raw response: %v", serviceResp.RawResponse)

	var response types.RemediationResponse
	if err := mapResponseToStruct(serviceResp.RawResponse, &response); err != nil {
		return types.RemediationResponse{}, fmt.Errorf("error parsing remediations response: %w", err)
	}

	return response, nil
}

// OrthosServiceClient implements the types.OrthosService interface
type OrthosServiceClient struct {
	URL    string
	Client *httpclient.Client
}

// GetOrthosByIDs sends request to the orthos service to retrieve orthos by their IDs
func (s *OrthosServiceClient) GetOrthosByIDs(ctx context.Context, ids []string) (types.OrthosResponse, error) {
	// Create the request body with IDs
	requestBody := map[string][]string{
		"ids": ids,
	}

	// Marshal to JSON for the HTTP request
	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return types.OrthosResponse{}, fmt.Errorf("error marshaling orthos request: %w", err)
	}

	// Make POST request to the orthos service
	serviceResp := s.Client.Call(ctx, http.MethodPost, s.URL+"/orthos/get", requestJSON)
	if serviceResp.Error != nil {
		return types.OrthosResponse{}, fmt.Errorf("error calling orthos service: %w", serviceResp.Error)
	}

	log.Printf("Received orthos service raw response: %v", serviceResp.RawResponse)

	var response types.OrthosResponse
	if err := mapResponseToStruct(serviceResp.RawResponse, &response); err != nil {
		return types.OrthosResponse{}, fmt.Errorf("error parsing orthos response: %w", err)
	}

	return response, nil
}

// WorkServerServiceClient implements the types.WorkServerService interface
type WorkServerServiceClient struct {
	URL    string
	Client *httpclient.Client
}

// PushOrthos sends a request to push orthos to the work server
func (s *WorkServerServiceClient) PushOrthos(ctx context.Context, orthos []types.Ortho) (types.WorkServerPushResponse, error) {
	// Create the request body with orthos
	requestBody := map[string][]types.Ortho{
		"orthos": orthos,
	}

	// Marshal to JSON for the HTTP request
	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return types.WorkServerPushResponse{}, fmt.Errorf("error marshaling work server push request: %w", err)
	}

	// Make POST request to the work server push endpoint
	serviceResp := s.Client.Call(ctx, http.MethodPost, s.URL+"/push", requestJSON)
	if serviceResp.Error != nil {
		return types.WorkServerPushResponse{}, fmt.Errorf("error calling work server: %w", serviceResp.Error)
	}

	log.Printf("Received work server push response: %v", serviceResp.RawResponse)

	var response types.WorkServerPushResponse
	if err := mapResponseToStruct(serviceResp.RawResponse, &response); err != nil {
		return types.WorkServerPushResponse{}, fmt.Errorf("error parsing work server push response: %w", err)
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

// NewOrthosService creates a new orthos service client
func NewOrthosService(url string, client *httpclient.Client) types.OrthosService {
	return &OrthosServiceClient{
		URL:    url,
		Client: client,
	}
}

// NewWorkServerService creates a new work server client
func NewWorkServerService(url string, client *httpclient.Client) types.WorkServerService {
	return &WorkServerServiceClient{
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
