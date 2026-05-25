# Distribution

## Data Model

- `distribution_agents`: one row per user application and agent status.
- `distribution_wallets`: independent RMB wallet for approved distribution agents.
- `distribution_wallet_ledger`: append-only ledger for recharge, spend, and admin adjustments.
- `distribution_assets`: immutable-ish tracking table for every distribution-generated balance code, subscription code, and API key package. It stores the original face value, original RMB cost, linked generated record, current effective status, and refund markers.

The distribution wallet is intentionally separate from the normal user balance.

## Key Files

- `backend/migrations/139_add_distribution_agents.sql`
- `backend/migrations/140_add_distribution_assets.sql`
- `backend/migrations/141_distribution_agent_rates_and_asset_refunds.sql`
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
5. Generation deducts RMB from the distribution wallet at creation time. The generated redeem code or API key and the `distribution_assets` record are created in the same transaction.
6. Users and admins can list generated assets and void active, unused assets. Voiding disables/expires the underlying generated record and refunds the original RMB cost to the distribution wallet.
7. Admins can update global generation ratios, set per-agent ratio overrides, freeze wallets, and manually adjust balances.

## Important Mechanisms

- User API:
  - `GET /api/v1/distribution`
  - `POST /api/v1/distribution/apply`
  - `GET /api/v1/distribution/ledger`
  - `GET /api/v1/distribution/assets`
  - `POST /api/v1/distribution/assets/:id/void`
  - `POST /api/v1/distribution/redeem-codes/balance`
  - `POST /api/v1/distribution/redeem-codes/subscription`
  - `GET /api/v1/distribution/api-key-groups`
  - `POST /api/v1/distribution/api-keys`
- Admin API:
  - `GET /api/v1/admin/distribution/settings`
  - `PUT /api/v1/admin/distribution/settings`
  - `PUT /api/v1/admin/distribution/agents/:user_id/rates`
  - `GET /api/v1/admin/distribution/applications`
  - `POST /api/v1/admin/distribution/applications/:user_id/review`
  - `GET /api/v1/admin/distribution/wallets`
  - `POST /api/v1/admin/distribution/wallets/:user_id/adjust`
  - `PUT /api/v1/admin/distribution/wallets/:user_id/status`
  - `GET /api/v1/admin/distribution/ledger`
  - `GET /api/v1/admin/distribution/assets`
  - `POST /api/v1/admin/distribution/assets/:id/void`
- Wallet generation runs in one transaction with the generated redeem code or API key.
- Asset void/refund also runs in one transaction. `distribution_assets.refunded_at/refunded_rmb` prevents duplicate refunds.
- Asset list status is derived from the linked `redeem_codes` or `api_keys` record where possible, so used/expired/disabled states reflect runtime state rather than only the creation snapshot.
- The user-facing agent page presents generated assets and wallet ledger as tabs in one history panel. Newly generated codes/API keys appear in the generated-assets action area for immediate copy, and asset search is sent through the existing `GET /api/v1/distribution/assets?search=...` parameter.
- Settings are stored in the existing Settings KV:
  - `distribution_rmb_per_usd`
  - `distribution_subscription_discount`
  - `distribution_api_key_group_ids` (JSON array of active standard group IDs exposed for agent API key generation)
- Agent-specific overrides are stored directly on `distribution_agents`:
  - `rmb_per_usd_override`
  - `subscription_discount_override`
- Effective ratio precedence is `agent override > global setting`. There is no product-template ratio layer.
- Agent API key generation uses the distribution group whitelist as the permission source. The user-facing agent page reads `GET /api/v1/distribution/api-key-groups`, and `POST /api/v1/distribution/api-keys` rejects groups outside `distribution_api_key_group_ids`; it does not expose every group returned by `/groups/available`.

## Known Pitfalls

- Do not mix distribution wallet balance with the normal user `balance`.
- Balance codes and API keys must be created only after wallet balance is verified.
- Subscription code generation uses RMB face value and the admin-configured discount ratio.
- User-facing distribution summary returns the effective ratios for that agent, not only the global settings.
- Voiding a used balance/subscription code must not refund. Voiding an unused redeem code marks it expired; voiding an API key disables the key.
- Refund amount is the original RMB cost recorded on the generated asset, not current global or agent ratio recalculation.
- The generated API key still belongs to the distributor account; the customer receives the string, not a separate customer-owned key record.
- Empty `distribution_api_key_group_ids` means no groups are exposed to agents for API key generation.
