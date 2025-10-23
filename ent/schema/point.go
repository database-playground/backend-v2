package schema

import (
	"time"

	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Point is the schema for the points (users' scores) resource.
type Point struct {
	ent.Schema
}

func (Point) Fields() []ent.Field {
	return []ent.Field{
		field.Int("points").
			Default(0),
		field.Time("granted_at").
			Default(time.Now).
			Annotations(entgql.OrderField("GRANTED_AT")),
		field.String("description").
			Optional(),
	}
}

func (Point) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("points").Unique().Required(),
	}
}

func (Point) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField().Directives(
			ScopeDirective("user:read"),
		),
		entgql.RelayConnection(),
		entgql.Mutations(
			entgql.MutationCreate(),
		),
	}
}
