package main

import (
	"context"
	"crochet/internal/httpserver"
	"crochet/pkg/config"
	"crochet/pkg/db"
	"crochet/pkg/redisstream"
	"crochet/pkg/telemetry"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

func main() {
	fx.New(
		telemetry.Module("repository"),
		db.Module,
		redisstream.Module,
		fx.Provide(
			config.Load,
			newRouter,
			NewHandler,
			fx.Annotate(
				func(q *db.Queries) db.QueriesInterface { return q },
				fx.As(new(db.QueriesInterface)),
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
