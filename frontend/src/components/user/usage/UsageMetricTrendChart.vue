<template>
  <section class="card overflow-hidden">
    <div class="border-b border-gray-200 px-5 py-4 dark:border-dark-700">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <Icon name="trendingUp" size="sm" class="text-primary-600 dark:text-primary-400" />
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('usage.trend.title') }}
            </h2>
          </div>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('usage.trend.subtitle') }}
          </p>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <button
            v-for="metric in selectableMetrics"
            :key="metric.key"
            type="button"
            class="inline-flex h-8 items-center gap-2 rounded-md border px-3 text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50"
            :class="metricButtonClass(metric.key)"
            :disabled="isMetricDisabled(metric.key)"
            @click="toggleMetric(metric.key)"
          >
            <span
              class="h-2 w-2 rounded-full"
              :style="{ backgroundColor: metric.color }"
            />
            {{ metric.label }}
          </button>
        </div>
      </div>

      <p
        v-if="isAtMetricLimit"
        class="mt-2 text-xs text-amber-600 dark:text-amber-400"
      >
        {{ t('usage.trend.maxMetricsHint') }}
      </p>
    </div>

    <div v-if="loading" class="flex h-72 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="chartData" class="h-72 px-4 py-4">
      <Line :data="chartData" :options="chartOptions" />
    </div>
    <div
      v-else
      class="flex h-72 items-center justify-center px-4 text-sm text-gray-500 dark:text-gray-400"
    >
      {{ t('usage.trend.noData') }}
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
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
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import type { TrendDataPoint } from '@/types'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

type MetricKey =
  | 'actual_cost'
  | 'total_tokens'
  | 'requests'
  | 'input_tokens'
  | 'output_tokens'
  | 'cache_creation_tokens'
  | 'cache_read_tokens'
  | 'cost'

interface MetricDefinition {
  key: MetricKey
  label: string
  color: string
  axis: 'volume' | 'cost'
  formatter: (value: number) => string
}

const MAX_METRICS = 4
const REQUIRED_METRICS: MetricKey[] = ['actual_cost', 'total_tokens']

const props = defineProps<{
  trendData: TrendDataPoint[]
  loading?: boolean
}>()

const { t } = useI18n()
const selectedOptionalMetrics = ref<MetricKey[]>(['requests'])

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))

const chartTheme = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  mutedText: isDarkMode.value ? '#9ca3af' : '#6b7280',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb'
}))

const formatTokens = (value: number): string => {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(2)}B`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(2)}K`
  return value.toLocaleString()
}

const formatCost = (value: number): string => {
  if (value >= 1000) return `$${(value / 1000).toFixed(2)}K`
  if (value >= 1) return `$${value.toFixed(2)}`
  if (value >= 0.01) return `$${value.toFixed(3)}`
  return `$${value.toFixed(4)}`
}

const metricDefinitions = computed<MetricDefinition[]>(() => [
  {
    key: 'actual_cost',
    label: t('usage.trend.metrics.actualCost'),
    color: '#16a34a',
    axis: 'cost',
    formatter: formatCost
  },
  {
    key: 'total_tokens',
    label: t('usage.trend.metrics.totalTokens'),
    color: '#d97706',
    axis: 'volume',
    formatter: formatTokens
  },
  {
    key: 'requests',
    label: t('usage.trend.metrics.requests'),
    color: '#2563eb',
    axis: 'volume',
    formatter: (value: number) => value.toLocaleString()
  },
  {
    key: 'input_tokens',
    label: t('usage.trend.metrics.inputTokens'),
    color: '#0891b2',
    axis: 'volume',
    formatter: formatTokens
  },
  {
    key: 'output_tokens',
    label: t('usage.trend.metrics.outputTokens'),
    color: '#7c3aed',
    axis: 'volume',
    formatter: formatTokens
  },
  {
    key: 'cache_creation_tokens',
    label: t('usage.trend.metrics.cacheWriteTokens'),
    color: '#ea580c',
    axis: 'volume',
    formatter: formatTokens
  },
  {
    key: 'cache_read_tokens',
    label: t('usage.trend.metrics.cacheReadTokens'),
    color: '#0d9488',
    axis: 'volume',
    formatter: formatTokens
  },
  {
    key: 'cost',
    label: t('usage.trend.metrics.standardCost'),
    color: '#64748b',
    axis: 'cost',
    formatter: formatCost
  }
])

