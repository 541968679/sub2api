# Public Settings Output Safety

## Data model

Public branding values remain ordinary Settings KV entries. `site_name`,
`site_logo`, and `doc_url` are administrator-controlled strings; persistence
does not imply that they are safe HTML or URL output.

## Key files

- Backend HTML boundaries: `internal/service/email_service.go`,
  `internal/handler/admin/setting_handler.go`, `internal/web/embed_on.go`.
- Frontend URL policy: `frontend/src/utils/url.ts`.
- Current consumers: `AppHeader.vue`, `AppSidebar.vue`, `HomeView.vue`, and
  `views/auth/LoginView.vue`. AuthLayout already uses the same logo policy.

## Core flow

Settings are stored and returned unchanged. At each output boundary, backend
HTML uses Go's `html.EscapeString`; frontend links and image sources use
`sanitizeUrl`. Documentation links allow only absolute HTTP(S). Site logos also
allow root-relative paths and `data:image/` URLs to preserve uploaded branding.

## Important mechanisms

- Escaping happens when producing HTML, not when saving settings, so plain-text
  consumers and administrator edits retain the configured value.
- Password-reset URLs are escaped both in `href` attributes and fallback text.
- Static contract tests enumerate the current frontend consumers so a new raw
  binding fails review unless it adopts the same policy.

## Known pitfalls

- Never interpolate `site_name` directly into HTML or `<title>` content.
- Never bind `doc_url` or `site_logo` directly to `href`/`src` merely because
  the value came from the public settings endpoint.
- Do not replace the logo policy with HTTP(S)-only validation: relative and
  uploaded data-image logos are supported product behavior.
