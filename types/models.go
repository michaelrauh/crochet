package types

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
