import { computed, reactive, ref, watch, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Account, WindowStats } from '@/types'
import type { AccountQualityStats } from '@/api/admin/accounts'
import type {
  SmartSchedulePlatform,
  SmartSchedulePlatformView,
  UserSmartScheduleView
} from '@/api/admin/users'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  applyQualityGateFormToDraft,
  percentToSuccessRate,
  qualityGateFormFromDraft,
  successRateToPercent,
  type QualityGateFormFields
} from '@/utils/accountQualityHardClose'
import { useQualityThresholdTemplate } from '@/composables/useQualityThresholdTemplate'

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
  current_concurrency?: number
}

export type SmartSchedulePlatformDraft = {
  enabled: boolean
  maxP50: number | ''
  successPercent: number | ''
  minSuccessSamples: number | ''
  minTtftSamples: number | ''
  condition: 'or' | 'and'
  cooldownMinutes: number
  accounts: SmartSchedulePoolMemberDraft[]
}

export type SmartScheduleAddScope = 'apikey' | 'oauth' | 'all'

const CANDIDATE_PAGE_SIZE = 1000

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

export function emptySmartScheduleDraft(): SmartSchedulePlatformDraft {
  return {
    enabled: false,
    maxP50: '',
    successPercent: '',
    minSuccessSamples: '',
    minTtftSamples: '',
    condition: 'or',
    cooldownMinutes: 15,
    accounts: []
  }
}

