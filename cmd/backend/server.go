package main

import (
	"go.uber.org/fx"

	"github.com/database-playground/backend-v2/internal/deps"

	_ "github.com/database-playground/backend-v2/ent/runtime"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	app := fx.New(
		fx.Provide(
			// Config
			BackendConfig,

			// Database
			EntClient,
			RedisClient,

			// External Services
			SqlRunner,

			// Internal Services
			AuthStorage,
			EventService,
			UserAccountContext,
			SubmissionService,
			RankingService,
			AnnotateService(AuthService),

			// Statistics
			PostHogClient,

			// GraphQL
			ApqCache,
			GqlgenHandler,

			// HTTP
			fx.Annotate(
				GinEngine,
				fx.ParamTags(`group:"services"`),
			),
		),
		fx.Invoke(deps.OTelSDK),
		fx.Invoke(GinLifecycle),
	)

	app.Run()
}
