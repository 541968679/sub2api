<template>
  <button
    type="button"
    class="group text-left p-5 rounded-2xl min-h-[280px] w-full bg-white/70 backdrop-blur-xl border border-gray-200/80 shadow-card dark:bg-dark-800/60 dark:border-dark-700/70 hover:-translate-y-1 hover:shadow-card-hover dark:hover:border-primary-500/30 hover:border-gray-300 transition-all duration-300 ease-out flex flex-col"
    @click="emit('click')"
  >
    <!-- Header: icon + name/model + status chip -->
    <div class="flex items-start gap-3">
      <span
        class="w-9 h-9 rounded-xl ring-1 ring-black/5 dark:ring-white/10 grid place-items-center flex-shrink-0 bg-gradient-to-br from-fuchsia-50 to-purple-100 text-fuchsia-600 dark:from-fuchsia-500/10 dark:to-purple-500/20 dark:text-fuchsia-300"
      >
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M4 7a2 2 0 012-2h12a2 2 0 012 2v10a2 2 0 01-2 2H6a2 2 0 01-2-2V7zm12.5 3a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zM4 15l4.3-4.3a2 2 0 012.83 0L16 15.5"
          />
        </svg>
      </span>
      <div class="flex-1 min-w-0">
        <div class="text-base font-semibold truncate text-gray-900 dark:text-gray-100">
          {{ item.name }}
        </div>
        <div class="mt-0.5 flex items-center gap-1.5 min-w-0">
          <span
            class="inline-flex items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium flex-shrink-0 bg-fuchsia-50 text-fuchsia-700 dark:bg-fuchsia-500/15 dark:text-fuchsia-300"
          >
            {{ t('channelStatus.imageSection.badge') }}
          </span>
          <span class="font-mono text-xs truncate text-gray-500 dark:text-gray-400">
            {{ item.model }}
          </span>
        </div>
      </div>
      <span
        class="px-2.5 py-1 rounded-full text-xs font-semibold flex-shrink-0"
        :class="statusBadgeClass(displayStatus)"
      >
        {{ statusLabel(displayStatus) }}
      </span>
    </div>

    <!-- Metrics: generation latency + image download latency -->
    <MonitorMetricPair
      primary-icon="bolt"
      :primary-label="t('channelStatus.imageSection.apiLatency')"
      :primary-value="formatLatency(item.latest_api_ms)"
      primary-unit="ms"
      secondary-icon="clock"
      :secondary-label="t('channelStatus.imageSection.downloadLatency')"
      :secondary-value="formatLatency(item.latest_download_ms)"
      secondary-unit="ms"
    />

    <!-- Divider -->
    <div class="mt-4 border-t border-gray-100 dark:border-dark-700/60"></div>

    <!-- Availability row -->
    <MonitorAvailabilityRow :window-label="availabilityLabel" :value="availabilityValue" />

    <!-- Timeline -->
    <MonitorTimeline :buckets="timelinePoints" :countdown-seconds="countdownSeconds" />
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ImageMonitorPublicView } from '@/api/imageChannelMonitor'
import type { MonitorTimelinePoint } from '@/api/channelMonitor'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import MonitorMetricPair from './MonitorMetricPair.vue'
import MonitorAvailabilityRow from './MonitorAvailabilityRow.vue'
import MonitorTimeline from './MonitorTimeline.vue'

const props = defineProps<{
  item: ImageMonitorPublicView
  window: '7d' | '15d' | '30d'
  availabilityValue: number | null
  countdownSeconds: number
}>()

const emit = defineEmits<{ (e: 'click'): void }>()

const { t } = useI18n()
const { statusLabel, statusBadgeClass, formatLatency } = useChannelMonitorFormat()

// "empty"(尚无检查记录)映射为 ''，走 statusLabel 的 unknown 文案与中性徽章。
const displayStatus = computed(() =>
  props.item.latest_status === 'empty' ? '' : props.item.latest_status
)

const availabilityLabel = computed(() => {
  const win = t(`channelStatus.windowTab.${props.window}`)
  return `${t('monitorCommon.availabilityPrefix')} · ${win}`
})

const timelinePoints = computed<MonitorTimelinePoint[]>(() =>
  props.item.timeline.map((p) => ({
    status: p.status,
    latency_ms: p.latency_ms,
    ping_latency_ms: null,
    checked_at: p.checked_at,
  }))
)
</script>
