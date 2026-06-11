# Auth

## Data Model

- User authentication state is persisted client-side in `auth_token`, `refresh_token`, `auth_user`, and `token_expires_at`.
- Legal consent acceptance is stored client-side per user under `legal_consent:user:{id}` with the accepted `version`, typed confirmation payload, and timestamp.
- Registration consent before account creation is stored in session storage under `register_legal_consent`; it includes the configured legal consent `version` so pending registration consent is invalidated when the version changes.
- Legal consent settings are runtime Settings KV entries, not database columns:
  - `legal_consent.enabled`
  - `legal_consent.version`
  - `legal_consent.content`
  - `legal_consent.confirmation_phrase`
  - `legal_consent.min_read_seconds`

## Key Files

- Backend settings:
  - `backend/internal/service/domain_constants.go`
  - `backend/internal/service/settings_view.go`
  - `backend/internal/service/setting_service.go`
  - `backend/internal/handler/dto/settings.go`
  - `backend/internal/handler/setting_handler.go`
  - `backend/internal/handler/admin/setting_handler.go`
- Frontend auth and consent:
  - `frontend/src/utils/legalConsent.ts`
  - `frontend/src/components/auth/LegalConsentDialog.vue`
  - `frontend/src/views/auth/LoginView.vue`
  - `frontend/src/views/auth/RegisterView.vue`
  - `frontend/src/views/auth/EmailVerifyView.vue`
  - `frontend/src/stores/auth.ts`
  - `frontend/src/stores/app.ts`
  - `frontend/src/views/admin/SettingsView.vue`

## Core Flow

1. Backend `SettingService.InitializeDefaultSettings` seeds the internal-research legal consent defaults.
2. Admin settings expose and update `legal_consent` through the normal settings API.
3. Public settings and SSR `window.__APP_CONFIG__` include `legal_consent`, letting auth pages read the configured terms before the first async settings refresh completes.
4. Login checks whether the authenticated user has accepted the configured version. If not, it opens `LegalConsentDialog`; accepting stores the configured version and then completes redirect.
5. Registration validates form prerequisites, shows `LegalConsentDialog`, stores a pending versioned consent payload, then completes direct registration or email verification.
6. Email verification rechecks the pending consent against the current configured version before marking the new user as accepted.
7. After public settings load, `appStore` asks `authStore` to enforce the current legal consent settings. If the active user has not accepted the configured version, the auth store clears the login state and preserves pending third-party auth session state.

## Important Mechanisms

- Changing only `legal_consent.version` is enough to invalidate previous local acceptances and force all users through the consent dialog again.
- `legal_consent.enabled=false` bypasses all legal consent checks and does not clear auth state.
- Empty admin-provided content, confirmation phrase, or version is normalized to the built-in internal-research defaults.
- `min_read_seconds` is clamped to `0..300` on both backend and frontend.
- Startup auth restoration only performs immediate legal consent validation when injected public settings are present. Without injected settings, the app restores auth first and then enforces the backend version after public settings load, avoiding false logouts caused by frontend fallback defaults.

## Known Pitfalls

- Do not treat the frontend default legal text as the source of truth. It is only a fallback when backend public settings are unavailable or incomplete.
- Do not reuse pending registration consent without checking its stored version against current settings.
- Any new public settings field that auth pages need before first render must be added to both `PublicSettings` and `PublicSettingsInjectionPayload`; the DTO drift test covers this contract.
