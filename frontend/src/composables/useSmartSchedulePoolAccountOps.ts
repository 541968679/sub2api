import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { SelectOption } from '@/components/common/Select.vue'
import type { Account, AdminGroup, ClaudeModel, Proxy, UpdateAccountRequest } from '@/types'

function isFallbackOnly(account: Account): boolean {
  return account.fallback_only === true || account.extra?.fallback_only === true
}

function buildExtraWithFallbackOnly(account: Account, enabled: boolean): Record<string, unknown> {
  const next: Record<string, unknown> = { ...(account.extra || {}) }
  if (enabled) {
    next.fallback_only = true
  } else {
    delete next.fallback_only
  }
  return next
}

function formatExportTimestamp() {
  const now = new Date()
  const pad2 = (value: number) => String(value).padStart(2, '0')
  return `${now.getFullYear()}${pad2(now.getMonth() + 1)}${pad2(now.getDate())}${pad2(now.getHours())}${pad2(now.getMinutes())}${pad2(now.getSeconds())}`
}

function sanitizeFilenamePart(value: string) {
  const normalized = value.trim().replace(/[\\/:*?"<>|]+/g, '-').replace(/\s+/g, '-')
  return normalized.slice(0, 80) || 'openai-oauth'
}

function downloadJsonFile(payload: unknown, filename: string) {
  const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}

export function useSmartSchedulePoolAccountOps(options: {
  patchPoolAccount: (account: Account) => void
}) {
  const { t } = useI18n()
  const appStore = useAppStore()

  const groups = ref<AdminGroup[]>([])
  const proxies = ref<Proxy[]>([])
  const inlineSavingId = ref<number | null>(null)
  const togglingSchedulable = ref<number | null>(null)

  const showEdit = ref(false)
  const edAcc = ref<Account | null>(null)
  const editingAccountId = ref<number | null>(null)
  const showStability = ref(false)
  const stabilityAcc = ref<Account | null>(null)
  const showTempUnsched = ref(false)
  const tempUnschedAcc = ref<Account | null>(null)
  const showTest = ref(false)
  const testingAcc = ref<Account | null>(null)
  const showStats = ref(false)
  const statsAcc = ref<Account | null>(null)
  const showReAuth = ref(false)
  const reAuthAcc = ref<Account | null>(null)
  const showUpdateRt = ref(false)
  const updateRtAcc = ref<Account | null>(null)
  const showSchedulePanel = ref(false)
  const scheduleAcc = ref<Account | null>(null)
  const scheduleModelOptions = ref<SelectOption[]>([])
  const showSparkShadowDialog = ref(false)
  const sparkShadowParent = ref<Account | null>(null)
  const inspectOpen = ref(false)
  const inspectSubjectId = ref<number | null>(null)
  const inspectSubjectLabel = ref('')
  const inspectTab = ref<'usage' | 'errors'>('usage')
  const menu = reactive<{
    show: boolean
    acc: Account | null
    pos: { top: number; left: number } | null
  }>({ show: false, acc: null, pos: null })

  function patch(account: Account) {
    options.patchPoolAccount(account)
  }

  async function saveAccountPatch(account: Account, updates: UpdateAccountRequest, optimistic: Account) {
    inlineSavingId.value = account.id
    const previous = { ...account }
    try {
      patch(optimistic)
      const updated = await adminAPI.accounts.update(account.id, updates)
      patch(updated)
    } catch (error: unknown) {
      patch(previous)
      appStore.showError(extractApiErrorMessage(error, t('admin.accounts.inlineEdit.saveFailed')))
    } finally {
      inlineSavingId.value = null
    }
  }

  async function handleInlineConcurrency(account: Account, value: number) {
    if (value < 1 || value === account.concurrency) return
    await saveAccountPatch(account, { concurrency: value }, { ...account, concurrency: value })
  }

  async function handleInlinePriority(account: Account, value: number) {
    if (value < 0 || value === (account.priority ?? 0)) return
    await saveAccountPatch(account, { priority: value }, { ...account, priority: value })
  }

  async function handleInlineUpstreamRate(account: Account, value: number) {
    if (value < 0 || value === (account.upstream_rate_multiplier ?? 1)) return
    await saveAccountPatch(
      account,
      { upstream_rate_multiplier: value },
      { ...account, upstream_rate_multiplier: value }
    )
  }

  async function handleToggleSchedulable(account: Account) {
    const nextSchedulable = !account.schedulable
    togglingSchedulable.value = account.id
    try {
      const updated = await adminAPI.accounts.setSchedulable(account.id, nextSchedulable)
      patch({ ...account, ...updated, schedulable: updated?.schedulable ?? nextSchedulable })
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.accounts.failedToToggleSchedulable')))
    } finally {
      togglingSchedulable.value = null
    }
  }

  async function handleToggleFallbackOnly(account: Account) {
    const next = !isFallbackOnly(account)
    const extra = buildExtraWithFallbackOnly(account, next)
    await saveAccountPatch(account, { extra }, { ...account, fallback_only: next, extra: extra as Account['extra'] })
  }

  async function handleEdit(account: Account) {
    const requestId = account.id
    editingAccountId.value = requestId
    try {
      const full = await adminAPI.accounts.getById(account.id)
      if (editingAccountId.value !== requestId) return
      edAcc.value = full
      showEdit.value = true
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.accounts.failedToLoad')))
    } finally {
      if (editingAccountId.value === requestId) editingAccountId.value = null
    }
  }

  function handleAccountUpdated(account: Account) {
    patch(account)
    if (edAcc.value?.id === account.id) edAcc.value = account
    if (stabilityAcc.value?.id === account.id) stabilityAcc.value = account
  }

  function openStabilityDialog(account: Account) {
    stabilityAcc.value = account
    showStability.value = true
  }

  function handleStabilityRecovered(account: Account) {
    patch(account)
    stabilityAcc.value = account
  }

  function handleShowTempUnsched(account: Account) {
    tempUnschedAcc.value = account
    showTempUnsched.value = true
  }

  function handleTempUnschedReset(account: Account) {
    showTempUnsched.value = false
    tempUnschedAcc.value = null
    patch(account)
  }

  function openUsageErrorInspect(account: Account, tab: 'usage' | 'errors') {
    inspectSubjectId.value = account.id
    inspectSubjectLabel.value = account.name || ''
    inspectTab.value = tab
    inspectOpen.value = true
  }

  function handleViewUsage(account: Account) {
    openUsageErrorInspect(account, 'usage')
  }

  function handleViewErrorRequests(account: Account) {
    openUsageErrorInspect(account, 'errors')
  }

  function openMenu(account: Account, event: MouseEvent) {
    menu.acc = account
    const padding = 8
    const menuWidth = 200
    const menuHeight = 240
    const viewportWidth = window.innerWidth
    const viewportHeight = window.innerHeight
    let left = Math.max(padding, Math.min(event.clientX - menuWidth, viewportWidth - menuWidth - padding))
    let top = event.clientY
    if (top + menuHeight > viewportHeight - padding) {
      top = Math.max(padding, event.clientY - menuHeight)
    }
    menu.pos = { top, left }
    menu.show = true
  }

  function handleTest(account: Account) {
    testingAcc.value = account
    showTest.value = true
  }

  function handleViewStats(account: Account) {
    statsAcc.value = account
    showStats.value = true
  }

  async function handleSchedule(account: Account) {
    scheduleAcc.value = account
    scheduleModelOptions.value = []
    showSchedulePanel.value = true
    try {
      const models = await adminAPI.accounts.getAvailableModels(account.id)
      scheduleModelOptions.value = models.map((model: ClaudeModel) => ({
        value: model.id,
        label: model.display_name || model.id
      }))
    } catch {
      scheduleModelOptions.value = []
    }
  }

  function handleReAuth(account: Account) {
    reAuthAcc.value = account
    showReAuth.value = true
  }

  function handleUpdateRefreshToken(account: Account) {
    updateRtAcc.value = account
    showUpdateRt.value = true
  }

  async function handleRefresh(account: Account) {
    try {
      const updated = await adminAPI.accounts.refreshCredentials(account.id)
      patch(updated)
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.accounts.failedToRefresh')))
    }
  }

  async function handleRecoverState(account: Account) {
    try {
      const updated = await adminAPI.accounts.recoverState(account.id)
      patch(updated)
      appStore.showSuccess(t('admin.accounts.recoverStateSuccess'))
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.accounts.recoverStateFailed')))
    }
  }

  async function handleResetQuota(account: Account) {
    try {
      const updated = await adminAPI.accounts.resetAccountQuota(account.id)
      patch(updated)
      appStore.showSuccess(t('common.success'))
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('common.error')))
    }
  }

  async function handleClearStuckRuntime(account: Account) {
    try {
      const result = await adminAPI.accounts.clearStuckRuntime(account.id)
      appStore.showSuccess(
        t('admin.accounts.clearStuckRuntimeSuccess', {
          sticky: result.sticky_deleted ?? 0
        })
      )
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.accounts.clearStuckRuntimeFailed')))
    }
  }

  async function handleSetPrivacy(account: Account) {
    try {
      const updated = await adminAPI.accounts.setPrivacy(account.id)
      patch(updated)
      appStore.showSuccess(t('common.success'))
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.accounts.privacyFailed')))
    }
  }

  async function handleExportCodexAuth(account: Account) {
    try {
      const payloads = await adminAPI.accounts.exportCodexAuth({ ids: [account.id], limit: 1 })
      const payload = payloads[0]
      if (!payload) {
        appStore.showWarning(t('admin.accounts.exportCodexAuthUnavailable'))
        return
      }
      downloadJsonFile(
        payload,
        `codex-auth-${sanitizeFilenamePart(account.name)}-${formatExportTimestamp()}.json`
      )
      appStore.showSuccess(t('admin.accounts.exportCodexAuthSuccess'))
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.accounts.exportCodexAuthFailed')))
    }
  }

  function handleCreateSparkShadow(account: Account) {
    sparkShadowParent.value = account
    showSparkShadowDialog.value = true
  }

  function cancelCreateSparkShadow() {
    showSparkShadowDialog.value = false
    sparkShadowParent.value = null
  }

  async function confirmCreateSparkShadow() {
    const parent = sparkShadowParent.value
    if (!parent) return
    try {
      await adminAPI.accounts.createSparkShadow(parent.id, { name: `${parent.name} (Spark)` })
      appStore.showSuccess(t('admin.accounts.createSparkShadowSuccess'))
      cancelCreateSparkShadow()
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.accounts.createSparkShadowFailed')))
    }
  }

  onMounted(() => {
    void Promise.all([adminAPI.proxies.getAllWithCount(), adminAPI.groups.getAll()])
      .then(([nextProxies, nextGroups]) => {
        proxies.value = nextProxies
        groups.value = nextGroups
      })
      .catch(() => {
        proxies.value = []
        groups.value = []
      })
  })

  return {
    groups,
    proxies,
    inlineSavingId,
    togglingSchedulable,
    isFallbackOnly,
    showEdit,
    edAcc,
    showStability,
    stabilityAcc,
    showTempUnsched,
    tempUnschedAcc,
    showTest,
    testingAcc,
    showStats,
    statsAcc,
    showReAuth,
    reAuthAcc,
    showUpdateRt,
    updateRtAcc,
    showSchedulePanel,
    scheduleAcc,
    scheduleModelOptions,
    showSparkShadowDialog,
    sparkShadowParent,
    inspectOpen,
    inspectSubjectId,
    inspectSubjectLabel,
    inspectTab,
    menu,
    handleInlineConcurrency,
    handleInlinePriority,
    handleInlineUpstreamRate,
    handleToggleSchedulable,
    handleToggleFallbackOnly,
    handleEdit,
    handleAccountUpdated,
    openStabilityDialog,
    handleStabilityRecovered,
    handleShowTempUnsched,
    handleTempUnschedReset,
    handleViewUsage,
    handleViewErrorRequests,
    openMenu,
    handleTest,
    handleViewStats,
    handleSchedule,
    handleReAuth,
    handleUpdateRefreshToken,
    handleRefresh,
    handleRecoverState,
    handleResetQuota,
    handleClearStuckRuntime,
    handleSetPrivacy,
    handleExportCodexAuth,
    handleCreateSparkShadow,
    cancelCreateSparkShadow,
    confirmCreateSparkShadow
  }
}
