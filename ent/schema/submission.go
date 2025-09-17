package schema

import (
	"time"

	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/database-playground/backend-v2/models"
)

type Submission struct {
	ent.Schema
}

func (Submission) Fields() []ent.Field {
	return []ent.Field{
		field.String("submitted_code").NotEmpty(), // SQL code submitted by the user
		field.Enum("status").
			Values("pending", "success", "failed"), // Status of the submission
		field.JSON("result", models.SubmissionResult{}), // Result of the submission
		field.Time("submitted_at").Default(time.Now),    // Time the submission was made
	}
}

func (Submission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("question", Question.Type).
			Ref("submissions").
			Unique().
			Required(),
		edge.From("user", User.Type).
			Ref("submissions").
			Unique().
			Required(),
	}
}

func (Submission) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField().Directives(
			ScopeDirective("submissions:read"),
		),
		entgql.RelayConnection(),
	}
}
