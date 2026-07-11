# Batch Image

Batch Image is an asynchronous Gemini image-generation workflow exposed under
`/v1/images/batches`. It is deliberately separate from the fork-local OpenAI
Images gateway and ImageChannelMonitor pipelines.

## Data model

- `batch_image_jobs` owns the public batch id, API-key/user ownership, provider
  state, immutable pricing snapshot, hold/capture state, retention timestamps,
  and bounded settlement retry metadata.
- `batch_image_items` indexes provider output by caller-supplied `custom_id` and
  records success, failure, output location, MIME type, and billed amount.
- `batch_image_events` records idempotent lifecycle events.
- `users.frozen_balance` isolates reserved Batch Image funds from spendable
  balance. `groups` contains the Batch Image gate and discount/hold multipliers.
- Migration `184_batch_image_workflow.sql` is additive and idempotent. Historical
  migrations are not edited.

## Key files

- Schemas: `backend/ent/schema/batch_image_{job,item,event}.go`, `user.go`, and
  `group.go`.
- HTTP: `backend/internal/handler/batch_image_handler.go` and
  `backend/internal/server/routes/gateway.go`.
- Public orchestration: `backend/internal/service/batch_image_public.go`.
- Provider implementations: `batch_image_provider_gemini.go` and
  `batch_image_provider_vertex.go`.
- Queue and processing: `batch_image_queue.go`, `batch_image_worker*.go`, and
  `batch_image_processor.go`.
- Billing: `batch_image_billing_hold.go`, `batch_image_settlement.go`, and the
  reserve/capture/release methods in `usage_billing_repo.go`.
- Persistence: `backend/internal/repository/batch_image_repo.go` and
  `batch_image_queue.go`.
- User workbench: `frontend/src/views/user/BatchImageGuideView.vue`,
  `frontend/src/api/batchImage.ts`, and `useBatchImageAccess.ts`.
- Admin group controls: `frontend/src/views/admin/GroupsView.vue`.

## Core flow

1. API-key middleware authenticates the request and supplies user, key, and
   group ownership. Every read, download, cancellation, and delete operation is
   scoped to that owner.
2. The public service requires both global `batch_image.enabled` and a Gemini
   group with `allow_image_generation` plus `allow_batch_image_generation`.
3. Submission validates limits and provider/account compatibility, resolves the
   normal image unit price, freezes the configured hold, persists a pricing
   snapshot, and enqueues the batch idempotently.
4. The Redis worker claims the job, submits/polls Gemini API or Vertex Batch
   Prediction, indexes result objects, and enters settlement.
5. Settlement bills successful images only. Capture and release request ids are
   deterministic, retries are bounded, and stale pre-submission holds are
   recovered by the billing recovery service.
6. Download and cleanup services enforce owner scope, size/concurrency limits,
   safe provider paths, and input/output retention.

## Important mechanisms

- `Idempotency-Key`, manifest hashes, repository state transitions, queue
  deduplication, and billing request ids make submission and settlement safe to
  retry.
- The pricing snapshot stores base unit price, group/account rate, batch
  discount, hold multiplier, and final billable/hold unit prices. Later pricing
  changes cannot rewrite an in-flight batch.
- Group invariants are enforced on both admin create/update and frontend edit:
  Batch Image is Gemini-only, requires normal image generation, multipliers are
  non-negative, and hold multiplier is at least the discount multiplier.
- `batch_image.enabled` and `batch_image.queue_enabled` both default to false;
  rollout therefore requires explicit configuration and eligible groups.

## Fork-local preservation boundary

- Do not route Batch Image through `OpenAIGatewayHandler.Images`, the image
  response spool, OpenAI Images feature gates/failover, or ImageChannelMonitor.
- Do not reuse display-token transformations for Batch Image. Stored ordinary
  billing, `actual_cost`, cache-read tokens/cost, and display pricing remain
  unchanged. Batch Image uses a separate frozen-balance ledger and image-count
  pricing snapshot.
- Do not change Claude-GPT bridge selection, curated model discovery/default
  mappings, Grok routing, account scheduler/failover semantics, ops logging, or
  user platform quota attribution while maintaining this module.
- New schema changes require a new migration after `184`, Ent regeneration, and
  owner-scope plus reserve/capture/release regression coverage.

## Known pitfalls

- The current repository Wire graph has pre-existing duplicate/missing binding
  failures. Keep provider sets and `wire_gen.go` aligned manually when
  `go generate ./cmd/server` cannot complete, then compile `cmd/server`.
- Provider output is untrusted. Never expose provider object paths, account ids,
  credentials, or raw internal errors in public JSON or ZIP manifests.
- A failed reserve must not enqueue work. A failed queue handoff must leave a
  recoverable persisted job/hold. Failed settlement must retry within the
  configured bound rather than silently completing.
