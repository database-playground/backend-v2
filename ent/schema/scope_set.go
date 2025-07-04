package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// ScopeSet is a set of scopes.
//
// This is used for controlling the scope of a group.
// For example, a group can have a scope set of "user:read" and "user:write",
// which means all users in this group can read and write their own user data.
type ScopeSet struct {
	ent.Schema
}

func (ScopeSet) Fields() []ent.Field {
	return []ent.Field{
		field.String("slug").NotEmpty().Unique().Immutable(),
		field.String("description").Optional(),
		field.JSON("scopes", []string{}).
			Default([]string{}),
	}
}

func (ScopeSet) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField().Directives(
			ScopeDirective("scopeset:read"),
		),
		entgql.Mutations(
			entgql.MutationCreate(),
			entgql.MutationUpdate(),
		),
		entgql.RelayConnection(),
	}
}
