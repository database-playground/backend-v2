package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
// Code generated by github.com/99designs/gqlgen version v0.17.76

import (
	"context"

	"entgo.io/contrib/entgql"
	"github.com/database-playground/backend-v2/ent"
	"github.com/database-playground/backend-v2/graph/defs"
)

// Node is the resolver for the node field.
func (r *queryResolver) Node(ctx context.Context, id int) (ent.Noder, error) {
	// FIXME: Do not implement the node resolver for now,
	// since we can't do scope check in the node resolver.
	return nil, defs.ErrNotImplemented
}

// Nodes is the resolver for the nodes field.
func (r *queryResolver) Nodes(ctx context.Context, ids []int) ([]ent.Noder, error) {
	// FIXME: Do not implement the node resolver for now,
	// since we can't do scope check in the node resolver.
	return nil, defs.ErrNotImplemented
}

// Databases is the resolver for the databases field.
func (r *queryResolver) Databases(ctx context.Context) ([]*ent.Database, error) {
	entClient := r.EntClient(ctx)

	return entClient.Database.Query().All(ctx)
}

// Groups is the resolver for the groups field.
func (r *queryResolver) Groups(ctx context.Context) ([]*ent.Group, error) {
	entClient := r.EntClient(ctx)

	return entClient.Group.Query().All(ctx)
}

// Questions is the resolver for the questions field.
func (r *queryResolver) Questions(ctx context.Context, after *entgql.Cursor[int], first *int, before *entgql.Cursor[int], last *int, orderBy *ent.QuestionOrder, where *ent.QuestionWhereInput) (*ent.QuestionConnection, error) {
	entClient := r.EntClient(ctx)

	return entClient.Question.Query().Paginate(ctx, after, first, before, last, ent.WithQuestionOrder(orderBy), ent.WithQuestionFilter(where.Filter))
}

// ScopeSets is the resolver for the scopeSets field.
func (r *queryResolver) ScopeSets(ctx context.Context) ([]*ent.ScopeSet, error) {
	entClient := r.EntClient(ctx)

	return entClient.ScopeSet.Query().All(ctx)
}

// Users is the resolver for the users field.
func (r *queryResolver) Users(ctx context.Context, after *entgql.Cursor[int], first *int, before *entgql.Cursor[int], last *int, orderBy *ent.UserOrder, where *ent.UserWhereInput) (*ent.UserConnection, error) {
	entClient := r.EntClient(ctx)

	return entClient.User.Query().Paginate(ctx, after, first, before, last, ent.WithUserOrder(orderBy), ent.WithUserFilter(where.Filter))
}

// Query returns QueryResolver implementation.
func (r *Resolver) Query() QueryResolver { return &queryResolver{r} }

// Question returns QuestionResolver implementation.
func (r *Resolver) Question() QuestionResolver { return &questionResolver{r} }

// User returns UserResolver implementation.
func (r *Resolver) User() UserResolver { return &userResolver{r} }

type queryResolver struct{ *Resolver }
type questionResolver struct{ *Resolver }
type userResolver struct{ *Resolver }
