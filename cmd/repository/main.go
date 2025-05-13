package main

import (
	"crochet/internal/handler"
	"crochet/internal/httpserver"
	"crochet/pkg/config"

	"go.uber.org/fx"
)

func main() {
	fx.New(
		fx.Provide(config.Load, httpserver.NewRouter, handler.NewHandler),
		fx.Invoke(handler.RegisterRoutes, httpserver.StartServer),
	).Run()
}
