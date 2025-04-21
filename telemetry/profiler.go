//go:build !test
// +build !test

// regular build file (not for tests)
package telemetry

import (
	"context"
	"log"
	"runtime"
	"time"

	pyroscope "github.com/grafana/pyroscope-go"
)

// ProfilerProvider wraps the pyroscope profiler
type ProfilerProvider struct {
	profiler *pyroscope.Profiler
	tags     map[string]string
	enabled  bool
}

// InitProfiler initializes a Pyroscope profiler
func InitProfiler(serviceName string, pyroscopeEndpoint string) (*ProfilerProvider, error) {
	log.Printf("Initializing Pyroscope profiler for service: %s with endpoint: %s", serviceName, pyroscopeEndpoint)

	// If the endpoint is empty, create a disabled profiler
	if pyroscopeEndpoint == "" {
		log.Println("Pyroscope endpoint is empty, profiling is disabled")
		return &ProfilerProvider{
			profiler: nil,
			tags:     make(map[string]string),
			enabled:  false,
		}, nil
	}

	// Set GOMAXPROCS explicitly to ensure CPU profiling works correctly
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Enable block and mutex profiling at the system level
	// A lower number means higher precision (more samples)
	runtime.SetBlockProfileRate(1)     // Profile every blocking event
	runtime.SetMutexProfileFraction(1) // Profile every mutex contention event

	// Define tags to be attached to all profiles
	tags := map[string]string{
		"service": serviceName,
		"env":     "development", // Can be made configurable later
	}

	// Format application name to ensure compatibility with Pyroscope UI
	appName := serviceName // Simple name format for better UI recognition

	// Configure the profiler with the supported options
	config := pyroscope.Config{
		ApplicationName: appName,
		ServerAddress:   pyroscopeEndpoint,
		Logger:          pyroscope.StandardLogger,
		ProfileTypes: []pyroscope.ProfileType{
			// CPU profiling
			pyroscope.ProfileCPU,
			// Memory profiling
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			// Block and mutex profiling
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
		},
		Tags:          tags,
		UploadRate:    5 * time.Second, // More frequent uploads for better visualization
		DisableGCRuns: false,           // Enable GC runs to ensure memory profiles are accurate
	}

	// Log configuration details
	log.Printf("Starting Pyroscope profiler with config: %+v", config)
	log.Printf("Block profile rate set to: %d", 1)
	log.Printf("Mutex profile fraction set to: %d", 1)

	// Try to start the profiler
	profiler, err := pyroscope.Start(config)
	if err != nil {
		// Return a non-fatal error and a disabled profiler
		log.Printf("Failed to start Pyroscope profiler: %v, continuing without profiling", err)
		return &ProfilerProvider{
			profiler: nil,
			tags:     tags,
			enabled:  false,
		}, nil
	}

	log.Println("Pyroscope profiler successfully started")
	return &ProfilerProvider{
		profiler: profiler,
		tags:     tags,
		enabled:  true,
	}, nil
}

// AddTag adds a tag to the profiler
func (pp *ProfilerProvider) AddTag(key, value string) {
	if pp == nil || !pp.enabled {
		return
	}
	pp.tags[key] = value
	// The tags are already passed during initialization
	// and can't be changed after in the current Pyroscope API
}

// TagWithContext adds context tags to profiling data
func (pp *ProfilerProvider) TagWithContext(ctx context.Context, key, value string) context.Context {
	if pp == nil || !pp.enabled {
		return ctx
	}
	pp.AddTag(key, value)
	return ctx
}

// Stop gracefully stops the profiler
func (pp *ProfilerProvider) Stop() {
	if pp == nil || !pp.enabled {
		return
	}
	pp.profiler.Stop()
}

// StopWithTimeout is a convenience method to stop the profiler with a timeout
func (pp *ProfilerProvider) StopWithTimeout(timeout time.Duration) {
	if pp == nil || !pp.enabled {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	done := make(chan struct{})
	go func() {
		pp.profiler.Stop()
		close(done)
	}()
	select {
	case <-done:
		log.Println("Pyroscope profiler stopped gracefully")
	case <-ctx.Done():
		log.Println("Pyroscope profiler stop timed out")
	}
}
