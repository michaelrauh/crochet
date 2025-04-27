package middleware

import (
	"fmt"

	"crochet/telemetry"

	"github.com/gin-gonic/gin"
)

func SetupGlobalMiddleware(router *gin.Engine, serviceName string, mp *telemetry.MeterProvider) {
	router.Use(func(c *gin.Context) {
		c.Set("service-name", serviceName)
		c.Next()
	})

	router.Use(Recovery())
	router.Use(Logger())
	router.Use(ErrorHandler())
	router.Use(Tracing(serviceName))

	router.Use(Metrics(serviceName, mp))
}

func SetupCommonComponents(serviceName string, jaegerEndpoint string, metricsEndpoint string, pyroscopeEndpoint string) (*gin.Engine, *telemetry.TracerProvider, *telemetry.MeterProvider, *telemetry.ProfilerProvider, error) {
	tp, err := telemetry.InitTracer(serviceName, jaegerEndpoint)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to initialize OpenTelemetry tracer: %w", err)
	}

	var mp *telemetry.MeterProvider
	mp, err = telemetry.InitMeter(serviceName, metricsEndpoint)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to initialize OpenTelemetry metrics: %w", err)
	}

	var pp *telemetry.ProfilerProvider
	pp, err = telemetry.InitProfiler(serviceName, pyroscopeEndpoint)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to initialize Pyroscope profiler: %w", err)
	}

	router := gin.New()
	SetupGlobalMiddleware(router, serviceName, mp)

	return router, tp, mp, pp, nil
}
