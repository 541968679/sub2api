import type { Account } from '@/types'
import type { AccountQualityStats } from '@/api/admin/accounts'
import type {
  SmartSchedulePairQuality,
  SmartSchedulePlatform,
  SmartScheduleProbeConcurrencyMode,
  UserSmartScheduleView
} from '@/api/admin/users'
import { clampSmartScheduleWindowN } from '@/utils/smartScheduleWindowN'
import { percentToSuccessRate } from '@/utils/accountQualityHardClose'

const DEFAULT_PLATFORMS: SmartSchedulePlatform[] = [
  'anthropic',
  'openai',
  'gemini',
  'antigravity',
  'grok'
]

export type PoolAdmissionState =
  | 'stopped'
  | 'paused'
  | 'cooling'
  | 'probing'
  | 'pair_full'
  | 'will_cool'
  | 'unsaved_preview'
  | 'resumed'
  | 'pinned'
  | 'selectable'

export type PoolQualityHint = 'resumed' | 'will_cool' | 'unsaved_preview'

export type PairAdmissionLiveState =
  | 'paused'
  | 'cooling'
  | 'probing'
  | 'resumed'
  | 'selectable'
  | 'pinned'

/** Switcher order: 暂停 / 冷却 / 考察 / 调度 / 豁免期 / 长期豁免. No implicit unpause default. */
export const PAIR_ADMISSION_LIVE_STATES = [
  'paused',
  'cooling',
  'probing',
  'selectable',
  'resumed',
  'pinned'
] as const satisfies readonly PairAdmissionLiveState[]

export function pairAdmissionLiveState(
  admission: PoolAdmissionState,
  paused = false,
  pinned = false
): PairAdmissionLiveState {
  if (paused || admission === 'paused') return 'paused'
  if (pinned || admission === 'pinned') return 'pinned'
  if (admission === 'cooling') return 'cooling'
  if (admission === 'probing') return 'probing'
  if (admission === 'resumed') return 'resumed'
  return 'selectable'
}

export type PoolAdmission = {
  state: PoolAdmissionState
  cooldownUntil?: string
}

export type SmartSchedulePoolFilters = {
  search: string
  type: string
  schedulable: '' | 'on' | 'off'
  admission: '' | PoolAdmissionState
}

export const EMPTY_SMART_SCHEDULE_POOL_FILTERS: SmartSchedulePoolFilters = {
  search: '',
  type: '',
  schedulable: '',
  admission: ''
}

export const POOL_ADMISSION_FILTER_STATES = [
  'selectable',
  'probing',
  'resumed',
  'pinned',
  'will_cool',
  'cooling',
  'paused',
  'pair_full',
  'stopped',
  'unsaved_preview'
] as const satisfies readonly PoolAdmissionState[]

export type PoolQualityGateDraft = {
  enabled?: boolean
  maxP50: number | ''
  successPercent: number | ''
  windowN: number | ''
  condition: 'or' | 'and'
}

export function resolvePairCap(maxConcurrency: number | null | undefined): number | null {
  return maxConcurrency != null && maxConcurrency >= 1 ? maxConcurrency : null
}

/** Display-only pair-badge denominator when no extra cap is set. Not a real cap. */
export const UNCAPPED_PAIR_DISPLAY_MAX = 999

export function pairOccupancyDisplayMax(pairCap: number | null | undefined): number {
  return resolvePairCap(pairCap) ?? UNCAPPED_PAIR_DISPLAY_MAX
}

/** Backend field is `probe_cap`. Other keys are read aliases only. */
export type BackendProbeCapFields = {
  probe_cap?: number | null
  probing_cap?: number | null
  in_flight_cap?: number | null
  pair_probe_cap?: number | null
}

export function readBackendProbeCap(
  source: BackendProbeCapFields | null | undefined
): number | null {
  if (!source) return null
  const raw =
    source.probe_cap
    ?? source.probing_cap
    ?? source.in_flight_cap
    ?? source.pair_probe_cap
  return typeof raw === 'number' && Number.isFinite(raw) && raw >= 1 ? Math.round(raw) : null
}

