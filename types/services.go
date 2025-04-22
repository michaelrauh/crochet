package types

import "context"

// This file contains service interfaces but implementations moved to clients package
// to avoid circular dependencies

// ContextService defines the interface for interacting with the context service
type ContextService interface {
	SendMessage(ctx context.Context, input ContextInput) (ContextResponse, error)
}

// RemediationsService defines the interface for interacting with the remediations service
type RemediationsService interface {
	FetchRemediations(ctx context.Context, request RemediationRequest) (RemediationResponse, error)
}

// OrthosService defines the interface for interacting with the orthos service
type OrthosService interface {
	GetOrthosByIDs(ctx context.Context, ids []string) (OrthosResponse, error)
}
