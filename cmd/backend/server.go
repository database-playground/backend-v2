package main

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	_ "github.com/database-playground/backend-v2/ent/runtime"
	"github.com/database-playground/backend-v2/internal/deps"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	app := fx.New(
		fx.Provide(
			deps.FxCommonModule,
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

	app.Start(context.Background())
	slog.Info("Server stopped")
}
