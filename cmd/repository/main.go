package main

import (
	"context"
	"crochet/internal/httpserver"
	"crochet/pkg/config"
	"crochet/pkg/db"
	"crochet/pkg/rabbitmq"
	"crochet/pkg/telemetry"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

func main() {
	fx.New(
		telemetry.Module("repository"),
		db.Module,
		rabbitmq.Module,
		fx.Provide(
			config.Load,
			newRouter,
			NewHandler,
			fx.Annotate(
				func(q *db.Queries) QueriesInterface { return q },
				fx.As(new(QueriesInterface)),
			),
		),
		fx.Invoke(RegisterRoutes, startServer),
	).Run()
}

func newRouter() *gin.Engine {
	return httpserver.NewRouter("repository")
}

func startServer(lc fx.Lifecycle, cfg *config.Config, r *gin.Engine) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go httpserver.Start(ctx, r, cfg)
			return nil
		},
	})
}
