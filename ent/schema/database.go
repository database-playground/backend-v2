package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Database refers to the SQLite schema of a question.
type Database struct {
	ent.Schema
}

func (Database) Fields() []ent.Field {
	return []ent.Field{
		field.String("slug").NotEmpty().Unique().Immutable(),
		field.String("description").Optional(),
		field.Text("schema").NotEmpty().Comment("SQL schema"),
		field.String("relation_figure").NotEmpty().Unique().Comment("relation figure"),
	}
}

func (Database) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("questions", Question.Type),
	}
}

func (Database) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField().Directives(
			ScopeDirective("database:read"),
		),
		entgql.Mutations(
			entgql.MutationCreate(),
			entgql.MutationUpdate(),
		),
	}
}
