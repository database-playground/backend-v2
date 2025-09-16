package schema

import (
	"time"

	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Events records the events (what users do) of a user.
type Events struct {
	ent.Schema
}

func (Events) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.String("type").
			NotEmpty(),
		field.Time("triggered_at").
			Default(time.Now),
		field.JSON("payload", map[string]any{}).
			Optional(),
	}
}

func (Events) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("events").Field("user_id").Unique().Required(),
	}
}

func (Events) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("type"),
		index.Fields("type", "user_id"),
	}
}

func (Events) Annotations() []schema.Annotation {
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
