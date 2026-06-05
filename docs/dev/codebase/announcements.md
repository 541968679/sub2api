# Announcements

## Data Model

- `announcements`: admin-authored announcement content, status, notify mode, targeting rules, schedule, display `surface`, and `popup_frequency`.
- `announcement_reads`: per-user state for `read_at`, `last_popup_dismissed_at`, and `banner_dismissed_at`.
- `surface` values:
  - `general`: regular announcement list and global popup queue.
  - `dashboard_banner`: user dashboard top banner.
  - `api_key_rules`: API key usage rules shown from the keys getting-started guide.
- `popup_frequency` values:
  - `once`: popup once per announcement/user after dismissal.
  - `daily`: popup at most once per backend-server date after dismissal.

## Key Files

- Backend domain/schema:
  - `backend/internal/domain/announcement.go`
  - `backend/ent/schema/announcement.go`
  - `backend/ent/schema/announcement_read.go`
  - `backend/migrations/148_extend_announcements_surfaces.sql`
- Backend service/repository/handlers:
  - `backend/internal/service/announcement.go`
  - `backend/internal/service/announcement_service.go`
  - `backend/internal/repository/announcement_repo.go`
  - `backend/internal/repository/announcement_read_repo.go`
  - `backend/internal/handler/announcement_handler.go`
  - `backend/internal/handler/admin/announcement_handler.go`
  - `backend/internal/handler/dto/announcement.go`
- Frontend:
  - `frontend/src/api/announcements.ts`
  - `frontend/src/api/admin/announcements.ts`
  - `frontend/src/stores/announcements.ts`
  - `frontend/src/views/admin/AnnouncementsView.vue`
  - `frontend/src/components/user/dashboard/DashboardAnnouncementBanner.vue`
  - `frontend/src/components/keys/GettingStartedGuide.vue`

## Core Flow

1. Admin creates or updates an announcement through `/api/v1/admin/announcements`, selecting status, notify mode, display surface, popup frequency, schedule, and targeting.
2. User clients call `GET /api/v1/announcements?surface=...`.
3. `AnnouncementService.ListForUser` applies active status, schedule, user balance, active subscription targeting, and optional surface filtering.
4. The backend computes `should_popup` for `surface=general` popup announcements.
5. Frontend global popup queue uses `should_popup` and records popup dismissal through `POST /api/v1/announcements/:id/popup-dismiss`.
6. Dashboard banner uses `surface=dashboard_banner` and records banner dismissal through `POST /api/v1/announcements/:id/banner-dismiss`.
7. API key rules use the latest visible `surface=api_key_rules` announcement and display it as a read-only modal.

## Important Mechanisms

- Popup dismissal is separate from announcement-center read state, but `popup-dismiss` also sets `read_at` when it was empty to preserve the old "close popup means read" experience.
- `banner-dismiss` does not set `read_at`; dashboard hiding is independent from read statistics.
- Admin read statistics still count only non-null `read_at`.
- Daily popup checks compare `last_popup_dismissed_at` to `time.Now()` in the backend server location.
- "Show again" for dashboard banners requires creating a new announcement ID. Editing a dismissed banner does not reset user dismissal state.

## Known Pitfalls

- `announcement_reads.read_at` is nullable. Repository write paths use raw SQL upserts so banner-only dismissal can create a row without marking the announcement read.
- Generated Ent files must be regenerated after schema changes with `go generate ./ent`; do not hand-edit generated Ent code.
- Frontend announcement list defaults to `surface=general`; surface-specific components should explicitly request their surface to avoid mixing popup/list/banner/rules content.
