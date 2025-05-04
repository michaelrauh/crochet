package types

import "context"

type ContextService interface {
	SendMessage(ctx context.Context, input ContextInput) (ContextResponse, error)
	GetVersion(ctx context.Context) (VersionResponse, error)
	GetContext(ctx context.Context) (ContextDataResponse, error)
	UpdateVersion(ctx context.Context, request VersionUpdateRequest) (VersionUpdateResponse, error)
}

// VersionResponse represents the response from the context service's version endpoint
type VersionResponse struct {
	Version int `json:"version"`
}

// VersionUpdateRequest represents a request to update the version in the database
type VersionUpdateRequest struct {
	Version int `json:"version"`
}

// VersionUpdateResponse represents the response after updating the version in the database
type VersionUpdateResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Version int    `json:"version"`
}

// ContextDataResponse represents the response from the context service's get context endpoint
type ContextDataResponse struct {
	Version    int        `json:"version"`
	Vocabulary []string   `json:"vocabulary"`
	Lines      [][]string `json:"lines"`
}

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
	OrthoID      string  `json:"orthoId"`
	Score        float64 `json:"score"`
	OrthoData    any     `json:"orthoData,omitempty"`    // Use any instead of interface{}
	Remediations any     `json:"remediations,omitempty"` // Use any instead of interface{}
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

// RabbitMQService defines methods for interacting with RabbitMQ
type RabbitMQService interface {
	// PushContext pushes context data to a RabbitMQ queue
	PushContext(ctx context.Context, contextInput ContextInput) error

	// PushVersion pushes version info to a RabbitMQ queue
	PushVersion(ctx context.Context, version VersionInfo) error

	// PushPairs pushes a batch of text pairs to a RabbitMQ queue
	PushPairs(ctx context.Context, pairs []Pair) error

	// PushSeed pushes seed ortho data to a RabbitMQ queue
	PushSeed(ctx context.Context, seed Ortho) error
}

// RepositoryService defines the interface for interacting with the repository service
// This implements the worker_process.md design where the Worker interfaces with a single Repository service
type RepositoryService interface {
	GetWork(ctx context.Context) (*WorkItem, error)
	GetContext(ctx context.Context) (*ContextDataResponse, error)
	PostResults(ctx context.Context, orthos []Ortho, remediations []RemediationTuple) (*ResultsResponse, error)
}
