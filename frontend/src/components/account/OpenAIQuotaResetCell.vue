<template>
  <div v-if="visible" data-testid="openai-quota-actions" class="space-y-1">
    <!--
      Unified action row. Parents that already render their own "local query"
      affordance (e.g. AccountUsageCell's active-sampling refresh) pass it in
      via the #pre-actions slot so the user sees a single row of related
      buttons rather than two near-duplicate "查询" rows.

      The 5h / 7d window bars are deliberately NOT rendered here — the local
      active-sampling display (UsageProgressBar in AccountUsageCell) already
      owns that real estate. This cell is purely about the rate-limit reset
      credit: query its count, consume one if needed.
    -->
    <div class="flex flex-wrap items-center gap-1.5">
      <slot name="pre-actions" />

      <button
        type="button"
        data-testid="query-openai-quota"
        class="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px] font-medium text-blue-600 transition-colors hover:bg-blue-50 disabled:cursor-not-allowed disabled:opacity-50 dark:text-blue-400 dark:hover:bg-blue-900/30"
        :disabled="loading || resetting"
        :title="countButtonTitle"
        @click="handleQuery"
      >
        <svg
          class="h-2.5 w-2.5"
          :class="{ 'animate-spin': loading }"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
          />
        </svg>
        {{ t('admin.accounts.openaiQuotaReset.count') }}<span v-if="data"> {{ availableResetCount }}</span>
      </button>

      <button
        type="button"
        data-testid="reset-openai-quota"
        class="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px] font-medium text-orange-600 transition-colors hover:bg-orange-50 disabled:cursor-not-allowed disabled:opacity-50 dark:text-orange-400 dark:hover:bg-orange-900/30"
        :disabled="resetting || loading || !canReset"
        :title="resetButtonTitle"
        @click="openResetConfirm"
      >
        <svg
          class="h-2.5 w-2.5"
          :class="{ 'animate-spin': resetting }"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M20 12a8 8 0 11-2.343-5.657L20 8m0 0V4m0 4h-4"
          />
        </svg>
        {{ t('admin.accounts.openaiQuotaReset.reset') }}
      </button>
    </div>

    <div v-if="primaryResetCreditExpiry" class="flex flex-wrap items-center gap-1">
      <span
        class="inline-flex max-w-full items-center rounded bg-gray-100 px-1.5 py-0.5 text-[10px] leading-4 text-gray-600 tabular-nums dark:bg-gray-800 dark:text-gray-300"
        :title="t('admin.accounts.openaiQuotaReset.expiresAtFull', { time: formatResetCreditExpiry(primaryResetCreditExpiry, 'full') })"
      >
        {{ t('admin.accounts.openaiQuotaReset.expiresAt', { time: formatResetCreditExpiry(primaryResetCreditExpiry, 'short') }) }}
      </span>
      <button
        v-if="hiddenResetCreditCount > 0"
        type="button"
        data-testid="reset-credit-expiry-toggle"
        class="inline-flex items-center rounded-full bg-gray-100 px-1.5 py-0.5 text-[10px] text-gray-600 dark:bg-gray-800 dark:text-gray-300"
        :aria-expanded="showResetCreditDetails"
        @click="showResetCreditDetails = !showResetCreditDetails"
      >
        +{{ hiddenResetCreditCount }}
      </button>
      <div
        v-if="showResetCreditDetails && resetCreditExpirations.length > 1"
        data-testid="reset-credit-expiry-details"
        class="grid gap-0.5 text-[10px] text-gray-600 dark:text-gray-300"
      >
        <span v-for="(expiresAt, index) in resetCreditExpirations" :key="`${expiresAt}-${index}`">
          {{ formatResetCreditExpiry(expiresAt, 'full') }}
        </span>
      </div>
    </div>

    <!-- Error / success feedback -->
    <div
      v-if="error"
      class="text-[10px] text-red-600 dark:text-red-400"
      :title="error"
    >
      {{ truncatedError }}
    </div>
    <div
      v-else-if="resetMessage"
      class="text-[10px] text-emerald-600 dark:text-emerald-400"
    >
      {{ resetMessage }}
    </div>

    <ConfirmDialog
      :show="showResetConfirm"
      :title="t('admin.accounts.openaiQuotaReset.confirmTitle')"
      :message="t('admin.accounts.openaiQuotaReset.confirmMessage', { count: availableResetCount })"
      :confirm-text="t('admin.accounts.openaiQuotaReset.reset')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmReset"
      @cancel="cancelReset"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account } from '@/types'
import {
  queryOpenAIQuota,
  resetOpenAIQuota,
  type OpenAIQuotaUsage,
  type OpenAIQuotaResetResult
} from '@/api/admin/accounts'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'

const props = defineProps<{
  account: Account
}>()

const { t } = useI18n()

// Visible only for OpenAI OAuth accounts.
const visible = computed(() => props.account.platform === 'openai' && props.account.type === 'oauth')

const loading = ref(false)
const resetting = ref(false)
const error = ref<string | null>(null)
const data = ref<OpenAIQuotaUsage | null>(null)
const resetMessage = ref<string | null>(null)
const showResetConfirm = ref(false)
const showResetCreditDetails = ref(false)
const redeemRequestId = ref<string | null>(null)

