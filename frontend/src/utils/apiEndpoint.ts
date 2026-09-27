// The address users copy into clients. An admin override wins; otherwise the
// page origin is the relay's unified endpoint.
export function resolveUnifiedApiEndpoint(
  configured: string | null | undefined,
  origin: string
): string {
  const value = (configured ?? '').trim()
  const chosen = value || origin.trim()
  return chosen.replace(/\/+$/, '')
}
