<template>
  <div class="card p-4">
    <div class="mb-4 flex items-center justify-between gap-3">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('usage.antigravity.curveTitle') }}
        </h3>
        <p class="text-xs text-gray-400">{{ t('usage.antigravity.curveSubtitle') }}</p>
      </div>
      <div v-if="topAnomaly" class="rounded-md border border-amber-200 bg-amber-50 px-2.5 py-1 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-300">
        {{ t('usage.antigravity.anomalyHint', { time: topAnomaly.label, score: topAnomaly.score.toFixed(1) }) }}
      </div>
    </div>

    <div v-if="loading" class="flex h-64 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="chartData" class="h-64">
      <Line :data="chartData" :options="lineOptions" />
    </div>
    <div v-else class="flex h-64 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
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
import type { AntigravityCreditCurve } from '@/api/admin/usage'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend, Filler)

const props = defineProps<{
  curve: AntigravityCreditCurve | null
  loading?: boolean
}>()

const { t } = useI18n()

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))
const colors = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb',
  credits: '#ef4444',
  calls: '#2563eb',
  tokens: '#10b981',
  quota: '#f59e0b',
  ratio: '#8b5cf6'
}))

const points = computed(() => props.curve?.points?.filter((p) => (
  p.credits_consumed > 0 || p.call_count > 0 || p.total_tokens > 0 || p.quota_used_usd > 0
)) || [])

const labels = computed(() => points.value.map((p) => formatBucket(p.start)))

const chartData = computed(() => {
  if (!points.value.length) return null
  return {
    labels: labels.value,
    datasets: [
      {
        label: t('usage.antigravity.creditsConsumed'),
        data: points.value.map((p) => p.credits_consumed),
        borderColor: colors.value.credits,
        backgroundColor: `${colors.value.credits}18`,
        fill: true,
        tension: 0.25,
        yAxisID: 'yCredits'
      },
      {
        label: t('usage.antigravity.calls'),
        data: points.value.map((p) => p.call_count),
        borderColor: colors.value.calls,
        backgroundColor: `${colors.value.calls}14`,
        fill: false,
        tension: 0.25,
        yAxisID: 'yCount'
      },
      {
        label: 'Tokens',
        data: points.value.map((p) => p.total_tokens),
        borderColor: colors.value.tokens,
        backgroundColor: `${colors.value.tokens}14`,
        fill: false,
        tension: 0.25,
        yAxisID: 'yTokens'
      },
      {
        label: t('usage.antigravity.quotaUsed'),
        data: points.value.map((p) => p.quota_used_usd),
        borderColor: colors.value.quota,
        backgroundColor: `${colors.value.quota}14`,
        fill: false,
        tension: 0.25,
        yAxisID: 'yCost'
      },
      {
        label: t('usage.antigravity.creditsPerCall'),
        data: points.value.map((p) => p.credits_per_call || 0),
        borderColor: colors.value.ratio,
        backgroundColor: `${colors.value.ratio}14`,
        borderDash: [5, 5],
        fill: false,
        tension: 0.25,
        yAxisID: 'yRatio'
      }
    ]
  }
})

const topAnomaly = computed(() => {
  const point = [...points.value]
    .filter((p) => p.anomaly_score >= 3 && p.credits_consumed >= 100)
    .sort((a, b) => b.anomaly_score - a.anomaly_score)[0]
  if (!point) return null
  return { label: formatBucket(point.start), score: point.anomaly_score }
})

const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { intersect: false, mode: 'index' as const },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: {
        color: colors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 12,
        font: { size: 11 }
      }
    },
    tooltip: {
      callbacks: {
        label: (context: any) => {
          const label = context.dataset.label
          const value = Number(context.raw || 0)
          if (context.dataset.yAxisID === 'yCost') return `${label}: $${formatCost(value)}`
          if (context.dataset.yAxisID === 'yTokens') return `${label}: ${formatCompact(value)}`
          return `${label}: ${formatCompact(value)}`
        },
        footer: (items: any[]) => {
          const index = items[0]?.dataIndex
          const point = points.value[index]
          if (!point) return ''
          const parts = [
            `${t('usage.antigravity.quotaPerCredit')}: ${point.quota_per_credit == null ? '-' : '$' + point.quota_per_credit.toFixed(6)}`,
            `${t('usage.antigravity.tokensPerCredit')}: ${point.tokens_per_credit == null ? '-' : formatCompact(point.tokens_per_credit)}`
          ]
          if (point.anomaly_description) parts.push(point.anomaly_description)
          return parts
        }
      }
    }
  },
  scales: {
    x: {
      grid: { color: colors.value.grid },
      ticks: { color: colors.value.text, font: { size: 10 }, maxRotation: 0 }
    },
    yCredits: {
      position: 'left' as const,
      grid: { color: colors.value.grid },
      ticks: { color: colors.value.credits, font: { size: 10 }, callback: (v: string | number) => formatCompact(Number(v)) }
    },
    yCount: {
      position: 'right' as const,
      grid: { drawOnChartArea: false },
      ticks: { color: colors.value.calls, font: { size: 10 }, callback: (v: string | number) => formatCompact(Number(v)) }
    },
    yTokens: { display: false },
    yCost: { display: false },
    yRatio: { display: false }
  }
}))

const formatBucket = (raw: string) => {
  const d = new Date(raw)
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const hour = String(d.getHours()).padStart(2, '0')
  return props.curve?.granularity === 'day' ? `${month}-${day}` : `${month}-${day} ${hour}:00`
}

const formatCompact = (value: number) => {
  if (value >= 1e9) return `${(value / 1e9).toFixed(2)}B`
  if (value >= 1e6) return `${(value / 1e6).toFixed(2)}M`
  if (value >= 1e3) return `${(value / 1e3).toFixed(2)}K`
  return value.toFixed(value % 1 === 0 ? 0 : 2)
}

const formatCost = (value: number) => {
  if (value >= 1000) return `${(value / 1000).toFixed(2)}K`
  if (value >= 1) return value.toFixed(2)
  return value.toFixed(4)
}
</script>
