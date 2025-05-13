package main

import (
	"config"
	"httpserver"

	"go.uber.org/fx"
)

func main() {
	fx.New(
		fx.Provide(config.Load, httpserver.NewRouter, NewHandler),
		fx.Invoke(RegisterRoutes, httpserver.StartServer),
	).Run()
}
