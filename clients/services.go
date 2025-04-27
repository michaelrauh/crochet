package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"crochet/httpclient"
	"crochet/types"
)

type ContextServiceClient struct {
	URL           string
	Client        *httpclient.GenericClient[types.ContextResponse]
	VersionClient *httpclient.GenericClient[types.VersionResponse]
	DataClient    *httpclient.GenericClient[types.ContextDataResponse]
}

type RemediationsServiceClient struct {
	URL          string
	Client       *httpclient.GenericClient[types.RemediationResponse]
	DeleteClient *httpclient.GenericClient[types.DeleteRemediationResponse]
	AddClient    *httpclient.GenericClient[types.AddRemediationResponse]
}

type OrthosServiceClient struct {
	URL        string
	GetClient  *httpclient.GenericClient[types.OrthosResponse]
	SaveClient *httpclient.GenericClient[types.OrthosSaveResponse]
}

func (s *ContextServiceClient) SendMessage(ctx context.Context, input types.ContextInput) (types.ContextResponse, error) {
	requestJSON, err := json.Marshal(input)
	if err != nil {
		return types.ContextResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	response, err := s.Client.GenericCall(ctx, http.MethodPost, s.URL+"/input", requestJSON)
	if err != nil {
		return types.ContextResponse{}, fmt.Errorf("service call failed: %w", err)
	}

	return response, nil
}

func (s *ContextServiceClient) GetVersion(ctx context.Context) (types.VersionResponse, error) {
	response, err := s.VersionClient.GenericCall(ctx, http.MethodGet, s.URL+"/version", nil)
	if err != nil {
		return types.VersionResponse{}, fmt.Errorf("error calling context version endpoint: %w", err)
	}

	return response, nil
}

func (s *ContextServiceClient) GetContext(ctx context.Context) (types.ContextDataResponse, error) {
	response, err := s.DataClient.GenericCall(ctx, http.MethodGet, s.URL+"/context", nil)
	if err != nil {
		return types.ContextDataResponse{}, fmt.Errorf("error calling context data endpoint: %w", err)
	}
	return response, nil
}

func (s *RemediationsServiceClient) FetchRemediations(ctx context.Context, request types.RemediationRequest) (types.RemediationResponse, error) {
	remediationTuples := make([]types.RemediationTuple, len(request.Pairs))
	for i, pair := range request.Pairs {
		remediationTuples[i] = types.RemediationTuple{
			Pair: pair,
		}
	}

	requestJSON, err := json.Marshal(remediationTuples)
	if err != nil {
		return types.RemediationResponse{}, fmt.Errorf("error marshaling remediation request: %w", err)
	}

	response, err := s.Client.GenericCall(ctx, http.MethodPost, s.URL+"/remediations", requestJSON)
	if err != nil {
		return types.RemediationResponse{}, fmt.Errorf("error calling remediations service: %w", err)
	}
	return response, nil
}

func (s *RemediationsServiceClient) DeleteRemediations(ctx context.Context, hashes []string) (types.DeleteRemediationResponse, error) {
	requestBody := map[string][]string{
		"hashes": hashes,
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return types.DeleteRemediationResponse{}, fmt.Errorf("error marshaling delete remediations request: %w", err)
	}

	response, err := s.DeleteClient.GenericCall(ctx, http.MethodPost, s.URL+"/delete", requestJSON)
	if err != nil {
		return types.DeleteRemediationResponse{}, fmt.Errorf("error calling remediations delete endpoint: %w", err)
	}

	return response, nil
}

func (s *RemediationsServiceClient) AddRemediations(ctx context.Context, remediations []types.RemediationTuple) (types.AddRemediationResponse, error) {
	requestJSON, err := json.Marshal(remediations)
	if err != nil {
		return types.AddRemediationResponse{}, fmt.Errorf("error marshaling add remediations request: %w", err)
	}

	response, err := s.AddClient.GenericCall(ctx, http.MethodPost, s.URL+"/remediations", requestJSON)

	return response, nil
}

func (s *OrthosServiceClient) GetOrthosByIDs(ctx context.Context, ids []string) (types.OrthosResponse, error) {
	requestBody := map[string][]string{
		"ids": ids,
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return types.OrthosResponse{}, fmt.Errorf("error marshaling orthos request: %w", err)
	}

	response, err := s.GetClient.GenericCall(ctx, http.MethodPost, s.URL+"/orthos/get", requestJSON)
	if err != nil {
		return types.OrthosResponse{}, fmt.Errorf("error calling orthos service: %w", err)
	}

	return response, nil
}

func (s *OrthosServiceClient) SaveOrthos(ctx context.Context, orthos []types.Ortho) (types.OrthosSaveResponse, error) {
	requestBody := map[string][]types.Ortho{
		"orthos": orthos,
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return types.OrthosSaveResponse{}, fmt.Errorf("error marshaling save orthos request: %w", err)
	}

	response, err := s.SaveClient.GenericCall(ctx, http.MethodPost, s.URL+"/orthos", requestJSON)
	if err != nil {
		return types.OrthosSaveResponse{}, fmt.Errorf("error calling orthos save endpoint: %w", err)
	}

	return response, nil
}

type WorkServerServiceClient struct {
	URL    string
	Client *httpclient.Client
}

// TODO Fix
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

// TODO Fix
func (s *WorkServerServiceClient) Pop(ctx context.Context) (types.WorkServerPopResponse, error) {
	// Make POST request to the work server pop endpoint
	serviceResp := s.Client.Call(ctx, http.MethodPost, s.URL+"/pop", nil)
	if serviceResp.Error != nil {
		return types.WorkServerPopResponse{}, fmt.Errorf("error calling work server pop endpoint: %w", serviceResp.Error)
	}

	log.Printf("Received work server pop response: %v", serviceResp.RawResponse)

	// Process the response manually to handle the Ortho object correctly
	var response types.WorkServerPopResponse
	if err := mapResponseToWorkServerPop(serviceResp.RawResponse, &response); err != nil {
		return types.WorkServerPopResponse{}, fmt.Errorf("error parsing work server pop response: %w", err)
	}

	return response, nil
}

// TODO fix
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

// TODO fix
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

type SearchClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewSearchClient(baseURL string) types.SearchService {
	return &SearchClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// TODO fix
func NewSearchClientWithHTTPClient(baseURL string, httpClient *http.Client) types.SearchService {
	return &SearchClient{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// TODO fix
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

func NewContextService(url string, client *httpclient.GenericClient[types.ContextResponse], versionClient *httpclient.GenericClient[types.VersionResponse], dataClient *httpclient.GenericClient[types.ContextDataResponse]) types.ContextService {
	return &ContextServiceClient{
		URL:           url,
		Client:        client,
		VersionClient: versionClient,
		DataClient:    dataClient,
	}
}

func NewRemediationsService(url string, client *httpclient.GenericClient[types.RemediationResponse], deleteClient *httpclient.GenericClient[types.DeleteRemediationResponse], AddClient *httpclient.GenericClient[types.AddRemediationResponse]) types.RemediationsService {
	return &RemediationsServiceClient{
		URL:          url,
		Client:       client,
		DeleteClient: deleteClient,
		AddClient:    AddClient,
	}
}

func NewOrthosService(url string, getClient *httpclient.GenericClient[types.OrthosResponse], saveClient *httpclient.GenericClient[types.OrthosSaveResponse]) types.OrthosService {
	return &OrthosServiceClient{
		URL:        url,
		GetClient:  getClient,
		SaveClient: saveClient,
	}
}

func NewWorkServerService(url string, client *httpclient.Client) types.WorkServerService {
	return &WorkServerServiceClient{
		URL:    url,
		Client: client,
	}
}

// TODO fix
func mapResponseToStruct(rawResponse map[string]interface{}, target interface{}) error {
	// If rawResponse is empty, try to initialize some default values
	if len(rawResponse) == 0 {
		// For common response types, initialize with empty values
		switch v := target.(type) {
		case *types.RemediationResponse:
			v.Status = "success"
			v.Hashes = []string{}
			return nil
		case *types.OrthosResponse:
			v.Status = "success"
			v.Orthos = []types.Ortho{}
			return nil
		case *types.OrthosSaveResponse:
			v.Status = "success"
			// We can't access NewIDs directly - handle this case differently
			return nil
		}
	}

	// Special handling for OrthosSaveResponse before general unmarshaling
	if _, ok := target.(*types.OrthosSaveResponse); ok {
		resp := target.(*types.OrthosSaveResponse)

		if status, ok := rawResponse["status"].(string); ok {
			resp.Status = status
		}

		if message, ok := rawResponse["message"].(string); ok {
			resp.Message = message
		}

		if count, ok := rawResponse["count"].(float64); ok {
			resp.Count = int(count)
		}

		// Store newIDs in a slice that we'll pass back to the caller
		if newIDsRaw, ok := rawResponse["newIDs"].([]interface{}); ok {
			newIDs := make([]string, len(newIDsRaw))
			for i, id := range newIDsRaw {
				if strID, ok := id.(string); ok {
					newIDs[i] = strID
				}
			}
			// Store in a field that actually exists
			resp.NewIDs = newIDs
		}

		return nil
	}

	// Continue with regular marshaling for other types
	jsonData, err := json.Marshal(rawResponse)
	if err != nil {
		return fmt.Errorf("error re-marshaling response: %w", err)
	}

	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("error unmarshaling response: %w", err)
	}

	return nil
}

// TODO fix
func mapResponseToOrthos(rawResponse map[string]interface{}, response *types.OrthosResponse) error {
	// Map the status, message, and count fields
	if status, ok := rawResponse["status"].(string); ok {
		response.Status = status
	}
	if message, ok := rawResponse["message"].(string); ok {
		response.Message = message
	}
	if count, ok := rawResponse["count"].(float64); ok {
		response.Count = int(count)
	}

	// Handle the orthos array
	if orthosRaw, ok := rawResponse["orthos"].([]interface{}); ok {
		response.Orthos = make([]types.Ortho, len(orthosRaw))

		for i, orthoRaw := range orthosRaw {
			if orthoMap, ok := orthoRaw.(map[string]interface{}); ok {
				// Map the basic fields
				if id, ok := orthoMap["id"].(string); ok {
					response.Orthos[i].ID = id
				}

				if shapeRaw, ok := orthoMap["shape"].([]interface{}); ok {
					response.Orthos[i].Shape = make([]int, len(shapeRaw))
					for j, dim := range shapeRaw {
						if dimFloat, ok := dim.(float64); ok {
							response.Orthos[i].Shape[j] = int(dimFloat)
						}
					}
				}

				if posRaw, ok := orthoMap["position"].([]interface{}); ok {
					response.Orthos[i].Position = make([]int, len(posRaw))
					for j, pos := range posRaw {
						if posFloat, ok := pos.(float64); ok {
							response.Orthos[i].Position[j] = int(posFloat)
						}
					}
				}

				if shell, ok := orthoMap["shell"].(float64); ok {
					response.Orthos[i].Shell = int(shell)
				}

				// Convert grid from map[string]interface{} to map[string]string
				if gridRaw, ok := orthoMap["grid"].(map[string]interface{}); ok {
					grid := make(map[string]string)
					for k, v := range gridRaw {
						if strVal, ok := v.(string); ok {
							grid[k] = strVal
						} else {
							// Convert non-string values to strings
							grid[k] = fmt.Sprintf("%v", v)
						}
					}
					response.Orthos[i].Grid = grid
				} else {
					response.Orthos[i].Grid = make(map[string]string)
				}
			}
		}
	}

	return nil
}

// TODO fix
func mapResponseToWorkServerPop(rawResponse map[string]interface{}, response *types.WorkServerPopResponse) error {
	// Map the status and message fields
	if status, ok := rawResponse["status"].(string); ok {
		response.Status = status
	}
	if message, ok := rawResponse["message"].(string); ok {
		response.Message = message
	}
	if id, ok := rawResponse["id"].(string); ok {
		response.ID = id
	}

	// Handle the ortho object if present
	if orthoRaw, ok := rawResponse["ortho"].(map[string]interface{}); ok && orthoRaw != nil {
		ortho := &types.Ortho{}

		// Map the basic fields
		if id, ok := orthoRaw["id"].(string); ok {
			ortho.ID = id
		}

		if shapeRaw, ok := orthoRaw["shape"].([]interface{}); ok {
			ortho.Shape = make([]int, len(shapeRaw))
			for j, dim := range shapeRaw {
				if dimFloat, ok := dim.(float64); ok {
					ortho.Shape[j] = int(dimFloat)
				}
			}
		}

		if posRaw, ok := orthoRaw["position"].([]interface{}); ok {
			ortho.Position = make([]int, len(posRaw))
			for j, pos := range posRaw {
				if posFloat, ok := pos.(float64); ok {
					ortho.Position[j] = int(posFloat)
				}
			}
		}

		if shell, ok := orthoRaw["shell"].(float64); ok {
			ortho.Shell = int(shell)
		}

		// Convert grid from map[string]interface{} to map[string]string
		if gridRaw, ok := orthoRaw["grid"].(map[string]interface{}); ok {
			grid := make(map[string]string)
			for k, v := range gridRaw {
				if strVal, ok := v.(string); ok {
					grid[k] = strVal
				} else {
					// Convert non-string values to strings
					grid[k] = fmt.Sprintf("%v", v)
				}
			}
			ortho.Grid = grid
		} else {
			ortho.Grid = make(map[string]string)
		}

		response.Ortho = ortho
	} else {
		response.Ortho = nil
	}

	return nil
}
