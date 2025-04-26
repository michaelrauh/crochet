package main

import (
	"crochet/types"
	"time"
)

// WorkItem represents an item in the work queue
type WorkItem struct {
	Ortho        types.Ortho `json:"ortho"`
	EnqueuedTime time.Time   `json:"enqueuedTime"`
	DequeuedTime *time.Time  `json:"dequeuedTime"`
	ID           string      `json:"id"`
	InProgress   bool        `json:"inProgress"`
	Payload      string      `json:"-"` // Serialized JSON payload (added for the sample endpoint)
}

// PushRequest represents a request to push orthos to the work queue
type PushRequest struct {
	Orthos []types.Ortho `json:"orthos"`
}

// PushResponse represents a response after pushing to the work queue
type PushResponse struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Count   int      `json:"count"`
	IDs     []string `json:"ids"`
}

// PopResponse represents a response when popping from the work queue
type PopResponse struct {
	Status  string       `json:"status"`
	Message string       `json:"message"`
	Ortho   *types.Ortho `json:"ortho"`
	ID      string       `json:"id"`
}

// AckRequest represents a request to acknowledge a work item
type AckRequest struct {
	ID string `json:"id"`
}

// AckResponse represents a response after acknowledging a work item
type AckResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
