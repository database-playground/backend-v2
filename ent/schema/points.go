package schema

import (
	"time"

	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Points is the schema for the points (users' scores) resource.
type Points struct {
	ent.Schema
}

func (Points) Fields() []ent.Field {
	return []ent.Field{
		field.Int("points").
			Default(0),
		field.Time("granted_at").
			Default(time.Now),
		field.String("description").
			Optional(),
	}
}

func (Points) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("points").Unique().Required(),
	}
}

func (Points) Annotations() []schema.Annotation {
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