export function memberProbingFromApi(member: {
  probing?: boolean
  admission?: string | null
  state?: string | null
} | null | undefined): boolean {
  if (!member) return false
  if (member.probing === true) return true
  return member.admission === 'probing' || member.state === 'probing'
}

/** True only from GET `pinned: true` or admission/state `pinned`. Never invented from expired 豁免期. */
export function memberPinnedFromApi(member: {
  pinned?: boolean
  admission?: string | null
  state?: string | null
} | null | undefined): boolean {
  if (!member) return false
  if (member.pinned === true) return true
  return member.admission === 'pinned' || member.state === 'pinned'
}

export function resolveProbeConcurrencyMode(
  value: string | null | undefined
): SmartScheduleProbeConcurrencyMode {
  return value === 'custom' ? 'custom' : 'follow_n'
}

/** PUT body value. follow_n → null. custom empty/NaN → null (caller must reject). Does not clamp. */
export function probeConcurrencyWriteValue(input: {
  mode?: string | null
  probeConcurrency?: number | '' | null
}): number | null {
  if (resolveProbeConcurrencyMode(input.mode) !== 'custom') {
    return null
  }
  if (input.probeConcurrency === '' || input.probeConcurrency == null) {
    return null
  }
  const n = Number(input.probeConcurrency)
  if (!Number.isFinite(n)) {
    return null
  }
  return Math.round(n)
}

/** custom requires 1–100. follow_n / omit is always valid. */
export function isValidProbeConcurrencyWrite(input: {
  mode?: string | null
  probeConcurrency?: number | '' | null
}): boolean {
  if (resolveProbeConcurrencyMode(input.mode) !== 'custom') {
    return true
  }
  const value = probeConcurrencyWriteValue(input)
  return value != null && value >= 1 && value <= 100
}

/** Selected probe in-flight number before member-cap clamp. Not account-quality N. */
export function resolveProbeConcurrencySelected(input: {
  mode?: string | null
  probeConcurrency?: number | '' | null
  windowN: number | '' | null | undefined
}): number {
  if (resolveProbeConcurrencyMode(input.mode) === 'custom') {
    return clampSmartScheduleWindowN(
      input.probeConcurrency === '' ? null : input.probeConcurrency
    )
  }
  return clampSmartScheduleWindowN(input.windowN === '' ? null : input.windowN)
}

/**
 * Probe in-flight cap: GET `probe_cap` if present, else min(selected, member cap)
 * or selected. Selected is window N when follow_n, or custom 1–100.
 * Never 999 — uncapped probe is still the selected value.
 */
export function resolveProbeConcurrency(input: {
  windowN: number | '' | null | undefined
  pairCap: number | null | undefined
  backendCap?: number | null
  mode?: string | null
  probeConcurrency?: number | '' | null
}): number {
  if (input.backendCap != null && Number.isFinite(input.backendCap) && input.backendCap >= 1) {
    return Math.round(input.backendCap)
  }
  const selected = resolveProbeConcurrencySelected(input)
  const cap = resolvePairCap(input.pairCap)
  return cap != null ? Math.min(selected, cap) : selected
}

export function pairOccupancyDisplayMaxForAdmission(input: {
  probing: boolean
  pinned?: boolean
  pairCap: number | null | undefined
  windowN: number | '' | null | undefined
  backendCap?: number | null
  mode?: string | null
  probeConcurrency?: number | '' | null
}): number {
  if (input.pinned) {
    return pairOccupancyDisplayMax(input.pairCap)
  }
  if (input.probing) {
    return resolveProbeConcurrency({
      windowN: input.windowN,
      pairCap: input.pairCap,
      backendCap: input.backendCap,
      mode: input.mode,
      probeConcurrency: input.probeConcurrency
    })
  }
  return pairOccupancyDisplayMax(input.pairCap)
}

export function isPairCooldownActive(cooldownUntil?: string | null, now = Date.now()): boolean {
  if (!cooldownUntil) return false
  const until = new Date(cooldownUntil).getTime()
  return Number.isFinite(until) && until > now
}

export function cooldownRemainingMinutes(cooldownUntil?: string | null, now = Date.now()): number {
  if (!cooldownUntil) return 0
  const until = new Date(cooldownUntil).getTime()
  if (!Number.isFinite(until) || until <= now) return 0
  return Math.max(1, Math.ceil((until - now) / 60_000))
}

