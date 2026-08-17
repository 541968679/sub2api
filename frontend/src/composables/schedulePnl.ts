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

export function pairAccountBalanceUsd(account: unknown): number | null {
  if (!account || typeof account !== 'object') return null
  const rec = account as {
    usage?: { balance_usd?: number | null } | null
    extra?: { balance_usd?: number | null } | null
    balance_usd?: number | null
  }
  if (rec.usage?.balance_usd != null) return rec.usage.balance_usd
  if (rec.balance_usd != null) return rec.balance_usd
  if (rec.extra?.balance_usd != null) return rec.extra.balance_usd
  return null
}
