package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// OpenAIOAuth7dCycle stores a closed Codex 7-day window's LiteLLM cost snapshot.
type OpenAIOAuth7dCycle struct {
	ent.Schema
}

func (OpenAIOAuth7dCycle) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "openai_oauth_7d_cycles"},
	}
}

func (OpenAIOAuth7dCycle) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id"),
		field.Time("window_start").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("window_end").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Float("litellm_cost").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("used_percent").
			Default(0),
		field.Time("closed_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (OpenAIOAuth7dCycle) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id", "window_end").Unique(),
		index.Fields("account_id"),
	}
}
