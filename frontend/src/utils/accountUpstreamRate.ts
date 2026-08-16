export function defaultUpstreamRateMultiplier(type: string): number {
  return type === 'oauth' || type === 'apikey' ? 0.15 : 1
}
