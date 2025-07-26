package main

import (
	"go.uber.org/fx"

	"github.com/database-playground/backend-v2/internal/deps"

	_ "github.com/database-playground/backend-v2/ent/runtime"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	app := fx.New(
		deps.FxSlogOption,
		deps.FxCommonModule,
		fx.Provide(
			AuthStorage,
			SqlRunner,
			AnnotateMiddleware(AuthMiddleware),
			AnnotateMiddleware(MachineMiddleware),
			AnnotateMiddleware(CorsMiddleware),
			AnnotateService(AuthService),
			GqlgenHandler,
			fx.Annotate(
				GinEngine,
				fx.ParamTags(`group:"services"`, `group:"middlewares"`),
			),
		),
		fx.Invoke(GinLifecycle),
	)

	app.Run()
}
