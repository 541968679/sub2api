<template>
  <div class="min-w-[5.5rem]">
    <div v-if="loading && !hasStats" class="h-3 w-14 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
    <div v-else-if="error && !hasStats" class="text-xs text-red-500">—</div>
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
    <div v-else class="text-sm text-gray-400 dark:text-dark-500">—</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AccountQualityStats } from '@/api/admin/accounts'

const props = withDefaults(
  defineProps<{
    mode: 'ttft' | 'success_rate'
    stats?: AccountQualityStats | null
    loading?: boolean
    error?: string | null
  }>(),
  {
    stats: null,
    loading: false,
    error: null
  }
)

const { t } = useI18n()

const hasStats = computed(() => props.stats != null)

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

const displayText = computed(() => {
  const stats = props.stats
  if (!stats || props.mode !== 'success_rate') return ''
  if (stats.success_rate == null) return ''
  return `${(stats.success_rate * 100).toFixed(1)}%`
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
  const rate = props.stats?.success_rate
  if (rate == null) return 'text-gray-700 dark:text-gray-300'
  if (rate < 0.9) return 'text-red-600 dark:text-red-400'
  if (rate < 0.95) return 'text-amber-600 dark:text-amber-400'
  return 'text-emerald-600 dark:text-emerald-400'
})

const tooltipText = computed(() => {
  const stats = props.stats
  if (!stats) return ''
  if (props.mode === 'ttft') {
    return t('admin.accounts.quality.ttftTooltip', {
      windowMinutes: Math.round((stats.window_seconds || 900) / 60),
      samples: stats.ttft_samples ?? 0,
      p50: stats.p50_ttft_ms ?? '—',
      p95: stats.p95_ttft_ms ?? '—',
      avg: stats.avg_ttft_ms ?? '—',
      max: stats.max_ttft_ms ?? '—'
    })
  }
  return t('admin.accounts.quality.tooltip', {
    windowMinutes: Math.round((stats.window_seconds || 900) / 60),
    success: stats.success_count ?? 0,
    error: stats.error_count ?? 0,
    ttftSamples: stats.ttft_samples ?? 0
  })
})
</script>
