//go:build test
// +build test

package telemetry

import (
	"context"
	"log"
	"time"
)

// ProfilerProvider is a stub implementation for tests
type ProfilerProvider struct {
	tags    map[string]string
	enabled bool
}

// InitProfiler initializes a stub Pyroscope profiler for testing
func InitProfiler(serviceName string, pyroscopeEndpoint string) (*ProfilerProvider, error) {
	log.Printf("[TEST] Initializing stub profiler for service: %s", serviceName)
	return &ProfilerProvider{
		tags:    make(map[string]string),
		enabled: false,
	}, nil
}

// AddTag is a no-op in the test stub
func (pp *ProfilerProvider) AddTag(key, value string) {
	if pp == nil {
		return
	}
	if pp.tags == nil {
		pp.tags = make(map[string]string)
	}
	pp.tags[key] = value
}

// TagWithContext is a no-op in the test stub
func (pp *ProfilerProvider) TagWithContext(ctx context.Context, key, value string) context.Context {
	if pp == nil {
		return ctx
	}
	pp.AddTag(key, value)
	return ctx
}

// Stop is a no-op in the test stub
func (pp *ProfilerProvider) Stop() {
	// No-op for tests
	log.Println("[TEST] Stopping stub profiler")
}

// StopWithTimeout is a no-op in the test stub
func (pp *ProfilerProvider) StopWithTimeout(timeout time.Duration) {
	// No-op for tests
	log.Printf("[TEST] Stopping stub profiler with timeout: %v", timeout)
}