export function hasQualityGateFromDraft(draft?: PoolQualityGateDraft | null): boolean {
  if (!draft) return false
  return (draft.maxP50 !== '' && draft.maxP50 != null) || (draft.successPercent !== '' && draft.successPercent != null)
}

export function isSavedQualityGateLive(saved?: PoolQualityGateDraft | null): boolean {
  return Boolean(saved?.enabled) && hasQualityGateFromDraft(saved)
}

export function pairQualityGateBreached(
  draft: PoolQualityGateDraft | undefined | null,
  pair: SmartSchedulePairQuality | null | undefined
): boolean {
  if (!draft || !pair || !hasQualityGateFromDraft(draft)) return false
  const n = clampSmartScheduleWindowN(draft.windowN === '' ? null : Number(draft.windowN))
  const judged: boolean[] = []
  if (draft.maxP50 !== '' && draft.maxP50 != null) {
    if (pair.ttft_samples >= n && pair.ttft_p50_ms != null) {
      judged.push(pair.ttft_p50_ms > Number(draft.maxP50))
    }
  }
  if (draft.successPercent !== '' && draft.successPercent != null) {
    const minRate = percentToSuccessRate(draft.successPercent)
    if (minRate != null && pair.ok_samples >= n && pair.success_rate != null) {
      judged.push(pair.success_rate < minRate)
    }
  }
  if (judged.length === 0) return false
  if (draft.condition === 'and') return judged.every(Boolean)
  return judged.some(Boolean)
}

/** @deprecated Use pairQualityGateBreached — will_cool reads pair windows, not account 15m cells. */
export function gateBreachedFromDraft(
  draft: PoolQualityGateDraft | undefined | null,
  pair: SmartSchedulePairQuality | null | undefined
): boolean {
  return pairQualityGateBreached(draft, pair)
}

function resumeUntilUnix(
  map: Record<string, number> | null | undefined,
  userId: number
): number | null {
  if (!map || userId <= 0) return null
  const raw = map[String(userId)]
  return typeof raw === 'number' && Number.isFinite(raw) && raw > 0 ? raw : null
}

/** True during 豁免期 (`resumed`): u: chip and/or remaining w: window. Selectable has no time grace. */
export function userQualityResumeActive(
  stats:
    | Pick<AccountQualityStats, 'resume_users' | 'resume_watching_users'>
    | null
    | undefined,
  userId: number,
  nowMs = Date.now()
): boolean {
  const nowSec = nowMs / 1000
  const watching = resumeUntilUnix(stats?.resume_watching_users, userId)
  if (watching != null && watching > nowSec) return true
  const chip = resumeUntilUnix(stats?.resume_users, userId)
  return chip != null && chip > nowSec
}

export function userQualityResumeChipActive(
  stats: Pick<AccountQualityStats, 'resume_users'> | null | undefined,
  userId: number,
  nowMs = Date.now()
): boolean {
  const chip = resumeUntilUnix(stats?.resume_users, userId)
  return chip != null && chip > nowMs / 1000
}

/**
 * Quality column hint. Saved+enabled pair-window miss without cooldown is will-cool (not a lock).
 * Draft-only miss (tighter unsaved form, or platform not enabled) is preview.
 * Chip or watching grace is 豁免期 (`resumed`). Selectable has no 15m w: fail-open.
 */
export function resolveQualityAdmissionHint(input: {
  draft?: PoolQualityGateDraft | null
  saved?: PoolQualityGateDraft | null
  pairQuality?: SmartSchedulePairQuality | null
  /** @deprecated Ignored. will_cool uses pairQuality, not account 15m cells. */
  stats?: unknown
  resumeActive?: boolean
  resumeChipActive?: boolean
}): PoolQualityHint | null {
  if (input.resumeChipActive || input.resumeActive) return 'resumed'
  if (isSavedQualityGateLive(input.saved) && pairQualityGateBreached(input.saved, input.pairQuality)) {
    return 'will_cool'
  }
  if (hasQualityGateFromDraft(input.draft) && pairQualityGateBreached(input.draft, input.pairQuality)) {
    return 'unsaved_preview'
  }
  return null
}

