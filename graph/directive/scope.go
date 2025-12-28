package directive

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/database-playground/backend-v2/graph/defs"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/scope"
	otelcodes "go.opentelemetry.io/otel/codes"
)

func ScopeDirective(ctx context.Context, obj any, next graphql.Resolver, fnScope string) (res any, err error) {
	ctx, span := tracer.Start(ctx, "ScopeDirective")
	defer span.End()

	if fnScope == "" {
		span.SetStatus(otelcodes.Ok, "GraphQL field is public, allowing access")
		return next(ctx)
	}

	user, ok := auth.GetUser(ctx)
	if !ok {
		span.SetStatus(otelcodes.Error, "User not found")
		return nil, defs.ErrUnauthorized
	}

	if scope.ShouldAllow(fnScope, user.Scopes) {
		span.SetStatus(otelcodes.Ok, "User has sufficient scope, allowing access")
		return next(ctx)
	}

	span.SetStatus(otelcodes.Error, "User does not have sufficient scope")
	return nil, defs.NewErrNoSufficientScope(fnScope)
}
