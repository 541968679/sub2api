import type { SchedulePnlSummary, SchedulePnlWindow } from '@/api/admin/users'

export function formatSchedulePnlUsd(value: number | null | undefined): string {
  if (value == null || Number.isNaN(value)) return '—'
  const abs = `$${Math.abs(value).toFixed(2)}`
  if (value > 0) return `+${abs}`
  if (value < 0) return `-${abs}`
  return abs
}

export function formatSchedulePnlUsdPlain(value: number | null | undefined): string {
  if (value == null || Number.isNaN(value)) return '—'
  return `$${value.toFixed(2)}`
}

export function formatSchedulePnlMargin(margin: number | null | undefined): string {
  if (margin == null || Number.isNaN(margin)) return '—'
  return `${(margin * 100).toFixed(1)}%`
}

export function formatSchedulePnlWindow(window: SchedulePnlWindow | null | undefined) {
  return {
    revenue: formatSchedulePnlUsdPlain(window?.revenue),
    cost: formatSchedulePnlUsdPlain(window?.cost),
    profit: formatSchedulePnlUsd(window?.profit),
    margin: formatSchedulePnlMargin(window?.margin ?? null)
  }
}

export function hasSchedulePnlWindow(window: SchedulePnlWindow | null | undefined): boolean {
  return window != null
}

export function hasSchedulePnlSummary(summary: SchedulePnlSummary | null | undefined): boolean {
  return hasSchedulePnlWindow(summary?.today) || hasSchedulePnlWindow(summary?.seven_day)
}

function readFiniteNumber(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return null
}

function accountExtra(account: unknown): Record<string, unknown> | null {
  if (!account || typeof account !== 'object') return null
  const extra = (account as { extra?: Record<string, unknown> | null }).extra
  return extra && typeof extra === 'object' ? extra : null
}

function parseIsoTime(value: unknown): Date | null {
  if (typeof value !== 'string' || value.trim() === '') return null
  const parsed = new Date(value)
  return Number.isFinite(parsed.getTime()) ? parsed : null
}

export function isOauthAccountType(account: unknown): boolean {
  return !!account && typeof account === 'object' && (account as { type?: unknown }).type === 'oauth'
}

export type OauthSevenDayQuota = {
  utilization: number
  resetsAt: string | null
}

function openaiSevenDayQuota(extra: Record<string, unknown>, now: Date): OauthSevenDayQuota | null {
  const used = readFiniteNumber(extra.codex_7d_used_percent)
  if (used == null) return null
  const resetAt = parseIsoTime(extra.codex_7d_reset_at)
  let resetsAt = resetAt
  if (!resetsAt) {
    const resetAfter = readFiniteNumber(extra.codex_7d_reset_after_seconds)
    if (resetAfter != null && resetAfter > 0) {
      const base = parseIsoTime(extra.codex_usage_updated_at) ?? now
      resetsAt = new Date(base.getTime() + resetAfter * 1000)
    }
  }
  let utilization = used
  if (resetsAt && resetsAt.getTime() <= now.getTime()) {
    utilization = 0
  }
  return { utilization, resetsAt: resetsAt ? resetsAt.toISOString() : null }
}

function passiveSevenDayQuota(extra: Record<string, unknown>): OauthSevenDayQuota | null {
  const utilFrac = readFiniteNumber(extra.passive_usage_7d_utilization)
  const resetUnix = readFiniteNumber(extra.passive_usage_7d_reset)
  if ((utilFrac == null || utilFrac <= 0) && (resetUnix == null || resetUnix <= 0)) {
    return null
  }
  return {
    utilization: (utilFrac ?? 0) * 100,
    resetsAt: resetUnix != null && resetUnix > 0 ? new Date(resetUnix * 1000).toISOString() : null
  }
}

/** Cached 7-day quota snapshot already on the account extra. No /usage fetch. */
export function oauthSevenDayQuota(account: unknown, now = new Date()): OauthSevenDayQuota | null {
  if (!isOauthAccountType(account)) return null
  const extra = accountExtra(account)
  if (!extra) return null
  const platform = (account as { platform?: unknown }).platform
  if (platform === 'openai') {
    return openaiSevenDayQuota(extra, now)
  }
  return passiveSevenDayQuota(extra)
}

export function pairAccountBalanceUsd(account: unknown): number | null {
  if (!account || typeof account !== 'object') return null
  const rec = account as {
    usage?: { balance_usd?: unknown } | null
    extra?: Record<string, unknown> | null
    balance_usd?: unknown
  }
  return (
    readFiniteNumber(rec.usage?.balance_usd) ??
    readFiniteNumber(rec.balance_usd) ??
    readFiniteNumber(rec.extra?.balance_usd) ??
    readFiniteNumber(rec.extra?.upstream_balance_usd)
  )
}

/** Same window as backend shouldRefreshUpstreamBalance. */
export const UPSTREAM_BALANCE_REFRESH_MS = 6 * 60 * 1000

export function supportsPairBalanceProbe(account: unknown): boolean {
  if (!account || typeof account !== 'object') return false
  const rec = account as { type?: unknown; platform?: unknown }
  if (rec.type !== 'apikey') return false
  return rec.platform === 'openai' || rec.platform === 'anthropic'
}

export function pairAccountBalanceUpdatedAt(account: unknown): Date | null {
  const extra = accountExtra(account)
  return extra ? parseIsoTime(extra.upstream_balance_at) : null
}

