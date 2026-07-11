# Affiliate

## Data Model

- `user_affiliates` stores invitation relationships, configured rebate rate, available/frozen quota, and lifetime quota.
- `user_affiliate_ledger` is the audit source for rebate accrual and transfers to balance.
- Migration `185_affiliate_ledger_audit_snapshots.sql` adds payment-order linkage and post-transfer snapshots. Historical rows that cannot be matched unambiguously remain nullable.

## Key Files

- `backend/internal/service/affiliate_service.go`
- `backend/internal/repository/affiliate_repo.go`
- `backend/internal/handler/admin/affiliate_handler.go`
- `backend/internal/server/routes/admin.go`
- `frontend/src/api/admin/affiliates.ts`
- `frontend/src/views/admin/affiliates/`

## Core Flow

1. A user binds an inviter through the existing affiliate flow.
2. Eligible balance payment or positive balance redeem completion calls the affiliate service.
3. The repository records an `accrue` ledger row, optionally linked to the payment order, and places quota in available or frozen storage.
4. Mature frozen quota is included in the admin overview and becomes transferable under the existing thaw/transfer rules.
5. Admin pages query invite, rebate, and transfer records with search, date range, sorting, and pagination.

## Important Mechanisms

- Payment orders pass `source_order_id`; admin rebate records join the ledger to the exact payment order rather than parsing audit log text.
- Transfer ledger rows capture balance, available quota, frozen quota, and lifetime quota after the transfer. Old rows expose `snapshot_available=false` instead of presenting current state as historical state.
- The migration backfill only links one-to-one audit/ledger matches within a bounded time window.
- The payment fulfillment audit remains best effort, but the ledger transaction is authoritative for affiliate state.

## Known Pitfalls

- Do not accrue both from a payment-generated redeem code and the payment order. The internal redeem context marker suppresses the former.
- Admin overview available quota includes already matured frozen accruals; omitting them understates transferable quota.
- Do not infer historical transfer snapshots from current user balances.
