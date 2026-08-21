<template>
  <BaseDialog
    :show="show"
    :title="dialogTitle"
    width="extra-wide"
    @close="emit('close')"
  >
    <div v-if="loading" class="flex h-72 items-center justify-center">
      <LoadingSpinner />
    </div>

    <div v-else class="space-y-5" data-test="user-quality-dialog">
      <section
        data-test="stability-error-calibers"
        class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-600 dark:bg-dark-800"
      >
        <h4 class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.users.quality.failoverTitle') }}
        </h4>
        <div class="mt-2 grid gap-2 sm:grid-cols-2">
          <div>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.stability.failoverTerminal') }}
            </p>
            <p class="font-mono text-sm font-medium text-gray-900 dark:text-white" data-test="stability-terminal-rate">
              {{ terminalRateDisplay }}
            </p>
          </div>
          <div>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.stability.failoverInclusive') }}
            </p>
            <p class="font-mono text-sm font-medium text-gray-900 dark:text-white" data-test="stability-failover-rate">
              {{ failoverRateDisplay }}
            </p>
          </div>
        </div>
        <p class="mt-1 text-xs text-gray-600 dark:text-gray-300" data-test="stability-caliber-samples">
          {{
            t('admin.accounts.stability.failoverSamples', {
              terminal: liveQualityStats?.terminal_error_count ?? liveQualityStats?.error_count ?? 0,
              failover: liveQualityStats?.failover_error_count ?? 0
            })
          }}
        </p>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400" data-test="stability-live-window-counts">
          {{
            t('admin.accounts.quality.windowCounts', {
              ttft: liveQualityStats?.ttft_samples ?? 0,
              ok: qualityRateWindowK(liveQualityStats),
              n: liveWindowN
            })
          }}
        </p>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400" data-test="user-quality-window-scope">
          {{ t('admin.users.quality.windowScope', { n: liveWindowN }) }}
        </p>
      </section>

      <section
        data-test="stability-bridge-rate"
        class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-600 dark:bg-dark-800"
      >
        <div class="flex flex-wrap items-baseline justify-between gap-2">
          <h4 class="text-sm font-medium text-gray-900 dark:text-white">
            {{ t('admin.accounts.stability.bridgeTitle') }}
          </h4>
          <span
            class="font-mono text-sm font-medium"
            :class="bridgeToneClass"
            data-test="stability-bridge-rate-value"
          >
            {{ bridgeRateDisplay }}
          </span>
        </div>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.users.quality.bridgeHint') }}
        </p>
        <p
          v-if="hasBridgeErrorRate"
          class="mt-1 text-xs text-gray-600 dark:text-gray-300"
          data-test="stability-bridge-samples"
        >
          {{
            t('admin.accounts.stability.bridgeSamples', {
              success: liveQualityStats?.bridge_success_count ?? 0,
              error: liveQualityStats?.bridge_error_count ?? 0
            })
          }}
        </p>
        <p
          v-else
          class="mt-1 text-xs text-gray-500 dark:text-gray-400"
          data-test="stability-bridge-empty"
        >
          {{ t('admin.accounts.stability.bridgeEmpty') }}
        </p>
      </section>

      <section>
        <div class="mb-2 flex flex-wrap items-start justify-between gap-3">
          <h4 class="text-sm font-medium text-gray-900 dark:text-white">
            {{ t('admin.accounts.stability.chartTitle') }}
          </h4>
          <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
            <input
              type="checkbox"
              class="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              data-test="stability-show-p95"
              :checked="showP95"
              @change="setShowP95(($event.target as HTMLInputElement).checked)"
            />
            {{ t('admin.accounts.stability.showP95') }}
          </label>
        </div>
        <p class="mb-3 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.users.quality.chartHint') }}
        </p>
        <div v-if="chartData" data-test="stability-chart" class="h-72">
          <Line :data="chartData" :options="chartOptions" />
        </div>
        <EmptyState
          v-else
          data-test="stability-empty"
          :title="t('admin.accounts.stability.noData')"
          :description="t('admin.users.quality.noDataHint')"
        />
        <p
          v-if="ttftAxis.clipped"
          data-test="stability-p95-clipped"
          class="mt-2 text-xs text-amber-700 dark:text-amber-300"
        >
          {{ t('admin.accounts.stability.p95ClippedHint') }}
        </p>
        <p
          v-if="samplesSummary"
          class="mt-2 text-xs text-gray-500 dark:text-gray-400"
        >
          {{ samplesSummary }}
        </p>
      </section>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler,
  type ChartData,
  type ChartDataset
} from 'chart.js'
import { Line } from 'vue-chartjs'
import { adminAPI } from '@/api/admin'
import type { AccountQualityHistoryItem, AccountQualityStats } from '@/api/admin/accounts'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  formatQualityBridgeErrorRate,
  formatQualityFailoverErrorRate,
  formatQualityTerminalErrorRate,
  hasDisplayableBridgeErrorRate
} from '@/utils/accountQualityStats'
import {
  ACCOUNT_QUALITY_WINDOW_N_DEFAULT,
  qualityRateWindowK,
  resolveAccountQualityWindowN
} from '@/utils/accountQualityWindowN'
import {
  clampSeriesToMax,
  computeStabilityTtftAxis,
  readShowP95Preference,
  writeShowP95Preference
} from '@/utils/accountStabilityChart'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const props = defineProps<{
  show: boolean
  userId: number | null
  title?: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const historyItems = ref<AccountQualityHistoryItem[]>([])
const liveQualityStats = ref<AccountQualityStats | null>(null)
const siteWindowN = ref(ACCOUNT_QUALITY_WINDOW_N_DEFAULT)
const showP95 = ref(readShowP95Preference())

const dialogTitle = computed(() =>
  t('admin.users.quality.title', { name: props.title || '—' })
)

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))

