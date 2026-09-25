import type { LoadtestProfile, LoadtestStreamMode, LoadtestTier } from '@/api/admin/channelLoadtest'

/** Mirrors backend/internal/pkg/loadtest/presets.go + payload bucket tables. */
const streamBuckets = [
  { tokens: 4000, weight: 1010 },
  { tokens: 14000, weight: 1299 },
  { tokens: 35000, weight: 3024 },
  { tokens: 70000, weight: 3484 },
  { tokens: 140000, weight: 1905 },
  { tokens: 220000, weight: 282 }
]

const syncBuckets = [
  { tokens: 350, weight: 9587 },
  { tokens: 900, weight: 4516 },
  { tokens: 4000, weight: 158 },
  { tokens: 12000, weight: 239 }
]

const defaultSyncRatio = 0.57

export interface ProfilePreset {
  tiers: LoadtestTier[]
  streamMode?: LoadtestStreamMode
  tools?: string
  concurrency?: number
}

function normalizeBuckets(buckets: { tokens: number; weight: number }[], total: number): LoadtestTier[] {
  if (total <= 0 || buckets.length === 0) return []
  const weightSum = buckets.reduce((sum, b) => sum + (b.weight > 0 ? b.weight : 0), 0)
  if (weightSum <= 0) return []
  const parts = buckets
    .map((b, idx) => {
      const raw = (b.weight / weightSum) * total
      const count = Math.floor(raw)
      return { tokens: b.tokens, count, frac: raw - count, idx }
    })
    .filter((p) => p.tokens > 0)
  let assigned = parts.reduce((sum, p) => sum + p.count, 0)
  let rem = total - assigned
  const order = [...parts].sort((a, b) => (b.frac === a.frac ? a.idx - b.idx : b.frac - a.frac))
  for (let i = 0; i < rem && i < order.length; i++) {
    const target = parts.find((p) => p.idx === order[i].idx)
    if (target) target.count++
  }
  return parts.filter((p) => p.count > 0).map((p) => ({ input_tokens: p.tokens, count: p.count }))
}

function mergeTiers(...groups: LoadtestTier[][]): LoadtestTier[] {
  const byTok = new Map<number, { count: number; order: number }>()
  let order = 0
  for (const group of groups) {
    for (const tier of group) {
      if (tier.input_tokens <= 0 || tier.count <= 0) continue
      const cur = byTok.get(tier.input_tokens)
      if (cur) {
        cur.count += tier.count
      } else {
        byTok.set(tier.input_tokens, { count: tier.count, order: order++ })
      }
    }
  }
  return [...byTok.entries()]
    .sort((a, b) => a[1].order - b[1].order)
    .map(([input_tokens, v]) => ({ input_tokens, count: v.count }))
}

/** Absolute tier template for a named profile (Admin fills the table from this). */
export function presetForProfile(profile: LoadtestProfile): ProfilePreset {
  switch (profile) {
    case 'smoke':
      return { tiers: [{ input_tokens: 80, count: 40 }] }
    case 'general':
      return {
        tiers: [
          { input_tokens: 4000, count: 40 },
          { input_tokens: 16000, count: 30 },
          { input_tokens: 32000, count: 20 },
          { input_tokens: 64000, count: 10 }
        ],
        streamMode: 'stream'
      }
    case 'user363-sla':
      return {
        tiers: [
          { input_tokens: 50000, count: 50 },
          { input_tokens: 80000, count: 38 },
          { input_tokens: 160000, count: 10 },
          { input_tokens: 380000, count: 2 }
        ],
        streamMode: 'stream',
        tools: 'off'
      }
    case 'user363-stream':
      return { tiers: normalizeBuckets(streamBuckets, 100), streamMode: 'stream' }
    case 'user363-sync':
      return { tiers: normalizeBuckets(syncBuckets, 100), streamMode: 'sync' }
    case 'user363': {
      const syncN = Math.round(defaultSyncRatio * 100)
      const streamN = 100 - syncN
      return {
        tiers: mergeTiers(normalizeBuckets(syncBuckets, syncN), normalizeBuckets(streamBuckets, streamN)),
        streamMode: 'auto'
      }
    }
    default:
      return { tiers: [] }
  }
}
