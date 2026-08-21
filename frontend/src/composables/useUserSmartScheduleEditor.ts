import { computed, reactive, ref, watch, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Account, WindowStats } from '@/types'
import type { AccountQualityStats } from '@/api/admin/accounts'
import type {
  SchedulePnlSummary,
  SmartSchedulePairQuality,
  SmartSchedulePlatform,
  SmartSchedulePlatformView,
  UserSmartScheduleView
} from '@/api/admin/users'
import {
  SMART_SCHEDULE_WINDOW_N_DEFAULT,
  clampSmartScheduleWindowN,
  resolveSmartScheduleWindowN
} from '@/utils/smartScheduleWindowN'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  ACCOUNT_QUALITY_WINDOW_SECONDS,
  optionalNumber,
  percentToSuccessRate,
  successRateToPercent,
  type QualityGateFormFields
} from '@/utils/accountQualityHardClose'
import { useQualityThresholdTemplate } from '@/composables/useQualityThresholdTemplate'
import {
  isCurrentlySchedulingAccount,
  memberProbingFromApi,
  pickDefaultSmartSchedulePlatform,
  readBackendProbeCap,
  resolvePairCap,
  userQualityResumeActive,
  userQualityResumeChipActive,
  type PairAdmissionLiveState
} from '@/composables/smartSchedulePoolAdmission'

export { isCurrentlySchedulingAccount }

export const SMART_SCHEDULE_PLATFORMS: SmartSchedulePlatform[] = [
  'anthropic',
  'openai',
  'gemini',
  'antigravity',
  'grok'
]

export type SmartSchedulePoolMemberDraft = {
  account_id: number
  max_concurrency: number | null
  sort_order?: number | null
  current_concurrency?: number
  cooldown_until?: string | null
  paused?: boolean
  probing?: boolean
  probe_cap?: number | null
}

export type SmartSchedulePlatformDraft = {
  enabled: boolean
  maxP50: number | ''
  successPercent: number | ''
  windowN: number | ''
  condition: 'or' | 'and'
  cooldownMinutes: number
  accounts: SmartSchedulePoolMemberDraft[]
}

export type SmartScheduleAddScope = 'apikey' | 'oauth' | 'all'

const CANDIDATE_PAGE_SIZE = 1000

function snapshotDraft(draft: SmartSchedulePlatformDraft | undefined): string {
  const row = draft ?? emptySmartScheduleDraft()
  return JSON.stringify({
    enabled: row.enabled,
    maxP50: row.maxP50,
    successPercent: row.successPercent,
    windowN: row.windowN,
    condition: row.condition,
    cooldownMinutes: row.cooldownMinutes,
    accounts: row.accounts.map((item) => ({
      account_id: item.account_id,
      max_concurrency: item.max_concurrency
    }))
  })
}

export function emptySmartScheduleDraft(): SmartSchedulePlatformDraft {
  return {
    enabled: false,
    maxP50: '',
    successPercent: '',
    windowN: SMART_SCHEDULE_WINDOW_N_DEFAULT,
    condition: 'or',
    cooldownMinutes: 15,
    accounts: []
  }
}

export function draftFromSavedSnapshot(raw: string | undefined): SmartSchedulePlatformDraft {
  if (!raw) return emptySmartScheduleDraft()
  try {
    const parsed = JSON.parse(raw) as Partial<SmartSchedulePlatformDraft>
    return {
      enabled: Boolean(parsed.enabled),
      maxP50: parsed.maxP50 ?? '',
      successPercent: parsed.successPercent ?? '',
      windowN: resolveSmartScheduleWindowN({
        quality_window_n: typeof parsed.windowN === 'number' ? parsed.windowN : null,
        quality_min_success_samples:
          typeof (parsed as { minSuccessSamples?: number }).minSuccessSamples === 'number'
            ? (parsed as { minSuccessSamples: number }).minSuccessSamples
            : null,
        quality_min_ttft_samples:
          typeof (parsed as { minTtftSamples?: number }).minTtftSamples === 'number'
            ? (parsed as { minTtftSamples: number }).minTtftSamples
            : null
      }),
      condition: parsed.condition === 'and' ? 'and' : 'or',
      cooldownMinutes: parsed.cooldownMinutes || 15,
      accounts: Array.isArray(parsed.accounts) ? parsed.accounts : []
    }
  } catch {
    return emptySmartScheduleDraft()
  }
}

type LocalPairResumeGrace = {
  chipUntil: number
  watchUntil: number
}

export type SmartSchedulePoolFetchNeeds = {
  quality: boolean
  today: boolean
  pnl: boolean
}