const chartTheme = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  mutedText: isDarkMode.value ? '#9ca3af' : '#6b7280',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb'
}))

const p50Values = computed(() => historyItems.value.map((point) => point.p50_ttft_ms ?? null))
const p95Values = computed(() => historyItems.value.map((point) => point.p95_ttft_ms ?? null))

const ttftAxis = computed(() =>
  computeStabilityTtftAxis({
    p50Values: p50Values.value,
    p95Values: p95Values.value,
    showP95: showP95.value
  })
)

type StabilityLineDataset = ChartDataset<'line'> & {
  rawMs?: Array<number | null>
}

const chartData = computed<ChartData<'line'> | null>(() => {
  if (!historyItems.value.length) return null
  const datasets: StabilityLineDataset[] = [
    {
      label: t('admin.accounts.stability.p50Series'),
      data: p50Values.value,
      borderColor: '#2563eb',
      backgroundColor: '#2563eb1f',
      pointRadius: 2,
      borderWidth: 2,
      tension: 0.25,
      spanGaps: true,
      yAxisID: 'yMs'
    }
  ]
  if (showP95.value) {
    datasets.push({
      label: t('admin.accounts.stability.p95Series'),
      data: clampSeriesToMax(p95Values.value, ttftAxis.value.max),
      rawMs: p95Values.value,
      borderColor: '#7c3aed',
      backgroundColor: '#7c3aed14',
      pointRadius: 2,
      borderWidth: 2,
      tension: 0.25,
      spanGaps: true,
      yAxisID: 'yMs'
    })
  }
  datasets.push({
    label: t('admin.accounts.stability.successRateSeries'),
    data: historyItems.value.map((point) =>
      point.success_rate == null ? null : Math.round(point.success_rate * 1000) / 10
    ),
    borderColor: '#059669',
    backgroundColor: '#05966914',
    pointRadius: 2,
    borderWidth: 2,
    tension: 0.25,
    spanGaps: true,
    yAxisID: 'yRate'
  })
  return {
    labels: historyItems.value.map((point) => formatCapturedAt(point.captured_at)),
    datasets
  }
})

