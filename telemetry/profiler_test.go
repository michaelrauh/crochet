package telemetry

import (
	"context"
	"testing"
	"time"
)

// TestInitProfilerWithEmptyEndpoint tests that InitProfiler doesn't fail
// when given an empty endpoint
func TestInitProfilerWithEmptyEndpoint(t *testing.T) {
	pp, err := InitProfiler("test-service", "")
	if err != nil {
		t.Fatalf("InitProfiler with empty endpoint returned error: %v", err)
	}
	if pp == nil {
		t.Fatal("InitProfiler with empty endpoint returned nil ProfilerProvider")
	}
	if pp.enabled {
		t.Fatal("ProfilerProvider should be disabled with empty endpoint")
	}

	// Test methods don't panic when profiler is disabled
	pp.AddTag("test", "value")
	ctx := context.Background()
	pp.TagWithContext(ctx, "test", "value")
	pp.Stop()
	pp.StopWithTimeout(5 * time.Second)
}

// TestNilProfilerProvider tests that methods don't panic with nil receiver
func TestNilProfilerProvider(t *testing.T) {
	// These shouldn't panic
	var pp *ProfilerProvider
	pp.AddTag("test", "value")
	ctx := context.Background()
	pp.TagWithContext(ctx, "test", "value")
	pp.Stop()
	pp.StopWithTimeout(5 * time.Second)
}

// TestInitProfilerWithInvalidEndpoint tests that InitProfiler returns a disabled
// profiler when given an invalid endpoint rather than returning an error
func TestInitProfilerWithInvalidEndpoint(t *testing.T) {
	pp, err := InitProfiler("test-service", "invalid-endpoint")
	if err != nil {
		t.Fatalf("InitProfiler with invalid endpoint returned error: %v", err)
	}
	if pp == nil {
		t.Fatal("InitProfiler with invalid endpoint returned nil ProfilerProvider")
	}
	if pp.enabled {
		t.Fatal("ProfilerProvider should be disabled with invalid endpoint")
	}
}
