package directive

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/database-playground/backend-v2/graph/defs"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/scope"
)

func ScopeDirective(ctx context.Context, obj any, next graphql.Resolver, fnScope string) (res any, err error) {
	if fnScope == "" {
		return next(ctx)
	}

	user, ok := auth.GetUser(ctx)
	if !ok {
		return nil, defs.ErrUnauthorized
	}

	if scope.ShouldAllow(fnScope, user.Scopes) {
		return next(ctx)
	}

	return nil, defs.ErrNoSufficientScope
}
