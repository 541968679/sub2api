<template>
  <component
    :is="clickable ? 'button' : 'div'"
    :type="clickable ? 'button' : undefined"
    class="min-w-[5.5rem]"
    :class="clickable ? 'w-full rounded-md px-0.5 py-0.5 text-left transition-colors hover:bg-gray-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-700' : undefined"
    :data-test="clickable ? 'account-quality-cell-button' : 'account-quality-cell'"
    :aria-label="clickable ? t('admin.accounts.stability.openAria') : undefined"
    :title="clickable && !tooltipText ? t('admin.accounts.stability.clickToOpen') : undefined"
    @click="onClick"
  >
    <div v-if="loading && !hasStats" class="h-3 w-14 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
    <div v-else-if="error && !hasStats" class="text-xs text-red-500">—</div>
    <div
      v-else-if="mode === 'combined' && (hasTtft || hasSuccessRate)"
      class="flex flex-col gap-0.5 font-mono text-[11px] leading-4"
      :title="tooltipText"
    >
      <div class="flex items-baseline gap-1" :class="p50ToneClass">
        <span class="text-[10px] font-sans text-gray-400 dark:text-gray-500">p50</span>
        <span class="text-sm font-medium">{{ hasTtft ? formatMs(stats!.p50_ttft_ms ?? stats!.avg_ttft_ms) : '—' }}</span>
      </div>
      <div class="flex items-baseline gap-1" :class="successToneClass">
        <span class="text-[10px] font-sans text-gray-400 dark:text-gray-500">{{ t('admin.accounts.quality.successShort') }}</span>
        <span class="text-sm font-medium">{{ successDisplay || '—' }}</span>
      </div>
      <div
        class="flex items-baseline gap-1"
        :class="failoverToneClass"
        data-test="account-quality-failover-rate"
      >
        <span class="text-[10px] font-sans text-gray-400 dark:text-gray-500">{{ t('admin.accounts.quality.failoverShort') }}</span>
        <span class="text-sm font-medium">{{ failoverDisplay || '—' }}</span>
      </div>
      <div class="font-sans text-[10px] text-gray-400 dark:text-gray-500" data-test="account-quality-window-counts">
        {{ t('admin.accounts.quality.windowCounts', { ttft: stats!.ttft_samples ?? 0, ok: okSamples, n: windowN }) }}
      </div>
    </div>
    <div
      v-else-if="mode === 'ttft' && hasTtft"
      class="flex flex-col gap-0.5 font-mono text-[11px] leading-4"
      :title="tooltipText"
    >
      <div class="flex items-baseline gap-1" :class="p50ToneClass">
        <span class="text-[10px] font-sans text-gray-400 dark:text-gray-500">p50</span>
        <span class="text-sm font-medium">{{ formatMs(stats!.p50_ttft_ms ?? stats!.avg_ttft_ms) }}</span>
      </div>
      <div class="flex items-baseline gap-1" :class="p95ToneClass">
        <span class="text-[10px] font-sans text-gray-400 dark:text-gray-500">p95</span>
        <span>{{ formatMs(stats!.p95_ttft_ms) }}</span>
      </div>
    </div>
    <div
      v-else-if="mode === 'success_rate' && displayText"
      class="text-sm font-mono"
      :class="successToneClass"
      :title="tooltipText"
    >
      {{ displayText }}
    </div>
    <div v-else class="flex flex-col items-start gap-0.5">
      <span class="text-sm text-gray-400 dark:text-dark-500">—</span>
      <span
        v-if="clickable"
        class="text-[10px] font-medium text-primary-600 dark:text-primary-400"
      >
        {{ t('admin.accounts.stability.openShort') }}
      </span>
    </div>
  </component>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AccountQualityStats } from '@/api/admin/accounts'
import {
  formatQualityFailoverSuccessRate,
  formatQualitySuccessRate,
  hasDisplayableQualityRate,
  qualityFailoverSuccessRateValue
} from '@/utils/accountQualityStats'
import { qualityRateWindowK, resolveAccountQualityWindowN } from '@/utils/accountQualityWindowN'

const props = withDefaults(
  defineProps<{
    mode: 'ttft' | 'success_rate' | 'combined'
    stats?: AccountQualityStats | null
    loading?: boolean
    error?: string | null
    clickable?: boolean
    minSamples?: number
    windowN?: number | null
  }>(),
  {
    stats: null,
    loading: false,
    error: null,
    clickable: false,
    minSamples: 1,
    windowN: null
  }
)

const emit = defineEmits<{
  (e: 'click'): void
}>()

const { t } = useI18n()

const hasStats = computed(() => props.stats != null)

