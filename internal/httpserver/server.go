package httpserver

import (
	"crochet/pkg/config"

	"context"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

func StartServer(lc fx.Lifecycle, router *gin.Engine, cfg *config.Config) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go router.Run("0.0.0.0:" + cfg.Port)
			return nil
		},
	})
}
