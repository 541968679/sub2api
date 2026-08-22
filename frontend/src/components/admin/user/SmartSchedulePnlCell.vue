<template>
  <button
    type="button"
    class="w-full min-w-[9rem] rounded-md px-1 py-1 text-left transition-colors hover:bg-gray-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-700"
    data-testid="smart-schedule-pnl-cell"
    :aria-label="t('admin.users.schedulePnl.openTrend')"
    :title="t('admin.users.schedulePnl.openTrend')"
    @click="emit('click')"
  >
    <div v-if="loading && !showBody" class="h-10 w-24 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
    <div v-else-if="!showBody" class="flex flex-col items-start gap-0.5">
      <span class="text-base text-gray-400 dark:text-dark-500">—</span>
      <span class="text-xs font-medium text-primary-600 dark:text-primary-400">
        {{ t('admin.users.schedulePnl.openTrend') }}
      </span>
    </div>
    <div v-else class="flex flex-col gap-1">
      <div
        v-if="isOauth"
        data-testid="smart-schedule-pnl-oauth-7d"
        :title="t('admin.users.schedulePnl.oauthQuotaHint')"
      >
        <UsageProgressBar
          v-if="oauthQuota"
          label="7d"
          :utilization="oauthQuota.utilization"
          :resets-at="oauthQuota.resetsAt"
          color="emerald"
        />
        <span v-else class="text-sm text-gray-400 dark:text-dark-500">—</span>
      </div>
      <div
        v-else
        class="flex flex-col items-start gap-0.5"
        data-testid="smart-schedule-pnl-balance"
      >
        <div class="flex items-center gap-1 text-sm font-semibold leading-5 text-gray-900 dark:text-white">
          <span>
            {{ t('admin.users.schedulePnl.balance') }}
            {{ balanceText }}
          </span>
          <span
            v-if="canProbeBalance"
            role="button"
            tabindex="0"
            class="inline-flex rounded p-0.5 text-gray-400 hover:bg-gray-100 hover:text-primary-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-600 dark:hover:text-primary-400"
            data-testid="smart-schedule-pnl-balance-refresh"
            :aria-label="t('admin.users.schedulePnl.balanceRefresh')"
            :title="t('admin.users.schedulePnl.balanceUpdatedHint')"
            @click.stop="emit('refresh-balance')"
            @keydown.enter.stop.prevent="emit('refresh-balance')"
          >
            <Icon name="refresh" size="xs" :class="balanceRefreshing ? 'animate-spin' : ''" />
          </span>
        </div>
        <span
          v-if="canProbeBalance"
          class="text-[11px] font-normal leading-4 text-gray-400 dark:text-dark-500"
          data-testid="smart-schedule-pnl-balance-updated"
        >
          {{ updatedText }}
        </span>
      </div>
      <template v-if="hasToday">
        <div class="flex flex-wrap items-baseline gap-x-1.5 gap-y-0.5">
          <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.users.schedulePnl.today') }}</span>
          <span class="text-base font-semibold leading-5 text-gray-900 dark:text-white">{{ today.profit }}</span>
          <span
            class="text-sm font-semibold leading-5"
            :class="marginClass"
            data-testid="smart-schedule-pnl-margin"
          >{{ today.margin }}</span>
        </div>
        <div class="text-xs leading-4 text-gray-500 dark:text-gray-400">
          {{ t('admin.users.schedulePnl.revenue') }} {{ today.revenue }}
          · {{ t('admin.users.schedulePnl.cost') }} {{ today.cost }}
        </div>
      </template>
      <span v-else class="text-sm text-gray-400 dark:text-dark-500">—</span>
      <div
        v-if="burnCompare"
        class="text-xs font-semibold leading-4"
        :class="burnCompare.status === 'match'
          ? 'text-emerald-600 dark:text-emerald-400'
          : 'text-amber-700 dark:text-amber-300'"
        :title="burnHint"
        data-testid="smart-schedule-pnl-burn"
        :data-burn-status="burnCompare.status"
      >
        {{ burnLine }}
      </div>
    </div>
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account, WindowStats } from '@/types'
import type { SchedulePnlSummary } from '@/api/admin/users'
import UsageProgressBar from '@/components/account/UsageProgressBar.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatRelativeTime } from '@/utils/format'
import {
  compareBalanceBurnToCost,
  formatSchedulePnlUsdPlain,
  formatSchedulePnlWindow,
  hasSchedulePnlWindow,
  isOauthAccountType,
  oauthSevenDayQuota,
  pairAccountBalanceUpdatedAt,
  pairAccountBalanceUsd,
  supportsPairBalanceProbe
} from '@/composables/schedulePnl'

const props = withDefaults(
  defineProps<{
    summary?: SchedulePnlSummary | null
    account?: Account | null
    todayStats?: WindowStats | null
    loading?: boolean
    balanceRefreshing?: boolean
  }>(),
  { summary: null, account: null, todayStats: null, loading: false, balanceRefreshing: false }
)

const emit = defineEmits<{ click: []; 'refresh-balance': [] }>()
const { t } = useI18n()

const isOauth = computed(() => isOauthAccountType(props.account))
const oauthQuota = computed(() => oauthSevenDayQuota(props.account))
const hasToday = computed(() => hasSchedulePnlWindow(props.summary?.today))
const canProbeBalance = computed(() => supportsPairBalanceProbe(props.account))
const balanceUsd = computed(() => (isOauth.value ? null : pairAccountBalanceUsd(props.account)))
const showBody = computed(() => hasToday.value || isOauth.value || balanceUsd.value != null || canProbeBalance.value)
const balanceText = computed(() => formatSchedulePnlUsdPlain(balanceUsd.value))
const updatedText = computed(() => {
  const at = pairAccountBalanceUpdatedAt(props.account)
  if (!at) return t('admin.users.schedulePnl.balanceNeverUpdated')
  return formatRelativeTime(at)
})
const today = computed(() => formatSchedulePnlWindow(props.summary?.today))
const marginClass = computed(() => {
  const profit = props.summary?.today?.profit
  if (profit == null || Number.isNaN(profit) || profit === 0) {
    return 'text-gray-600 dark:text-gray-300'
  }
  return profit > 0
    ? 'text-emerald-600 dark:text-emerald-400'
    : 'text-red-600 dark:text-red-400'
})
const burnCompare = computed(() =>
  isOauth.value ? null : compareBalanceBurnToCost(props.account, props.todayStats?.cost)
)
const burnLine = computed(() => {
  const cmp = burnCompare.value
  if (!cmp) return ''
  const balance = formatSchedulePnlUsdPlain(cmp.balanceRate)
  const cost = formatSchedulePnlUsdPlain(cmp.costRate)
  if (cmp.status === 'match') {
    return t('admin.users.schedulePnl.burnMatch', { rate: balance })
  }
  return t('admin.users.schedulePnl.burnMismatch', { balance, cost })
})
const burnHint = computed(() =>
  burnCompare.value?.status === 'mismatch'
    ? t('admin.users.schedulePnl.burnMismatchHint')
    : t('admin.users.schedulePnl.burnMatchHint')
)
</script>
