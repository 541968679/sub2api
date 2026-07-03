package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// ImageChannelMonitor stores OpenAI-compatible image generation monitor config.
type ImageChannelMonitor struct {
	ent.Schema
}

func (ImageChannelMonitor) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			NotEmpty().
			MaxLen(100),
		field.Enum("source_type").
			Values("custom", "account").
			Default("custom"),
		field.String("endpoint").
			Optional().
			Default("").
			MaxLen(500),
		field.String("api_key_encrypted").
			Optional().
			Default("").
			Sensitive(),
		field.Int64("account_id").
			Optional().
			Nillable(),
		field.String("account_name").
			Optional().
			Default("").
			MaxLen(200),
		field.String("model").
			NotEmpty().
			MaxLen(200),
		field.String("prompt").
			NotEmpty().
			MaxLen(2000),
		field.String("size").
			Optional().
			Default("1024x1024").
			MaxLen(32),
		field.String("quality").
			Optional().
			Default("auto").
			MaxLen(32),
		field.Int("n").
			Default(1).
			Range(1, 10),
		field.Bool("download_image").
			Default(true),
		field.Bool("enabled").
			Default(true),
		field.Int("interval_seconds").
			Default(300).
			Range(15, 3600),
		field.Int("timeout_seconds").
			Default(300).
			Range(30, 600),
		field.Time("last_checked_at").
			Optional().
			Nillable(),
		field.Int64("created_by"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (ImageChannelMonitor) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("histories", ImageChannelMonitorHistory.Type),
	}
}
