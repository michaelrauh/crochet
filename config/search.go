package config

// SearchConfig contains specific configuration for the Search service
type SearchConfig struct {
	BaseConfig
	ContextServiceURL      string `envconfig:"CONTEXT_SERVICE_URL" required:"true"`
	OrthosServiceURL       string `envconfig:"ORTHOS_SERVICE_URL" required:"true"`
	RemediationsServiceURL string `envconfig:"REMEDIATIONS_SERVICE_URL" required:"true"`
	WorkServerURL          string `envconfig:"WORK_SERVER_URL" required:"true"`
}

// Validate checks the search configuration for required values
func (c *SearchConfig) Validate() error {
	// First validate the base config
	if err := c.BaseConfig.Validate(); err != nil {
		return err
	}

	// Validate search-specific config
	if c.ContextServiceURL == "" {
		return NewConfigError("CONTEXT_SERVICE_URL is required")
	}
	if c.OrthosServiceURL == "" {
		return NewConfigError("ORTHOS_SERVICE_URL is required")
	}
	if c.RemediationsServiceURL == "" {
		return NewConfigError("REMEDIATIONS_SERVICE_URL is required")
	}
	if c.WorkServerURL == "" {
		return NewConfigError("WORK_SERVER_URL is required")
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
