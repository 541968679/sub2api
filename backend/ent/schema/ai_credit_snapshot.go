package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AICreditSnapshot 记录 Antigravity AI Credits 的历史余额采样点。
//
// 远端 credits 余额只能实时查询，无法得知某个时间窗内"消耗了多少"，因此
// 采用定时采样方式：后台任务每 15 分钟拉一次所有启用 antigravity 账号的
// 余额（按 Google email 去重），持久化到本表。后续通过相邻采样点的正向
// delta 之和得到时间窗内消耗量，用于计算"每 credit 对应的额度/调用次数"。
//
// 删除策略：硬删除。本表是运营统计的原始数据，可按时间范围批量清理。
type AICreditSnapshot struct {
	ent.Schema
}

func (AICreditSnapshot) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ai_credit_snapshots"},
	}
}

func (AICreditSnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.String("email").
			MaxLen(255),
		field.String("credit_type").
			MaxLen(50),
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,6)"}),
		field.Time("captured_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (AICreditSnapshot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("email", "captured_at"),
		index.Fields("captured_at"),
	}
}
