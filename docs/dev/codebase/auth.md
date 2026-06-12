# Auth

## Data Model

- User authentication state is persisted client-side in `auth_token`, `refresh_token`, `auth_user`, and `token_expires_at`.
- Self-service email and first-time OAuth registrations create users with
  `status=pending_approval`. These users are stored in the normal `users`
  table but do not receive access or refresh tokens until an administrator
  changes the status to `active`.
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
  - `frontend/src/api/auth.ts`
  - `frontend/src/utils/legalConsent.ts`
  - `frontend/src/utils/authError.ts`
  - `frontend/src/components/auth/LegalConsentDialog.vue`
  - `frontend/src/views/auth/LoginView.vue`
  - `frontend/src/views/auth/RegisterView.vue`
  - `frontend/src/views/auth/EmailVerifyView.vue`
  - `frontend/src/stores/auth.ts`
  - `frontend/src/stores/app.ts`
  - `frontend/src/views/admin/SettingsView.vue`
- Admin approval:
  - `backend/internal/service/admin_service.go`
  - `backend/internal/handler/admin/user_handler.go`
  - `frontend/src/views/admin/UsersView.vue`
  - `frontend/src/components/admin/user/UserEditModal.vue`

## Core Flow

1. Backend `SettingService.InitializeDefaultSettings` seeds the internal-research legal consent defaults.
2. Admin settings expose and update `legal_consent` through the normal settings API.
3. Public settings and SSR `window.__APP_CONFIG__` include `legal_consent`, letting auth pages read the configured terms before the first async settings refresh completes.
4. Login checks whether the authenticated user has accepted the configured version. If not, it opens `LegalConsentDialog`; accepting stores the configured version and then completes redirect.
5. Registration validates form prerequisites, shows `LegalConsentDialog`, stores a pending versioned consent payload, then submits an internal test authorization application through direct registration or email verification.
6. Email verification rechecks the pending consent against the current configured version before marking the new user as accepted.
7. Self-service email registration and first-time LinuxDo/OIDC/WeChat account creation return a pending application response with `status=pending_approval`, `message`, and `user`; no token is returned or stored.
8. Pending users cannot log in. The backend returns `USER_PENDING_APPROVAL`, and the login page shows a pending-approval message instead of restoring auth state.
9. Administrators approve applications from the user list or edit dialog by changing the user status to `active`. Only active users can receive auth tokens and access normal authenticated pages.
10. After public settings load, `appStore` asks `authStore` to enforce the current legal consent settings. If the active user has not accepted the configured version, the auth store clears the login state and preserves pending third-party auth session state.

## Important Mechanisms

- `pending_approval` is a first-class user status accepted by admin update APIs and user filters, but `User.IsActive()` remains true only for `active`.
- `AuthService.validateUserCanAuthenticate` maps pending users to `ErrUserPendingApproval`; other non-active statuses still use `ErrUserNotActive`.
- Registration paths intentionally call post-registration bootstrap without touching login timestamps, because the account has not actually authenticated yet.
- `RegisterOAuthEmailAccount` and `LoginOrRegisterOAuthWithTokenPair` return a `nil` token pair for pending users. OAuth handlers must only record successful login when a token pair exists.
- Changing only `legal_consent.version` is enough to invalidate previous local acceptances and force all users through the consent dialog again.
- `legal_consent.enabled=false` bypasses all legal consent checks and does not clear auth state.
- Empty admin-provided content, confirmation phrase, or version is normalized to the built-in internal-research defaults.
- `min_read_seconds` is clamped to `0..300` on both backend and frontend.
- Startup auth restoration only performs immediate legal consent validation when injected public settings are present. Without injected settings, the app restores auth first and then enforces the backend version after public settings load, avoiding false logouts caused by frontend fallback defaults.

## Known Pitfalls

- Do not treat the frontend default legal text as the source of truth. It is only a fallback when backend public settings are unavailable or incomplete.
- Do not treat registration success as authentication success. A pending registration response has a `user` but no `access_token`; frontend stores must clear auth while preserving any pending OAuth browser session state.
- Do not call `RecordSuccessfulLogin` for pending application responses. It should only run after a real token pair is issued.
- Do not reuse pending registration consent without checking its stored version against current settings.
- Any new public settings field that auth pages need before first render must be added to both `PublicSettings` and `PublicSettingsInjectionPayload`; the DTO drift test covers this contract.
