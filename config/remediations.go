package config

// RemediationsConfig contains specific configuration for the Remediations service
type RemediationsConfig struct {
	BaseConfig
	// Add any remediations-specific configuration fields here
}

// Validate checks the remediations configuration for required values
func (c *RemediationsConfig) Validate() error {
	// First validate the base config
	if err := c.BaseConfig.Validate(); err != nil {
		return err
	}

	// Add any remediations-specific validation here

	return nil
}

// LoadRemediationsConfig loads the configuration for the Remediations service
func LoadRemediationsConfig() (*RemediationsConfig, error) {
	cfg := &RemediationsConfig{
		BaseConfig: BaseConfig{
			ServiceName: "remediations",
		},
	}

	err := LoadAndValidate("REMEDIATIONS", cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
