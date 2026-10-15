# Affiliate

邀请返利。返利记在邀请账本上，不进入 `users.balance`，也不改分销钱包。

## Data Model

- `user_affiliates` stores the one-bind inviter, invite permission, quota, per-inviter overrides, and the master/salesperson edge.
  - `sales_parent_id` is the master who recruited this salesperson. It is written only at registration.
  - `sales_code` belongs to a master. It is unique among non-null sales codes and is also checked against `aff_code` when generated. Disabling the master keeps the code.
  - `is_master` is set by an administrator. A row with `sales_parent_id` cannot become a master.
  - `master_cut_percent` is the master's cut of each subordinate's customer rebate. Null or 0 means no cut until the master sets one.
  - `aff_invite_enabled` defaults to false. Migration `255_affiliate_invite_permission.sql` does not grandfather existing inviters.
  - `aff_quota` is settleable. A negative value is debt.
  - `aff_frozen_quota` is not settleable until `frozen_until`.
  - `aff_history_quota` only increases. Reverse and settle do not shrink it.
  - Nullable `aff_rebate_rate_percent`, `aff_rebate_per_invitee_cap`, `aff_rebate_freeze_hours`, and `aff_rebate_duration_days` inherit the global setting when null. Explicit 0 is a real value: rate 0 pays nothing, cap 0 is unlimited, freeze 0 is immediately settleable, duration 0 never expires.
- `user_affiliate_ledger` is the audit source. `action` is `accrue`, `reverse`, `settle`, or historical `transfer`.
  - New rows set `source_kind` (`payment_order`, `redeem_code`, `admin_balance`), `source_id`, and for reverses `reverse_seq`.
  - A split writes two rows for one source. The direct row has `user_id` and `direct_inviter_id` equal to the salesperson and `accrual_role = direct`. The master row has `user_id` equal to the master, `direct_inviter_id` equal to the salesperson, and `accrual_role = master_cut`. Both store `cut_percent_snapshot`.
  - Historical payment accruals keep `source_order_id` and a null `source_kind`. They are not backfilled and must not be updated. A null `accrual_role` is the old direct rebate and stays outside the role indexes.
- Migration 256 replaces the two partial unique indexes so they include `accrual_role`. Accrue uniqueness is `(source_kind, source_id, accrual_role)`. Reverse uniqueness adds `reverse_seq`. Rows with a null role or null source stay outside those indexes, so payment accrual still checks `source_order_id`. Any existing accrue for a source, including a legacy null-role row, blocks a later master cut on that same source.
- There is no Ent schema for these tables. SQL is raw.

## Key Files

