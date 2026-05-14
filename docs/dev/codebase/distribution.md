# Distribution

## Data Model

- `distribution_agents`: one row per user application and agent status.
- `distribution_wallets`: independent RMB wallet for approved distribution agents.
- `distribution_wallet_ledger`: append-only ledger for recharge, spend, and admin adjustments.

The distribution wallet is intentionally separate from the normal user balance.

## Key Files

- `backend/migrations/139_add_distribution_agents.sql`
- `backend/internal/service/distribution.go`
- `backend/internal/repository/distribution_repo.go`
- `backend/internal/handler/distribution_handler.go`
- `backend/internal/service/domain_constants.go`
- `backend/internal/service/setting_service.go`
- `backend/internal/server/routes/user.go`
- `backend/internal/server/routes/admin.go`
- `frontend/src/views/user/DistributionView.vue`
- `frontend/src/views/admin/DistributionView.vue`

## Core Flow

1. A signed-in user opens `/distribution` and sees application status, RMB wallet summary, and ledger.
2. If they have no application, they can submit contact information and an application reason.
3. Admins review applications at `/admin/distribution`.
4. Approved agents can generate:
   - balance redeem codes
   - subscription redeem codes
   - fixed-quota API keys
5. Generation deducts RMB from the distribution wallet at creation time. The generated asset is created in the same transaction.
6. Admins can update generation ratios, freeze wallets, and manually adjust balances.

## Important Mechanisms

- User API:
  - `GET /api/v1/distribution`
  - `POST /api/v1/distribution/apply`
  - `GET /api/v1/distribution/ledger`
  - `POST /api/v1/distribution/redeem-codes/balance`
  - `POST /api/v1/distribution/redeem-codes/subscription`
  - `POST /api/v1/distribution/api-keys`
- Admin API:
  - `GET /api/v1/admin/distribution/settings`
  - `PUT /api/v1/admin/distribution/settings`
  - `GET /api/v1/admin/distribution/applications`
  - `POST /api/v1/admin/distribution/applications/:user_id/review`
  - `GET /api/v1/admin/distribution/wallets`
  - `POST /api/v1/admin/distribution/wallets/:user_id/adjust`
  - `PUT /api/v1/admin/distribution/wallets/:user_id/status`
  - `GET /api/v1/admin/distribution/ledger`
- Wallet generation runs in one transaction with the generated redeem code or API key.
- Settings are stored in the existing Settings KV:
  - `distribution_rmb_per_usd`
  - `distribution_subscription_discount`

## Known Pitfalls

- Do not mix distribution wallet balance with the normal user `balance`.
- Balance codes and API keys must be created only after wallet balance is verified.
- Subscription code generation uses RMB face value and the admin-configured discount ratio.
- The generated API key still belongs to the distributor account; the customer receives the string, not a separate customer-owned key record.
