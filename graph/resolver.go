package graph

import (
	"context"
	"errors"

	"github.com/99designs/gqlgen/graphql"
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/graph/defs"
	"github.com/database-playground/backend-v2/graph/directive"
	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/events"
	"github.com/database-playground/backend-v2/internal/ranking"
	"github.com/database-playground/backend-v2/internal/sqlrunner"
	"github.com/database-playground/backend-v2/internal/submission"
	"github.com/database-playground/backend-v2/internal/useraccount"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.opentelemetry.io/otel/trace"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

// Resolver is the resolver root.
type Resolver struct {
	ent         *ent.Client
	auth        auth.Storage
	sqlrunner   *sqlrunner.SqlRunner
	useraccount *useraccount.Context

	eventService      *events.EventService
	submissionService *submission.SubmissionService
	rankingService    *ranking.Service
}

// NewResolver creates a new resolver.
func NewResolver(ent *ent.Client, auth auth.Storage, sqlrunner *sqlrunner.SqlRunner, useraccount *useraccount.Context, eventService *events.EventService, submissionService *submission.SubmissionService, rankingService *ranking.Service) *Resolver {
	return &Resolver{ent, auth, sqlrunner, useraccount, eventService, submissionService, rankingService}
}

// NewSchema creates a graphql executable schema.
func NewSchema(
	ent *ent.Client,
	auth auth.Storage,
	sqlrunner *sqlrunner.SqlRunner,
	useraccount *useraccount.Context,
	eventService *events.EventService,
	submissionService *submission.SubmissionService,
	rankingService *ranking.Service,
) graphql.ExecutableSchema {
	return NewExecutableSchema(Config{
		Resolvers: NewResolver(ent, auth, sqlrunner, useraccount, eventService, submissionService, rankingService),
		Directives: DirectiveRoot{
			Scope: directive.ScopeDirective,
		},
	})
}

func (r *Resolver) EntClient(ctx context.Context) *ent.Client {
	if entClient := ent.FromContext(ctx); entClient != nil {
		return entClient
	}

	return r.ent
}

func NewErrorPresenter() graphql.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		traceID := trace.SpanContextFromContext(ctx).TraceID().String()

		var gqlErr defs.GqlError
		if errors.As(err, &gqlErr) {
			return &gqlerror.Error{
				Message: gqlErr.Message,
				Path:    graphql.GetPath(ctx),
				Extensions: map[string]any{
					"code":     gqlErr.Code,
					"trace_id": traceID,
				},
			}
		}

		return graphql.DefaultErrorPresenter(ctx, err)
	}
}
