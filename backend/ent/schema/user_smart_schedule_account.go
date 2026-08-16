package schema

import (
	"fmt"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserSmartScheduleAccount is one pool member for a user's smart-schedule platform.
type UserSmartScheduleAccount struct {
	ent.Schema
}

func (UserSmartScheduleAccount) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_smart_schedule_accounts"},
		field.ID("account_id", "user_id"),
	}
}

func (UserSmartScheduleAccount) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id"),
		field.Int64("user_id"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("platform").
			MaxLen(32).
			NotEmpty().
			Validate(func(s string) error {
				switch s {
				case "anthropic", "openai", "gemini", "antigravity", "grok":
					return nil
				default:
					return fmt.Errorf("platform %q is not allowed", s)
				}
			}),
		field.Int("max_concurrency").
			Optional().
			Nillable(),
	}
}

func (UserSmartScheduleAccount) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).
			Unique().
			Required().
			Field("user_id"),
		edge.To("account", Account.Type).
			Unique().
			Required().
			Field("account_id"),
	}
}

func (UserSmartScheduleAccount) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "platform"),
		index.Fields("account_id"),
	}
}
