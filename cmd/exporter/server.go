package main

import (
	"github.com/database-playground/backend-v2/internal/deps"
	"go.uber.org/fx"
)

func main() {
	app := fx.New(
		fx.Provide(
			ExporterConfig,
			EntClient,
			PrometheusMetrics,
		),
		fx.Invoke(deps.OTelSDK),
		fx.Invoke(PrometheusHTTPHandler),
	)

	app.Run()
}
