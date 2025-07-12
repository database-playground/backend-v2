package graph

import (
	"context"
	"errors"

	"github.com/99designs/gqlgen/graphql"
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/graph/defs"
	"github.com/database-playground/backend-v2/graph/directive"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/useraccount"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

// Resolver is the resolver root.
type Resolver struct {
	ent  *ent.Client
	auth auth.Storage
}

// NewSchema creates a graphql executable schema.
func NewSchema(ent *ent.Client, auth auth.Storage) graphql.ExecutableSchema {
	return NewExecutableSchema(Config{
		Resolvers: &Resolver{ent, auth},
		Directives: DirectiveRoot{
			Scope: directive.ScopeDirective,
		},
	})
}

func (r *Resolver) UserAccount(ctx context.Context) *useraccount.Context {
	entClient := ent.FromContext(ctx)

	return useraccount.NewContext(entClient, r.auth)
}

func (r *Resolver) EntClient(ctx context.Context) *ent.Client {
	if entClient := ent.FromContext(ctx); entClient != nil {
		return entClient
	}

	return r.ent
}

func NewErrorPresenter() graphql.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		var gqlErr defs.GqlError
		if errors.As(err, &gqlErr) {
			return &gqlerror.Error{
				Message: gqlErr.Message,
				Path:    graphql.GetPath(ctx),
				Extensions: map[string]any{
					"code": gqlErr.Code,
				},
			}
		}

		return graphql.DefaultErrorPresenter(ctx, err)
	}
}
