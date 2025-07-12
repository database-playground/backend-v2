package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Question struct {
	ent.Schema
}

func (Question) Fields() []ent.Field {
	return []ent.Field{
		field.String("category").NotEmpty().Unique().Immutable().Comment("Question category, e.g. 'query'").Annotations(
			entgql.OrderField("CATEGORY"),
		),
		field.Enum("difficulty").NamedValues(
			"Unspecified", "unspecified",
			"Easy", "easy",
			"Medium", "medium",
			"Hard", "hard",
		).
			Default("medium").
			Comment("Question difficulty, e.g. 'easy'").
			Annotations(entgql.OrderField("DIFFICULTY")),
		field.String("title").Comment("Question title"),
		field.Text("description").Comment("Question stem"),
		field.Text("reference_answer").Annotations(
			entgql.Directives(ScopeDirective("answer:read")),
		).Comment("Reference answer"),
	}
}

func (Question) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("database", Database.Type),
	}
}

func (Question) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField().Directives(
			ScopeDirective("question:read"),
		),
		entgql.Mutations(
			entgql.MutationCreate(),
			entgql.MutationUpdate(),
		),
		entgql.RelayConnection(),
	}
}
