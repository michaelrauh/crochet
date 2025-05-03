// filepath: /Users/michaelrauh/dev/crochet/config/repository.go
package config

// RepositoryConfig contains specific configuration for the Repository service
type RepositoryConfig struct {
	BaseConfig
	ContextServiceURL      string `envconfig:"CONTEXT_SERVICE_URL" required:"true"`
	RemediationsServiceURL string `envconfig:"REMEDIATIONS_SERVICE_URL" required:"true"`
	OrthosServiceURL       string `envconfig:"ORTHOS_SERVICE_URL" required:"true"`
	WorkServerURL          string `envconfig:"WORK_SERVER_URL" required:"true"`
	ContextDBEndpoint      string `envconfig:"CONTEXT_DB_ENDPOINT" required:"true"`                      // Added for direct DB access
	RabbitMQURL            string `envconfig:"RABBITMQ_URL" default:"amqp://guest:guest@rabbitmq:5672/"` // Added for RabbitMQ configuration
}

// Validate checks the repository configuration for required values
func (c *RepositoryConfig) Validate() error {
	// First validate the base config
	if err := c.BaseConfig.Validate(); err != nil {
		return err
	}

	// Then validate repository-specific config
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
	if c.ContextDBEndpoint == "" {
		return NewConfigError("CONTEXT_DB_ENDPOINT is required")
	}
	if c.RabbitMQURL == "" {
		return NewConfigError("RABBITMQ_URL is required")
	}
	return nil
}

// LoadRepositoryConfig loads the configuration for the Repository service
func LoadRepositoryConfig() (*RepositoryConfig, error) {
	cfg := &RepositoryConfig{
		BaseConfig: BaseConfig{
			ServiceName: "repository",
		},
	}
	err := LoadAndValidate("REPOSITORY", cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