const windowN = computed(() =>
  resolveAccountQualityWindowN({
    account_quality_window_n: props.stats?.account_quality_window_n ?? props.windowN,
    window_n: props.stats?.window_n,
    n: props.stats?.n
  })
)

const okSamples = computed(() => qualityRateWindowK(props.stats))

const hasTtft = computed(() => {
  const s = props.stats
  if (!s || !s.ttft_samples) return false
  return s.p50_ttft_ms != null || s.avg_ttft_ms != null
})

function formatMs(ms: number | null | undefined): string {
  if (ms == null) return '—'
  if (ms >= 10000) return `${(ms / 1000).toFixed(1)}s`
  return `${ms}ms`
}

const hasSuccessRate = computed(() => hasDisplayableQualityRate(props.stats, props.minSamples))

const successDisplay = computed(() => formatQualitySuccessRate(props.stats, props.minSamples) ?? '')

const failoverSuccessRate = computed(() =>
  qualityFailoverSuccessRateValue(props.stats, props.minSamples)
)

const failoverDisplay = computed(
  () => formatQualityFailoverSuccessRate(props.stats, props.minSamples) ?? ''
)

const displayText = computed(() => {
  if (props.mode !== 'success_rate') return ''
  return successDisplay.value
})

const p50ToneClass = computed(() => {
  const ms = props.stats?.p50_ttft_ms ?? props.stats?.avg_ttft_ms
  if (ms == null) return 'text-gray-700 dark:text-gray-300'
  if (ms >= 3000) return 'text-red-600 dark:text-red-400'
  if (ms >= 1500) return 'text-amber-600 dark:text-amber-400'
  return 'text-gray-700 dark:text-gray-300'
})

const p95ToneClass = computed(() => {
  const ms = props.stats?.p95_ttft_ms
  if (ms == null) return 'text-gray-500 dark:text-gray-400'
  if (ms >= 5000) return 'text-red-600 dark:text-red-400'
  if (ms >= 2500) return 'text-amber-600 dark:text-amber-400'
  return 'text-gray-500 dark:text-gray-400'
})

const successToneClass = computed(() => {
  if (!hasSuccessRate.value) return 'text-gray-700 dark:text-gray-300'
  const rate = props.stats?.success_rate
  if (rate == null) return 'text-gray-700 dark:text-gray-300'
  if (rate < 0.9) return 'text-red-600 dark:text-red-400'
  if (rate < 0.95) return 'text-amber-600 dark:text-amber-400'
  return 'text-emerald-600 dark:text-emerald-400'
})

const failoverToneClass = computed(() => {
  const rate = failoverSuccessRate.value
  if (rate == null) return 'text-gray-700 dark:text-gray-300'
  if (rate < 0.9) return 'text-red-600 dark:text-red-400'
  if (rate < 0.95) return 'text-amber-600 dark:text-amber-400'
  return 'text-emerald-600 dark:text-emerald-400'
})

const tooltipText = computed(() => {
  const stats = props.stats
  if (!stats) return ''
  const clickHint = props.clickable ? t('admin.accounts.stability.clickToOpen') : ''
  let base = ''
  if (props.mode === 'combined') {
    const parts = [
      t('admin.accounts.quality.ttftTooltip', {
        n: windowN.value,
        samples: stats.ttft_samples ?? 0,
        p50: stats.p50_ttft_ms ?? '—',
        p95: stats.p95_ttft_ms ?? '—',
        avg: stats.avg_ttft_ms ?? '—',
        max: stats.max_ttft_ms ?? '—'
      }),
      t('admin.accounts.quality.tooltip', {
        n: windowN.value,
        success: stats.success_count ?? 0,
        error: stats.error_count ?? 0,
        ttftSamples: stats.ttft_samples ?? 0
      }),
      t('admin.accounts.quality.failoverTooltip', {
        n: windowN.value,
        error: stats.failover_error_count ?? '—'
      })
    ]
    base = parts.join('\n')
  } else if (props.mode === 'ttft') {
    base = t('admin.accounts.quality.ttftTooltip', {
      n: windowN.value,
      samples: stats.ttft_samples ?? 0,
      p50: stats.p50_ttft_ms ?? '—',
      p95: stats.p95_ttft_ms ?? '—',
      avg: stats.avg_ttft_ms ?? '—',
      max: stats.max_ttft_ms ?? '—'
    })
  } else {
    base = t('admin.accounts.quality.tooltip', {
      n: windowN.value,
      success: stats.success_count ?? 0,
      error: stats.error_count ?? 0,
      ttftSamples: stats.ttft_samples ?? 0
    })
  }
  return clickHint ? `${base}\n${clickHint}` : base
})

function onClick(event: Event) {
  if (!props.clickable) return
  event.stopPropagation()
  emit('click')
}
</script>
