package redisstream

import (
	"context"
	"crochet/pkg/config"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(newRedisClient),
	fx.Provide(NewQueue),
	fx.Invoke(initRedisStream),
)

func newRedisClient(cfg *config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Pass,
		DB:       cfg.Redis.DB,
	})
}

// initRedisStream sets up the necessary Redis stream infrastructure during application startup
func initRedisStream(lc fx.Lifecycle, client *redis.Client) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Use the domain-focused function from init.go
			return InitializeStream(client, "db", "db")
		},
	})
}
