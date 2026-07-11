# Subscription Management

## Data model

`user_subscriptions` uses Ent's `SoftDeleteMixin`. A revoked subscription keeps
its persisted `status` and receives `deleted_at`; `revoked` is an admin API
display state derived from `deleted_at`, not a value written to `status`.
Migration `016_soft_delete_partial_unique_indexes.sql` owns the partial unique
index on `(user_id, group_id) WHERE deleted_at IS NULL`, so revoked history can
coexist with at most one live subscription for the same user and group.

## Key files

- `backend/internal/repository/user_subscription_repo.go`: active-scoped reads,
  admin include-deleted reads, soft delete, and atomic restore.
- `backend/internal/service/subscription_service.go`: assign, revoke, restore,
  quota maintenance, L1 cache, and cross-instance invalidation.
- `backend/internal/handler/admin/subscription_handler.go`: admin actions.
- `backend/internal/server/routes/admin.go`: explicit revoke/restore routes.
- `frontend/src/views/admin/SubscriptionsView.vue`: revoked filtering and
  restore action.

## Core flow

Revoke reads the live subscription, soft-deletes it, synchronously invalidates
the local Ristretto entry and Redis billing cache, then publishes the cache key
so other instances invalidate their L1 entry. Restore performs a fresh
include-deleted read, rejects non-revoked rows and any existing live row for the
same user/group, restores with an atomic `deleted_at IS NOT NULL` update, maps an
expired formerly-active row to `expired`, and runs the same cache invalidation.

Admin list/detail queries intentionally use include-deleted access and expose
`revoked_at`. User subscription APIs, billing eligibility, quota checks,
expiry workers, and active lookups retain Ent's default soft-delete scope.

## Important mechanisms

- `POST /api/v1/admin/subscriptions/:id/revoke` is the canonical revoke route;
  the historical `DELETE /:id` remains compatible.
- `POST /api/v1/admin/subscriptions/:id/restore` restores only revoked rows.
- The Redis Pub/Sub subscriber is owned by `SubscriptionService` and is stopped
  by `Stop()`; construction is not blocked on the Redis subscription handshake.
- Database uniqueness remains the final concurrency guard during restore.

## Known pitfalls

- Do not persist `status=revoked`; doing so breaks restore status preservation.
- Do not apply `SkipSoftDelete` to billing or user-facing repository methods.
- Cache invalidation after revoke/restore must stay synchronous locally and in
  Redis; background-only invalidation can authorize a revoked subscription.
- Ent schema generation does not replace migration `016`; no new schema or
  migration is required for revoke/restore.