const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: {
        color: chartTheme.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        boxWidth: 8,
        boxHeight: 8,
        padding: 12,
        font: { size: 11 }
      }
    },
    tooltip: {
      callbacks: {
        label(ctx: {
          dataset: { label?: string; yAxisID?: string; rawMs?: Array<number | null> }
          dataIndex: number
          parsed: { y: number | null }
        }) {
          const raw = ctx.dataset.rawMs?.[ctx.dataIndex]
          const value = raw ?? ctx.parsed.y
          if (value == null) return ctx.dataset.label ?? ''
          if (ctx.dataset.yAxisID === 'yRate') return `${ctx.dataset.label}: ${value}%`
          return `${ctx.dataset.label}: ${value}ms`
        }
      }
    }
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: {
        color: chartTheme.value.mutedText,
        maxRotation: 0,
        autoSkip: true,
        font: { size: 10 }
      }
    },
    yMs: {
      position: 'left' as const,
      beginAtZero: true,
      ...(ttftAxis.value.max != null ? { max: ttftAxis.value.max } : {}),
      grid: { color: chartTheme.value.grid },
      ticks: {
        color: chartTheme.value.text,
        font: { size: 10 },
        callback: (value: string | number) => `${value}ms`
      }
    },
    yRate: {
      position: 'right' as const,
      beginAtZero: true,
      suggestedMax: 100,
      grid: { drawOnChartArea: false },
      ticks: {
        color: '#059669',
        font: { size: 10 },
        callback: (value: string | number) => `${value}%`
      }
    }
  }
}))

function setShowP95(value: boolean) {
  showP95.value = value
  writeShowP95Preference(value)
}

const liveWindowN = computed(() =>
  resolveAccountQualityWindowN({
    ...liveQualityStats.value,
    account_quality_window_n:
      liveQualityStats.value?.account_quality_window_n ?? siteWindowN.value
  })
)

const samplesSummary = computed(() => {
  const last = historyItems.value[historyItems.value.length - 1]
  if (!last) return ''
  const n = resolveAccountQualityWindowN({
    ...last,
    account_quality_window_n: last.account_quality_window_n ?? liveWindowN.value
  })
  return t('admin.accounts.stability.samplesSummary', {
    success: last.success_count ?? 0,
    error: last.error_count ?? 0,
    ok: qualityRateWindowK(last),
    ttft: last.ttft_samples ?? 0,
    n
  })
})

const hasBridgeErrorRate = computed(() => hasDisplayableBridgeErrorRate(liveQualityStats.value))
const bridgeRateDisplay = computed(() => formatQualityBridgeErrorRate(liveQualityStats.value) ?? '—')
const terminalRateDisplay = computed(() => formatQualityTerminalErrorRate(liveQualityStats.value) ?? '—')
const failoverRateDisplay = computed(() => formatQualityFailoverErrorRate(liveQualityStats.value) ?? '—')

const bridgeToneClass = computed(() => {
  if (!hasBridgeErrorRate.value) return 'text-gray-500 dark:text-gray-400'
  const rate = liveQualityStats.value?.bridge_error_rate
  if (rate == null) return 'text-gray-500 dark:text-gray-400'
  if (rate >= 0.1) return 'text-red-600 dark:text-red-400'
  if (rate >= 0.05) return 'text-amber-600 dark:text-amber-400'
  return 'text-gray-700 dark:text-gray-300'
})

function formatCapturedAt(raw: string): string {
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  return `${month}-${day} ${hour}:${minute}`
}

function isCanceled(error: unknown): boolean {
  const info = error as { name?: string; code?: string }
  return info?.name === 'AbortError' || info?.name === 'CanceledError' || info?.code === 'ERR_CANCELED'
}

async function load() {
  if (!props.userId) return
  loading.value = true
  historyItems.value = []
  liveQualityStats.value = null
  try {
    const [history, qualityBatch, template] = await Promise.all([
      adminAPI.users.getQualityHistory(props.userId),
      adminAPI.users.getBatchQualityStats([props.userId]),
      adminAPI.settings?.getQualityHardCloseSettings
        ? adminAPI.settings.getQualityHardCloseSettings().catch(() => null)
        : Promise.resolve(null)
    ])
    historyItems.value = history.items ?? []
    liveQualityStats.value = qualityBatch.stats?.[String(props.userId)] ?? null
    if (template) {
      siteWindowN.value = resolveAccountQualityWindowN(template)
    }
  } catch (error: unknown) {
    if (isCanceled(error)) return
    appStore.showError(extractApiErrorMessage(error, t('admin.users.quality.loadFailed')))
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.show, props.userId] as const,
  ([open]) => {
    if (open && props.userId) {
      void load()
    }
  },
  { immediate: true }
)
</script>
