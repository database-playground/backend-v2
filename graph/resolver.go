package graph

import (
	"context"
	"errors"

	"github.com/99designs/gqlgen/graphql"
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/scope"
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
			Scope: func(ctx context.Context, obj any, next graphql.Resolver, fnScope string) (res any, err error) {
				if fnScope == "" {
					return next(ctx)
				}

				user, ok := auth.GetUser(ctx)
				if !ok {
					return nil, errors.New("unauthorized")
				}

				if scope.ShouldAllow(fnScope, user.Scopes) {
					return next(ctx)
				}

				return nil, errors.New("no sufficient scope")
			},
		},
	})
}
