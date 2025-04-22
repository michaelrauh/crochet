package config

// WorkServerConfig contains specific configuration for the WorkServer service
type WorkServerConfig struct {
	BaseConfig
	// Timeout in seconds for automatic requeue if no ACK is received
	RequeueTimeoutSeconds int `envconfig:"REQUEUE_TIMEOUT_SECONDS" default:"300"` // 5 minutes default
}

// Validate checks the workserver configuration for required values
func (c *WorkServerConfig) Validate() error {
	// First validate the base config
	if err := c.BaseConfig.Validate(); err != nil {
		return err
	}

	// Then validate workserver-specific config
	if c.RequeueTimeoutSeconds <= 0 {
		return NewConfigError("REQUEUE_TIMEOUT_SECONDS must be greater than 0")
	}

	return nil
}

// LoadWorkServerConfig loads the configuration for the WorkServer service
func LoadWorkServerConfig() (*WorkServerConfig, error) {
	cfg := &WorkServerConfig{
		BaseConfig: BaseConfig{
			ServiceName: "workserver",
		},
	}
	err := LoadAndValidate("WORKSERVER", cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