export function useUserSmartScheduleEditor(
  userId: Ref<number | null>,
  options?: { poolFetchNeeds?: SmartSchedulePoolFetchNeeds }
) {
  const { t } = useI18n()
  const appStore = useAppStore()
  const { templateBusy: qualityTemplateBusy, applyQualityTemplate, saveQualityTemplate } =
    useQualityThresholdTemplate()

  const loading = ref(false)
  const submitting = ref(false)
  const copying = ref(false)
  const statsLoading = ref(false)
  const emptyPoolError = ref(false)
  const initialLoaded = ref(false)
  const skipNextPlatformWatch = ref(false)
  const candidatesLoaded = ref(false)
  const candidatesLoading = ref(false)
  const candidatesPlatform = ref<SmartSchedulePlatform | null>(null)
  const activePlatform = ref<SmartSchedulePlatform>('anthropic')
  const copyFromPlatform = ref<SmartSchedulePlatform | ''>('')
  const selectedAddAccountId = ref(0)
  const bulkCap = ref<number | null>(null)
  const drafts = reactive<Record<SmartSchedulePlatform, SmartSchedulePlatformDraft>>(
    {} as Record<SmartSchedulePlatform, SmartSchedulePlatformDraft>
  )
  const poolAccounts = ref<Account[]>([])
  const candidateAccounts = ref<Account[]>([])
  const qualityStatsById = ref<Record<string, AccountQualityStats>>({})
  const todayStatsById = ref<Record<string, WindowStats>>({})
  const pairPnlById = ref<Record<string, SchedulePnlSummary>>({})
  const pairQualityById = ref<Record<string, SmartSchedulePairQuality>>({})
  const savedSnapshots = reactive<Record<SmartSchedulePlatform, string>>(
    {} as Record<SmartSchedulePlatform, string>
  )
  const selectedAccountIds = ref<number[]>([])
  const refreshing = ref(false)
  const localResumeGraceByAccount = ref<Record<number, LocalPairResumeGrace>>({})
  const localPausedByAccount = ref<Record<number, boolean>>({})
  const localProbingByAccount = ref<Record<number, boolean>>({})

  const currentDraft = computed(() => drafts[activePlatform.value])
  const currentSavedDraft = computed(() => draftFromSavedSnapshot(savedSnapshots[activePlatform.value]))
  const otherPlatforms = computed(() =>
    SMART_SCHEDULE_PLATFORMS.filter((platform) => platform !== activePlatform.value)
  )
  const addableAccounts = computed(() => {
    const used = new Set((currentDraft.value?.accounts ?? []).map((item) => item.account_id))
    return candidateAccounts.value.filter((account) => !used.has(account.id))
  })
  const addableSchedulingApi = computed(() =>
    addableAccounts.value.filter((account) => account.type === 'apikey' && isCurrentlySchedulingAccount(account))
  )
  const addableSchedulingOauth = computed(() =>
    addableAccounts.value.filter((account) => account.type === 'oauth' && isCurrentlySchedulingAccount(account))
  )
  const addableSchedulingAll = computed(() =>
    addableAccounts.value.filter((account) => isCurrentlySchedulingAccount(account))
  )

  function viewToDraft(view: SmartSchedulePlatformView | undefined): SmartSchedulePlatformDraft {
    const draft = emptySmartScheduleDraft()
    if (!view) return draft
    draft.enabled = view.enabled
    draft.maxP50 = view.quality_max_p50_ttft_ms ?? ''
    draft.successPercent = successRateToPercent(view.quality_min_success_rate) ?? ''
    draft.windowN = resolveSmartScheduleWindowN(view)
    draft.condition = view.quality_condition === 'and' ? 'and' : 'or'
    draft.cooldownMinutes = view.cooldown_minutes || 15
    draft.accounts = (view.accounts ?? []).map((item) => ({
      account_id: item.account_id,
      max_concurrency: item.max_concurrency ?? null,
      sort_order: item.sort_order ?? null,
      current_concurrency: item.current_concurrency ?? 0,
      cooldown_until: item.cooldown_until ?? null,
      paused: Boolean(item.paused),
      probing: memberProbingFromApi(item),
      probe_cap: readBackendProbeCap(item)
    }))
    return draft
  }

  function captureSnapshot(platform: SmartSchedulePlatform) {
    savedSnapshots[platform] = snapshotDraft(drafts[platform])
  }

  function captureAllSnapshots() {
    for (const platform of SMART_SCHEDULE_PLATFORMS) {
      captureSnapshot(platform)
    }
  }

  function isPlatformDirty(platform: SmartSchedulePlatform): boolean {
    return snapshotDraft(drafts[platform]) !== (savedSnapshots[platform] ?? snapshotDraft(emptySmartScheduleDraft()))
  }

  function applyView(view: UserSmartScheduleView) {
    for (const platform of SMART_SCHEDULE_PLATFORMS) {
      drafts[platform] = viewToDraft(view.platforms?.[platform])
    }
    captureAllSnapshots()
  }

  function applyPlatformView(platform: SmartSchedulePlatform, view: UserSmartScheduleView) {
    drafts[platform] = viewToDraft(view.platforms?.[platform])
    captureSnapshot(platform)
  }

  function mergeRuntimeMembers(platform: SmartSchedulePlatform, view: SmartSchedulePlatformView | undefined) {
    const draft = drafts[platform]
    if (!draft || !view) return
    const byID = new Map((view.accounts ?? []).map((item) => [item.account_id, item]))
    for (const member of draft.accounts) {
      const live = byID.get(member.account_id)
      if (!live) continue
      member.current_concurrency = live.current_concurrency ?? 0
      member.cooldown_until = live.cooldown_until ?? null
      member.sort_order = live.sort_order ?? null
      member.paused = Boolean(live.paused)
      member.probing = memberProbingFromApi(live)
      member.probe_cap = readBackendProbeCap(live)
    }
  }

  function applyTemplateToDraft(fields: QualityGateFormFields) {
    if (!currentDraft.value) return
    currentDraft.value.maxP50 = fields.quality_max_p50_ttft_ms ?? ''
    currentDraft.value.successPercent = fields.quality_min_success_rate_percent ?? ''
    currentDraft.value.windowN = resolveSmartScheduleWindowN({
      quality_min_success_samples: fields.quality_min_success_samples,
      quality_min_ttft_samples: fields.quality_min_ttft_samples
    })
    currentDraft.value.condition = fields.quality_condition
  }

  function qualityGateFormFromDraft(draft: SmartSchedulePlatformDraft): QualityGateFormFields {
    const n = clampSmartScheduleWindowN(draft.windowN === '' ? null : Number(draft.windowN))
    return {
      quality_max_p50_ttft_ms: optionalNumber(draft.maxP50),
      quality_min_success_rate_percent: optionalNumber(draft.successPercent),
      quality_min_success_samples: n,
      quality_min_ttft_samples: n,
      quality_condition: draft.condition
    }
  }

  function memberCap(accountId: number): number {
    return currentDraft.value?.accounts.find((item) => item.account_id === accountId)?.max_concurrency ?? 0
  }

  function memberCapOrNull(accountId: number): number | null {
    return resolvePairCap(currentDraft.value?.accounts.find((item) => item.account_id === accountId)?.max_concurrency)
  }

  function memberCurrent(accountId: number): number {
    return currentDraft.value?.accounts.find((item) => item.account_id === accountId)?.current_concurrency ?? 0
  }

  function memberCooldownUntil(accountId: number): string | null {
    return currentDraft.value?.accounts.find((item) => item.account_id === accountId)?.cooldown_until ?? null
  }

  function memberPaused(accountId: number): boolean {
    if (Object.prototype.hasOwnProperty.call(localPausedByAccount.value, accountId)) {
      return localPausedByAccount.value[accountId]
    }
    return Boolean(currentDraft.value?.accounts.find((item) => item.account_id === accountId)?.paused)
  }

  function memberProbing(accountId: number): boolean {
    if (Object.prototype.hasOwnProperty.call(localProbingByAccount.value, accountId)) {
      return localProbingByAccount.value[accountId]
    }
    return Boolean(currentDraft.value?.accounts.find((item) => item.account_id === accountId)?.probing)
  }

  function memberProbeCap(accountId: number): number | null {
    return readBackendProbeCap({
      probe_cap: currentDraft.value?.accounts.find((item) => item.account_id === accountId)?.probe_cap
    })
  }

  function memberSortOrder(accountId: number): number | null {
    const value = currentDraft.value?.accounts.find((item) => item.account_id === accountId)?.sort_order
    return typeof value === 'number' && Number.isFinite(value) ? value : null
  }

  function applyMemberSortOrders(assignments: Array<{ account_id: number; sort_order: number }>) {
    if (!currentDraft.value) return
    const byID = new Map(assignments.map((item) => [item.account_id, item.sort_order]))
    for (const member of currentDraft.value.accounts) {
      const next = byID.get(member.account_id)
      if (next != null) member.sort_order = next
    }
  }

  function patchLocalResume(accountId: number, next: LocalPairResumeGrace | null) {
    if (!next) {
      const copy = { ...localResumeGraceByAccount.value }
      delete copy[accountId]
      localResumeGraceByAccount.value = copy
    } else {
      localResumeGraceByAccount.value = {
        ...localResumeGraceByAccount.value,
        [accountId]: next
      }
    }
    const uid = userId.value
    if (!uid) return
    const key = String(accountId)
    const stats = qualityStatsById.value[key]
    const resumeUsers = { ...(stats?.resume_users ?? {}) }
    const resumeWatching = { ...(stats?.resume_watching_users ?? {}) }
    if (next && next.chipUntil > 0) resumeUsers[String(uid)] = next.chipUntil
    else delete resumeUsers[String(uid)]
    if (next && next.watchUntil > 0) resumeWatching[String(uid)] = next.watchUntil
    else delete resumeWatching[String(uid)]
    if (!stats && !next) return
    qualityStatsById.value = {
      ...qualityStatsById.value,
      [key]: {
        ...(stats ?? {
          window_seconds: ACCOUNT_QUALITY_WINDOW_SECONDS,
          success_count: 0,
          error_count: 0,
          success_rate: null,
          avg_ttft_ms: null,
          ttft_samples: 0
        }),
        resume_users: resumeUsers,
        resume_watching_users: resumeWatching
      }
    }
  }

  function applyLocalAdmission(
    accountId: number,
    state: PairAdmissionLiveState,
    cooldownUntil?: string | null,
    probeCap?: number | null
  ) {
    const member = currentDraft.value?.accounts.find((item) => item.account_id === accountId)
    const nowSec = Math.floor(Date.now() / 1000)
    localPausedByAccount.value = { ...localPausedByAccount.value, [accountId]: state === 'paused' }
    localProbingByAccount.value = { ...localProbingByAccount.value, [accountId]: state === 'probing' }
    if (member) {
      member.paused = state === 'paused'
      member.probing = state === 'probing'
      member.probe_cap = state === 'probing' ? (probeCap ?? member.probe_cap ?? null) : null
    }
    if (state === 'paused') {
      if (member) member.cooldown_until = null
      patchLocalResume(accountId, null)
      return
    }
    if (state === 'cooling') {
      if (member) {
        member.cooldown_until =
          cooldownUntil
          ?? new Date(Date.now() + (currentDraft.value?.cooldownMinutes || 15) * 60_000).toISOString()
      }
      patchLocalResume(accountId, null)
      return
    }
    if (member) member.cooldown_until = null
    if (state === 'probing' || state === 'selectable') {
      patchLocalResume(accountId, null)
      return
    }
    patchLocalResume(accountId, {
      chipUntil: nowSec + ACCOUNT_QUALITY_WINDOW_SECONDS,
      watchUntil: nowSec + 2 * ACCOUNT_QUALITY_WINDOW_SECONDS
    })
  }

  function memberResumeChipActive(accountId: number, now = Date.now()): boolean {
    const local = localResumeGraceByAccount.value[accountId]
    if (local && local.chipUntil * 1000 > now) return true
    return userQualityResumeChipActive(qualityStatsById.value[String(accountId)], userId.value ?? 0, now)
  }

  function memberResumeActive(accountId: number, now = Date.now()): boolean {
    const local = localResumeGraceByAccount.value[accountId]
    if (local && (local.watchUntil * 1000 > now || local.chipUntil * 1000 > now)) return true
    return userQualityResumeActive(qualityStatsById.value[String(accountId)], userId.value ?? 0, now)
  }

  function effectivePairMax(_account: Account): number {
    return memberCapOrNull(_account.id) ?? 0
  }

  function setMemberCap(accountId: number, value: number) {
    const member = currentDraft.value?.accounts.find((item) => item.account_id === accountId)
    if (!member) return
    member.max_concurrency = value >= 1 ? value : null
  }

  function mergeRuntimeAccount(previous: Account, updated: Account): Account {
    return {
      ...updated,
      current_concurrency: updated.current_concurrency ?? previous.current_concurrency,
      current_window_cost: updated.current_window_cost ?? previous.current_window_cost,
      active_sessions: updated.active_sessions ?? previous.active_sessions,
      group_ids: updated.group_ids ?? previous.group_ids,
      groups: updated.groups ?? previous.groups,
      extra: updated.extra ?? previous.extra
    }
  }

  function patchAccountList(list: Account[], updated: Account): Account[] {
    const index = list.findIndex((item) => item.id === updated.id)
    if (index === -1) return list
    const next = [...list]
    next[index] = mergeRuntimeAccount(list[index], updated)
    return next
  }

  function patchPoolAccount(updated: Account) {
    poolAccounts.value = patchAccountList(poolAccounts.value, updated)
    candidateAccounts.value = patchAccountList(candidateAccounts.value, updated)
  }

  function applyCapToAll() {
    if (!currentDraft.value || bulkCap.value == null || bulkCap.value < 1) return
    for (const member of currentDraft.value.accounts) {
      member.max_concurrency = bulkCap.value
    }
  }

  function applyCapToAccounts(accountIds: number[]) {
    if (!currentDraft.value || bulkCap.value == null || bulkCap.value < 1 || accountIds.length === 0) return
    const targets = new Set(accountIds)
    for (const member of currentDraft.value.accounts) {
      if (targets.has(member.account_id)) {
        member.max_concurrency = bulkCap.value
      }
    }
  }

  function pruneSelection(validIds?: Set<number>) {
    if (!validIds) {
      const used = new Set((currentDraft.value?.accounts ?? []).map((item) => item.account_id))
      selectedAccountIds.value = selectedAccountIds.value.filter((id) => used.has(id))
      return
    }
    selectedAccountIds.value = selectedAccountIds.value.filter((id) => validIds.has(id))
  }

  function toggleAccountSelection(accountId: number) {
    if (selectedAccountIds.value.includes(accountId)) {
      selectedAccountIds.value = selectedAccountIds.value.filter((id) => id !== accountId)
      return
    }
    selectedAccountIds.value = [...selectedAccountIds.value, accountId]
  }

  function selectMatching(accountIds: number[]) {
    selectedAccountIds.value = [...new Set(accountIds)]
  }

  function clearSelection() {
    selectedAccountIds.value = []
  }

  function clonePoolMembers() {
    return (currentDraft.value?.accounts ?? []).map((item) => ({ ...item }))
  }

  async function persistMembershipChange(
    previousAccounts: SmartSchedulePoolMemberDraft[],
    options?: {
      previousEnabled?: boolean
      enabled?: boolean
      successKey?: string
      silent?: boolean
    }
  ) {
    const ok = await persistCurrentPlatform({
      enabled: options?.enabled,
      successKey: options?.successKey,
      silent: options?.silent
    })
    if (!ok && currentDraft.value) {
      currentDraft.value.accounts = previousAccounts
      if (options?.previousEnabled != null) {
        currentDraft.value.enabled = options.previousEnabled
      }
      await loadPoolDetails()
    }
    return ok
  }

  function addSelectedAccount() {
    if (!selectedAddAccountId.value) return
    void addAccountById(selectedAddAccountId.value)
  }

  async function addAccountById(accountId: number) {
    if (!currentDraft.value || submitting.value) return
    if (currentDraft.value.accounts.some((item) => item.account_id === accountId)) return
    const previous = clonePoolMembers()
    currentDraft.value.accounts.push({ account_id: accountId, max_concurrency: null })
    selectedAddAccountId.value = 0
    emptyPoolError.value = false
    void loadPoolDetails()
    await persistMembershipChange(previous, { silent: true })
  }

  async function addAccountsByIds(accountIds: number[]) {
    if (!currentDraft.value || accountIds.length === 0 || submitting.value) return 0
    const used = new Set(currentDraft.value.accounts.map((item) => item.account_id))
    const previous = clonePoolMembers()
    let added = 0
    for (const accountId of accountIds) {
      if (used.has(accountId)) continue
      currentDraft.value.accounts.push({ account_id: accountId, max_concurrency: null })
      used.add(accountId)
      added += 1
    }
    if (added === 0) return 0
    emptyPoolError.value = false
    void loadPoolDetails()
    const ok = await persistMembershipChange(previous, { silent: true })
    return ok ? added : 0
  }

  async function addSchedulingAccounts(scope: SmartScheduleAddScope) {
    if (!currentDraft.value || submitting.value) return
    await ensureCandidates()
    const targets =
      scope === 'apikey'
        ? addableSchedulingApi.value
        : scope === 'oauth'
          ? addableSchedulingOauth.value
          : addableSchedulingAll.value
    if (targets.length === 0) {
      appStore.showWarning(t('admin.users.smartSchedule.addSchedulingNone'))
      return
    }
    const used = new Set(currentDraft.value.accounts.map((item) => item.account_id))
    const previous = clonePoolMembers()
    let added = 0
    for (const account of targets) {
      if (used.has(account.id)) continue
      currentDraft.value.accounts.push({ account_id: account.id, max_concurrency: null })
      used.add(account.id)
      added += 1
    }
    emptyPoolError.value = false
    await loadPoolDetails()
    if (added > 0) {
      const ok = await persistMembershipChange(previous, { silent: true })
      if (!ok) return
    }
    appStore.showSuccess(t('admin.users.smartSchedule.addSchedulingSuccess', { count: String(targets.length) }))
  }

  function removeAccounts(accountIds: number[]) {
    if (!currentDraft.value || accountIds.length === 0 || submitting.value) return
    const removeSet = new Set(accountIds)
    const previous = clonePoolMembers()
    const wasEnabled = currentDraft.value.enabled
    currentDraft.value.accounts = currentDraft.value.accounts.filter((item) => !removeSet.has(item.account_id))
    pruneSelection()
    const emptied = currentDraft.value.accounts.length === 0
    if (emptied) currentDraft.value.enabled = false
    void loadPoolDetails()
    void persistMembershipChange(previous, {
      previousEnabled: wasEnabled,
      enabled: emptied ? false : currentDraft.value.enabled,
      successKey: emptied && wasEnabled ? 'admin.users.smartSchedule.disableSuccess' : undefined,
      silent: !(emptied && wasEnabled)
    })
  }

  function removeAccount(accountId: number) {
    removeAccounts([accountId])
  }

  function resolvePoolFetchNeeds(): SmartSchedulePoolFetchNeeds {
    return options?.poolFetchNeeds ?? { quality: true, today: true, pnl: true }
  }

  async function loadPoolDetails() {
    const ids = currentDraft.value?.accounts.map((item) => item.account_id) ?? []
    if (ids.length === 0) {
      poolAccounts.value = []
      qualityStatsById.value = {}
      todayStatsById.value = {}
      pairPnlById.value = {}
      pairQualityById.value = {}
      return
    }
    const needs = resolvePoolFetchNeeds()
    const spinStats = needs.quality || needs.today || needs.pnl
    if (spinStats) statsLoading.value = true
    try {
      const listedPromise = adminAPI.accounts.list(1, ids.length, {
        platform: activePlatform.value,
        ids: ids.join(','),
        lite: '1'
      })
      const qualityPromise = needs.quality
        ? adminAPI.accounts.getBatchQualityStats(ids).catch(() => ({ stats: {} as Record<string, AccountQualityStats> }))
        : Promise.resolve({ stats: {} as Record<string, AccountQualityStats> })
      const todayPromise = needs.today
        ? adminAPI.accounts.getBatchTodayStats(ids).catch(() => ({ stats: {} as Record<string, WindowStats> }))
        : Promise.resolve({ stats: {} as Record<string, WindowStats> })
      const pnlPromise = needs.pnl && userId.value
        ? adminAPI.users.getSmartSchedulePnlPairs(userId.value, ids).catch(() => ({ pairs: {} as Record<string, SchedulePnlSummary> }))
        : Promise.resolve({ pairs: {} as Record<string, SchedulePnlSummary> })
      const pairQualityPromise = userId.value
        ? adminAPI.users.getSmartSchedulePairQualityBatch(userId.value, ids).catch(() => ({
            pairs: {} as Record<string, SmartSchedulePairQuality>
          }))
        : Promise.resolve({ pairs: {} as Record<string, SmartSchedulePairQuality> })
      const [listed, quality, today, pnl, pairQuality] = await Promise.all([
        listedPromise,
        qualityPromise,
        todayPromise,
        pnlPromise,
        pairQualityPromise
      ])
      const byID = new Map((listed.items ?? []).map((item) => [item.id, item]))
      poolAccounts.value = ids.map((id) => byID.get(id)).filter((item): item is Account => Boolean(item))
      qualityStatsById.value = needs.quality ? (quality.stats ?? {}) : {}
      todayStatsById.value = needs.today ? (today.stats ?? {}) : {}
      pairPnlById.value = needs.pnl ? (pnl.pairs ?? {}) : {}
      pairQualityById.value = pairQuality.pairs ?? {}
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.users.smartSchedule.loadFailed')))
    } finally {
      statsLoading.value = false
    }
  }

  async function loadCandidates(opts?: { force?: boolean }) {
    const platform = activePlatform.value
    if (!opts?.force && candidatesLoaded.value && candidatesPlatform.value === platform) {
      return
    }
    if (candidatesLoading.value) return
    candidatesLoading.value = true
    try {
      const filters = { platform, lite: '1' }
      const first = await adminAPI.accounts.list(1, CANDIDATE_PAGE_SIZE, filters)
      const items = [...(first.items ?? [])]
      const pages = first.pages ?? 1
      for (let page = 2; page <= pages; page++) {
        const next = await adminAPI.accounts.list(page, CANDIDATE_PAGE_SIZE, filters)
        items.push(...(next.items ?? []))
      }
      candidateAccounts.value = items
      candidatesLoaded.value = true
      candidatesPlatform.value = platform
    } catch {
      candidateAccounts.value = []
      candidatesLoaded.value = false
      candidatesPlatform.value = null
    } finally {
      candidatesLoading.value = false
    }
  }

  async function ensureCandidates() {
    await loadCandidates()
  }

  function resetCandidatesForPlatformChange() {
    if (candidatesLoaded.value && candidatesPlatform.value === activePlatform.value) {
      void loadCandidates({ force: true })
      return
    }
    candidateAccounts.value = []
    candidatesLoaded.value = false
    candidatesPlatform.value = null
  }

  async function loadAll(options?: {
    pickPlatform?: boolean
    preserveDirty?: boolean
    silent?: boolean
    refreshCandidatesIfLoaded?: boolean
  }) {
    if (!userId.value) return
    const firstPaint = !initialLoaded.value && !options?.silent
    if (firstPaint) loading.value = true
    else refreshing.value = true
    emptyPoolError.value = false
    try {
      const view = await adminAPI.users.getSmartSchedule(userId.value)
      if (options?.preserveDirty) {
        for (const platform of SMART_SCHEDULE_PLATFORMS) {
          if (isPlatformDirty(platform)) {
            mergeRuntimeMembers(platform, view.platforms?.[platform])
          } else {
            applyPlatformView(platform, view)
          }
        }
      } else {
        applyView(view)
      }
      if (options?.pickPlatform) {
        const next = pickDefaultSmartSchedulePlatform(view, SMART_SCHEDULE_PLATFORMS)
        if (next !== activePlatform.value) {
          skipNextPlatformWatch.value = true
          activePlatform.value = next
        }
      }
      await loadPoolDetails()
      if (options?.refreshCandidatesIfLoaded && candidatesLoaded.value) {
        void loadCandidates({ force: true })
      }
      initialLoaded.value = true
    } catch (error: unknown) {
      if (!options?.silent) {
        appStore.showError(extractApiErrorMessage(error, t('admin.users.smartSchedule.loadFailed')))
      }
    } finally {
      loading.value = false
      refreshing.value = false
    }
  }

  async function refreshAll(options?: { silent?: boolean }) {
    await loadAll({
      preserveDirty: true,
      silent: options?.silent ?? true,
      refreshCandidatesIfLoaded: true
    })
  }

  function buildWrite(enabled = currentDraft.value.enabled) {
    const draft = currentDraft.value
    return {
      enabled,
      quality_max_p50_ttft_ms: draft.maxP50 === '' ? null : Number(draft.maxP50),
      quality_min_success_rate: percentToSuccessRate(draft.successPercent),
      quality_window_n: clampSmartScheduleWindowN(draft.windowN === '' ? null : Number(draft.windowN)),
      quality_min_success_samples: clampSmartScheduleWindowN(draft.windowN === '' ? null : Number(draft.windowN)),
      quality_min_ttft_samples: clampSmartScheduleWindowN(draft.windowN === '' ? null : Number(draft.windowN)),
      quality_condition: draft.condition,
      cooldown_minutes: draft.cooldownMinutes || 15,
      accounts: draft.accounts.map((item) => ({
        account_id: item.account_id,
        platform: activePlatform.value,
        max_concurrency: item.max_concurrency,
        sort_order: item.sort_order ?? null
      }))
    }
  }

  async function persistSortOrders(
    assignments: Array<{ account_id: number; sort_order: number }>,
    errorKey?: string,
    options?: { silent?: boolean }
  ) {
    if (!userId.value || assignments.length === 0) return false
    const previous = (currentDraft.value?.accounts ?? []).map((item) => ({ ...item }))
    applyMemberSortOrders(assignments)
    try {
      const view = await adminAPI.users.updateSmartScheduleSortOrder(userId.value, activePlatform.value, {
        accounts: assignments
      })
      mergeRuntimeMembers(activePlatform.value, view.platforms?.[activePlatform.value])
      return true
    } catch (error: unknown) {
      if (currentDraft.value) currentDraft.value.accounts = previous
      if (!options?.silent) {
        appStore.showError(
          extractApiErrorMessage(error, t(errorKey || 'admin.users.smartSchedule.autoSortFailed'))
        )
      }
      return false
    }
  }

  async function persistCurrentPlatform(options?: {
    enabled?: boolean
    successKey?: string
    silent?: boolean
  }) {
    if (!userId.value || !currentDraft.value) return false
    const nextEnabled = options?.enabled ?? currentDraft.value.enabled
    if (nextEnabled && currentDraft.value.accounts.length === 0) {
      emptyPoolError.value = true
      currentDraft.value.enabled = false
      return false
    }
    submitting.value = true
    emptyPoolError.value = false
    const previousEnabled = currentDraft.value.enabled
    currentDraft.value.enabled = nextEnabled
    try {
      const view = await adminAPI.users.updateSmartSchedule(userId.value, activePlatform.value, buildWrite(nextEnabled))
      applyPlatformView(activePlatform.value, view)
      await loadPoolDetails()
      if (!options?.silent) {
        appStore.showSuccess(t(options?.successKey || 'admin.users.smartSchedule.updateSuccess'))
      }
      return true
    } catch (error: unknown) {
      currentDraft.value.enabled = previousEnabled
      appStore.showError(extractApiErrorMessage(error, t('admin.users.smartSchedule.updateFailed')))
      return false
    } finally {
      submitting.value = false
    }
  }

  async function onToggleEnabled() {
    if (!currentDraft.value || submitting.value) return
    const nextEnabled = !currentDraft.value.enabled
    await persistCurrentPlatform({
      enabled: nextEnabled,
      successKey: nextEnabled
        ? 'admin.users.smartSchedule.enableSuccess'
        : 'admin.users.smartSchedule.disableSuccess'
    })
  }

  async function onSave() {
    await persistCurrentPlatform()
  }

  async function onCopy() {
    if (!userId.value || !copyFromPlatform.value) return
    copying.value = true
    try {
      const view = await adminAPI.users.copySmartSchedule(
        userId.value,
        activePlatform.value,
        copyFromPlatform.value
      )
      applyPlatformView(activePlatform.value, view)
      copyFromPlatform.value = ''
      await loadPoolDetails()
      appStore.showSuccess(t('admin.users.smartSchedule.copySuccess'))
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.users.smartSchedule.copyFailed')))
    } finally {
      copying.value = false
    }
  }

  async function setPairAdmission(accountId: number, state: PairAdmissionLiveState) {
    if (!userId.value) return
    try {
      const result = await adminAPI.accounts.resumeSmartSchedule(accountId, userId.value, state)
      const nextState =
        result.state === 'cooling'
        || result.state === 'selectable'
        || result.state === 'resumed'
        || result.state === 'paused'
        || result.state === 'probing'
          ? result.state
          : state
      applyLocalAdmission(
        accountId,
        nextState,
        result.cooldown_until,
        readBackendProbeCap(result)
      )
      const toast =
        nextState === 'paused'
          ? 'admin.users.smartSchedule.switchSuccessPaused'
          : nextState === 'cooling'
            ? 'admin.users.smartSchedule.switchSuccessCooling'
            : nextState === 'probing'
              ? 'admin.users.smartSchedule.switchSuccessProbing'
              : nextState === 'selectable'
                ? 'admin.users.smartSchedule.switchSuccessSelectable'
                : 'admin.users.smartSchedule.resumeSuccess'
      appStore.showSuccess(t(toast))
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.users.smartSchedule.switchFailed')))
    }
  }

  const isDirty = computed(() => isPlatformDirty(activePlatform.value))

  watch(
    userId,
    (id) => {
      localResumeGraceByAccount.value = {}
      localPausedByAccount.value = {}
      localProbingByAccount.value = {}
      if (id) {
        void loadAll({ pickPlatform: true })
      }
    },
    { immediate: true }
  )

  watch(activePlatform, () => {
    emptyPoolError.value = false
    selectedAddAccountId.value = 0
    clearSelection()
    if (skipNextPlatformWatch.value) {
      skipNextPlatformWatch.value = false
      return
    }
    if (userId.value) {
      void loadPoolDetails()
      resetCandidatesForPlatformChange()
    }
  })

  return {
    platforms: SMART_SCHEDULE_PLATFORMS,
    loading,
    initialLoaded,
    submitting,
    copying,
    refreshing,
    statsLoading,
    candidatesLoaded,
    candidatesLoading,
    candidatesReady: computed(
      () => candidatesLoaded.value && candidatesPlatform.value === activePlatform.value
    ),
    emptyPoolError,
    isDirty,
    activePlatform,
    copyFromPlatform,
    selectedAddAccountId,
    bulkCap,
    drafts,
    poolAccounts,
    qualityStatsById,
    todayStatsById,
    pairPnlById,
    pairQualityById,
    currentDraft,
    currentSavedDraft,
    otherPlatforms,
    addableAccounts,
    addableSchedulingApi,
    addableSchedulingOauth,
    addableSchedulingAll,
    qualityTemplateBusy,
    applyQualityTemplate,
    saveQualityTemplate,
    applyTemplateToDraft,
    qualityGateFormFromDraft,
    memberCap,
    memberCapOrNull,
    memberCurrent,
    memberCooldownUntil,
    memberPaused,
    memberProbing,
    memberProbeCap,
    memberSortOrder,
    persistSortOrders,
    memberResumeActive,
    memberResumeChipActive,
    effectivePairMax,
    setMemberCap,
    patchPoolAccount,
    applyCapToAll,
    applyCapToAccounts,
    selectedAccountIds,
    toggleAccountSelection,
    selectMatching,
    clearSelection,
    addSelectedAccount,
    addAccountById,
    addAccountsByIds,
    addSchedulingAccounts,
    removeAccount,
    removeAccounts,
    onToggleEnabled,
    onSave,
    onCopy,
    setPairAdmission,
    refreshAll,
    ensureCandidates,
    loadPoolDetails
  }
}
