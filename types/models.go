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
