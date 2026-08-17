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

function readFiniteUsd(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return null
}

export function pairAccountBalanceUsd(account: unknown): number | null {
  if (!account || typeof account !== 'object') return null
  const rec = account as {
    usage?: { balance_usd?: unknown } | null
    extra?: Record<string, unknown> | null
    balance_usd?: unknown
  }
  return (
    readFiniteUsd(rec.usage?.balance_usd) ??
    readFiniteUsd(rec.balance_usd) ??
    readFiniteUsd(rec.extra?.balance_usd) ??
    readFiniteUsd(rec.extra?.upstream_balance_usd)
  )
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
    const value = readFiniteUsd(rec.v)
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
