-- 168_subscription_plan_member_groups.sql
--
-- Mixed/bundle subscription support.
--
-- A subscription plan can now bundle multiple subscription-type groups. One
-- purchase fans out into N independent user_subscription rows (one per member
-- group), each with its own quota pool. The effective member set is
-- unique(group_id ∪ member_group_ids), with group_id kept as the primary /
-- representative group for price, sort, display and single-group backward
-- compatibility.
--
-- payment_orders snapshots the member set at order creation so that editing the
-- plan afterwards does not change async / retried fulfillment accounting.
--
-- Additive, idempotent, no backfill: existing single-group plans/orders keep an
-- empty array ('[]') and behave exactly as before.

ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS member_group_ids JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS member_group_ids JSONB NOT NULL DEFAULT '[]'::jsonb;
