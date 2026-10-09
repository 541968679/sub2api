<template>
  <div class="card p-4">
    <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
      {{ t('admin.accounts.stability.usageChartTitle') }}
    </h3>
    <div v-if="loading" class="flex items-center justify-center" :class="heightClass">
      <LoadingSpinner />
    </div>
    <div v-else-if="trendData.length > 0 && chartData" :class="heightClass">
      <Line :data="chartData" :options="lineOptions" />
    </div>
    <div
      v-else
      class="flex items-center justify-center text-sm text-gray-500 dark:text-gray-400"
      :class="heightClass"
    >
      {{ t('admin.dashboard.noDataAvailable') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { TrendDataPoint } from '@/types'
import {
  buildStabilityCacheRateSeries,
  cacheReadTokenPercent,
  cacheRequestHitPercent,
  formatCachePercent,
  formatRateFraction
} from '@/utils/cacheReadRates'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend, Filler)

const { t } = useI18n()

const props = withDefaults(
  defineProps<{
    trendData: TrendDataPoint[]
    loading?: boolean
    heightClass?: string
  }>(),
  { heightClass: 'h-80' }
)

type RateKind = 'request' | 'token'

interface TrendDataset {
  label: string
  data: Array<number | null>
  borderColor: string
  backgroundColor: string
  fill: boolean
  tension: number
  borderWidth: number
  pointRadius: number
  pointHoverRadius: number
  order: number
  borderDash?: number[]
  yAxisID?: string
  spanGaps?: boolean
  rateKind?: RateKind
}

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))

const chartColors = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb',
  input: '#3b82f6',
  output: '#10b981',
  cacheCreation: '#f59e0b',
  requestRate: '#8b5cf6',
  tokenRatio: '#06b6d4'
}))

function line(partial: Pick<TrendDataset, 'label' | 'data' | 'borderColor'> & Partial<TrendDataset>): TrendDataset {
  return {
    backgroundColor: 'transparent',
    fill: false,
    tension: 0.3,
    borderWidth: 2,
    pointRadius: 2,
    pointHoverRadius: 3,
    order: 2,
    ...partial
  }
}

const chartData = computed(() => {
  if (!props.trendData?.length) return null
  const series = buildStabilityCacheRateSeries(props.trendData)
  const colors = chartColors.value
  const datasets: TrendDataset[] = [
    line({
      label: t('admin.accounts.stability.usageCacheRequestRate'),
      data: series.requestRate,
      borderColor: colors.requestRate,
      borderDash: [6, 4],
      borderWidth: 2.5,
      order: 0,
      yAxisID: 'yPercent',
      spanGaps: true,
      rateKind: 'request'
    }),
    line({
      label: t('admin.accounts.stability.usageCacheTokenRatio'),
      data: series.tokenRatio,
      borderColor: colors.tokenRatio,
      borderWidth: 2.5,
      order: 0,
      yAxisID: 'yPercent',
      spanGaps: true,
      rateKind: 'token'
    }),
    line({
      label: t('admin.accounts.stability.usageCacheCreation'),
      data: series.cacheCreation,
      borderColor: colors.cacheCreation,
      borderWidth: 2.5,
      order: 1
    }),
    line({
      label: t('admin.accounts.stability.usageInput'),
      data: series.input,
      borderColor: colors.input
    }),
    line({
      label: t('admin.accounts.stability.usageOutput'),
      data: series.output,
      borderColor: colors.output
    })
  ]
  return {
    labels: props.trendData.map((point) => point.date),
    datasets
  }
})

function formatTokens(value: number): string {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(2)}B`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(2)}K`
  return value.toLocaleString()
}

function rateTooltip(kind: RateKind, point: TrendDataPoint, label: string): string {
  if (kind === 'request') {
    const percent = cacheRequestHitPercent(point.cache_hit_requests, point.requests)
    const fraction = formatRateFraction(point.cache_hit_requests, point.requests)
    return `${label}: ${formatCachePercent(percent)} (${fraction ?? '—'})`
  }
  const percent = cacheReadTokenPercent(point.cache_read_tokens, point.total_tokens)
  const fraction = formatRateFraction(point.cache_read_tokens, point.total_tokens)
  return `${label}: ${formatCachePercent(percent)} (${fraction ?? '—'})`
}

const lineOptions = computed(() => ({
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
        color: chartColors.value.text,
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
        label: (context: {
          dataset: { label?: string; yAxisID?: string; rateKind?: RateKind }
          dataIndex: number
          raw: unknown
        }) => {
          const point = props.trendData[context.dataIndex]
          const label = context.dataset.label ?? ''
          if (point && context.dataset.rateKind) {
            return rateTooltip(context.dataset.rateKind, point, label)
          }
          const raw = typeof context.raw === 'number' ? context.raw : 0
          return `${label}: ${formatTokens(raw)}`
        }
      }
    }
  },
  scales: {
    x: {
      grid: { color: chartColors.value.grid },
      ticks: { color: chartColors.value.text, font: { size: 10 } }
    },
    y: {
      grid: { color: chartColors.value.grid },
      ticks: {
        color: chartColors.value.text,
        font: { size: 10 },
        callback: (value: string | number) => formatTokens(Number(value))
      }
    },
    yPercent: {
      position: 'right' as const,
      min: 0,
      max: 100,
      grid: { drawOnChartArea: false },
      ticks: {
        color: chartColors.value.requestRate,
        font: { size: 10 },
        callback: (value: string | number) => `${value}%`
      }
    }
  }
}))
</script>
