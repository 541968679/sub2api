<template>
  <section class="space-y-4" data-test="stability-usage-panel">
    <div class="flex flex-wrap items-center gap-3">
      <DateRangePicker
        v-model:start-date="startDate"
        v-model:end-date="endDate"
        @change="onDateRangeChange"
      />
      <div class="w-28">
        <Select v-model="granularity" :options="granularityOptions" @change="load" />
      </div>
      <p class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.accounts.stability.usageRangeHint') }}
      </p>
    </div>

    <div class="grid gap-3 sm:grid-cols-2">
      <article
        class="rounded-xl border border-gray-200 bg-white px-4 py-3 dark:border-dark-600 dark:bg-dark-800"
        data-test="stability-cache-request-rate"
      >
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.stability.usageCacheRequestRate') }}
        </p>
        <p
          class="mt-1 font-mono text-2xl font-semibold text-gray-900 dark:text-white"
          data-test="stability-cache-request-rate-value"
        >
          {{ formatCachePercent(requestRate) }}
        </p>
        <p class="mt-1 text-sm text-gray-700 dark:text-gray-200" data-test="stability-cache-request-rate-fraction">
          {{ requestFraction ?? '—' }}
          <span v-if="requestFraction">{{ t('admin.accounts.stability.usageCacheRequestRateUnit') }}</span>
        </p>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.stability.usageCacheRequestRateHint') }}
        </p>
      </article>
      <article
        class="rounded-xl border border-gray-200 bg-white px-4 py-3 dark:border-dark-600 dark:bg-dark-800"
        data-test="stability-cache-token-ratio"
      >
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.stability.usageCacheTokenRatio') }}
        </p>
        <p
          class="mt-1 font-mono text-2xl font-semibold text-gray-900 dark:text-white"
          data-test="stability-cache-token-ratio-value"
        >
          {{ formatCachePercent(tokenRatio) }}
        </p>
        <p class="mt-1 text-sm text-gray-700 dark:text-gray-200" data-test="stability-cache-token-ratio-fraction">
          {{ tokenFraction ?? '—' }}
          <span v-if="tokenFraction">{{ t('admin.accounts.stability.usageCacheTokenRatioUnit') }}</span>
        </p>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.stability.usageCacheTokenRatioHint') }}
        </p>
      </article>
    </div>

    <div class="grid items-start gap-4 lg:grid-cols-[11rem_minmax(0,1fr)]">
      <div class="grid grid-cols-3 gap-2 lg:grid-cols-1">
        <div
          v-for="card in costCards"
          :key="card.key"
          class="rounded-xl border border-gray-200 bg-white px-3 py-3 dark:border-dark-600 dark:bg-dark-800"
          :data-test="card.testId"
        >
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ card.label }}</p>
          <p class="mt-1 font-mono text-lg font-semibold text-gray-900 dark:text-white">
            ${{ card.value }}
          </p>
        </div>
      </div>
      <AccountStabilityUsageChart :trend-data="trend" :loading="loading" height-class="h-80" />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminUsageStatsResponse } from '@/api/admin/usage'
import AccountStabilityUsageChart from '@/components/account/AccountStabilityUsageChart.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import { useAppStore } from '@/stores/app'
import type { TrendDataPoint } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import { cacheReadTokenPercent, cacheRequestHitPercent, formatCachePercent, formatRateFraction } from '@/utils/cacheReadRates'

const props = defineProps<{
  accountId: number
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const trend = ref<TrendDataPoint[]>([])
const stats = ref<AdminUsageStatsResponse | null>(null)
const granularity = ref<'day' | 'hour'>('hour')

function formatLocalDate(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const today = formatLocalDate(new Date())
const startDate = ref(today)
const endDate = ref(today)

const granularityOptions = computed(() => [
  { value: 'day', label: t('admin.dashboard.day') },
  { value: 'hour', label: t('admin.dashboard.hour') }
])

function formatCost(value: number | undefined | null): string {
  const numeric = Number(value)
  const safe = Number.isFinite(numeric) ? numeric : 0
  if (safe >= 1000) return (safe / 1000).toFixed(2) + 'K'
  if (safe >= 1) return safe.toFixed(2)
  if (safe >= 0.01) return safe.toFixed(3)
  return safe.toFixed(4)
}

const requestRate = computed(() =>
  cacheRequestHitPercent(stats.value?.cache_hit_requests, stats.value?.total_requests)
)
const tokenRatio = computed(() =>
  cacheReadTokenPercent(stats.value?.total_cache_read_tokens, stats.value?.total_tokens)
)
const requestFraction = computed(() =>
  formatRateFraction(stats.value?.cache_hit_requests, stats.value?.total_requests)
)
const tokenFraction = computed(() =>
  formatRateFraction(stats.value?.total_cache_read_tokens, stats.value?.total_tokens)
)

const costCards = computed(() => [
  {
    key: 'actual',
    testId: 'stability-usage-actual',
    label: t('admin.accounts.stability.usageActual'),
    value: formatCost(stats.value?.total_actual_cost)
  },
  {
    key: 'standard',
    testId: 'stability-usage-standard',
    label: t('admin.accounts.stability.usageStandard'),
    value: formatCost(stats.value?.total_cost)
  },
  {
    key: 'account',
    testId: 'stability-usage-account',
    label: t('admin.accounts.stability.usageAccount'),
    value: formatCost(stats.value?.total_account_cost)
  }
])

let loadSeq = 0

async function load() {
  const seq = ++loadSeq
  loading.value = true
  try {
    const [trendRes, statsRes] = await Promise.all([
      adminAPI.dashboard.getUsageTrend({
        account_id: props.accountId,
        start_date: startDate.value,
        end_date: endDate.value,
        granularity: granularity.value
      }),
      adminAPI.usage.getStats({
        account_id: props.accountId,
        start_date: startDate.value,
        end_date: endDate.value
      })
    ])
    if (seq !== loadSeq) return
    trend.value = trendRes.trend ?? []
    stats.value = statsRes
  } catch (error: unknown) {
    if (seq !== loadSeq) return
    trend.value = []
    stats.value = null
    appStore.showError(extractApiErrorMessage(error, t('admin.accounts.stability.usageLoadFailed')))
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

function onDateRangeChange(range: { startDate: string; endDate: string }) {
  const start = new Date(range.startDate)
  const end = new Date(range.endDate)
  const days = Math.ceil((end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24))
  granularity.value = days <= 1 ? 'hour' : 'day'
  void load()
}

watch(
  () => props.accountId,
  () => {
    void load()
  },
  { immediate: true }
)
</script>
