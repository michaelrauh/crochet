package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// ServiceResponse represents a response from an external service
type ServiceResponse struct {
	RawResponse map[string]interface{}
	StatusCode  int
	Error       error
}

// ClientOptions allows for configuring the HTTP client
type ClientOptions struct {
	DialTimeout   time.Duration
	DialKeepAlive time.Duration
	MaxIdleConns  int
	ClientTimeout time.Duration
}

// DefaultClientOptions returns sensible defaults for HTTP client options
func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		DialTimeout:   10 * time.Second,
		DialKeepAlive: 30 * time.Second,
		MaxIdleConns:  10,
		ClientTimeout: 30 * time.Second,
	}
}

type Client struct {
	httpClient *http.Client
}

type GenericClient[T any] struct {
	httpClient *http.Client
}

// New creates a new HTTP client with the specified timeout
func New(timeout time.Duration) *Client {
	return &Client{}
}

func NewClient(options ClientOptions) *Client {
	// Create client with OpenTelemetry instrumentation
	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   options.ClientTimeout,
	}

	return &Client{
		httpClient: client,
	}
}

func NewGenericClient(options ClientOptions) *GenericClient[any] {
	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   options.ClientTimeout,
	}

	return &GenericClient[any]{
		httpClient: client,
	}
}

// NewDefaultClient creates a new HTTP client with default options
func NewDefaultClient() *Client {
	return NewClient(DefaultClientOptions())
}

func NewDefaultGenericClient[T any]() *GenericClient[T] {
	return &GenericClient[T]{
		httpClient: &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
			Timeout:   DefaultClientOptions().ClientTimeout,
		},
	}
}

// TODO remove
func (c *Client) Call(ctx context.Context, method, url string, payload []byte) ServiceResponse {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(payload))
	if err != nil {
		return ServiceResponse{
			Error: fmt.Errorf("error creating request: %w", err),
		}
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ServiceResponse{
			Error: fmt.Errorf("error calling service: %w", err),
		}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("Response from service %s: %s", url, string(body))

	if resp.StatusCode != http.StatusOK {
		return ServiceResponse{
			StatusCode: resp.StatusCode,
			Error:      fmt.Errorf("service error: %s, Status Code: %d", string(body), resp.StatusCode),
		}
	}

	// Parse JSON into a map that can be decoded
	var rawResponse map[string]any // Use any instead of interface{}
	if err := json.Unmarshal(body, &rawResponse); err != nil {
		return ServiceResponse{
			StatusCode: resp.StatusCode,
			Error:      fmt.Errorf("invalid response: %w", err),
		}
	}

	return ServiceResponse{
		RawResponse: rawResponse,
		StatusCode:  resp.StatusCode,
		Error:       nil,
	}
}

// TODO simplify
func (c *GenericClient[T]) GenericCall(ctx context.Context, method, url string, payload []byte) (T, error) {
	var result T

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(payload))
	if err != nil {
		return result, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return result, fmt.Errorf("error calling service: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("Response from service %s: %s", url, string(body))

	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("service error: %s, Status Code: %d", string(body), resp.StatusCode)
	}

	// Parse JSON into the generic type T
	if err := json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("invalid response: %w", err)
	}

	return result, nil
}
