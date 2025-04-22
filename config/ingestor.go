package config

// IngestorConfig contains specific configuration for the Ingestor service
type IngestorConfig struct {
	BaseConfig
	ContextServiceURL      string `envconfig:"CONTEXT_SERVICE_URL" required:"true"`
	RemediationsServiceURL string `envconfig:"REMEDIATIONS_SERVICE_URL" required:"true"`
	OrthosServiceURL       string `envconfig:"ORTHOS_SERVICE_URL" required:"true"`
	WorkServerURL          string `envconfig:"WORK_SERVER_URL" required:"true"`
}

// Validate checks the ingestor configuration for required values
func (c *IngestorConfig) Validate() error {
	// First validate the base config
	if err := c.BaseConfig.Validate(); err != nil {
		return err
	}

	// Then validate ingestor-specific config
	if c.ContextServiceURL == "" {
		return NewConfigError("CONTEXT_SERVICE_URL is required")
	}
	if c.RemediationsServiceURL == "" {
		return NewConfigError("REMEDIATIONS_SERVICE_URL is required")
	}
	if c.OrthosServiceURL == "" {
		return NewConfigError("ORTHOS_SERVICE_URL is required")
	}
	if c.WorkServerURL == "" {
		return NewConfigError("WORK_SERVER_URL is required")
	}
	return nil
}

// LoadIngestorConfig loads the configuration for the Ingestor service
func LoadIngestorConfig() (*IngestorConfig, error) {
	cfg := &IngestorConfig{
		BaseConfig: BaseConfig{
			ServiceName: "ingestor",
		},
	}

	err := LoadAndValidate("INGESTOR", cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
