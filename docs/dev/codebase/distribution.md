# Distribution

## Data Model

- `distribution_agents`: one row per user application and agent status.
- `distribution_wallets`: independent wallet for approved distribution agents.
- `distribution_wallet_ledger`: append-only wallet ledger for future recharge, spend, and rebate operations.

The distribution wallet is intentionally separate from the normal user balance.

## Key Files

- `backend/migrations/139_add_distribution_agents.sql`
- `backend/internal/service/distribution.go`
- `backend/internal/repository/distribution_repo.go`
- `backend/internal/handler/distribution_handler.go`
- `backend/internal/server/routes/user.go`
- `backend/internal/server/routes/admin.go`
- `frontend/src/views/user/DistributionView.vue`
- `frontend/src/views/admin/DistributionView.vue`

## Core Flow

1. A signed-in user opens `/distribution` and sees their current application and wallet summary.
2. If they have no application, they can submit contact information and an application reason.
3. Admins review applications at `/admin/distribution`.
4. Approving an application creates or ensures the user's distribution wallet.
5. Users can view wallet ledger records after approval; the first release does not yet write recharge/spend/rebate ledger entries.

## Important Mechanisms

- User API:
  - `GET /api/v1/distribution`
  - `POST /api/v1/distribution/apply`
  - `GET /api/v1/distribution/ledger`
- Admin API:
  - `GET /api/v1/admin/distribution/applications`
  - `POST /api/v1/admin/distribution/applications/:user_id/review`
- Wallet creation is delayed until approval. Pending applications do not create wallets.
- The first implementation only covers application, review, summary, and ledger viewing.

## Known Pitfalls

- Do not mix distribution wallet balance with the normal user `balance`.
- Future recharge, redeem-code generation, API-key package generation, and subscription coupon cashback must write `distribution_wallet_ledger` atomically with balance changes.
- Concrete discount/cashback formulas are intentionally not hard-coded in the first release.
