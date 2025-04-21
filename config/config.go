package config

import (
	"fmt"
	"log"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// ConfigError represents a configuration error
type ConfigError struct {
	Message string
}

// Error implements the error interface
func (e *ConfigError) Error() string {
	return fmt.Sprintf("configuration error: %s", e.Message)
}

// NewConfigError creates a new configuration error
func NewConfigError(message string) *ConfigError {
	return &ConfigError{Message: message}
}

// BaseConfig contains configuration fields common to all services
type BaseConfig struct {
	// Service identification
	ServiceName string `envconfig:"SERVICE_NAME" required:"true"`
	Host        string `envconfig:"HOST" required:"true"`
	Port        string `envconfig:"PORT" required:"true"`

	// OpenTelemetry configuration
	JaegerEndpoint  string `envconfig:"JAEGER_ENDPOINT" required:"true"`
	MetricsEndpoint string `envconfig:"METRICS_ENDPOINT" default:"otel-collector:4317"`

	// Pyroscope configuration
	PyroscopeEndpoint string `envconfig:"PYROSCOPE_ENDPOINT" default:"http://pyroscope:4040"`

	// HTTP Client configuration
	DialTimeout   time.Duration `envconfig:"DIAL_TIMEOUT" default:"10s"`
	DialKeepAlive time.Duration `envconfig:"DIAL_KEEPALIVE" default:"30s"`
	MaxIdleConns  int           `envconfig:"MAX_IDLE_CONNS" default:"10"`
	ClientTimeout time.Duration `envconfig:"CLIENT_TIMEOUT" default:"30s"`
}

// Validate checks the base configuration for required values
func (c *BaseConfig) Validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("SERVICE_NAME is required")
	}
	if c.Host == "" {
		return fmt.Errorf("HOST is required")
	}
	if c.Port == "" {
		return fmt.Errorf("PORT is required")
	}
	if c.JaegerEndpoint == "" {
		return fmt.Errorf("JAEGER_ENDPOINT is required")
	}
	return nil
}

// GetAddress returns the fully formatted address for the service
func (c *BaseConfig) GetAddress() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

// LoadConfig loads configuration from environment variables into the provided struct
func LoadConfig(prefix string, cfg interface{}) error {
	if err := envconfig.Process(prefix, cfg); err != nil {
		return fmt.Errorf("failed to process environment variables: %w", err)
	}
	return nil
}

// LoadAndValidate loads and validates configuration
func LoadAndValidate(prefix string, cfg interface{}) error {
	if err := LoadConfig(prefix, cfg); err != nil {
		return err
	}

	// Check if the config implements the Validator interface
	if validator, ok := cfg.(Validator); ok {
		if err := validator.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Validator interface for config structs with custom validation
type Validator interface {
	Validate() error
}

// LogConfig logs the configuration values for debugging
func LogConfig(cfg interface{}) {
	log.Printf("Configuration: %+v", cfg)
}
