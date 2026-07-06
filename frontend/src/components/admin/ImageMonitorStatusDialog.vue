<template>
  <BaseDialog :show="show" :title="dialogTitle" width="extra-wide" @close="emit('close')">
    <div v-if="monitor" class="space-y-4">
      <!-- Window tabs -->
      <div
        class="inline-flex rounded-lg border border-gray-200 bg-gray-100 p-1 dark:border-dark-700 dark:bg-dark-800"
        role="tablist"
      >
        <button
          v-for="w in WINDOWS"
          :key="w"
          type="button"
          role="tab"
          :aria-selected="activeWindow === w"
          class="rounded-md px-4 py-1.5 text-sm font-medium transition"
          :class="activeWindow === w
            ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-900 dark:text-primary-200'
            : 'text-gray-600 hover:text-gray-900 dark:text-dark-300 dark:hover:text-white'"
          @click="switchWindow(w)"
        >
          {{ t(`admin.imageChannelMonitor.statusDialog.window.${w}`) }}
        </button>
      </div>

      <!-- Summary metric cards -->
      <dl v-if="data" class="grid grid-cols-2 gap-3 text-sm sm:grid-cols-3 md:grid-cols-6">
        <div class="rounded-md bg-gray-50 p-3 dark:bg-dark-800">
          <dt class="text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.imageChannelMonitor.statusDialog.availability') }}
          </dt>
          <dd
            class="mt-1 font-semibold tabular-nums"
            :class="availabilityToneClass"
          >
            {{ data.summary.total > 0 ? `${data.summary.availability.toFixed(1)}%` : '-' }}
          </dd>
        </div>
        <MetricCard :label="t('admin.imageChannelMonitor.statusDialog.checks')" :value="String(data.summary.total)" />
        <MetricCard
          :label="t('admin.imageChannelMonitor.statusDialog.failures')"
          :value="String(data.summary.failures)"
          :tone="data.summary.failures > 0 ? 'warn' : undefined"
        />
        <MetricCard :label="t('admin.imageChannelMonitor.statusDialog.avgApi')" :value="formatMs(data.summary.avg_api_total_ms)" />
        <MetricCard :label="t('admin.imageChannelMonitor.statusDialog.maxApi')" :value="formatMs(data.summary.max_api_total_ms)" />
        <MetricCard :label="t('admin.imageChannelMonitor.statusDialog.avgDownload')" :value="formatMs(data.summary.avg_image_download_ms)" />
      </dl>

      <!-- Latency lines + failure bars -->
      <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
        <div class="mb-2 text-xs font-medium text-gray-500 dark:text-dark-400">
          {{ t('admin.imageChannelMonitor.statusDialog.chartTitle') }}
        </div>
        <div class="h-64">
          <Chart v-if="chartData" type="bar" :data="chartData" :options="chartOptions" />
          <div v-else class="flex h-full items-center justify-center text-sm text-gray-400 dark:text-dark-500">
            {{ loading ? t('common.loading') : t('common.noData') }}
          </div>
        </div>
      </div>

      <!-- Recent-checks strip (per-check, same data as the row strip) -->
      <div class="rounded-lg border border-gray-200 px-3 pb-2 dark:border-dark-700">
        <MonitorTimeline :buckets="stripPoints" :countdown-seconds="0" />
      </div>
    </div>

    <template #footer>
      <button type="button" class="btn btn-primary" @click="emit('close')">
        {{ t('common.close') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, h, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  LineElement,
  PointElement,
  BarElement,
  CategoryScale,
  LinearScale,
  Tooltip,
  Legend,
} from 'chart.js'
import { Chart } from 'vue-chartjs'
import BaseDialog from '@/components/common/BaseDialog.vue'
import MonitorTimeline from '@/components/user/monitor/MonitorTimeline.vue'
import type { MonitorTimelinePoint } from '@/api/channelMonitor'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import imageChannelMonitorAPI, {
  type ImageChannelMonitor,
  type ImageMonitorTimelineResponse,
  type ImageMonitorTimelineWindow,
} from '@/api/admin/imageChannelMonitor'

ChartJS.register(LineElement, PointElement, BarElement, CategoryScale, LinearScale, Tooltip, Legend)

const props = defineProps<{
  show: boolean
  monitor: ImageChannelMonitor | null
}>()

const emit = defineEmits<{ (e: 'close'): void }>()

const { t } = useI18n()
const appStore = useAppStore()

const WINDOWS: ImageMonitorTimelineWindow[] = ['24h', '7d', '30d']
const activeWindow = ref<ImageMonitorTimelineWindow>('24h')
const data = ref<ImageMonitorTimelineResponse | null>(null)
const loading = ref(false)
let requestSeq = 0

const MetricCard = (p: { label: string; value: string; tone?: 'warn' }) =>
  h('div', { class: 'rounded-md bg-gray-50 p-3 dark:bg-dark-800' }, [
    h('dt', { class: 'text-xs text-gray-500 dark:text-dark-400' }, p.label),
    h(
      'dd',
      {
        class: [
          'mt-1 font-semibold tabular-nums',
          p.tone === 'warn' ? 'text-red-600 dark:text-red-300' : 'text-gray-900 dark:text-white',
        ],
      },
      p.value
    ),
  ])

const dialogTitle = computed(() =>
  props.monitor
    ? `${t('admin.imageChannelMonitor.statusDialog.title')} · ${props.monitor.name}`
    : t('admin.imageChannelMonitor.statusDialog.title')
)

const availabilityToneClass = computed(() => {
  const availability = data.value?.summary.availability ?? 0
  if ((data.value?.summary.total ?? 0) === 0) return 'text-gray-900 dark:text-white'
  if (availability >= 99) return 'text-emerald-600 dark:text-emerald-300'
  if (availability >= 90) return 'text-amber-600 dark:text-amber-300'
  return 'text-red-600 dark:text-red-300'
})

const stripPoints = computed<MonitorTimelinePoint[]>(() =>
  (props.monitor?.timeline ?? []).map((p) => ({
    status: p.status,
    latency_ms: p.latency_ms,
    ping_latency_ms: null,
    checked_at: p.checked_at,
  }))
)

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))

