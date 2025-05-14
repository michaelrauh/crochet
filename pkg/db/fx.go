package db

import (
	"context"
	"crochet/pkg/config"
	"database/sql"

	_ "github.com/lib/pq" // postgres driver

	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(newDB),
	fx.Provide(newQueries),
)

func newDB(cfg *config.Config, lc fx.Lifecycle) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return db.Close()
		},
	})
	return db, nil
}

func newQueries(dbConn *sql.DB) *Queries {
	return New(dbConn)
}
