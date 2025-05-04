package types

import (
	"encoding/json"
	"fmt"
)

// Corpus represents the incoming text data to be processed
type Corpus struct {
	Title string `mapstructure:"title" json:"title"`
	Text  string `mapstructure:"text" json:"text"`
}

// ContextInput represents the data sent to the context service
type ContextInput struct {
	Title      string     `mapstructure:"title" json:"title"`
	Vocabulary []string   `mapstructure:"vocabulary" json:"vocabulary"`
	Subphrases [][]string `mapstructure:"subphrases" json:"subphrases"`
}

type DatabaseQueueInput struct {
	Payload []byte `mapstructure:"payload" json:"payload"`
}

// ContextResponse represents the response from the context service
type ContextResponse struct {
	Version       int        `mapstructure:"version" json:"version"`
	NewSubphrases [][]string `mapstructure:"newSubphrases" json:"newSubphrases"`
	NewVocabulary []string   `mapstructure:"newVocabulary" json:"newVocabulary"`
}

// ContextMemoryStore represents the in-memory storage for the context service
type ContextMemoryStore struct {
	Vocabulary map[string]struct{}
	Subphrases map[string]struct{}
}

// RemediationRequest represents the request data for the remediation service
type RemediationRequest struct {
	Pairs [][]string `mapstructure:"pairs" json:"pairs"`
}

// RemediationResponse represents the response from the remediation service
type RemediationResponse struct {
	Status string   `mapstructure:"status" json:"status"`
	Hashes []string `mapstructure:"hashes" json:"hashes"`
}

// RemediationTuple represents a single remediation with a string pair
type RemediationTuple struct {
	Pair []string `mapstructure:"pair" json:"pair"`
}

// RemediationMemoryStore represents the in-memory storage for the remediation service
type RemediationMemoryStore struct {
	Remediations []RemediationTuple
}

// AddRemediationRequest represents the request to add new remediations
type AddRemediationRequest struct {
	Remediations []RemediationTuple `mapstructure:"remediations" json:"remediations"`
}

// AddRemediationResponse represents the response after adding remediations
type AddRemediationResponse struct {
	Status  string `mapstructure:"status" json:"status"`
	Message string `mapstructure:"message" json:"message"`
	Count   int    `mapstructure:"count" json:"count"`
}

// DeleteRemediationRequest represents the request to delete remediations by hash
type DeleteRemediationRequest struct {
	Hashes []string `mapstructure:"hashes" json:"hashes"`
}

// DeleteRemediationResponse represents the response after deleting remediations
type DeleteRemediationResponse struct {
	Status  string `mapstructure:"status" json:"status"`
	Message string `mapstructure:"message" json:"message"`
	Count   int    `mapstructure:"count" json:"count"`
}

// Ortho represents orthogonal data structure
type Ortho struct {
	Grid     map[string]string `mapstructure:"grid" json:"grid"`
	Shape    []int             `mapstructure:"shape" json:"shape"`
	Position []int             `mapstructure:"position" json:"position"`
	Shell    int               `mapstructure:"shell" json:"shell"`
	ID       string            `mapstructure:"id" json:"id"`
}

// OrthosResponse represents the response from the orthos service
type OrthosResponse struct {
	Status  string  `mapstructure:"status" json:"status"`
	Message string  `mapstructure:"message" json:"message"`
	Count   int     `mapstructure:"count" json:"count"`
	Orthos  []Ortho `mapstructure:"orthos" json:"orthos"`
}

// WorkServerPushResponse represents the response from the workserver's push endpoint
type WorkServerPushResponse struct {
	Status  string   `mapstructure:"status" json:"status"`
	Message string   `mapstructure:"message" json:"message"`
	Count   int      `mapstructure:"count" json:"count"`
	IDs     []string `mapstructure:"ids" json:"ids"`
}

// VersionInfo represents version information for a corpus
type VersionInfo struct {
	Version int `json:"version"`
}

// Pair represents a pair of text lines from a corpus
type Pair struct {
	Left  string `json:"left"`
	Right string `json:"right"`
}

// DBQueueItem represents an item in the database processing queue
// with a type field to determine how to handle it and a payload that contains the serialized data
type DBQueueItem struct {
	Type    string          `json:"type"`    // Type of the item: "version", "pair", "context", "ortho", "remediation", "remediation_delete"
	Payload json.RawMessage `json:"payload"` // Raw JSON payload that can be unmarshaled based on Type
}

// DB queue item types
const (
	DBQueueItemTypeVersion           = "version"
	DBQueueItemTypePair              = "pair"
	DBQueueItemTypeContext           = "context"
	DBQueueItemTypeOrtho             = "ortho"
	DBQueueItemTypeRemediation       = "remediation"
	DBQueueItemTypeRemediationDelete = "remediation_delete"
)

