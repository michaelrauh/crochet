package types

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"crochet/httpclient"
)

// ContextService defines the interface for interacting with the context service
type ContextService interface {
	SendMessage(ctx context.Context, message string) (map[string]interface{}, error)
}

// RealContextService implements the ContextService interface
type RealContextService struct {
	URL    string
	Client *httpclient.Client
}

// SendMessage sends a message to the context service and returns the response
func (s *RealContextService) SendMessage(ctx context.Context, message string) (map[string]interface{}, error) {
	serviceResp := s.Client.Call(ctx, http.MethodPost, s.URL+"/input", []byte(message))
	if serviceResp.Error != nil {
		return nil, fmt.Errorf("error calling context service: %w", serviceResp.Error)
	}

	log.Printf("Parsed context service response: %v", serviceResp.RawResponse)
	return serviceResp.RawResponse, nil
}

// RemediationsService defines the interface for interacting with the remediations service
type RemediationsService interface {
	FetchRemediations(ctx context.Context, subphrases [][]string) (map[string]interface{}, error)
}

// RealRemediationsService implements the RemediationsService interface
type RealRemediationsService struct {
	URL    string
	Client *httpclient.Client
}

// FetchRemediations sends subphrases to the remediations service and returns the response
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

	// Marshal to JSON for the HTTP request
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