const selectableMetrics = computed(() =>
  metricDefinitions.value.filter((metric) => !REQUIRED_METRICS.includes(metric.key))
)

const selectedMetricKeys = computed<MetricKey[]>(() => [
  ...REQUIRED_METRICS,
  ...selectedOptionalMetrics.value
])

const selectedMetrics = computed(() =>
  metricDefinitions.value.filter((metric) => selectedMetricKeys.value.includes(metric.key))
)

const isAtMetricLimit = computed(() => selectedMetricKeys.value.length >= MAX_METRICS)

const isMetricSelected = (key: MetricKey) => selectedOptionalMetrics.value.includes(key)

const isMetricDisabled = (key: MetricKey) => !isMetricSelected(key) && isAtMetricLimit.value

const metricButtonClass = (key: MetricKey) => {
  if (isMetricSelected(key)) {
    return 'border-primary-500 bg-primary-50 text-primary-700 dark:border-primary-500 dark:bg-primary-900/30 dark:text-primary-200'
  }
  return 'border-gray-200 bg-white text-gray-600 hover:border-gray-300 hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300 dark:hover:border-dark-600 dark:hover:bg-dark-700'
}

const toggleMetric = (key: MetricKey) => {
  if (REQUIRED_METRICS.includes(key)) return

  if (isMetricSelected(key)) {
    selectedOptionalMetrics.value = selectedOptionalMetrics.value.filter((metric) => metric !== key)
    return
  }

  if (!isAtMetricLimit.value) {
    selectedOptionalMetrics.value = [...selectedOptionalMetrics.value, key]
  }
}

const chartData = computed(() => {
  if (!props.trendData?.length) return null

  return {
    labels: props.trendData.map((point) => point.date),
    datasets: selectedMetrics.value.map((metric) => ({
      label: metric.label,
      data: props.trendData.map((point) => Number(point[metric.key] ?? 0)),
      borderColor: metric.color,
      backgroundColor: `${metric.color}1f`,
      pointBackgroundColor: metric.color,
      pointBorderColor: metric.color,
      pointRadius: 2,
      pointHoverRadius: 4,
      borderWidth: 2,
      fill: metric.key === 'actual_cost' || metric.key === 'total_tokens',
      tension: 0.32,
      yAxisID: metric.axis === 'cost' ? 'yCost' : 'yVolume',
      formatter: metric.formatter
    }))
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
      align: 'start' as const,
      labels: {
        color: chartTheme.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        boxWidth: 8,
        boxHeight: 8,
        padding: 14,
        font: {
          size: 11
        }
      }
    },
    tooltip: {
      callbacks: {
        label: (context: any) => {
          const formatter = context.dataset.formatter as ((value: number) => string) | undefined
          const value = Number(context.raw ?? 0)
          return `${context.dataset.label}: ${formatter ? formatter(value) : value.toLocaleString()}`
        }
      }
    }
  },
  scales: {
    x: {
      grid: {
        display: false
      },
      ticks: {
        color: chartTheme.value.mutedText,
        maxRotation: 0,
        autoSkip: true,
        font: {
          size: 10
        }
      }
    },
    yVolume: {
      position: 'left' as const,
      beginAtZero: true,
      grid: {
        color: chartTheme.value.grid
      },
      ticks: {
        color: chartTheme.value.text,
        font: {
          size: 10
        },
        callback: (value: string | number) => formatTokens(Number(value))
      }
    },
    yCost: {
      position: 'right' as const,
      beginAtZero: true,
      grid: {
        drawOnChartArea: false
      },
      ticks: {
        color: '#16a34a',
        font: {
          size: 10
        },
        callback: (value: string | number) => formatCost(Number(value))
      }
    }
  }
}))
</script>
