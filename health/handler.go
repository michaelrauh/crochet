package health

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"
)

// HealthStatus represents the current health state of the service
type HealthStatus struct {
	Status      string            `json:"status"`
	ServiceName string            `json:"service"`
	Version     string            `json:"version,omitempty"`
	Details     map[string]string `json:"details,omitempty"`
}

// Options represents configurable options for the health check handler
type Options struct {
	// ServiceName is the name of the service
	ServiceName string
	// Version is the version of the service
	Version string
	// Details contains additional details to include in the health status
	Details map[string]string
	// AdditionalChecks is a map of additional health check functions
	AdditionalChecks map[string]CheckFunc
}

// CheckFunc is a function that performs a health check and returns a result
type CheckFunc func() (bool, string)

// MemoryCheck creates a basic memory check function
func MemoryCheck() CheckFunc {
	return func() (bool, string) {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		memoryUsage := memStats.Alloc / (1024 * 1024) // Convert to MB
		return true, fmt.Sprintf("%d MB", memoryUsage)
	}
}

// UptimeCheck creates an uptime check function
func UptimeCheck(startTime time.Time) CheckFunc {
	return func() (bool, string) {
		uptime := time.Since(startTime)
		return true, uptime.String()
	}
}

// DependencyCheck creates a check function for an HTTP dependency
func DependencyCheck(name, url string, timeout time.Duration) CheckFunc {
	return func() (bool, string) {
		client := &http.Client{
			Timeout: timeout,
		}

		start := time.Now()
		resp, err := client.Get(url)
		duration := time.Since(start)

		if err != nil {
			return false, fmt.Sprintf("Error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return true, fmt.Sprintf("OK (%d ms)", duration.Milliseconds())
		} else {
			return false, fmt.Sprintf("Status code: %d", resp.StatusCode)
		}
	}
}

// NewHandler creates a new health check handler with the given options
func NewHandler(opts Options) http.HandlerFunc {
	if opts.Details == nil {
		opts.Details = make(map[string]string)
	}

	// Add basic system information to details
	if opts.Details["goVersion"] == "" {
		opts.Details["goVersion"] = runtime.Version()
	}

	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Health check endpoint called for %s", opts.ServiceName)

		// Prepare the health status response
		status := HealthStatus{
			Status:      "OK",
			ServiceName: opts.ServiceName,
			Version:     opts.Version,
			Details:     make(map[string]string),
		}

		// Copy details from options
		for k, v := range opts.Details {
			status.Details[k] = v
		}

		// Add system information
		status.Details["goVersion"] = runtime.Version()
		status.Details["goArch"] = runtime.GOARCH
		status.Details["goOS"] = runtime.GOOS
		status.Details["hostname"], _ = os.Hostname()

		// Run additional health checks if provided
		allChecksOK := true
		if opts.AdditionalChecks != nil {
			for name, checkFn := range opts.AdditionalChecks {
				ok, msg := checkFn()
				status.Details[name] = msg
				if !ok {
					allChecksOK = false
					status.Status = "DEGRADED"
				}
			}
		}

		// Set response headers and status code
		w.Header().Set("Content-Type", "application/json")
		statusCode := http.StatusOK
		if !allChecksOK {
			statusCode = http.StatusServiceUnavailable
		}

		w.WriteHeader(statusCode)

		// Write the JSON response
		if err := json.NewEncoder(w).Encode(status); err != nil {
			log.Printf("Error writing health check response: %v", err)
		}
	}
}
