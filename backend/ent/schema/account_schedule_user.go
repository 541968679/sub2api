package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AccountScheduleUser holds the edge schema for account_schedule_users.
// It stores which users an account is allowed or denied for scheduling.
type AccountScheduleUser struct {
	ent.Schema
}

func (AccountScheduleUser) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "account_schedule_users"},
		field.ID("account_id", "user_id"),
	}
}

func (AccountScheduleUser) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id"),
		field.Int64("user_id"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (AccountScheduleUser) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("account", Account.Type).
			Unique().
			Required().
			Field("account_id"),
		edge.To("user", User.Type).
			Unique().
			Required().
			Field("user_id"),
	}
}

func (AccountScheduleUser) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
	}
}
