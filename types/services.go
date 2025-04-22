package types

import "context"

// This file contains service interfaces but implementations moved to clients package
// to avoid circular dependencies

// ContextService defines the interface for interacting with the context service
type ContextService interface {
	SendMessage(ctx context.Context, input ContextInput) (ContextResponse, error)
	GetVersion(ctx context.Context) (VersionResponse, error)
	GetContext(ctx context.Context) (ContextDataResponse, error)
}

// VersionResponse represents the response from the context service's version endpoint
type VersionResponse struct {
	Version int `json:"version"`
}

// ContextDataResponse represents the response from the context service's get context endpoint
type ContextDataResponse struct {
	Version    int        `json:"version"`
	Vocabulary []string   `json:"vocabulary"`
	Lines      [][]string `json:"lines"`
}

// RemediationsService defines the interface for interacting with the remediations service
type RemediationsService interface {
	FetchRemediations(ctx context.Context, request RemediationRequest) (RemediationResponse, error)
	DeleteRemediations(ctx context.Context, hashes []string) (DeleteRemediationResponse, error)
	AddRemediations(ctx context.Context, remediations []RemediationTuple) (AddRemediationResponse, error)
}

// OrthosService defines the interface for interacting with the orthos service
type OrthosService interface {
	GetOrthosByIDs(ctx context.Context, ids []string) (OrthosResponse, error)
	SaveOrthos(ctx context.Context, orthos []Ortho) (OrthosSaveResponse, error)
}

// OrthosSaveResponse represents the response after saving orthos
type OrthosSaveResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// WorkServerService defines the interface for interacting with the work server
type WorkServerService interface {
	PushOrthos(ctx context.Context, orthos []Ortho) (WorkServerPushResponse, error)
	Pop(ctx context.Context) (WorkServerPopResponse, error)
	Ack(ctx context.Context, id string) (WorkServerAckResponse, error)
	Nack(ctx context.Context, id string) (WorkServerAckResponse, error)
}

// WorkServerPopResponse represents the response when popping an item from the work queue
type WorkServerPopResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Ortho   *Ortho `json:"ortho"`
	ID      string `json:"id"`
}

// WorkServerAckResponse represents the response after acknowledging a work item
type WorkServerAckResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// SearchService defines the interface for interacting with the search service
type SearchService interface {
	Search(ctx context.Context, request SearchRequest) (SearchResponse, error)
}

// SearchRequest represents a search query
type SearchRequest struct {
	Query      string `json:"query"`
	MaxResults int    `json:"maxResults,omitempty"`
	Offset     int    `json:"offset,omitempty"`
}

// SearchResult represents a single search result
type SearchResult struct {
	OrthoID      string      `json:"orthoId"`
	Score        float64     `json:"score"`
	OrthoData    interface{} `json:"orthoData,omitempty"`
	Remediations interface{} `json:"remediations,omitempty"`
}

// SearchResponse represents the response to a search query
type SearchResponse struct {
	Results    []SearchResult `json:"results"`
	TotalCount int            `json:"totalCount"`
	QueryTime  float64        `json:"queryTimeMs"`
}

// OrthosGetRequest represents a request to fetch orthos by IDs
type OrthosGetRequest struct {
	IDs []string `json:"ids"`
}

// RemediationsGetRequest represents a request to fetch remediations by ortho IDs
type RemediationsGetRequest struct {
	OrthoIDs []string `json:"orthoIds"`
}