function bucketLabel(iso: string): string {
  const d = new Date(iso)
  if (activeWindow.value === '24h') {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  if (activeWindow.value === '7d') {
    return d.toLocaleString([], { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleDateString([], { month: '2-digit', day: '2-digit' })
}

const chartData = computed(() => {
  if (!data.value || data.value.buckets.length === 0) return null
  const buckets = data.value.buckets
  return {
    labels: buckets.map((b) => bucketLabel(b.bucket_start)),
    datasets: [
      {
        type: 'line' as const,
        label: t('admin.imageChannelMonitor.statusDialog.seriesApi'),
        data: buckets.map((b) => b.avg_api_total_ms),
        borderColor: '#6366f1',
        backgroundColor: '#6366f1',
        spanGaps: false,
        tension: 0.25,
        pointRadius: 2,
        yAxisID: 'y',
      },
      {
        type: 'line' as const,
        label: t('admin.imageChannelMonitor.statusDialog.seriesDownload'),
        data: buckets.map((b) => b.avg_image_download_ms),
        borderColor: '#f59e0b',
        backgroundColor: '#f59e0b',
        spanGaps: false,
        tension: 0.25,
        pointRadius: 2,
        yAxisID: 'y',
      },
      {
        type: 'bar' as const,
        label: t('admin.imageChannelMonitor.statusDialog.seriesFailures'),
        data: buckets.map((b) => b.failed + b.error),
        backgroundColor: 'rgba(239, 68, 68, 0.35)',
        borderRadius: 2,
        yAxisID: 'y1',
      },
    ],
  }
})

const chartOptions = computed(() => {
  const grid = isDarkMode.value ? '#374151' : '#f3f4f6'
  const text = isDarkMode.value ? '#9ca3af' : '#6b7280'
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index' as const, intersect: false },
    plugins: {
      legend: { labels: { color: text, boxWidth: 12, font: { size: 11 } } },
    },
    scales: {
      x: { grid: { display: false }, ticks: { color: text, font: { size: 10 }, maxTicksLimit: 12 } },
      y: {
        beginAtZero: true,
        title: { display: true, text: 'ms', color: text, font: { size: 10 } },
        grid: { color: grid },
        ticks: { color: text, font: { size: 10 } },
      },
      y1: {
        beginAtZero: true,
        position: 'right' as const,
        grid: { display: false },
        ticks: { color: text, font: { size: 10 }, precision: 0 },
      },
    },
  }
})

function formatMs(value: number | null): string {
  return typeof value === 'number' ? `${value.toLocaleString()} ms` : '-'
}

async function load() {
  if (!props.monitor) return
  const seq = ++requestSeq
  loading.value = true
  try {
    const res = await imageChannelMonitorAPI.timeline(props.monitor.id, activeWindow.value)
    if (seq === requestSeq) data.value = res
  } catch (err) {
    appStore.showError(
      extractApiErrorMessage(err, t('admin.imageChannelMonitor.statusDialog.loadError'))
    )
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

function switchWindow(w: ImageMonitorTimelineWindow) {
  if (activeWindow.value === w) return
  activeWindow.value = w
  void load()
}

watch(
  () => [props.show, props.monitor?.id] as const,
  ([show]) => {
    if (show && props.monitor) {
      activeWindow.value = '24h'
      data.value = null
      void load()
    }
  }
)
</script>
