package config

// ContextConfig contains specific configuration for the Context service
type ContextConfig struct {
	BaseConfig
	// Add any context-specific configuration fields here
}

// Validate checks the context configuration for required values
func (c *ContextConfig) Validate() error {
	// First validate the base config
	if err := c.BaseConfig.Validate(); err != nil {
		return err
	}

	// Add any context-specific validation here

	return nil
}

// LoadContextConfig loads the configuration for the Context service
func LoadContextConfig() (*ContextConfig, error) {
	cfg := &ContextConfig{
		BaseConfig: BaseConfig{
			ServiceName: "context",
		},
	}

	err := LoadAndValidate("CONTEXT", cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