export function resolvePoolAdmission(input: {
  account: Pick<Account, 'status' | 'schedulable' | 'temp_unschedulable_until' | 'rate_limit_reset_at'>
  pairCap: number | null
  pairCurrent: number
  cooldownUntil?: string | null
  paused?: boolean
  /** Live probe mark. Missing / false is not probing (no backfill). */
  probing?: boolean
  /** Live long-term exemption. Missing / false is not pinned (never invented from expired 豁免期). */
  pinned?: boolean
  qualityHint?: PoolQualityHint | null
  now?: number
}): PoolAdmission {
  const now = input.now ?? Date.now()
  if (!isCurrentlySchedulingAccount(input.account)) {
    return { state: 'stopped' }
  }
  if (input.paused) {
    return { state: 'paused' }
  }
  if (input.pinned) {
    return { state: 'pinned' }
  }
  if (isPairCooldownActive(input.cooldownUntil, now)) {
    return { state: 'cooling', cooldownUntil: input.cooldownUntil ?? undefined }
  }
  if (input.probing) {
    return { state: 'probing' }
  }
  if (input.pairCap != null && input.pairCap >= 1 && input.pairCurrent >= input.pairCap) {
    return { state: 'pair_full' }
  }
  if (input.qualityHint === 'resumed') {
    return { state: 'resumed' }
  }
  if (input.qualityHint === 'will_cool') {
    return { state: 'will_cool' }
  }
  if (input.qualityHint === 'unsaved_preview') {
    return { state: 'unsaved_preview' }
  }
  return { state: 'selectable' }
}

export function matchesPoolFilters(
  account: Account,
  admission: PoolAdmissionState,
  filters: SmartSchedulePoolFilters
): boolean {
  const query = filters.search.trim().toLowerCase()
  if (query) {
    const email = String(account.extra?.email_address || account.credentials?.email || account.parent_email || '')
    const haystack = `${account.name} ${account.id} ${email}`.toLowerCase()
    if (!haystack.includes(query)) return false
  }
  if (filters.type && account.type !== filters.type) return false
  if (filters.schedulable === 'on' && !account.schedulable) return false
  if (filters.schedulable === 'off' && account.schedulable) return false
  if (filters.admission && admission !== filters.admission) return false
  return true
}

export function isCurrentlySchedulingAccount(account: {
  status?: string
  schedulable?: boolean
  temp_unschedulable_until?: string | null
  rate_limit_reset_at?: string | null
}): boolean {
  if (account.status !== 'active' || !account.schedulable) return false
  const now = Date.now()
  if (account.temp_unschedulable_until) {
    const until = new Date(account.temp_unschedulable_until).getTime()
    if (Number.isFinite(until) && until > now) return false
  }
  if (account.rate_limit_reset_at) {
    const until = new Date(account.rate_limit_reset_at).getTime()
    if (Number.isFinite(until) && until > now) return false
  }
  return true
}

export function pickDefaultSmartSchedulePlatform(
  view: UserSmartScheduleView | null | undefined,
  platforms: readonly SmartSchedulePlatform[] = DEFAULT_PLATFORMS
): SmartSchedulePlatform {
  const hinted = view?.default_platform
  if (hinted && platforms.includes(hinted as SmartSchedulePlatform)) {
    return hinted as SmartSchedulePlatform
  }
  const rows = platforms.map((platform) => {
    const row = view?.platforms?.[platform]
    return {
      platform,
      enabled: Boolean(row?.enabled && (row.accounts?.length ?? 0) > 0),
      members: row?.accounts?.length ?? 0,
      updatedAt: row?.updated_at ? new Date(row.updated_at).getTime() : 0
    }
  })
  const enabled = rows.filter((row) => row.enabled)
  const withPool = rows.filter((row) => row.members > 0)
  const pool = enabled.length > 0 ? enabled : withPool
  if (pool.length === 0) return platforms[0] ?? 'anthropic'
  return pool.reduce((best, row) => {
    if (row.updatedAt > best.updatedAt) return row
    if (row.updatedAt === best.updatedAt && row.members > best.members) return row
    return best
  }).platform
}