- `backend/migrations/255_affiliate_invite_permission.sql`
- `backend/migrations/256_affiliate_master_agent.sql`
- `backend/internal/service/affiliate_policy.go`
- `backend/internal/service/affiliate_ledger_service.go`
- `backend/internal/service/affiliate_service.go`
- `backend/internal/repository/affiliate_ledger_repo.go`
- `backend/internal/repository/affiliate_repo.go`
- `backend/internal/service/payment_fulfillment.go`
- `backend/internal/service/redeem_service.go`
- `backend/internal/service/payment_refund.go`
- `backend/internal/service/admin_service.go`
- `backend/internal/handler/admin/affiliate_handler.go`
- `backend/internal/server/routes/admin.go`
- `frontend/src/utils/oauthAffiliate.ts`
- `frontend/src/views/auth/RegisterView.vue`
- `frontend/src/views/user/AffiliateView.vue`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/views/admin/affiliates/`

## Core Flow

1. An administrator enables `aff_invite_enabled`. `GET /api/v1/auth/me` returns that flag. The user sidebar and `/affiliate` stay closed until both the global switch and this flag are on.
2. A new user binds one code. `?aff=` calls the customer bind and sets `inviter_id`. `?sales=` looks up `sales_code` only, sets `sales_parent_id`, and turns that user's invite permission on. It does not set `inviter_id`. Both query params are rejected before the user row is created. A sales code used as a customer code fails closed, and the reverse fails closed. Bind failures of a single code do not block registration. The customer relationship is not rewritten later, and v1 has no admin rebind of `sales_parent_id`.
3. A successful balance credit accrues rebate inside the same transaction:
   - online balance orders accrue `payment_order` from `CreditAmount` (falling back to `Amount` only when credit is empty);
   - a normal balance redeem code accrues `redeem_code` from its face value;
   - an admin balance increase accrues `admin_balance` from the positive delta.
   The generated redeem code of an online order is not accrued a second time.
4. The inviter row is locked, then the master row if the inviter has a sales parent. Locks use ascending user id. Permission, duration, rate, cap, and freeze are resolved on the direct inviter. Permission off or an expired duration writes nothing, so there is nothing to split. The per-invitee cap uses gross R: accrue minus reverse where `source_user_id` is the invitee and (`user_id` is the inviter or `direct_inviter_id` is the inviter).
5. If the direct inviter has an active master, and the effective cut is above 0, R is split. Effective cut is `min(stored cut, current admin ceiling)`. The ceiling setting is `affiliate_master_cut_max_percent` (default 50, absolute clamp 80). The master amount is rounded to 8 decimals first. The salesperson receives R minus that amount. A zero share is skipped. If any positive share is inserted, the source is processed, so a retry cannot accrue it again. No master, an inactive master, a cut of 0, or the inviter being the master themselves leaves the whole R on one direct row.
6. Refunds and admin balance decreases reverse both original rows by their stored amounts, not a recomputed rate. Reverse rows keep `accrual_role` and `direct_inviter_id`. Admin decreases group the two roles of one source, split the take in proportion, and give the remainder to the last positive row. Already accrued quota stays.
7. An administrator records an offline payout with `POST /api/v1/admin/affiliates/users/:user_id/settle`. The note is required. The amount must be greater than 0 and no greater than settleable quota. Mature frozen quota is thawed inside that transaction before the check. Thaw matches reverses of the same role. A null-role historical accrue matches a null-role reverse or a `direct` reverse.
8. `PUT /api/v1/user/aff/master-cut` stores the master's cut. It rejects a value outside 0 through the current ceiling. `PUT /api/v1/admin/affiliates/users/:user_id` applies `is_master` after the invite flag, so enabling a master also enables invite. Enabling generates `sales_code` when empty (`S` plus the rest of a normal affiliate code).

## Important Mechanisms

- `POST /api/v1/user/aff/transfer` stays mounted and returns `AFFILIATE_TRANSFER_DISABLED`. It does not credit `users.balance`. Historical transfers are not clawed back.
- Payment reverse proportion is `original rebate * refunded credited / original credited`, capped by what has not already been reversed. `reverse_seq` is the refunded credited amount in 1e-8 units. The current product allows one refund per order.
- A frozen reverse reduces `aff_frozen_quota` first while that accrue is still frozen, then `aff_quota`. Thaw later moves `GREATEST(amount - matching reverses, 0)`, so a partial frozen reverse is not thawed twice.
- Admin balance changes that have the affiliate service wired create the adjustment redeem record in the same transaction and fail the balance change if that record cannot be written. Tests that leave the service unset keep the older best-effort path.
- Clearing a custom user disables invite, nulls rate, cap, freeze, and duration, resets the customer code, and sets `is_master` false. It keeps `sales_code` and does not clear children's `sales_parent_id`. Existing quota remains. New cuts stop because the master flag is off.
- Disabling a master stops the sales code from binding new salespeople and stops new cuts. Accrued quota stays settleable. Disabling the master's invite permission does not by itself stop recruiting while `is_master` stays true.
- Global off stops new customer binds, new sales binds, and new rebate. A sales bind while global is off returns success without revealing whether the code exists. Existing settleable quota can still be settled.
- The same pair cannot be both a customer-invite edge and a sales edge. Registration rejects both params. The sales bind also rejects the pair when either user is the other's `inviter_id`.
- Frontend stores `?sales=` in `sales_referral_code` and `oauth_sales_code`. Those keys are not the customer-code keys. If both kinds resolve, the page stores and sends neither.
- Rebate records still join `payment_orders` on `source_order_id`. The transfers list reads `transfer`, `settle`, and `reverse`.

## Known Pitfalls

- Do not accrue both the payment order and the redeem code it generates. The payment fact on the redeem context wins over the skip flag.
- Do not update historical accrue rows to fill `source_kind`. Payment reverse finds them with `source_kind IS NULL AND source_order_id = order id`, then writes a sourced reverse.
- A unique violation inside `withTx` must return an error from the callback. Returning nil would commit the quota change. The outer accrue maps that conflict to a no-op. A duplicate reverse rolls the surrounding payment or admin transaction back.
- `aff_history_quota` is not a payable balance. Settle and reverse must not subtract it.
- Rollback of the new binary while the global switch is on would treat every profile as able to invite, because the old binary ignores `aff_invite_enabled`. The old binary also writes accrues without `accrual_role`, and migration 256's unique index does not cover those null-role inserts, so a replay can accrue again. Turn the global switch off before that rollback.
- Do not pass a sales code to the customer bind, and do not treat `aff_code` as a sales code. The two lookups are different columns.
- Do not attach a late `master_cut` row onto a source that already has any accrue row. Do not edit migration 255 or historical accrue rows.
- A unique violation on either split insert must return from the `withTx` callback so both quota updates roll back. The outer sourced accrual maps that conflict to a no-op.
- Platform payout for one recharge stays equal to R. The cut is not a second percentage of the recharge.
- Display billing, `actual_cost`, and quota deduction are not inputs to this ledger. Do not read `usage_logs` to compute rebate.
- The distribution wallet is a different product. Do not settle affiliate quota by crediting that wallet or `users.balance`.