export function useUserSmartScheduleEditor(userId: Ref<number | null>) {
  const { t } = useI18n()
  const appStore = useAppStore()
  const { templateBusy: qualityTemplateBusy, applyQualityTemplate, saveQualityTemplate } =
    useQualityThresholdTemplate()

  const loading = ref(false)
  const submitting = ref(false)
  const copying = ref(false)
  const statsLoading = ref(false)
  const emptyPoolError = ref(false)
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

  const currentDraft = computed(() => drafts[activePlatform.value])
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
    draft.minSuccessSamples = view.quality_min_success_samples ?? ''
    draft.minTtftSamples = view.quality_min_ttft_samples ?? ''
    draft.condition = view.quality_condition === 'and' ? 'and' : 'or'
    draft.cooldownMinutes = view.cooldown_minutes || 15
    draft.accounts = (view.accounts ?? []).map((item) => ({
      account_id: item.account_id,
      max_concurrency: item.max_concurrency ?? null,
      current_concurrency: item.current_concurrency ?? 0
    }))
    return draft
  }

  function applyView(view: UserSmartScheduleView) {
    for (const platform of SMART_SCHEDULE_PLATFORMS) {
      drafts[platform] = viewToDraft(view.platforms?.[platform])
    }
  }

  function applyPlatformView(platform: SmartSchedulePlatform, view: UserSmartScheduleView) {
    drafts[platform] = viewToDraft(view.platforms?.[platform])
  }

  function applyTemplateToDraft(fields: QualityGateFormFields) {
    if (!currentDraft.value) return
    applyQualityGateFormToDraft(currentDraft.value, fields)
  }

  function memberCap(accountId: number): number {
    return currentDraft.value?.accounts.find((item) => item.account_id === accountId)?.max_concurrency ?? 0
  }

  function memberCurrent(accountId: number): number {
    return currentDraft.value?.accounts.find((item) => item.account_id === accountId)?.current_concurrency ?? 0
  }

  function effectivePairMax(account: Account): number {
    const cap = memberCap(account.id)
    if (cap >= 1) return cap
    return account.concurrency || 0
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

  function addSelectedAccount() {
    if (!currentDraft.value || !selectedAddAccountId.value) return
    if (currentDraft.value.accounts.some((item) => item.account_id === selectedAddAccountId.value)) return
    currentDraft.value.accounts.push({ account_id: selectedAddAccountId.value, max_concurrency: null })
    selectedAddAccountId.value = 0
    emptyPoolError.value = false
    void loadPoolDetails()
  }

  function addAccountById(accountId: number) {
    selectedAddAccountId.value = accountId
    addSelectedAccount()
  }

  function addSchedulingAccounts(scope: SmartScheduleAddScope) {
    if (!currentDraft.value) return
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
    for (const account of targets) {
      if (used.has(account.id)) continue
      currentDraft.value.accounts.push({ account_id: account.id, max_concurrency: null })
      used.add(account.id)
    }
    emptyPoolError.value = false
    void loadPoolDetails()
    appStore.showSuccess(t('admin.users.smartSchedule.addSchedulingSuccess', { count: String(targets.length) }))
  }

  function removeAccount(accountId: number) {
    if (!currentDraft.value) return
    const wasEnabled = currentDraft.value.enabled
    currentDraft.value.accounts = currentDraft.value.accounts.filter((item) => item.account_id !== accountId)
    if (currentDraft.value.accounts.length === 0) {
      currentDraft.value.enabled = false
      if (wasEnabled) {
        void persistCurrentPlatform({
          enabled: false,
          successKey: 'admin.users.smartSchedule.disableSuccess'
        })
      }
    }
    void loadPoolDetails()
  }

  async function loadPoolDetails() {
    const ids = currentDraft.value?.accounts.map((item) => item.account_id) ?? []
    if (ids.length === 0) {
      poolAccounts.value = []
      qualityStatsById.value = {}
      todayStatsById.value = {}
      return
    }
    statsLoading.value = true
    try {
      const [listed, quality, today] = await Promise.all([
        adminAPI.accounts.list(1, ids.length, { platform: activePlatform.value, ids: ids.join(',') }),
        adminAPI.accounts.getBatchQualityStats(ids),
        adminAPI.accounts.getBatchTodayStats(ids)
      ])
      const byID = new Map((listed.items ?? []).map((item) => [item.id, item]))
      poolAccounts.value = ids.map((id) => byID.get(id)).filter((item): item is Account => Boolean(item))
      qualityStatsById.value = quality.stats ?? {}
      todayStatsById.value = today.stats ?? {}
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.users.smartSchedule.loadFailed')))
    } finally {
      statsLoading.value = false
    }
  }

  async function loadCandidates() {
    try {
      const filters = { platform: activePlatform.value }
      const first = await adminAPI.accounts.list(1, CANDIDATE_PAGE_SIZE, filters)
      const items = [...(first.items ?? [])]
      const pages = first.pages ?? 1
      for (let page = 2; page <= pages; page++) {
        const next = await adminAPI.accounts.list(page, CANDIDATE_PAGE_SIZE, filters)
        items.push(...(next.items ?? []))
      }
      candidateAccounts.value = items
    } catch {
      candidateAccounts.value = []
    }
  }

  async function loadAll() {
    if (!userId.value) return
    loading.value = true
    emptyPoolError.value = false
    try {
      const view = await adminAPI.users.getSmartSchedule(userId.value)
      applyView(view)
      await Promise.all([loadPoolDetails(), loadCandidates()])
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.users.smartSchedule.loadFailed')))
    } finally {
      loading.value = false
    }
  }

  function buildWrite(enabled = currentDraft.value.enabled) {
    const draft = currentDraft.value
    return {
      enabled,
      quality_max_p50_ttft_ms: draft.maxP50 === '' ? null : Number(draft.maxP50),
      quality_min_success_rate: percentToSuccessRate(draft.successPercent),
      quality_min_success_samples: draft.minSuccessSamples === '' ? null : Number(draft.minSuccessSamples),
      quality_min_ttft_samples: draft.minTtftSamples === '' ? null : Number(draft.minTtftSamples),
      quality_condition: draft.condition,
      cooldown_minutes: draft.cooldownMinutes || 15,
      accounts: draft.accounts.map((item) => ({
        account_id: item.account_id,
        platform: activePlatform.value,
        max_concurrency: item.max_concurrency
      }))
    }
  }

  async function persistCurrentPlatform(options?: { enabled?: boolean; successKey?: string }) {
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
      appStore.showSuccess(t(options?.successKey || 'admin.users.smartSchedule.updateSuccess'))
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

  async function resumePair(accountId: number) {
    if (!userId.value) return
    try {
      await adminAPI.accounts.resumeSmartSchedule(accountId, userId.value)
      appStore.showSuccess(t('admin.users.smartSchedule.resumeSuccess'))
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.users.smartSchedule.resumeFailed')))
    }
  }

  watch(
    userId,
    (id) => {
      if (id) {
        activePlatform.value = 'anthropic'
        void loadAll()
      }
    },
    { immediate: true }
  )

  watch(activePlatform, () => {
    emptyPoolError.value = false
    selectedAddAccountId.value = 0
    if (userId.value) {
      void Promise.all([loadPoolDetails(), loadCandidates()])
    }
  })

  return {
    platforms: SMART_SCHEDULE_PLATFORMS,
    loading,
    submitting,
    copying,
    statsLoading,
    emptyPoolError,
    activePlatform,
    copyFromPlatform,
    selectedAddAccountId,
    bulkCap,
    drafts,
    poolAccounts,
    qualityStatsById,
    todayStatsById,
    currentDraft,
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
    memberCurrent,
    effectivePairMax,
    setMemberCap,
    patchPoolAccount,
    applyCapToAll,
    addSelectedAccount,
    addAccountById,
    addSchedulingAccounts,
    removeAccount,
    onToggleEnabled,
    onSave,
    onCopy,
    resumePair
  }
}
