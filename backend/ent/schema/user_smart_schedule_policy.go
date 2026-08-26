package schema

import (
	"fmt"

	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserSmartSchedulePolicy is one (user, platform) smart-schedule policy row.
type UserSmartSchedulePolicy struct {
	ent.Schema
}

func (UserSmartSchedulePolicy) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_smart_schedule_policies"},
	}
}

func (UserSmartSchedulePolicy) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (UserSmartSchedulePolicy) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
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
		field.Bool("enabled").
			Default(false),
		field.Int("quality_max_p50_ttft_ms").
			Optional().
			Nillable(),
		field.Float("quality_min_success_rate").
			Optional().
			Nillable(),
		field.Int("quality_min_success_samples").
			Optional().
			Nillable(),
		field.Int("quality_min_ttft_samples").
			Optional().
			Nillable(),
		field.String("quality_condition").
			Optional().
			Nillable().
			MaxLen(8),
		field.Int("cooldown_minutes").
			Default(15).
			SchemaType(map[string]string{dialect.Postgres: "integer"}),
		field.Bool("soft_cooldown").
			Default(false),
	}
}

func (UserSmartSchedulePolicy) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("smart_schedule_policies").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (UserSmartSchedulePolicy) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "platform").
			Unique(),
		index.Fields("user_id"),
	}
}
