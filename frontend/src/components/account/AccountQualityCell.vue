<template>
  <div class="min-w-[4.5rem]">
    <div v-if="loading && !hasStats" class="h-3 w-12 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
    <div v-else-if="error && !hasStats" class="text-xs text-red-500">—</div>
    <div
      v-else-if="displayText"
      class="text-sm font-mono"
      :class="toneClass"
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

const displayText = computed(() => {
  const stats = props.stats
  if (!stats) return ''
  if (props.mode === 'ttft') {
    if (stats.avg_ttft_ms == null) return ''
    return `${stats.avg_ttft_ms}ms`
  }
  if (stats.success_rate == null) return ''
  return `${(stats.success_rate * 100).toFixed(1)}%`
})

const toneClass = computed(() => {
  const stats = props.stats
  if (!stats) return 'text-gray-700 dark:text-gray-300'
  if (props.mode === 'ttft') {
    const ms = stats.avg_ttft_ms
    if (ms == null) return 'text-gray-700 dark:text-gray-300'
    if (ms >= 3000) return 'text-red-600 dark:text-red-400'
    if (ms >= 1500) return 'text-amber-600 dark:text-amber-400'
    return 'text-gray-700 dark:text-gray-300'
  }
  const rate = stats.success_rate
  if (rate == null) return 'text-gray-700 dark:text-gray-300'
  if (rate < 0.9) return 'text-red-600 dark:text-red-400'
  if (rate < 0.95) return 'text-amber-600 dark:text-amber-400'
  return 'text-emerald-600 dark:text-emerald-400'
})

const tooltipText = computed(() => {
  const stats = props.stats
  if (!stats) return ''
  return t('admin.accounts.quality.tooltip', {
    windowMinutes: Math.round((stats.window_seconds || 900) / 60),
    success: stats.success_count ?? 0,
    error: stats.error_count ?? 0,
    ttftSamples: stats.ttft_samples ?? 0
  })
})
</script>