const availableResetCount = computed(() => data.value?.rate_limit_reset_credits?.available_count ?? 0)
const resetCreditExpirations = computed(() =>
  (data.value?.rate_limit_reset_credits?.credits ?? [])
    .map((credit) => credit.expires_at?.trim() ?? '')
    .filter((expiresAt) => expiresAt.length > 0)
    .sort(compareResetCreditExpiry)
)
const primaryResetCreditExpiry = computed(() => resetCreditExpirations.value[0] ?? '')
const hiddenResetCreditCount = computed(() => Math.max(resetCreditExpirations.value.length - 1, 0))
const isShadow = computed(() => props.account.parent_account_id != null)
const canReset = computed(() => !isShadow.value && availableResetCount.value > 0)

const resetButtonTitle = computed(() => {
  if (isShadow.value) return t('admin.accounts.openaiQuotaReset.resetTooltipShadow')
  if (!data.value) return t('admin.accounts.openaiQuotaReset.resetTooltipNeedQuery')
  if (!canReset.value) return t('admin.accounts.openaiQuotaReset.resetTooltipNoCredits')
  return t('admin.accounts.openaiQuotaReset.resetTooltipReady')
})

// "次数" button doubles as the upstream-query trigger and the count display.
// Tooltip differs between "click to load" (no data yet) and "click to refresh".
const countButtonTitle = computed(() => {
  if (!data.value) return t('admin.accounts.openaiQuotaReset.countTooltipLoad')
  return t('admin.accounts.openaiQuotaReset.countTooltipRefresh')
})

const truncatedError = computed(() => {
  if (!error.value) return ''
  return error.value.length > 80 ? `${error.value.slice(0, 80)}…` : error.value
})

const compareResetCreditExpiry = (a: string, b: string): number => {
  const aTime = new Date(a).getTime()
  const bTime = new Date(b).getTime()
  const normalizedA = Number.isNaN(aTime) ? Number.POSITIVE_INFINITY : aTime
  const normalizedB = Number.isNaN(bTime) ? Number.POSITIVE_INFINITY : bTime
  return normalizedA === normalizedB ? a.localeCompare(b) : normalizedA - normalizedB
}

const formatResetCreditExpiry = (value: string, style: 'short' | 'full'): string => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(undefined, {
    ...(style === 'full' ? { year: 'numeric' as const } : {}),
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(date)
}

const extractErrorMessage = (e: unknown): string => {
  // The project's axios response interceptor (api/client.ts) flattens server
  // errors into { status, code, message, reason, ... } and re-rejects them, so
  // the message lives at the top level rather than under .response.data. Fall
  // back to the raw axios shape for the cancellation/network branches that
  // bypass the flattening, and finally to the generic i18n string.
  const err = e as {
    message?: string
    reason?: string
    response?: { data?: { message?: string; error?: string } }
  }
  return (
    err?.message ||
    err?.reason ||
    err?.response?.data?.message ||
    err?.response?.data?.error ||
    t('common.error')
  )
}

const handleQuery = async () => {
  if (loading.value) return
  loading.value = true
  error.value = null
  resetMessage.value = null
  try {
    data.value = await queryOpenAIQuota(props.account.id)
  } catch (e) {
    error.value = extractErrorMessage(e)
  } finally {
    loading.value = false
  }
}

const openResetConfirm = () => {
  if (resetting.value || loading.value) return
  if (!canReset.value) {
    error.value = t('admin.accounts.openaiQuotaReset.noCreditsAvailable')
    return
  }
  if (!redeemRequestId.value) {
    redeemRequestId.value = generateRedeemRequestId()
  }
  showResetConfirm.value = true
}

const cancelReset = () => {
  showResetConfirm.value = false
  redeemRequestId.value = null
}

const generateRedeemRequestId = (): string => {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

const confirmReset = async () => {
  showResetConfirm.value = false
  if (resetting.value) return
  if (!canReset.value) {
    error.value = t('admin.accounts.openaiQuotaReset.noCreditsAvailable')
    return
  }
	if (!redeemRequestId.value) return
  resetting.value = true
  error.value = null
  resetMessage.value = null
  try {
    const result: OpenAIQuotaResetResult = await resetOpenAIQuota(props.account.id, {
      confirm: true,
      redeem_request_id: redeemRequestId.value
    })
    // Refresh the reset-credit count so the badge reflects the consumed credit.
    // handleQuery clears resetMessage on entry, so the success toast is set
    // AFTER it resolves.
    await handleQuery()
    resetMessage.value = t('admin.accounts.openaiQuotaReset.resetSuccess', {
      windows: result.windows_reset
    })
    redeemRequestId.value = null
  } catch (e) {
    error.value = extractErrorMessage(e)
  } finally {
    resetting.value = false
  }
}

watch(
  () => props.account.id,
  () => {
    // Account row may be reused across paginated lists; reset local state.
    data.value = null
    error.value = null
    resetMessage.value = null
    loading.value = false
    resetting.value = false
    showResetConfirm.value = false
    showResetCreditDetails.value = false
    redeemRequestId.value = null
  }
)
</script>
