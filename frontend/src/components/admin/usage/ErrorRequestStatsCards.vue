<template>
  <div class="space-y-4">
    <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <div class="card p-4">
        <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
          {{ t('usage.errors.stats.totalRequests') }}
        </p>
        <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
          {{ loading ? '—' : formatCount(stats?.total_requests) }}
        </p>
      </div>
      <div class="card p-4">
        <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
          {{ t('usage.errors.stats.terminalErrors') }}
        </p>
        <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
          {{ loading ? '—' : formatCount(stats?.terminal_error_requests_filtered) }}
        </p>
        <p class="mt-0.5 text-[11px] text-gray-400">
          {{ t('usage.errors.stats.bizTerminal', { count: formatCount(stats?.terminal_error_requests) }) }}
        </p>
      </div>
      <div class="card p-4">
        <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
          {{ t('usage.errors.stats.errorRate') }}
        </p>
        <p class="mt-1 text-2xl font-semibold" :class="rateClass">
          {{ loading ? '—' : formatRate(stats?.error_rate) }}
        </p>
      </div>
      <div class="card p-4">
        <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
          {{ t('usage.errors.stats.rawRows') }}
        </p>
        <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
          {{ loading ? '—' : formatCount(stats?.raw_error_rows) }}
        </p>
        <p class="mt-0.5 text-[11px] text-gray-400">
          {{ t('usage.errors.stats.rawRowsHint') }}
        </p>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-3 lg:grid-cols-3">
      <div class="card p-4">
        <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500">
          {{ t('usage.errors.stats.topStatusCodes') }}
        </p>
        <ul class="space-y-1 text-sm">
          <li v-for="b in stats?.top_status_codes || []" :key="'s-' + b.key" class="flex justify-between gap-2">
            <span class="font-mono text-gray-700 dark:text-gray-300">{{ b.key }}</span>
            <span class="text-gray-500">{{ b.count }}</span>
          </li>
          <li v-if="!loading && !(stats?.top_status_codes?.length)" class="text-xs text-gray-400">—</li>
        </ul>
      </div>
      <div class="card p-4">
        <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500">
          {{ t('usage.errors.stats.topRequestedModels') }}
        </p>
        <ul class="space-y-1 text-sm">
          <li v-for="b in stats?.top_requested_models || []" :key="'r-' + b.key" class="flex justify-between gap-2">
            <span class="truncate font-mono text-xs text-gray-700 dark:text-gray-300" :title="b.key">{{ b.key }}</span>
            <span class="shrink-0 text-gray-500">{{ b.count }}</span>
          </li>
          <li v-if="!loading && !(stats?.top_requested_models?.length)" class="text-xs text-gray-400">—</li>
        </ul>
      </div>
      <div class="card p-4">
        <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500">
          {{ t('usage.errors.stats.topUpstreamModels') }}
        </p>
        <ul class="space-y-1 text-sm">
          <li v-for="b in stats?.top_upstream_models || []" :key="'u-' + b.key" class="flex justify-between gap-2">
            <span class="truncate font-mono text-xs text-gray-700 dark:text-gray-300" :title="b.key">{{ b.key }}</span>
            <span class="shrink-0 text-gray-500">{{ b.count }}</span>
          </li>
          <li v-if="!loading && !(stats?.top_upstream_models?.length)" class="text-xs text-gray-400">—</li>
        </ul>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { OpsErrorLogStats } from '@/api/admin/ops'

const props = defineProps<{
  stats: OpsErrorLogStats | null
  loading?: boolean
}>()

const { t } = useI18n()

const rateClass = computed(() => {
  const rate = props.stats?.error_rate ?? 0
  if (rate >= 0.2) return 'text-red-600 dark:text-red-400'
  if (rate >= 0.05) return 'text-amber-600 dark:text-amber-400'
  return 'text-emerald-600 dark:text-emerald-400'
})

function formatCount(n?: number | null) {
  if (n == null || Number.isNaN(n)) return '0'
  return n.toLocaleString()
}

function formatRate(rate?: number | null) {
  if (rate == null || Number.isNaN(rate)) return '0%'
  return `${(rate * 100).toFixed(2)}%`
}
</script>
