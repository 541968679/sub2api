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
        class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-200"
      >
        {{ t('admin.accounts.stability.pauseBanner', { time: pauseResumeTime }) }}
      </div>

      <p
        v-if="globalEnabled === false"
        data-test="global-disabled-hint"
        class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300"
      >
        {{ t('admin.accounts.stability.globalDisabledHint') }}
      </p>

      <section>
        <h4 class="mb-2 text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.accounts.stability.chartTitle') }}
        </h4>
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
          v-if="samplesSummary"
          class="mt-2 text-xs text-gray-500 dark:text-gray-400"
        >
          {{ samplesSummary }}
        </p>
      </section>

      <section class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700">
        <h4 class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.accounts.stability.formTitle') }}
        </h4>

        <div class="flex items-center justify-between">
          <div>
            <label class="font-medium text-gray-900 dark:text-white">
              {{ t('admin.accounts.stability.enabled') }}
            </label>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.stability.enabledHint') }}
            </p>
          </div>
          <Toggle v-model="form.enabled" />
        </div>

        <div class="flex items-center justify-between">
          <div>
            <label class="font-medium text-gray-900 dark:text-white">
              {{ t('admin.accounts.stability.useGlobal') }}
            </label>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.stability.useGlobalHint') }}
            </p>
          </div>
          <Toggle v-model="form.use_global" />
        </div>

        <div
          v-if="!form.use_global"
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
              {{ t('admin.accounts.stability.minSuccessSamples') }}
            </label>
            <input
              v-model="form.min_success_samples"
              type="number"
              min="1"
              class="input w-full"
              :placeholder="resolvedPlaceholder(resolved?.min_success_samples)"
            />
          </div>
          <div>
            <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.accounts.stability.minTtftSamples') }}
            </label>
            <input
              v-model="form.min_ttft_samples"
              type="number"
              min="1"
              class="input w-full"
              :placeholder="resolvedPlaceholder(resolved?.min_ttft_samples)"
            />
          </div>
          <div>
            <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.accounts.stability.condition') }}
            </label>
            <select v-model="form.condition" class="input w-full">
              <option value="">{{ t('admin.accounts.stability.conditionInherit') }}</option>
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
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import { adminAPI } from '@/api/admin'
import type {
  AccountQualityHardCloseOverlay,
  AccountQualityHardCloseResolved,
  AccountQualityHistoryItem,
  QualityHardCloseCondition
} from '@/api/admin/accounts'
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
import { formatDateTime } from '@/utils/format'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const props = defineProps<{
  show: boolean
  account: Account | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const historyItems = ref<AccountQualityHistoryItem[]>([])
const globalEnabled = ref<boolean | null>(null)
const resolved = ref<AccountQualityHardCloseResolved | null>(null)

const form = reactive({
  enabled: false,
  use_global: true,
  max_p50_ttft_ms: '' as number | string,
  min_success_rate_percent: '' as number | string,
  pause_minutes: '' as number | string,
  min_success_samples: '' as number | string,
  min_ttft_samples: '' as number | string,
  condition: '' as QualityHardCloseCondition | ''
})

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

const chartData = computed(() => {
  if (!historyItems.value.length) return null
  return {
    labels: historyItems.value.map((point) => formatCapturedAt(point.captured_at)),
    datasets: [
      {
        label: t('admin.accounts.stability.p50Series'),
        data: historyItems.value.map((point) => point.p50_ttft_ms ?? null),
        borderColor: '#2563eb',
        backgroundColor: '#2563eb1f',
        pointRadius: 2,
        borderWidth: 2,
        tension: 0.25,
        spanGaps: true,
        yAxisID: 'yMs'
      },
      {
        label: t('admin.accounts.stability.p95Series'),
        data: historyItems.value.map((point) => point.p95_ttft_ms ?? null),
        borderColor: '#7c3aed',
        backgroundColor: '#7c3aed14',
        pointRadius: 2,
        borderWidth: 2,
        tension: 0.25,
        spanGaps: true,
        yAxisID: 'yMs'
      },
      {
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
      }
    ]
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

const samplesSummary = computed(() => {
  const last = historyItems.value[historyItems.value.length - 1]
  if (!last) return ''
  return t('admin.accounts.stability.samplesSummary', {
    success: last.success_count ?? 0,
    error: last.error_count ?? 0,
    ttft: last.ttft_samples ?? 0
  })
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
  form.use_global = true
  form.max_p50_ttft_ms = ''
  form.min_success_rate_percent = ''
  form.pause_minutes = ''
  form.min_success_samples = ''
  form.min_ttft_samples = ''
  form.condition = ''
}

function applyOverlay(overlay: AccountQualityHardCloseOverlay) {
  form.enabled = overlay.enabled
  form.use_global = overlay.use_global
  form.max_p50_ttft_ms = overlay.max_p50_ttft_ms ?? ''
  form.min_success_rate_percent = successRateToPercent(overlay.min_success_rate) ?? ''
  form.pause_minutes = overlay.pause_minutes ?? ''
  form.min_success_samples = overlay.min_success_samples ?? ''
  form.min_ttft_samples = overlay.min_ttft_samples ?? ''
  form.condition = overlay.condition ?? ''
}

function buildOverlayPayload(): AccountQualityHardCloseOverlay {
  if (form.use_global) {
    return {
      enabled: form.enabled,
      use_global: true,
      max_p50_ttft_ms: null,
      min_success_rate: null,
      pause_minutes: null,
      min_success_samples: null,
      min_ttft_samples: null,
      condition: null
    }
  }
  const condition = form.condition === 'or' || form.condition === 'and' ? form.condition : null
  return {
    enabled: form.enabled,
    use_global: false,
    max_p50_ttft_ms: optionalNumber(form.max_p50_ttft_ms),
    min_success_rate: percentToSuccessRate(form.min_success_rate_percent),
    pause_minutes: optionalNumber(form.pause_minutes),
    min_success_samples: optionalNumber(form.min_success_samples),
    min_ttft_samples: optionalNumber(form.min_ttft_samples),
    condition
  }
}

async function load() {
  if (!props.account) return
  loading.value = true
  historyItems.value = []
  globalEnabled.value = null
  resolved.value = null
  resetForm()
  try {
    const [history, hardClose] = await Promise.all([
      adminAPI.accounts.getQualityHistory(props.account.id),
      adminAPI.accounts.getQualityHardClose(props.account.id)
    ])
    historyItems.value = history.items ?? []
    globalEnabled.value = hardClose.global_enabled
    resolved.value = hardClose.resolved
    applyOverlay(hardClose.overlay)
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
    applyOverlay(updated.overlay)
    appStore.showSuccess(t('admin.accounts.stability.saveSuccess'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.accounts.stability.saveFailed')))
  } finally {
    saving.value = false
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
