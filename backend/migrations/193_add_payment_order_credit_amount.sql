-- Align payment_orders with Ent schema: credit_amount (USD credited to balance).
-- Idempotent for host DBs that already received a manual ADD COLUMN.

ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS credit_amount numeric(20, 6) NOT NULL DEFAULT 0;

COMMENT ON COLUMN payment_orders.credit_amount IS
    'USD credited to balance = amount / cny_per_usd at order time';
