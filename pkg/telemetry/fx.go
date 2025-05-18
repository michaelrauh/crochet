package telemetry

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

func Module(serviceName string) fx.Option {
	return fx.Options(
		fx.Supply(Params{ServiceName: serviceName}),
		fx.Provide(NewTracerProvider),
		fx.Invoke(RegisterGlobal),
		fx.Invoke(registerMetricsEndpoint),
		fx.Invoke(startRedisStreamMetricsCollector),
	)
}

func registerMetricsEndpoint(lc fx.Lifecycle, router *gin.Engine) {
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// startRedisStreamMetricsCollector creates and starts a metrics collector for the Redis "db" stream
func startRedisStreamMetricsCollector(lc fx.Lifecycle, redisClient *redis.Client) {
	collector := NewStreamMetricsCollector(redisClient, "db", 5*time.Second)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			collector.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			collector.Stop()
			return nil
		},
	})
}
