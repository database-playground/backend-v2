package schema

import (
	"time"

	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type CheatRecord struct {
	ent.Schema
}

func (CheatRecord) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.String("reason"),
		field.String("resolved_reason").Optional(),
		field.Time("resolved_at").Optional(),
		field.Time("cheated_at").Default(time.Now),
	}
}

func (CheatRecord) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("cheat_records").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (CheatRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField().Directives(
			ScopeDirective("cheat_record:read"),
		),
		entgql.RelayConnection(),
	}
}
