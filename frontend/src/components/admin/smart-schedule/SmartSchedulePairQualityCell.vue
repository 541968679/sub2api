<template>
  <button
    type="button"
    class="w-full min-w-[5.5rem] rounded-md px-0.5 py-0.5 text-left transition-colors hover:bg-gray-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-700"
    data-testid="smart-schedule-pair-quality-cell"
    :title="t('admin.users.smartSchedule.pairQualityClickHint')"
    @click="emit('click')"
  >
    <div v-if="loading && !quality" class="h-3 w-14 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
    <div
      v-else-if="quality"
      class="flex flex-col gap-0.5 font-mono text-[11px] leading-4"
    >
      <div
        v-if="phaseLabel"
        class="font-sans text-[10px] text-gray-400 dark:text-gray-500"
        data-testid="smart-schedule-pair-quality-phase"
      >
        {{ phaseLabel }}
      </div>
      <div class="flex items-baseline gap-1" :class="p50ToneClass">
        <span class="font-sans text-[10px] text-gray-400 dark:text-gray-500">p50</span>
        <span class="text-sm font-medium">{{ p50Display }}</span>
      </div>
      <div class="flex items-baseline gap-1" :class="successToneClass">
        <span class="font-sans text-[10px] text-gray-400 dark:text-gray-500">{{ t('admin.users.smartSchedule.pairSuccessShort') }}</span>
        <span class="text-sm font-medium">{{ successDisplay }}</span>
      </div>
      <div class="font-sans text-[10px] text-gray-400 dark:text-gray-500" data-testid="smart-schedule-pair-quality-counts">
        {{ t('admin.users.smartSchedule.pairWindowCounts', pairQualityCountParams(quality)) }}
      </div>
      <div
        v-if="kc.show"
        class="font-sans text-[10px]"
        :class="kcToneClass"
        data-testid="smart-schedule-pair-quality-kc"
      >
        <template v-if="kc.showK && kc.showC">
          {{ t('admin.users.smartSchedule.pairLatencyKC', { slow: kc.slow, k: kc.k, consec: kc.consec, c: kc.c }) }}
        </template>
        <template v-else-if="kc.showK">
          {{ t('admin.users.smartSchedule.pairLatencyK', { slow: kc.slow, k: kc.k }) }}
        </template>
        <template v-else>
          {{ t('admin.users.smartSchedule.pairLatencyC', { consec: kc.consec, c: kc.c }) }}
        </template>
      </div>
    </div>
    <div v-else class="flex flex-col items-start gap-0.5">
      <span class="text-sm text-gray-400 dark:text-dark-500">—</span>
      <span class="text-[10px] font-medium text-primary-600 dark:text-primary-400">
        {{ t('admin.users.smartSchedule.pairQualityOpenShort') }}
      </span>
    </div>
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SmartSchedulePairQuality } from '@/api/admin/users'
import { pairQualityCountParams, pairQualityLatencyKCParams } from '@/utils/smartScheduleWindowN'

const props = withDefaults(
  defineProps<{
    quality?: SmartSchedulePairQuality | null
    loading?: boolean
  }>(),
  {
    quality: null,
    loading: false
  }
)

const emit = defineEmits<{ click: [] }>()
const { t } = useI18n()

function formatMs(ms: number | null | undefined): string {
  if (ms == null) return '—'
  if (ms >= 10000) return `${(ms / 1000).toFixed(1)}s`
  return `${Math.round(ms)}ms`
}

function firstFinite(...vals: Array<number | null | undefined>): number | null {
  for (const value of vals) {
    if (value != null && Number.isFinite(value)) return value
  }
  return null
}

const p50Ms = computed(() =>
  firstFinite(
    props.quality?.ttft_p50_ms,
    props.quality?.probe?.p50_ttft_ms,
    props.quality?.sched?.p50_ttft_ms,
    props.quality?.soft?.p50_ttft_ms
  )
)

const successRate = computed(() =>
  firstFinite(
    props.quality?.success_rate,
    props.quality?.probe?.success_rate,
    props.quality?.sched?.success_rate,
    props.quality?.soft?.success_rate
  )
)

const p50Display = computed(() => formatMs(p50Ms.value))

const successDisplay = computed(() => {
  const rate = successRate.value
  if (rate == null) return '—'
  return `${(rate * 100).toFixed(1)}%`
})

const p50ToneClass = computed(() => {
  const ms = p50Ms.value
  if (ms == null) return 'text-gray-700 dark:text-gray-300'
  if (ms >= 3000) return 'text-red-600 dark:text-red-400'
  if (ms >= 1500) return 'text-amber-600 dark:text-amber-400'
  return 'text-gray-700 dark:text-gray-300'
})

const successToneClass = computed(() => {
  const rate = successRate.value
  if (rate == null) return 'text-gray-700 dark:text-gray-300'
  if (rate < 0.9) return 'text-red-600 dark:text-red-400'
  if (rate < 0.95) return 'text-amber-600 dark:text-amber-400'
  return 'text-emerald-600 dark:text-emerald-400'
})

const phaseLabel = computed(() => {
  const phase = props.quality?.metrics_phase
  if (phase === 'probe') return t('admin.users.smartSchedule.pairMetricsPhaseProbe')
  if (phase === 'sched') return t('admin.users.smartSchedule.pairMetricsPhaseSched')
  if (phase === 'soft') return t('admin.users.smartSchedule.pairMetricsPhaseSoft')
  return ''
})

const kc = computed(() =>
  props.quality ? pairQualityLatencyKCParams(props.quality) : { show: false, showK: false, showC: false, slow: 0, consec: 0, k: 0, c: 0 }
)

const kcToneClass = computed(() => {
  const row = kc.value
  if (!row.show) return 'text-gray-400 dark:text-gray-500'
  if ((row.showK && row.slow >= row.k) || (row.showC && row.consec >= row.c)) {
    return 'text-red-600 dark:text-red-400'
  }
  return 'text-gray-400 dark:text-gray-500'
})
</script>
