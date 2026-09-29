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

export function isHongKongEndpointName(name: string | null | undefined): boolean {
  const n = (name ?? '').trim().toLowerCase()
  return n === '香港' || n === 'hong kong' || n === 'hongkong'
}

export function hasHongKongCustomEndpoint(
  custom: Array<{ name?: string }> | null | undefined
): boolean {
  return (custom ?? []).some((ep) => isHongKongEndpointName(ep.name))
}

export function endpointChipName(opts: {
  isDefault: boolean
  name: string
  hasHongKong: boolean
  t: (key: string) => string
}): string {
  if (opts.isDefault) {
    return opts.hasHongKong ? opts.t('keys.endpoints.overseas') : opts.t('keys.endpoints.title')
  }
  if (isHongKongEndpointName(opts.name)) {
    return opts.t('keys.endpoints.hongKong')
  }
  return opts.name
}
