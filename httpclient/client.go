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

type ClientOptions struct {
	DialTimeout   time.Duration
	DialKeepAlive time.Duration
	MaxIdleConns  int
	ClientTimeout time.Duration
}

func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		DialTimeout:   10 * time.Second,
		DialKeepAlive: 30 * time.Second,
		MaxIdleConns:  10,
		ClientTimeout: 30 * time.Second,
	}
}

type GenericClient[T any] struct {
	httpClient *http.Client
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

func NewDefaultGenericClient[T any]() *GenericClient[T] {
	return &GenericClient[T]{
		httpClient: &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
			Timeout:   DefaultClientOptions().ClientTimeout,
		},
	}
}

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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return result, fmt.Errorf("service error: %s, Status Code: %d", string(body), resp.StatusCode)
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("invalid response: %w", err)
	}

	return result, nil
}
