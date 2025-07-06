package main

import (
	"context"
	"log/slog"
	"os"

	"go.uber.org/fx"

	"github.com/database-playground/backend-v2/internal/deps"

	_ "github.com/database-playground/backend-v2/ent/runtime"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	app := fx.New(
		deps.FxCommonModule,
		fx.Provide(
			annotateAsMiddleware(provideAuthMiddleware),
			annotateAsService(provideAuthService),
			provideGqlgenHandler,
			fx.Annotate(
				provideGinEngine,
				fx.ParamTags(`group:"services"`, `group:"middlewares"`),
			),
		),
		fx.Invoke(newGinLifecycle),
	)

	if err := app.Start(context.Background()); err != nil {
		slog.Error("error starting server", "error", err)
		os.Exit(1)
	}

	slog.Info("Server stopped")
}
