package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// ImageChannelMonitorHistory stores per-run image generation timing data.
type ImageChannelMonitorHistory struct {
	ent.Schema
}

func (ImageChannelMonitorHistory) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("monitor_id"),
		field.Enum("status").
			Values("operational", "degraded", "failed", "error").
			Default("error"),
		field.Int("http_status").
			Optional().
			Nillable(),
		field.Int("api_header_ms").
			Optional().
			Nillable(),
		field.Int("api_body_ms").
			Optional().
			Nillable(),
		field.Int("api_total_ms").
			Optional().
			Nillable(),
		field.Int("json_bytes").
			Optional().
			Nillable(),
		field.Bool("has_url").
			Default(false),
		field.Bool("has_b64_json").
			Default(false),
		field.String("image_url_host").
			Optional().
			Default("").
			MaxLen(255),
		field.Int("image_first_byte_ms").
			Optional().
			Nillable(),
		field.Int("image_download_ms").
			Optional().
			Nillable(),
		field.Int64("image_bytes").
			Optional().
			Nillable(),
		field.String("image_content_type").
			Optional().
			Default("").
			MaxLen(100),
		field.Int("image_width").
			Optional().
			Nillable(),
		field.Int("image_height").
			Optional().
			Nillable(),
		field.String("error_stage").
			Optional().
			Default("").
			MaxLen(64),
		field.String("message").
			Optional().
			Default("").
			MaxLen(500),
		field.Time("checked_at").
			Default(time.Now),
	}
}

func (ImageChannelMonitorHistory) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("monitor", ImageChannelMonitor.Type).
			Ref("histories").
			Field("monitor_id").
			Unique().
			Required(),
	}
}
