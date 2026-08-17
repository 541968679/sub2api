<template>
  <BaseDialog
    :show="show"
    :title="title || t('admin.users.schedulePnl.dialogTitle')"
    width="extra-wide"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <div class="flex flex-wrap gap-2" data-testid="schedule-pnl-range-tabs">
        <button
          v-for="item in ranges"
          :key="item.value"
          type="button"
          class="rounded-md px-2.5 py-1 text-sm"
          :class="item.value === range
            ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
            : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700'"
          :data-testid="`schedule-pnl-range-${item.value}`"
          @click="range = item.value"
        >
          {{ item.label }}
        </button>
      </div>
      <div v-if="loading" class="flex h-72 items-center justify-center">
        <LoadingSpinner />
      </div>
      <div v-else-if="chartData" data-testid="schedule-pnl-chart" class="h-72">
        <Line :data="chartData" :options="chartOptions" />
      </div>
      <EmptyState
        v-else
        data-testid="schedule-pnl-empty"
        :title="t('admin.users.schedulePnl.noData')"
        :description="t('admin.users.schedulePnl.noDataHint')"
      />
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
  type ChartOptions
} from 'chart.js'
import { Line } from 'vue-chartjs'
import { adminAPI } from '@/api/admin'
import type { SchedulePnlTrend, SchedulePnlTrendRange } from '@/api/admin/users'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const props = defineProps<{
  show: boolean
  userId: number | null
  accountId?: number | null
  title?: string
}>()

const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()

const range = ref<SchedulePnlTrendRange>('24h')
const loading = ref(false)
const trend = ref<SchedulePnlTrend | null>(null)

const ranges = computed(() => [
  { value: '24h' as const, label: t('admin.users.schedulePnl.range24h') },
  { value: 'today' as const, label: t('admin.users.schedulePnl.rangeToday') },
  { value: 'yesterday' as const, label: t('admin.users.schedulePnl.rangeYesterday') },
  { value: '7d' as const, label: t('admin.users.schedulePnl.range7d') }
])

const hasSeries = computed(() =>
  (trend.value?.points ?? []).some((point) => point.revenue != null || point.cost != null)
)

const chartData = computed<ChartData<'line'> | null>(() => {
  if (!hasSeries.value || !trend.value) return null
  const labels = trend.value.points.map((point) => point.bucket)
  return {
    labels,
    datasets: [
      series(t('admin.users.schedulePnl.revenue'), trend.value.points.map((p) => p.revenue), '#2563eb'),
      series(t('admin.users.schedulePnl.cost'), trend.value.points.map((p) => p.cost), '#dc2626'),
      series(t('admin.users.schedulePnl.profit'), trend.value.points.map((p) => p.profit), '#059669'),
      series(t('admin.users.schedulePnl.margin'), trend.value.points.map((p) => p.margin == null ? null : p.margin * 100), '#d97706', 'y1')
    ]
  }
})

const chartOptions = computed<ChartOptions<'line'>>(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index', intersect: false },
  plugins: { legend: { display: true } },
  scales: {
    y: { beginAtZero: false },
    y1: { position: 'right', beginAtZero: true, grid: { drawOnChartArea: false } }
  }
}))

function series(
  label: string,
  data: Array<number | null>,
  color: string,
  yAxisID = 'y'
) {
  return {
    label,
    data,
    borderColor: color,
    backgroundColor: color,
    yAxisID,
    spanGaps: false,
    tension: 0.2,
    pointRadius: 2
  }
}

async function loadTrend() {
  if (!props.show || !props.userId) {
    trend.value = null
    return
  }
  loading.value = true
  try {
    trend.value = await adminAPI.users.getSmartSchedulePnlTrend(
      props.userId,
      range.value,
      props.accountId ?? undefined
    )
  } catch {
    trend.value = null
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.show, props.userId, props.accountId, range.value] as const,
  ([open]) => {
    if (open) void loadTrend()
    else {
      range.value = '24h'
      trend.value = null
    }
  }
)
</script>
