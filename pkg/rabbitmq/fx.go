package rabbitmq

import (
	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(
		NewConnection,
		NewQueue,
	),
)
