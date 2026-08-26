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
    <div v-else class="space-y-5" data-testid="smart-schedule-pair-quality-dialog">
      <section class="grid gap-2 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 sm:grid-cols-3 dark:border-dark-600 dark:bg-dark-800">
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">p50</p>
          <p class="font-mono text-sm font-medium text-gray-900 dark:text-white">{{ currentP50 }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.users.smartSchedule.pairSuccessShort') }}</p>
          <p class="font-mono text-sm font-medium text-gray-900 dark:text-white">{{ currentSuccess }}</p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.users.smartSchedule.pairWindowLabel') }}</p>
          <p class="font-mono text-sm font-medium text-gray-900 dark:text-white">{{ currentCounts }}</p>
        </div>
      </section>

      <section>
        <h4 class="mb-2 text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.users.smartSchedule.pairTrendTitle') }}
        </h4>
        <div v-if="chartData" data-testid="smart-schedule-pair-quality-chart" class="h-72">
          <Line :data="chartData" :options="chartOptions" />
        </div>
        <EmptyState
          v-else
          data-testid="smart-schedule-pair-quality-empty"
          :title="t('admin.users.smartSchedule.pairTrendEmpty')"
          :description="t('admin.users.smartSchedule.pairTrendEmptyHint')"
        />
      </section>

      <section>
        <h4 class="mb-2 text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.users.smartSchedule.pairEventsTitle') }}
        </h4>
        <ul
          v-if="events.length > 0"
          class="divide-y divide-gray-100 rounded-lg border border-gray-200 dark:divide-dark-700 dark:border-dark-600"
          data-testid="smart-schedule-pair-quality-events"
        >
          <li
            v-for="(event, index) in events"
            :key="`${event.at}-${event.type}-${index}`"
            class="flex flex-wrap items-center justify-between gap-2 px-3 py-2 text-sm"
          >
            <span class="text-gray-800 dark:text-gray-100">{{ eventLabel(event.type) }}</span>
            <div class="flex min-w-0 flex-col items-end gap-0.5">
              <span class="font-mono text-xs text-gray-500 dark:text-gray-400">{{ formatDateTime(event.at) }}</span>
              <span
                v-if="event.detail"
                class="max-w-md text-right text-[11px] text-gray-600 dark:text-gray-300"
                data-testid="smart-schedule-pair-quality-event-detail"
              >
                {{ event.detail }}
              </span>
            </div>
          </li>
        </ul>
        <p v-else class="text-sm text-gray-500 dark:text-gray-400" data-testid="smart-schedule-pair-quality-events-empty">
          {{ t('admin.users.smartSchedule.pairEventsEmpty') }}
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
  type ChartOptions
} from 'chart.js'
import { Line } from 'vue-chartjs'
import { adminAPI } from '@/api/admin'
import type {
  SmartSchedulePairQuality,
  SmartSchedulePairQualityDetail,
  SmartSchedulePairQualityEvent
} from '@/api/admin/users'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { Account } from '@/types'
import { formatDateTime } from '@/utils/format'
import { pairQualityCountParams } from '@/utils/smartScheduleWindowN'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const props = defineProps<{
  show: boolean
  userId: number | null
  account: Account | null
  platform?: string
}>()

const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()

const loading = ref(false)
const detail = ref<SmartSchedulePairQualityDetail | null>(null)

const dialogTitle = computed(() => {
  const name = props.account?.name || t('admin.users.smartSchedule.unknownUser')
  return t('admin.users.smartSchedule.pairQualityDialogTitle', { name })
})

const current = computed<SmartSchedulePairQuality | null>(() => {
  if (detail.value?.current) return detail.value.current
  const last = detail.value?.snapshots?.[detail.value.snapshots.length - 1]
  return last ?? null
})

function formatMs(ms: number | null | undefined): string {
  if (ms == null) return '—'
  if (ms >= 10000) return `${(ms / 1000).toFixed(1)}s`
  return `${Math.round(ms)}ms`
}

function formatRate(rate: number | null | undefined): string {
  if (rate == null || !Number.isFinite(rate)) return '—'
  return `${(rate * 100).toFixed(1)}%`
}

const currentP50 = computed(() => formatMs(current.value?.ttft_p50_ms))
const currentSuccess = computed(() => formatRate(current.value?.success_rate))
const currentCounts = computed(() => {
  if (!current.value) return '—'
  return t('admin.users.smartSchedule.pairWindowCounts', pairQualityCountParams(current.value))
})

const events = computed<SmartSchedulePairQualityEvent[]>(() => detail.value?.events ?? [])

const hasSeries = computed(() => (detail.value?.snapshots ?? []).length > 0)

const chartData = computed<ChartData<'line'> | null>(() => {
  if (!hasSeries.value || !detail.value) return null
  const points = detail.value.snapshots
  return {
    labels: points.map((point) => formatDateTime(point.captured_at)),
    datasets: [
      {
        label: 'p50',
        data: points.map((point) => point.ttft_p50_ms ?? null),
        borderColor: '#2563eb',
        backgroundColor: '#2563eb',
        yAxisID: 'y',
        spanGaps: false,
        tension: 0.2,
        pointRadius: 2
      },
      {
        label: t('admin.users.smartSchedule.pairSuccessShort'),
        data: points.map((point) => (point.success_rate == null ? null : point.success_rate * 100)),
        borderColor: '#059669',
        backgroundColor: '#059669',
        yAxisID: 'y1',
        spanGaps: false,
        tension: 0.2,
        pointRadius: 2
      }
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
    y1: { position: 'right', beginAtZero: true, max: 100, grid: { drawOnChartArea: false } }
  }
}))

function eventLabel(type: string) {
  const key = type.toLowerCase()
  if (key === 'cooldown_start' || key === 'cooling' || key === 'cooldown_begin') {
    return t('admin.users.smartSchedule.pairEventCooldownStart')
  }
  if (key === 'cooldown_end' || key === 'cooldown_cleared') {
    return t('admin.users.smartSchedule.pairEventCooldownEnd')
  }
  if (key === 'resumed' || key === 'enter_resumed' || key === 'exemption') {
    return t('admin.users.smartSchedule.pairEventResumed')
  }
  if (key === 'pinned' || key === 'enter_pinned') {
    return t('admin.users.smartSchedule.pairEventPinned')
  }
  if (key === 'selectable') {
    return t('admin.users.smartSchedule.pairEventSelectable')
  }
  if (key === 'probe_enter' || key === 'probing' || key === 'enter_probing') {
    return t('admin.users.smartSchedule.pairEventProbeEnter')
  }
  if (key === 'probe_graduate' || key === 'graduate') {
    return t('admin.users.smartSchedule.pairEventProbeGraduate')
  }
  if (key === 'expiry_zero' || key === 'window_cleared' || key === 'zero') {
    return t('admin.users.smartSchedule.pairEventExpiryZero')
  }
  if (key === 'soft_cooldown_end') {
    return t('admin.users.smartSchedule.pairEventSoftCooldownEnd')
  }
  return type
}

async function loadDetail() {
  if (!props.show || !props.userId || !props.account) {
    detail.value = null
    return
  }
  loading.value = true
  try {
    detail.value = await adminAPI.users.getSmartSchedulePairQualityDetail(
      props.userId,
      props.account.id,
      props.platform
    )
  } catch {
    detail.value = { current: null, snapshots: [], events: [] }
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.show, props.userId, props.account?.id, props.platform] as const,
  ([open]) => {
    if (open) void loadDetail()
    else detail.value = null
  },
  { immediate: true }
)
</script>
