package main

import (
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
			annotateAsMiddleware(provideMachineMiddleware),
			annotateAsService(provideAuthService),
			provideGqlgenHandler,
			fx.Annotate(
				provideGinEngine,
				fx.ParamTags(`group:"services"`, `group:"middlewares"`),
			),
		),
		fx.Invoke(newGinLifecycle),
	)

	app.Run()
}
