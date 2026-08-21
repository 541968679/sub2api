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

    <div v-else class="space-y-5">
      <div
        v-if="showPauseBanner"
        data-test="quality-pause-banner"
        class="flex flex-wrap items-start justify-between gap-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-200"
      >
        <div class="min-w-0 space-y-1">
          <p>{{ t('admin.accounts.stability.pauseBanner', { time: pauseResumeTime }) }}</p>
          <p class="text-xs text-amber-700 dark:text-amber-300">
            {{ t('admin.accounts.stability.resumeHint') }}
          </p>
        </div>
        <button
          type="button"
          class="btn btn-secondary btn-sm shrink-0"
          data-test="stability-resume-now"
          :disabled="resumeBusy || !account"
          @click="resumeNow"
        >
          {{ t('admin.accounts.stability.resumeNow') }}
        </button>
      </div>

      <p
        v-if="globalEnabled === false"
        data-test="global-disabled-hint"
        class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300"
      >
        {{ t('admin.accounts.stability.globalDisabledHint') }}
      </p>

      <section
        data-test="stability-error-calibers"
        class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-600 dark:bg-dark-800"
      >
        <h4 class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.accounts.stability.failoverTitle') }}
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
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.stability.failoverSchedule') }}:
          {{ scheduleUseFailover ? t('admin.accounts.stability.failoverInclusive') : t('admin.accounts.stability.failoverTerminal') }}
        </p>
        <div class="mt-3 flex items-start justify-between gap-3 border-t border-gray-200 pt-3 dark:border-dark-600">
          <div>
            <p class="text-sm font-medium text-gray-900 dark:text-white">
              {{ t('admin.accounts.stability.failoverToggle') }}
            </p>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.stability.failoverToggleHint') }}
            </p>
          </div>
          <Toggle
            :model-value="scheduleUseFailover"
            data-test="stability-failover-toggle"
            @update:model-value="saveFailoverToggle"
          />
        </div>
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
          {{ t('admin.accounts.stability.bridgeHint') }}
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
          {{ t('admin.accounts.stability.chartHint') }}
        </p>
        <div v-if="chartData" data-test="stability-chart" class="h-72">
          <Line :data="chartData" :options="chartOptions" />
        </div>
        <EmptyState
          v-else
          data-test="stability-empty"
          :title="t('admin.accounts.stability.noData')"
          :description="t('admin.accounts.stability.noDataHint')"
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

      <section class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h4 class="text-sm font-medium text-gray-900 dark:text-white">
              {{ t('admin.accounts.stability.formTitle') }}
            </h4>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.stability.templateHint') }}
            </p>
          </div>
          <div class="flex flex-wrap gap-2">
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              data-test="stability-apply-template"
              :disabled="templateBusy || !account"
              @click="applyTemplate"
            >
              {{ t('admin.accounts.stability.applyTemplate') }}
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              data-test="stability-save-template"
              :disabled="templateBusy || !account"
              @click="saveTemplate"
            >
              {{ t('admin.accounts.stability.saveTemplate') }}
            </button>
          </div>
        </div>

        <div class="flex items-center justify-between">
          <div>
            <label class="font-medium text-gray-900 dark:text-white">
              {{ t('admin.accounts.stability.enabled') }}
            </label>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.stability.enabledHint') }}
            </p>
          </div>
          <Toggle v-model="form.enabled" data-test="stability-hard-close-enabled" />
        </div>

        <div
          data-test="stability-override-fields"
          class="grid gap-4 sm:grid-cols-2"
        >
          <div>
            <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.accounts.stability.maxP50') }}
            </label>
            <input
              v-model="form.max_p50_ttft_ms"
              type="number"
              min="1"
              class="input w-full"
              :placeholder="resolvedPlaceholder(resolved?.max_p50_ttft_ms)"
            />
          </div>
          <div>
            <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.accounts.stability.minSuccessRate') }}
            </label>
            <input
              v-model="form.min_success_rate_percent"
              type="number"
              min="0.1"
              max="100"
              step="0.1"
              class="input w-full"
              :placeholder="resolvedPercentPlaceholder(resolved?.min_success_rate)"
            />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.stability.minSuccessRateHint') }}
            </p>
          </div>
          <div>
            <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.accounts.stability.pauseMinutes') }}
            </label>
            <input
              v-model="form.pause_minutes"
              type="number"
              min="1"
              max="1440"
              class="input w-full"
              :placeholder="resolvedPlaceholder(resolved?.pause_minutes)"
            />
          </div>
          <div>
            <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.accounts.stability.windowN') }}
            </label>
            <input
              :value="siteWindowN"
              type="number"
              min="1"
              max="100"
              class="input w-full"
              data-test="stability-window-n"
              disabled
              readonly
            />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.stability.windowNHint') }}
            </p>
          </div>
          <div>
            <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.accounts.stability.condition') }}
            </label>
            <select v-model="form.condition" class="input w-full">
              <option value="or">{{ t('admin.accounts.stability.conditionOr') }}</option>
              <option value="and">{{ t('admin.accounts.stability.conditionAnd') }}</option>
            </select>
          </div>
        </div>
      </section>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" @click="emit('close')">
        {{ t('common.cancel') }}
      </button>
      <button
        type="button"
        class="btn btn-primary"
        data-test="stability-save"
        :disabled="saving || !account"
        @click="save"
      >
        {{ saving ? t('common.saving') : t('common.save') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
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
import type {
  AccountQualityHardCloseOverlay,
  AccountQualityHardCloseResolved,
  AccountQualityHistoryItem,
  AccountQualityStats,
  QualityHardCloseCondition
} from '@/api/admin/accounts'
import type { QualityHardCloseSettings } from '@/api/admin/settings'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import type { Account } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  isQualityHardClosePaused,
  optionalNumber,
  percentToSuccessRate,
  successRateToPercent
} from '@/utils/accountQualityHardClose'
import {
  ACCOUNT_QUALITY_WINDOW_N_DEFAULT,
  echoAccountQualityWindowN,
  qualityRateWindowK,
  resolveAccountQualityWindowN
} from '@/utils/accountQualityWindowN'
import {
  formatQualityBridgeErrorRate,
  formatQualityFailoverErrorRate,
  formatQualityTerminalErrorRate,
  hasDisplayableBridgeErrorRate
} from '@/utils/accountQualityStats'
import {
  clampSeriesToMax,
  computeStabilityTtftAxis,
  readShowP95Preference,
  writeShowP95Preference
} from '@/utils/accountStabilityChart'
import { formatDateTime } from '@/utils/format'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const props = defineProps<{
  show: boolean
  account: Account | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'recovered', account: Account): void
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const templateBusy = ref(false)
const resumeBusy = ref(false)
const historyItems = ref<AccountQualityHistoryItem[]>([])
const liveQualityStats = ref<AccountQualityStats | null>(null)
const scheduleUseFailover = ref(false)
const failoverToggleBusy = ref(false)
const globalEnabled = ref<boolean | null>(null)
const resolved = ref<AccountQualityHardCloseResolved | null>(null)
const showP95 = ref(readShowP95Preference())

const form = reactive({
  enabled: false,
  max_p50_ttft_ms: '' as number | string,
  min_success_rate_percent: '' as number | string,
  pause_minutes: '' as number | string,
  condition: 'or' as QualityHardCloseCondition
})
const globalWindowN = ref(ACCOUNT_QUALITY_WINDOW_N_DEFAULT)

const dialogTitle = computed(() =>
  t('admin.accounts.stability.title', { name: props.account?.name || '—' })
)

const showPauseBanner = computed(() =>
  isQualityHardClosePaused(props.account?.temp_unschedulable_until, props.account?.temp_unschedulable_reason)
)

const pauseResumeTime = computed(() => formatDateTime(props.account?.temp_unschedulable_until))

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
        label(ctx: { dataset: { label?: string; yAxisID?: string; rawMs?: Array<number | null> }; dataIndex: number; parsed: { y: number | null } }) {
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

const resolvedWindowN = computed(() =>
  resolved.value ? resolveAccountQualityWindowN(resolved.value) : undefined
)

const siteWindowN = computed(() =>
  resolvedWindowN.value ?? globalWindowN.value
)

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
    account_quality_window_n: last.account_quality_window_n ?? resolvedWindowN.value
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

function resolvedPlaceholder(value: number | null | undefined): string {
  return value == null ? '' : String(value)
}

function resolvedPercentPlaceholder(rate: number | null | undefined): string {
  const percent = successRateToPercent(rate)
  return percent == null ? '' : String(percent)
}

function resetForm() {
  form.enabled = false
  form.max_p50_ttft_ms = ''
  form.min_success_rate_percent = ''
  form.pause_minutes = ''
  form.condition = 'or'
}

function overlayHasOwnThresholds(overlay: AccountQualityHardCloseOverlay): boolean {
  return !overlay.use_global
    || overlay.max_p50_ttft_ms != null
    || overlay.min_success_rate != null
    || overlay.pause_minutes != null
    || overlay.condition != null
}

function applyThresholds(source: {
  max_p50_ttft_ms?: number | null
  min_success_rate?: number | null
  pause_minutes?: number | null
  condition?: QualityHardCloseCondition | string | null
}) {
  form.max_p50_ttft_ms = source.max_p50_ttft_ms ?? ''
  form.min_success_rate_percent = successRateToPercent(source.min_success_rate) ?? ''
  form.pause_minutes = source.pause_minutes ?? ''
  form.condition = source.condition === 'and' ? 'and' : 'or'
}

function applyOverlay(overlay: AccountQualityHardCloseOverlay, resolvedView: AccountQualityHardCloseResolved | null) {
  form.enabled = overlay.enabled
  if (overlayHasOwnThresholds(overlay)) {
    applyThresholds(overlay)
    return
  }
  if (resolvedView) {
    applyThresholds(resolvedView)
    return
  }
  resetForm()
  form.enabled = overlay.enabled
}

function buildOverlayPayload(): AccountQualityHardCloseOverlay {
  const condition = form.condition === 'and' ? 'and' : 'or'
  return {
    enabled: form.enabled,
    use_global: false,
    max_p50_ttft_ms: optionalNumber(form.max_p50_ttft_ms),
    min_success_rate: percentToSuccessRate(form.min_success_rate_percent),
    pause_minutes: optionalNumber(form.pause_minutes),
    account_quality_window_n: null,
    min_success_samples: null,
    min_ttft_samples: null,
    condition
  }
}

function buildTemplatePayload(current: QualityHardCloseSettings): QualityHardCloseSettings {
  return {
    enabled: current.enabled,
    max_p50_ttft_ms: optionalNumber(form.max_p50_ttft_ms),
    min_success_rate: percentToSuccessRate(form.min_success_rate_percent),
    pause_minutes: optionalNumber(form.pause_minutes) ?? current.pause_minutes,
    ...echoAccountQualityWindowN(resolveAccountQualityWindowN(current)),
    condition: form.condition === 'and' ? 'and' : 'or',
    schedule_use_failover_error_rate: scheduleUseFailover.value
  }
}

async function load() {
  if (!props.account) return
  loading.value = true
  historyItems.value = []
  liveQualityStats.value = null
  globalEnabled.value = null
  resolved.value = null
  resetForm()
  try {
    const [history, hardClose, qualityBatch, template] = await Promise.all([
      adminAPI.accounts.getQualityHistory(props.account.id),
      adminAPI.accounts.getQualityHardClose(props.account.id),
      adminAPI.accounts.getBatchQualityStats([props.account.id]),
      adminAPI.settings.getQualityHardCloseSettings()
    ])
    historyItems.value = history.items ?? []
    liveQualityStats.value = qualityBatch.stats?.[String(props.account.id)] ?? null
    scheduleUseFailover.value = template.schedule_use_failover_error_rate === true
    globalWindowN.value = resolveAccountQualityWindowN(template)
    globalEnabled.value = hardClose.global_enabled
    resolved.value = hardClose.resolved
    applyOverlay(hardClose.overlay, hardClose.resolved)
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.accounts.stability.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!props.account) return
  saving.value = true
  try {
    const updated = await adminAPI.accounts.updateQualityHardClose(props.account.id, buildOverlayPayload())
    globalEnabled.value = updated.global_enabled
    resolved.value = updated.resolved
    applyOverlay(updated.overlay, updated.resolved)
    appStore.showSuccess(t('admin.accounts.stability.saveSuccess'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.accounts.stability.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function saveFailoverToggle(on: boolean) {
  if (failoverToggleBusy.value) return
  failoverToggleBusy.value = true
  try {
    const current = await adminAPI.settings.getQualityHardCloseSettings()
    const updated = await adminAPI.settings.updateQualityHardCloseSettings({
      ...current,
      schedule_use_failover_error_rate: on
    })
    scheduleUseFailover.value = updated.schedule_use_failover_error_rate === true
    if (props.account) {
      const qualityBatch = await adminAPI.accounts.getBatchQualityStats([props.account.id])
      liveQualityStats.value = qualityBatch.stats?.[String(props.account.id)] ?? null
    }
    appStore.showSuccess(t('admin.settings.qualityHardClose.saved'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.qualityHardClose.saveFailed')))
  } finally {
    failoverToggleBusy.value = false
  }
}

async function applyTemplate() {
  templateBusy.value = true
  try {
    const template = await adminAPI.settings.getQualityHardCloseSettings()
    applyThresholds(template)
    appStore.showSuccess(t('admin.accounts.stability.applyTemplateSuccess'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.accounts.stability.applyTemplateFailed')))
  } finally {
    templateBusy.value = false
  }
}

async function resumeNow() {
  if (!props.account) return
  resumeBusy.value = true
  try {
    const updated = await adminAPI.accounts.recoverState(props.account.id)
    emit('recovered', updated)
    appStore.showSuccess(t('admin.accounts.stability.resumeSuccess'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.accounts.stability.resumeFailed')))
  } finally {
    resumeBusy.value = false
  }
}

async function saveTemplate() {
  templateBusy.value = true
  try {
    const current = await adminAPI.settings.getQualityHardCloseSettings()
    await adminAPI.settings.updateQualityHardCloseSettings(buildTemplatePayload(current))
    appStore.showSuccess(t('admin.accounts.stability.saveTemplateSuccess'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.accounts.stability.saveTemplateFailed')))
  } finally {
    templateBusy.value = false
  }
}

watch(
  () => [props.show, props.account?.id] as const,
  ([open]) => {
    if (open && props.account) {
      void load()
    }
  },
  { immediate: true }
)
</script>
