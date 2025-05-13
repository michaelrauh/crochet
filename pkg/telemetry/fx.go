package telemetry

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"
)

func Module(serviceName string) fx.Option {
	return fx.Options(
		fx.Supply(Params{ServiceName: serviceName}),
		fx.Provide(NewTracerProvider),
		fx.Invoke(RegisterGlobal),
		fx.Invoke(registerMetricsEndpoint),
	)
}

func registerMetricsEndpoint(lc fx.Lifecycle, router *gin.Engine) {
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}