export function shouldRefreshPairBalance(account: unknown, now = new Date(), force = false): boolean {
  if (!supportsPairBalanceProbe(account)) return false
  if (force) return true
  const updatedAt = pairAccountBalanceUpdatedAt(account)
  if (!updatedAt) return true
  return now.getTime() - updatedAt.getTime() >= UPSTREAM_BALANCE_REFRESH_MS
}

export function applyUsageBalanceToAccountExtra(
  extra: Record<string, unknown> | null | undefined,
  usage: {
    balance_usd?: number | null
    balance_updated_at?: string | null
    balance_source?: string
    balance_error?: string
    balance_unlimited?: boolean
    balance_used_usd?: number | null
  }
): Record<string, unknown> {
  const next: Record<string, unknown> = { ...(extra ?? {}) }
  if (usage.balance_usd != null) next.upstream_balance_usd = usage.balance_usd
  if (usage.balance_updated_at) next.upstream_balance_at = usage.balance_updated_at
  if (usage.balance_source != null) next.upstream_balance_source = usage.balance_source
  if (usage.balance_error != null) next.upstream_balance_error = usage.balance_error
  if (usage.balance_unlimited != null) next.upstream_balance_unlimited = usage.balance_unlimited
  if (usage.balance_used_usd != null) {
    next.upstream_balance_used_usd = usage.balance_used_usd
    next.display_balance_used_usd = usage.balance_used_usd
  }
  return next
}

export type BalanceBurnSample = { t: Date; v: number; kind?: string }

export type BalanceCostBurnCompare = {
  status: 'match' | 'mismatch'
  balanceRate: number
  costRate: number
}

const BURN_KIND_BALANCE_USD = 'balance_usd'
const BURN_MIN_SPAN_MS = 10 * 60 * 1000
const BURN_INCREASE_EPS = 1e-6
const BURN_ZERO_EPS = 1e-9
const BURN_REL_TOL = 0.3
const BURN_ABS_TOL = 0.05
const COST_BURN_MIN_HOURS = 0.25

function parseSampleTime(value: unknown): Date | null {
  if (typeof value !== 'string' || value.trim() === '') return null
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? null : parsed
}

export function parseBalanceBurnSamples(extra: unknown): BalanceBurnSample[] {
  if (!extra || typeof extra !== 'object') return []
  const raw = (extra as Record<string, unknown>).burn_samples
  if (!Array.isArray(raw)) return []
  const out: BalanceBurnSample[] = []
  for (const item of raw) {
    if (!item || typeof item !== 'object') continue
    const rec = item as { t?: unknown; v?: unknown; kind?: unknown }
    if (typeof rec.kind === 'string' && rec.kind !== '' && rec.kind !== BURN_KIND_BALANCE_USD) {
      continue
    }
    const value = readFiniteNumber(rec.v)
    const at = parseSampleTime(rec.t)
    if (value == null || !at) continue
    out.push({
      t: at,
      v: value,
      kind: typeof rec.kind === 'string' ? rec.kind : BURN_KIND_BALANCE_USD
    })
  }
  return out
}

export function computeBalanceBurnPerHour(samples: BalanceBurnSample[]): number | null {
  if (samples.length < 2) return null
  const pts = [...samples].sort((a, b) => a.t.getTime() - b.t.getTime())
  let start = 0
  for (let i = 1; i < pts.length; i++) {
    if (pts[i].v > pts[i - 1].v + BURN_INCREASE_EPS) start = i
  }
  const epoch = pts.slice(start)
  if (epoch.length < 2) return null
  const spanMs = epoch[epoch.length - 1].t.getTime() - epoch[0].t.getTime()
  if (spanMs < BURN_MIN_SPAN_MS) return null

  const t0 = epoch[0].t.getTime()
  const n = epoch.length
  let sumX = 0
  let sumY = 0
  let sumXY = 0
  let sumXX = 0
  for (const point of epoch) {
    const x = (point.t.getTime() - t0) / 3_600_000
    sumX += x
    sumY += point.v
    sumXY += x * point.v
    sumXX += x * x
  }
  const denom = n * sumXX - sumX * sumX
  if (Math.abs(denom) < BURN_ZERO_EPS) return null
  const slope = (n * sumXY - sumX * sumY) / denom
  if (slope >= -BURN_ZERO_EPS) return 0
  return -slope
}

export function impliedCostBurnPerHour(todayCost: number | null | undefined, now = new Date()): number | null {
  if (todayCost == null || !Number.isFinite(todayCost) || todayCost < 0) return null
  const elapsedHours = Math.max(
    COST_BURN_MIN_HOURS,
    now.getHours() + now.getMinutes() / 60 + now.getSeconds() / 3600
  )
  return todayCost / elapsedHours
}

export function compareBalanceBurnToCost(
  account: unknown,
  todayCost: number | null | undefined,
  now = new Date()
): BalanceCostBurnCompare | null {
  if (pairAccountBalanceUsd(account) == null) return null
  const extra = account && typeof account === 'object' ? (account as { extra?: unknown }).extra : null
  const balanceRate = computeBalanceBurnPerHour(parseBalanceBurnSamples(extra))
  const costRate = impliedCostBurnPerHour(todayCost, now)
  if (balanceRate == null || costRate == null) return null
  const absDiff = Math.abs(balanceRate - costRate)
  const relDiff = absDiff / Math.max(balanceRate, costRate, BURN_ABS_TOL)
  return {
    status: absDiff <= BURN_ABS_TOL || relDiff <= BURN_REL_TOL ? 'match' : 'mismatch',
    balanceRate,
    costRate
  }
}