// CreateVersionQueueItem creates a DBQueueItem for a version update
func CreateVersionQueueItem(version VersionInfo) (DBQueueItem, error) {
	payload, err := json.Marshal(version)
	if err != nil {
		return DBQueueItem{}, err
	}
	return DBQueueItem{
		Type:    DBQueueItemTypeVersion,
		Payload: payload,
	}, nil
}

// CreatePairQueueItem creates a DBQueueItem for a pair
func CreatePairQueueItem(pair Pair) (DBQueueItem, error) {
	payload, err := json.Marshal(pair)
	if err != nil {
		return DBQueueItem{}, err
	}
	return DBQueueItem{
		Type:    DBQueueItemTypePair,
		Payload: payload,
	}, nil
}

// CreateContextQueueItem creates a DBQueueItem for context data
func CreateContextQueueItem(context ContextInput) (DBQueueItem, error) {
	payload, err := json.Marshal(context)
	if err != nil {
		return DBQueueItem{}, err
	}
	return DBQueueItem{
		Type:    DBQueueItemTypeContext,
		Payload: payload,
	}, nil
}

// CreateOrthoQueueItem creates a DBQueueItem for an ortho
func CreateOrthoQueueItem(ortho Ortho) (DBQueueItem, error) {
	payload, err := json.Marshal(ortho)
	if err != nil {
		return DBQueueItem{}, err
	}
	return DBQueueItem{
		Type:    DBQueueItemTypeOrtho,
		Payload: payload,
	}, nil
}

// CreateRemediationQueueItem creates a DBQueueItem for a remediation
func CreateRemediationQueueItem(remediation RemediationTuple) (DBQueueItem, error) {
	payload, err := json.Marshal(remediation)
	if err != nil {
		return DBQueueItem{}, err
	}
	return DBQueueItem{
		Type:    DBQueueItemTypeRemediation,
		Payload: payload,
	}, nil
}

// CreateRemediationDeleteQueueItem creates a DBQueueItem for a remediation deletion
func CreateRemediationDeleteQueueItem(remediationID string) (DBQueueItem, error) {
	payload, err := json.Marshal(map[string]string{"id": remediationID})
	if err != nil {
		return DBQueueItem{}, err
	}
	return DBQueueItem{
		Type:    DBQueueItemTypeRemediationDelete,
		Payload: payload,
	}, nil
}

// GetVersion extracts a VersionInfo from the DBQueueItem
func (item *DBQueueItem) GetVersion() (VersionInfo, error) {
	if item.Type != DBQueueItemTypeVersion {
		return VersionInfo{}, fmt.Errorf("incorrect item type: expected %s, got %s", DBQueueItemTypeVersion, item.Type)
	}
	var version VersionInfo
	err := json.Unmarshal(item.Payload, &version)
	return version, err
}

// GetPair extracts a Pair from the DBQueueItem
func (item *DBQueueItem) GetPair() (Pair, error) {
	if item.Type != DBQueueItemTypePair {
		return Pair{}, fmt.Errorf("incorrect item type: expected %s, got %s", DBQueueItemTypePair, item.Type)
	}
	var pair Pair
	err := json.Unmarshal(item.Payload, &pair)
	return pair, err
}

// GetContext extracts a ContextInput from the DBQueueItem
func (item *DBQueueItem) GetContext() (ContextInput, error) {
	if item.Type != DBQueueItemTypeContext {
		return ContextInput{}, fmt.Errorf("incorrect item type: expected %s, got %s", DBQueueItemTypeContext, item.Type)
	}
	var context ContextInput
	err := json.Unmarshal(item.Payload, &context)
	return context, err
}

// GetOrtho extracts an Ortho from the DBQueueItem
func (item *DBQueueItem) GetOrtho() (Ortho, error) {
	if item.Type != DBQueueItemTypeOrtho {
		return Ortho{}, fmt.Errorf("incorrect item type: expected %s, got %s", DBQueueItemTypeOrtho, item.Type)
	}
	var ortho Ortho
	err := json.Unmarshal(item.Payload, &ortho)
	return ortho, err
}

// GetRemediation extracts a RemediationTuple from the DBQueueItem
func (item *DBQueueItem) GetRemediation() (RemediationTuple, error) {
	if item.Type != DBQueueItemTypeRemediation {
		return RemediationTuple{}, fmt.Errorf("incorrect item type: expected %s, got %s", DBQueueItemTypeRemediation, item.Type)
	}
	var remediation RemediationTuple
	err := json.Unmarshal(item.Payload, &remediation)
	return remediation, err
}

// GetRemediationDeleteID extracts a remediation ID from the DBQueueItem
func (item *DBQueueItem) GetRemediationDeleteID() (string, error) {
	if item.Type != DBQueueItemTypeRemediationDelete {
		return "", fmt.Errorf("incorrect item type: expected %s, got %s", DBQueueItemTypeRemediationDelete, item.Type)
	}
	var data map[string]string
	err := json.Unmarshal(item.Payload, &data)
	if err != nil {
		return "", err
	}
	return data["id"], nil
}
