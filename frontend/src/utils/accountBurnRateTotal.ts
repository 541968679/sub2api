import { computeBalanceBurnPerHour, type BalanceBurnSample } from '@/composables/schedulePnl'

/** Sum of the balance burn-rate estimates already shown on the admin account list. */
export type AccountBurnRateTotal = {
  perHour: number
  accountCount: number
  visible: boolean
}

const KIND_USD = 'balance_usd'
const KIND_CNY = 'balance_cny'

const emptyTotal = (): AccountBurnRateTotal => ({
  perHour: 0,
  accountCount: 0,
  visible: false
})

function readExtra(account: unknown): Record<string, unknown> | null {
  if (!account || typeof account !== 'object') return null
  const extra = (account as { extra?: unknown }).extra
  if (!extra || typeof extra !== 'object') return null
  return extra as Record<string, unknown>
}

function isUnlimitedBalance(extra: Record<string, unknown>): boolean {
  const value = extra.upstream_balance_unlimited
  if (value === true || value === 1) return true
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    return normalized === 'true' || normalized === '1'
  }
  return false
}

function balanceCurrency(extra: Record<string, unknown>): 'CNY' | 'USD' {
  const raw = extra.upstream_balance_currency
  if (typeof raw === 'string' && raw.trim().toUpperCase() === 'CNY') return 'CNY'
  return 'USD'
}

function readSampleValue(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return null
}

function parseBurnSamplesOfKind(extra: Record<string, unknown>, kind: string): BalanceBurnSample[] {
  const raw = extra.burn_samples
  if (!Array.isArray(raw)) return []
  const out: BalanceBurnSample[] = []
  for (const item of raw) {
    if (!item || typeof item !== 'object') continue
    const rec = item as { t?: unknown; v?: unknown; kind?: unknown }
    const sampleKind = typeof rec.kind === 'string' ? rec.kind : ''
    if (kind === KIND_USD) {
      if (sampleKind !== '' && sampleKind !== KIND_USD) continue
    } else if (sampleKind !== kind) {
      continue
    }
    const value = readSampleValue(rec.v)
    if (value == null || typeof rec.t !== 'string' || rec.t.trim() === '') continue
    const at = new Date(rec.t)
    if (Number.isNaN(at.getTime())) continue
    out.push({ t: at, v: value, kind: sampleKind || kind })
  }
  return out
}

/** Same digit rule as the per-account burn line. */
export function formatAccountBurnRateAmount(value: number): string {
  if (!Number.isFinite(value)) return '0.00'
  const abs = Math.abs(value)
  if (abs >= 100) return value.toFixed(0)
  if (abs >= 10) return value.toFixed(1)
  return value.toFixed(2)
}

/**
 * Add the balance burn rates already estimated for the loaded account rows.
 * Percent window rates are a different unit and stay out of the total.
 * Accounts with too few samples, or an unlimited balance, are skipped.
 */
export function sumAccountBalanceBurnRates(
  accounts: readonly unknown[],
  now = new Date()
): AccountBurnRateTotal {
  if (!accounts.length) return emptyTotal()
  let perHour = 0
  let accountCount = 0
  for (const account of accounts) {
    const extra = readExtra(account)
    if (!extra || isUnlimitedBalance(extra)) continue
    const kind = balanceCurrency(extra) === 'CNY' ? KIND_CNY : KIND_USD
    const rate = computeBalanceBurnPerHour(parseBurnSamplesOfKind(extra, kind), now)
    if (rate == null) continue
    perHour += rate
    accountCount += 1
  }
  return {
    perHour,
    accountCount,
    visible: accountCount > 0
  }
}
