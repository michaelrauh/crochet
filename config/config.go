package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
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

// DefaultContextServiceName is the default name for the context service
const DefaultContextServiceName = "context"

// DefaultContextPort is the default port for the context service
const DefaultContextPort = 8081

// DefaultContextHost is the default host for the context service
const DefaultContextHost = "0.0.0.0"

// DefaultContextJaegerEndpoint is the default endpoint for Jaeger
const DefaultContextJaegerEndpoint = "jaeger:4317"

// DefaultContextMetricsEndpoint is the default endpoint for metrics
const DefaultContextMetricsEndpoint = "otel-collector:4317"

// DefaultContextPyroscopeEndpoint is the default endpoint for Pyroscope
const DefaultContextPyroscopeEndpoint = "http://pyroscope:4040"

// DefaultContextLibSQLEndpoint is the default endpoint for libsql
const DefaultContextLibSQLEndpoint = "http://libsql:8080"

// Context contains the configuration for the context service
type Context struct {
	ServiceName       string
	Port              int
	Host              string
	JaegerEndpoint    string
	MetricsEndpoint   string
	PyroscopeEndpoint string
	LibSQLEndpoint    string
}

// NewContext creates a new context configuration with default values
func NewContext() Context {
	return Context{
		ServiceName:       DefaultContextServiceName,
		Port:              DefaultContextPort,
		Host:              DefaultContextHost,
		JaegerEndpoint:    DefaultContextJaegerEndpoint,
		MetricsEndpoint:   DefaultContextMetricsEndpoint,
		PyroscopeEndpoint: DefaultContextPyroscopeEndpoint,
		LibSQLEndpoint:    DefaultContextLibSQLEndpoint,
	}
}

// GetContext loads the context configuration from environment variables
func GetContext() Context {
	config := NewContext()

	if serviceName := os.Getenv("CONTEXT_SERVICE_NAME"); serviceName != "" {
		config.ServiceName = serviceName
	}

	if port := os.Getenv("CONTEXT_PORT"); port != "" {
		if portInt, err := strconv.Atoi(port); err == nil {
			config.Port = portInt
		}
	}

	if host := os.Getenv("CONTEXT_HOST"); host != "" {
		config.Host = host
	}

	if jaegerEndpoint := os.Getenv("CONTEXT_JAEGER_ENDPOINT"); jaegerEndpoint != "" {
		config.JaegerEndpoint = jaegerEndpoint
	}

	if metricsEndpoint := os.Getenv("CONTEXT_METRICS_ENDPOINT"); metricsEndpoint != "" {
		config.MetricsEndpoint = metricsEndpoint
	}

	if pyroscopeEndpoint := os.Getenv("CONTEXT_PYROSCOPE_ENDPOINT"); pyroscopeEndpoint != "" {
		config.PyroscopeEndpoint = pyroscopeEndpoint
	}

	if libSQLEndpoint := os.Getenv("CONTEXT_LIBSQL_ENDPOINT"); libSQLEndpoint != "" {
		config.LibSQLEndpoint = libSQLEndpoint
	}

	return config
}

// GetContextAddr returns the address for the context service
func (c Context) GetContextAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
