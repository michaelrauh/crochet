//go:build !test
// +build !test

package telemetry

import (
	"context"
	"log"
	"runtime"
	"time"

	pyroscope "github.com/grafana/pyroscope-go"
)

type ProfilerProvider struct {
	profiler *pyroscope.Profiler
	tags     map[string]string
	enabled  bool
}

func InitProfiler(serviceName string, pyroscopeEndpoint string) (*ProfilerProvider, error) {
	if pyroscopeEndpoint == "" {
		log.Println("Pyroscope endpoint is empty, profiling is disabled")
		return &ProfilerProvider{
			profiler: nil,
			tags:     make(map[string]string),
			enabled:  false,
		}, nil
	}

	runtime.GOMAXPROCS(runtime.NumCPU())

	runtime.SetBlockProfileRate(1)     // Profile every blocking event
	runtime.SetMutexProfileFraction(1) // Profile every mutex contention event

	tags := map[string]string{
		"service": serviceName,
		"env":     "development", // Can be made configurable later
	}

	appName := serviceName // Simple name format for better UI recognition

	config := pyroscope.Config{
		ApplicationName: appName,
		ServerAddress:   pyroscopeEndpoint,
		Logger:          pyroscope.StandardLogger,
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
		},
		Tags:          tags,
		UploadRate:    5 * time.Second, // More frequent uploads for better visualization
		DisableGCRuns: false,           // Enable GC runs to ensure memory profiles are accurate
	}

	log.Printf("Starting Pyroscope profiler with config: %+v", config)
	log.Printf("Block profile rate set to: %d", 1)
	log.Printf("Mutex profile fraction set to: %d", 1)

	profiler, err := pyroscope.Start(config)
	if err != nil {
		// Log the error and exit the application
		log.Fatalf("Failed to start Pyroscope profiler: %v", err)
	}

	log.Println("Pyroscope profiler successfully started")
	return &ProfilerProvider{
		profiler: profiler,
		tags:     tags,
		enabled:  true,
	}, nil
}

func (pp *ProfilerProvider) AddTag(key, value string) {
	if pp == nil || !pp.enabled {
		return
	}
	pp.tags[key] = value
}

func (pp *ProfilerProvider) TagWithContext(ctx context.Context, key, value string) context.Context {
	if pp == nil || !pp.enabled {
		return ctx
	}
	pp.AddTag(key, value)
	return ctx
}

func (pp *ProfilerProvider) Stop() {
	if pp == nil || !pp.enabled {
		return
	}
	pp.profiler.Stop()
}

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
