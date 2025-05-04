package config

// SearchConfig contains specific configuration for the Search service
type SearchConfig struct {
	BaseConfig
	RepositoryServiceURL string `envconfig:"REPOSITORY_SERVICE_URL" required:"true"`
	// Legacy service URLs - to be removed after transition
	ContextServiceURL      string `envconfig:"CONTEXT_SERVICE_URL" required:"false"`
	OrthosServiceURL       string `envconfig:"ORTHOS_SERVICE_URL" required:"false"`
	RemediationsServiceURL string `envconfig:"REMEDIATIONS_SERVICE_URL" required:"false"`
	WorkServerURL          string `envconfig:"WORK_SERVER_URL" required:"false"`
}

// Validate checks the search configuration for required values
func (c *SearchConfig) Validate() error {
	// First validate the base config
	if err := c.BaseConfig.Validate(); err != nil {
		return err
	}

	// Validate repository service URL
	if c.RepositoryServiceURL == "" {
		return NewConfigError("REPOSITORY_SERVICE_URL is required")
	}

	return nil
}

// LoadSearchConfig loads the configuration for the Search service
func LoadSearchConfig() (*SearchConfig, error) {
	cfg := &SearchConfig{
		BaseConfig: BaseConfig{
			ServiceName: "search",
		},
	}
	err := LoadAndValidate("SEARCH", cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
