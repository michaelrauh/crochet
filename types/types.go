package types

// WorkItem represents a work unit in the work queue
type WorkItem struct {
	ID        string      `json:"id"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

// WorkResponse represents a response containing work to be done
type WorkResponse struct {
	Version int       `json:"version"`
	Work    *WorkItem `json:"work"`
	Receipt string    `json:"receipt,omitempty"`
}

// ResultsRequest represents a request sent to the repository with orthos and remediations
type ResultsRequest struct {
	Orthos       []Ortho            `json:"orthos"`
	Remediations []RemediationTuple `json:"remediations"`
	Receipt      string             `json:"receipt"`
}

// ResultsResponse represents the response from the repository after processing orthos and remediations
type ResultsResponse struct {
	Status            string `json:"status"`
	Version           int    `json:"version"`
	NewOrthosCount    int    `json:"newOrthosCount"`
	RemediationsCount int    `json:"remediationsCount"`
}
