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

// Event records the events (what users do) of a user.
type Event struct {
	ent.Schema
}

func (Event) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.String("type").
			NotEmpty(),
		field.Time("triggered_at").
			Default(time.Now).
			Annotations(entgql.OrderField("TRIGGERED_AT")),
		field.JSON("payload", map[string]any{}).
			Optional(),
	}
}

func (Event) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("events").Field("user_id").Unique().Required(),
	}
}

func (Event) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("type"),
		index.Fields("type", "user_id"),
	}
}

func (Event) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField().Directives(
			ScopeDirective("user:read"),
		),
		entgql.RelayConnection(),
		entgql.OrderField("triggered_at"),
	}
}
