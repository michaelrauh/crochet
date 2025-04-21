package types

// This file contains service interfaces but implementations moved to clients package
// to avoid circular dependencies

// ContextService defines the interface for interacting with the context service
type ContextService interface {
	SendMessage(input ContextInput) (ContextResponse, error)
}

// RemediationsService defines the interface for interacting with the remediations service
type RemediationsService interface {
	FetchRemediations(request RemediationRequest) (RemediationResponse, error)
}
