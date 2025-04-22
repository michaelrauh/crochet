package clients

import (
	"bytes"
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

// GetVersion gets the current version from the context service
func (s *ContextServiceClient) GetVersion(ctx context.Context) (types.VersionResponse, error) {
	// Make GET request to the context service version endpoint
	serviceResp := s.Client.Call(ctx, http.MethodGet, s.URL+"/version", nil)
	if serviceResp.Error != nil {
		return types.VersionResponse{}, fmt.Errorf("error calling context version endpoint: %w", serviceResp.Error)
	}

	log.Printf("Received context version response: %v", serviceResp.RawResponse)

	var response types.VersionResponse
	if err := mapResponseToStruct(serviceResp.RawResponse, &response); err != nil {
		return types.VersionResponse{}, fmt.Errorf("error parsing context version response: %w", err)
	}

	return response, nil
}

// GetContext gets the current context data from the context service
func (s *ContextServiceClient) GetContext(ctx context.Context) (types.ContextDataResponse, error) {
	// Make GET request to the context service context data endpoint
	serviceResp := s.Client.Call(ctx, http.MethodGet, s.URL+"/context", nil)
	if serviceResp.Error != nil {
		return types.ContextDataResponse{}, fmt.Errorf("error calling context data endpoint: %w", serviceResp.Error)
	}

	log.Printf("Received context data response: %v", serviceResp.RawResponse)

	var response types.ContextDataResponse
	if err := mapResponseToStruct(serviceResp.RawResponse, &response); err != nil {
		return types.ContextDataResponse{}, fmt.Errorf("error parsing context data response: %w", err)
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

// DeleteRemediations sends a request to delete specific remediations by their hashes
func (s *RemediationsServiceClient) DeleteRemediations(ctx context.Context, hashes []string) (types.DeleteRemediationResponse, error) {
	// Create the request body with hashes to delete
	requestBody := map[string][]string{
		"hashes": hashes,
	}

	// Marshal to JSON for the HTTP request
	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return types.DeleteRemediationResponse{}, fmt.Errorf("error marshaling delete remediations request: %w", err)
	}

	// Make POST request to the remediations delete endpoint
	serviceResp := s.Client.Call(ctx, http.MethodPost, s.URL+"/delete", requestJSON)
	if serviceResp.Error != nil {
		return types.DeleteRemediationResponse{}, fmt.Errorf("error calling remediations delete endpoint: %w", serviceResp.Error)
	}

	log.Printf("Received remediations delete response: %v", serviceResp.RawResponse)

	var response types.DeleteRemediationResponse
	if err := mapResponseToStruct(serviceResp.RawResponse, &response); err != nil {
		return types.DeleteRemediationResponse{}, fmt.Errorf("error parsing remediations delete response: %w", err)
	}

	return response, nil
}

// AddRemediations sends a request to add new remediations
func (s *RemediationsServiceClient) AddRemediations(ctx context.Context, remediations []types.RemediationTuple) (types.AddRemediationResponse, error) {
	// Create the request body with remediations to add
	requestBody := map[string][]types.RemediationTuple{
		"remediations": remediations,
	}

	// Marshal to JSON for the HTTP request
	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return types.AddRemediationResponse{}, fmt.Errorf("error marshaling add remediations request: %w", err)
	}

	// Make POST request to the remediations add endpoint
	serviceResp := s.Client.Call(ctx, http.MethodPost, s.URL+"/add", requestJSON)
	if serviceResp.Error != nil {
		return types.AddRemediationResponse{}, fmt.Errorf("error calling remediations add endpoint: %w", serviceResp.Error)
	}

	log.Printf("Received remediations add response: %v", serviceResp.RawResponse)

	var response types.AddRemediationResponse
	if err := mapResponseToStruct(serviceResp.RawResponse, &response); err != nil {
		return types.AddRemediationResponse{}, fmt.Errorf("error parsing remediations add response: %w", err)
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

// SaveOrthos sends a request to save new orthos
func (s *OrthosServiceClient) SaveOrthos(ctx context.Context, orthos []types.Ortho) (types.OrthosSaveResponse, error) {
	// Create the request body with orthos to save
	requestBody := map[string][]types.Ortho{
		"orthos": orthos,
	}

	// Marshal to JSON for the HTTP request
	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return types.OrthosSaveResponse{}, fmt.Errorf("error marshaling save orthos request: %w", err)
	}

	// Make POST request to the orthos service save endpoint
	serviceResp := s.Client.Call(ctx, http.MethodPost, s.URL+"/orthos", requestJSON)
	if serviceResp.Error != nil {
		return types.OrthosSaveResponse{}, fmt.Errorf("error calling orthos save endpoint: %w", serviceResp.Error)
	}

	log.Printf("Received orthos save response: %v", serviceResp.RawResponse)

	var response types.OrthosSaveResponse
	if err := mapResponseToStruct(serviceResp.RawResponse, &response); err != nil {
		return types.OrthosSaveResponse{}, fmt.Errorf("error parsing orthos save response: %w", err)
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

// Pop gets the next work item from the work server
func (s *WorkServerServiceClient) Pop(ctx context.Context) (types.WorkServerPopResponse, error) {
	// Make POST request to the work server pop endpoint
	serviceResp := s.Client.Call(ctx, http.MethodPost, s.URL+"/pop", nil)
	if serviceResp.Error != nil {
		return types.WorkServerPopResponse{}, fmt.Errorf("error calling work server pop endpoint: %w", serviceResp.Error)
	}

	log.Printf("Received work server pop response: %v", serviceResp.RawResponse)

	var response types.WorkServerPopResponse
	if err := mapResponseToStruct(serviceResp.RawResponse, &response); err != nil {
		return types.WorkServerPopResponse{}, fmt.Errorf("error parsing work server pop response: %w", err)
	}

	return response, nil
}

// Ack acknowledges that a work item has been processed
func (s *WorkServerServiceClient) Ack(ctx context.Context, id string) (types.WorkServerAckResponse, error) {
	// Create the request body with the ID to acknowledge
	requestBody := map[string]string{
		"id": id,
	}

	// Marshal to JSON for the HTTP request
	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return types.WorkServerAckResponse{}, fmt.Errorf("error marshaling work server ack request: %w", err)
	}

	// Make POST request to the work server ack endpoint
	serviceResp := s.Client.Call(ctx, http.MethodPost, s.URL+"/ack", requestJSON)
	if serviceResp.Error != nil {
		return types.WorkServerAckResponse{}, fmt.Errorf("error calling work server ack endpoint: %w", serviceResp.Error)
	}

	log.Printf("Received work server ack response: %v", serviceResp.RawResponse)

	var response types.WorkServerAckResponse
	if err := mapResponseToStruct(serviceResp.RawResponse, &response); err != nil {
		return types.WorkServerAckResponse{}, fmt.Errorf("error parsing work server ack response: %w", err)
	}

	return response, nil
}

// Nack marks a work item as not processed and returns it to the queue
func (s *WorkServerServiceClient) Nack(ctx context.Context, id string) (types.WorkServerAckResponse, error) {
	// Create the request body with the ID to negative acknowledge
	requestBody := map[string]string{
		"id": id,
	}

	// Marshal to JSON for the HTTP request
	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return types.WorkServerAckResponse{}, fmt.Errorf("error marshaling work server nack request: %w", err)
	}

	// Make POST request to the work server nack endpoint
	serviceResp := s.Client.Call(ctx, http.MethodPost, s.URL+"/nack", requestJSON)
	if serviceResp.Error != nil {
		return types.WorkServerAckResponse{}, fmt.Errorf("error calling work server nack endpoint: %w", serviceResp.Error)
	}

	log.Printf("Received work server nack response: %v", serviceResp.RawResponse)

	var response types.WorkServerAckResponse
	if err := mapResponseToStruct(serviceResp.RawResponse, &response); err != nil {
		return types.WorkServerAckResponse{}, fmt.Errorf("error parsing work server nack response: %w", err)
	}

	return response, nil
}

// SearchClient implements types.SearchService
type SearchClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewSearchClient creates a new client for the search service
func NewSearchClient(baseURL string) types.SearchService {
	return &SearchClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// NewSearchClientWithHTTPClient creates a new client for the search service with a custom HTTP client
func NewSearchClientWithHTTPClient(baseURL string, httpClient *http.Client) types.SearchService {
	return &SearchClient{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// Search sends a search request to the search service
func (c *SearchClient) Search(ctx context.Context, request types.SearchRequest) (types.SearchResponse, error) {
	reqBody, err := json.Marshal(request)
	if err != nil {
		return types.SearchResponse{}, fmt.Errorf("error marshaling search request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/search", c.baseURL), bytes.NewBuffer(reqBody))
	if err != nil {
		return types.SearchResponse{}, fmt.Errorf("error creating search request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.SearchResponse{}, fmt.Errorf("error sending search request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.SearchResponse{}, fmt.Errorf("search service returned status code %d", resp.StatusCode)
	}

	var searchResponse types.SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
		return types.SearchResponse{}, fmt.Errorf("error decoding search response: %v", err)
	}

	return searchResponse, nil
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
