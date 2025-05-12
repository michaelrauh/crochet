package main

import (
	"context"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

func StartServer(lc fx.Lifecycle, router *gin.Engine) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go router.Run("0.0.0.0:8080")
			return nil
		},
	})
}
