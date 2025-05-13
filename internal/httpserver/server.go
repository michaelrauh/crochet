package httpserver

import (
	"crochet/pkg/config"
	"fmt"

	"context"

	"github.com/gin-gonic/gin"
)

func Start(ctx context.Context, router *gin.Engine, cfg *config.Config) error {
	addr := fmt.Sprintf("0.0.0.0:%s", cfg.Port)
	return router.Run(addr)
}
