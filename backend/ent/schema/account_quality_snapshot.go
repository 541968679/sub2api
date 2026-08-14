package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AccountQualitySnapshot stores a periodic copy of the live 15-minute account
// quality window (same contract as list quality-stats). Rows are written every
// 5 minutes for accounts that had traffic; empty windows are not persisted.
type AccountQualitySnapshot struct {
	ent.Schema
}

func (AccountQualitySnapshot) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "account_quality_snapshots"},
	}
}

func (AccountQualitySnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id"),
		field.Time("captured_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int("window_seconds").
			Default(900),
		field.Int64("success_count").
			Default(0),
		field.Int64("error_count").
			Default(0),
		field.Int64("ttft_samples").
			Default(0),
		field.Float("success_rate").
			Optional().
			Nillable(),
		field.Int("avg_ttft_ms").
			Optional().
			Nillable(),
		field.Int("p50_ttft_ms").
			Optional().
			Nillable(),
		field.Int("p95_ttft_ms").
			Optional().
			Nillable(),
		field.Int("max_ttft_ms").
			Optional().
			Nillable(),
	}
}

func (AccountQualitySnapshot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id", "captured_at").Unique(),
		index.Fields("captured_at"),
		index.Fields("account_id"),
	}
}
