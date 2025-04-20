package health

import (
	"net/http"
	"time"
)

// ServiceHealth represents a complete health check implementation for a service
type ServiceHealth struct {
	Options   Options
	startTime time.Time
	handler   http.HandlerFunc
}

// New creates a new ServiceHealth instance with the provided options
func New(opts Options) *ServiceHealth {
	startTime := time.Now()

	if opts.AdditionalChecks == nil {
		opts.AdditionalChecks = make(map[string]CheckFunc)
	}

	// Add default checks
	opts.AdditionalChecks["memory"] = MemoryCheck()
	opts.AdditionalChecks["uptime"] = UptimeCheck(startTime)

	return &ServiceHealth{
		Options:   opts,
		startTime: startTime,
		handler:   WithTracing(NewHandler(opts), opts.ServiceName),
	}
}

// Handler returns the HTTP handler for the health check endpoint
func (sh *ServiceHealth) Handler() http.HandlerFunc {
	return sh.handler
}

// AddCheck adds a new health check to the service
func (sh *ServiceHealth) AddCheck(name string, check CheckFunc) {
	sh.Options.AdditionalChecks[name] = check
	// Rebuild the handler with the updated checks
	sh.handler = WithTracing(NewHandler(sh.Options), sh.Options.ServiceName)
}

// AddDependencyCheck adds a dependency check for an HTTP service
func (sh *ServiceHealth) AddDependencyCheck(name, url string) {
	sh.AddCheck(name, DependencyCheck(name, url, 5*time.Second))
}

// RegisterHandler registers the health check handler with the provided mux
func (sh *ServiceHealth) RegisterHandler(mux *http.ServeMux, pattern string) {
	mux.HandleFunc(pattern, sh.Handler())
}
