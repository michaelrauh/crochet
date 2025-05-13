package main

import (
	"crochet/internal/httpserver"
	"crochet/pkg/config"

	"go.uber.org/fx"
)

func main() {
	fx.New(
		fx.Provide(config.Load, httpserver.NewRouter, NewHandler),
		fx.Invoke(RegisterRoutes, httpserver.StartServer),
	).Run()
}
