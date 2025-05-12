package main

import (
	"go.uber.org/fx"
)

func main() {
	fx.New(
		fx.Provide(NewRouter, NewHandler),
		fx.Invoke(RegisterRoutes, StartServer),
	).Run()
}
