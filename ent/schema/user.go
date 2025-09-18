package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// User is the schema for the user resource.
type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			NotEmpty(),
		field.String("email").
			NotEmpty().
			Unique().
			Immutable().
			Annotations(entgql.OrderField("EMAIL")),
		field.String("avatar").
			Optional(),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("group", Group.Type).Unique().Required(),
		edge.To("points", Points.Type),
		edge.To("events", Events.Type),
		edge.To("submissions", Submission.Type),
	}
}

func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimestampMixin{},
	}
}

func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField().Directives(
			ScopeDirective("user:read"),
		),
		entgql.Mutations(
			entgql.MutationCreate(),
			entgql.MutationUpdate(),
		),
		entgql.RelayConnection(),
	}
}
