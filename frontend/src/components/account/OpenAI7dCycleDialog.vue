<template>
  <BaseDialog
    :show="show"
    :title="t('usage.cycleHistoryTitle')"
    width="extra-wide"
    @close="emit('close')"
  >
    <div v-if="loading" class="flex h-64 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else class="space-y-4">
      <p class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('usage.cycleHistoryHint') }}
      </p>
      <section>
        <h4 class="mb-2 text-sm font-medium text-gray-900 dark:text-white">
          {{ t('usage.cycleChartTitle') }}
        </h4>
        <div v-if="chartData" data-test="openai-7d-cycle-chart" class="h-64">
          <Line :data="chartData" :options="chartOptions" />
        </div>
        <EmptyState
          v-else
          data-test="openai-7d-cycle-empty"
          :title="t('usage.cycleEmpty')"
          :description="t('usage.cycleEmptyHint')"
        />
      </section>
      <section>
        <h4 class="mb-2 text-sm font-medium text-gray-900 dark:text-white">
          {{ t('usage.cycleTableTitle') }}
        </h4>
        <div v-if="items.length" class="overflow-x-auto">
          <table class="min-w-full text-left text-xs text-gray-700 dark:text-gray-300">
            <thead class="border-b border-gray-200 text-[11px] text-gray-500 dark:border-dark-600 dark:text-gray-400">
              <tr>
                <th class="px-2 py-1.5 font-medium">{{ t('usage.cycleWindow') }}</th>
                <th class="px-2 py-1.5 font-medium">{{ t('usage.cycleLiteLLM') }}</th>
                <th class="px-2 py-1.5 font-medium">{{ t('usage.cycleAccount') }}</th>
                <th class="px-2 py-1.5 font-medium">{{ t('usage.cycleUser') }}</th>
                <th class="px-2 py-1.5 font-medium">{{ t('usage.cycleUsed') }}</th>
                <th class="px-2 py-1.5 font-medium">{{ t('usage.cycleRequests') }}</th>
                <th class="px-2 py-1.5 font-medium">{{ t('usage.cycleTokens') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(row, index) in items"
                :key="`${row.window_end}-${index}`"
                class="border-b border-gray-100 dark:border-dark-700"
                :data-test="row.current ? 'openai-7d-cycle-current' : 'openai-7d-cycle-closed'"
              >
                <td class="whitespace-nowrap px-2 py-1.5">
                  <span
                    class="mr-1 rounded px-1 py-0.5 text-[10px] font-medium"
                    :class="row.current
                      ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
                      : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'"
                  >
                    {{ row.current ? t('usage.cycleCurrent') : t('usage.cycleClosed') }}
                  </span>
                  {{ formatWindow(row.window_start) }} – {{ formatWindow(row.window_end) }}
                </td>
                <td class="px-2 py-1.5 font-mono">{{ formatMoney(row.litellm_cost) }}</td>
                <td class="px-2 py-1.5 font-mono">{{ formatMoney(row.account_cost) }}</td>
                <td class="px-2 py-1.5 font-mono">{{ formatMoney(row.user_cost) }}</td>
                <td class="px-2 py-1.5 font-mono">{{ formatPercent(row.used_percent) }}</td>
                <td class="px-2 py-1.5 font-mono">{{ formatCount(row.requests) }}</td>
                <td class="px-2 py-1.5 font-mono">{{ formatCount(row.tokens) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary" @click="emit('close')">
        {{ t('common.close') }}
      </button>
    </template>
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
  type ChartOptions
} from 'chart.js'
import { Line } from 'vue-chartjs'
import { adminAPI } from '@/api/admin'
import type { OpenAI7dCycleHistoryItem } from '@/api/admin/accounts'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { useAppStore } from '@/stores/app'
import type { Account } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatCompactNumber } from '@/utils/format'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const props = defineProps<{
  show: boolean
  account: Account | null
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const items = ref<OpenAI7dCycleHistoryItem[]>([])

watch(
  () => [props.show, props.account?.id] as const,
  async ([show, accountId]) => {
    if (!show || !accountId) {
      items.value = []
      return
    }
    loading.value = true
    try {
      const history = await adminAPI.accounts.getOpenAI7dCycleHistory(accountId)
      items.value = history?.items ?? []
    } catch (error) {
      items.value = []
      appStore.showError(extractApiErrorMessage(error, t('usage.cycleLoadFailed')))
    } finally {
      loading.value = false
    }
  },
  { immediate: true }
)

const chartRows = computed(() => {
  return [...items.value].reverse()
})

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))

const chartData = computed<ChartData<'line'> | null>(() => {
  if (!chartRows.value.length) return null
  const labels = chartRows.value.map((row) => formatWindow(row.window_end))
  return {
    labels,
    datasets: [
      {
        label: t('usage.cycleLiteLLM'),
        data: chartRows.value.map((row) => row.litellm_cost),
        borderColor: '#10b981',
        backgroundColor: 'rgba(16, 185, 129, 0.12)',
        fill: false,
        tension: 0.25
      },
      {
        label: t('usage.cycleAccount'),
        data: chartRows.value.map((row) => row.account_cost),
        borderColor: '#6366f1',
        backgroundColor: 'rgba(99, 102, 241, 0.12)',
        fill: false,
        tension: 0.25
      }
    ]
  }
})

const chartOptions = computed<ChartOptions<'line'>>(() => {
  const text = isDarkMode.value ? '#e5e7eb' : '#374151'
  const grid = isDarkMode.value ? '#374151' : '#e5e7eb'
  return {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        labels: { color: text }
      }
    },
    scales: {
      x: {
        ticks: { color: text, maxRotation: 0 },
        grid: { color: grid }
      },
      y: {
        beginAtZero: true,
        ticks: { color: text },
        grid: { color: grid }
      }
    }
  }
})

function formatWindow(raw: string): string {
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  return `${month}-${day} ${hour}:${minute}`
}

function formatMoney(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '$0.00'
  if (value !== 0 && Math.abs(value) < 0.01) {
    return `$${value.toFixed(4)}`
  }
  return `$${value.toFixed(2)}`
}

function formatPercent(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '0%'
  return `${Math.round(value)}%`
}

function formatCount(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '0'
  return formatCompactNumber(value, { allowBillions: false })
}
</script>
