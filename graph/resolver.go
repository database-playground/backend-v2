package graph

import (
	"context"
	"errors"

	"github.com/99designs/gqlgen/graphql"
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/graph/defs"
	"github.com/database-playground/backend-v2/graph/directive"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

// Resolver is the resolver root.
type Resolver struct{ client *ent.Client }

// NewSchema creates a graphql executable schema.
func NewSchema(client *ent.Client) graphql.ExecutableSchema {
	return NewExecutableSchema(Config{
		Resolvers: &Resolver{client},
		Directives: DirectiveRoot{
			Scope: directive.ScopeDirective,
		},
	})
}

func NewErrorPresenter() graphql.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		if errors.Is(err, defs.ErrUnauthorized) {
			return &gqlerror.Error{
				Message: "require authentication",
				Path:    graphql.GetPath(ctx),
				Extensions: map[string]any{
					"code": "UNAUTHORIZED",
				},
			}
		}

		if errors.Is(err, defs.ErrNoSufficientScope) {
			return &gqlerror.Error{
				Message: "no sufficient scope",
				Path:    graphql.GetPath(ctx),
				Extensions: map[string]any{
					"code": "FORBIDDEN",
				},
			}
		}

		return graphql.DefaultErrorPresenter(ctx, err)
	}
}
