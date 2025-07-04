package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Group is the schema for the group resource.
//
// "Group" is a high-level concepts to distinguish users
// (e.g. "Admin", "Class 1", "Class 2", etc.)
type Group struct {
	ent.Schema
}

func (Group) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			NotEmpty(),
		field.String("description").
			Optional(),
	}
}

func (Group) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimestampMixin{},
	}
}

func (Group) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("scope_set", ScopeSet.Type),
	}
}

func (Group) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField().Directives(
			ScopeDirective("group:read"),
		),
		entgql.Mutations(
			entgql.MutationCreate(),
			entgql.MutationUpdate(),
		),
		entgql.RelayConnection(),
	}
}
