import type { TrendDataPoint } from '@/types'

export interface StabilityCacheRateSeries {
  input: number[]
  output: number[]
  cacheCreation: number[]
  requestRate: Array<number | null>
  tokenRatio: Array<number | null>
}

function finiteNumber(value: number | null | undefined): number | null {
  if (value == null || !Number.isFinite(value)) return null
  return value
}

/** 缓存读取率：有缓存读取的请求 / 全部请求。有缓存指 cache_read_tokens > 0。分母为 0 或分子未知时返回 null。 */
export function cacheRequestHitPercent(
  cacheHitRequests: number | null | undefined,
  totalRequests: number | null | undefined
): number | null {
  const hits = finiteNumber(cacheHitRequests)
  const requests = finiteNumber(totalRequests)
  if (hits == null || requests == null || requests <= 0 || hits < 0) return null
  return (hits / requests) * 100
}

/** 缓存读取比例：cache_read_tokens / total_tokens。总 token = 输入 + 输出 + 缓存创建 + 缓存读取。 */
export function cacheReadTokenPercent(
  cacheReadTokens: number | null | undefined,
  totalTokens: number | null | undefined
): number | null {
  const read = finiteNumber(cacheReadTokens)
  const total = finiteNumber(totalTokens)
  if (read == null || total == null || total <= 0 || read < 0) return null
  return (read / total) * 100
}

export function formatCachePercent(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '—'
  const abs = Math.abs(value)
  if (abs === 0) return '0%'
  if (abs >= 10) return `${value.toFixed(1)}%`
  if (abs >= 1) return `${value.toFixed(2)}%`
  return `${value.toFixed(3)}%`
}

export function formatRateCount(value: number): string {
  return Math.round(value).toLocaleString('en-US')
}

/** 分母为 0 或分子未知时返回 null，避免把缺口画成 0/0。 */
export function formatRateFraction(
  numerator: number | null | undefined,
  denominator: number | null | undefined
): string | null {
  const top = finiteNumber(numerator)
  const bottom = finiteNumber(denominator)
  if (top == null || bottom == null || bottom <= 0 || top < 0) return null
  return `${formatRateCount(top)} / ${formatRateCount(bottom)}`
}

function volume(value: number | null | undefined): number {
  const numeric = finiteNumber(value)
  if (numeric == null || numeric < 0) return 0
  return numeric
}

export function buildStabilityCacheRateSeries(points: TrendDataPoint[]): StabilityCacheRateSeries {
  return {
    input: points.map((point) => volume(point.input_tokens)),
    output: points.map((point) => volume(point.output_tokens)),
    cacheCreation: points.map((point) => volume(point.cache_creation_tokens)),
    requestRate: points.map((point) => cacheRequestHitPercent(point.cache_hit_requests, point.requests)),
    tokenRatio: points.map((point) => cacheReadTokenPercent(point.cache_read_tokens, point.total_tokens))
  }
}
