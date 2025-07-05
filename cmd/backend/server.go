package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"go.uber.org/fx"

	_ "github.com/database-playground/backend-v2/ent/runtime"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	app := fx.New(
		fx.Provide(provideConfig),
		fx.Provide(provideEntClient),
		fx.Provide(provideRedisClient),
		fx.Provide(provideAuthService),
		fx.Provide(provideGqlgenHandler),
		fx.Invoke(newGinLifecycle),
	)

	app.Start(ctx)

	<-ctx.Done()
	slog.Info("Gracefully shutting down server (Ctrl+C again to force stop)...")
	cancel()

	app.Stop(ctx)

	slog.Info("Server stopped")
}
