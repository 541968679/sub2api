<template>
  <div class="card p-4">
    <div class="mb-3 flex items-center justify-between">
      <div>
        <h3 class="text-sm font-semibold text-gray-700 dark:text-gray-200">
          {{ t('usage.antigravity.title') }}
        </h3>
        <p class="text-xs text-gray-400">{{ t('usage.antigravity.subtitle') }}</p>
      </div>
      <button
        class="btn btn-secondary btn-sm inline-flex items-center gap-1.5"
        :disabled="refreshing"
        @click="onRefresh"
      >
        <svg
          class="h-4 w-4"
          :class="refreshing ? 'animate-spin' : ''"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M4 4v5h.582M20 20v-5h-.581M19.419 9A7.974 7.974 0 0012 5a8 8 0 00-7.938 7M4.582 15A7.974 7.974 0 0012 19a8 8 0 007.938-7"
          />
        </svg>
        {{ t('usage.antigravity.refreshNow') }}
      </button>
    </div>

    <div v-if="!stats" class="py-6 text-center text-xs text-gray-400">
      {{ t('usage.antigravity.loading') }}
    </div>

    <div v-else-if="insufficientSnapshot" class="py-6 text-center text-xs text-amber-600 dark:text-amber-400">
      {{ t('usage.antigravity.insufficientSnapshot') }}
    </div>

    <div v-else class="grid grid-cols-2 gap-4 lg:grid-cols-4">
      <div>
        <p class="text-xs font-medium text-gray-500">{{ t('usage.antigravity.creditsConsumed') }}</p>
        <p class="text-xl font-bold">{{ formatNumber(stats.credits_consumed) }}</p>
        <p v-if="creditTypeBreakdown" class="truncate text-xs text-gray-400" :title="creditTypeBreakdown">
          {{ creditTypeBreakdown }}
        </p>
      </div>
      <div>
        <p class="text-xs font-medium text-gray-500">{{ t('usage.antigravity.quotaUsed') }}</p>
        <p class="text-xl font-bold text-green-600">${{ stats.quota_used_usd.toFixed(4) }}</p>
        <p class="text-xs text-gray-400">{{ t('usage.antigravity.calls') }}: {{ stats.call_count.toLocaleString() }}</p>
      </div>
      <div>
        <p class="text-xs font-medium text-gray-500">{{ t('usage.antigravity.quotaPerCredit') }}</p>
        <p class="text-xl font-bold">
          <span v-if="stats.quota_per_credit != null">${{ stats.quota_per_credit.toFixed(6) }}</span>
          <span v-else class="text-gray-300">—</span>
        </p>
        <p class="text-xs text-gray-400">{{ t('usage.antigravity.perCredit') }}</p>
      </div>
      <div>
        <p class="text-xs font-medium text-gray-500">{{ t('usage.antigravity.callsPerCredit') }}</p>
        <p class="text-xl font-bold">
          <span v-if="stats.calls_per_credit != null">{{ stats.calls_per_credit.toFixed(3) }}</span>
          <span v-else class="text-gray-300">—</span>
        </p>
        <p class="text-xs text-gray-400">{{ t('usage.antigravity.perCredit') }}</p>
      </div>
    </div>

    <p v-if="stats" class="mt-2 text-xs text-gray-400">
      {{ t('usage.antigravity.samplingMeta', { emails: stats.emails_sampled, snapshots: stats.snapshot_count }) }}
      <span v-if="throttleHint" class="ml-2 text-amber-500">· {{ throttleHint }}</span>
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AntigravityUsageRatio } from '@/api/admin/usage'

const props = defineProps<{
  stats: AntigravityUsageRatio | null
}>()

const emit = defineEmits<{
  (e: 'refresh'): void
}>()

const { t } = useI18n()

const refreshing = ref(false)
const throttleHint = ref('')

const insufficientSnapshot = computed(() => {
  if (!props.stats) return false
  return props.stats.snapshot_count < 2 || props.stats.credits_consumed === 0
})

const creditTypeBreakdown = computed(() => {
  if (!props.stats) return ''
  const entries = Object.entries(props.stats.credits_by_type).filter(([, v]) => v > 0)
  if (entries.length === 0) return ''
  return entries.map(([k, v]) => `${k}: ${formatNumber(v)}`).join(' · ')
})

const formatNumber = (v: number) => {
  if (v >= 1e9) return (v / 1e9).toFixed(2) + 'B'
  if (v >= 1e6) return (v / 1e6).toFixed(2) + 'M'
  if (v >= 1e3) return (v / 1e3).toFixed(2) + 'K'
  return v.toFixed(2)
}

const onRefresh = async () => {
  if (refreshing.value) return
  refreshing.value = true
  throttleHint.value = ''
  try {
    emit('refresh')
  } finally {
    // parent owns the actual request lifecycle; unblock button after a tick
    setTimeout(() => { refreshing.value = false }, 300)
  }
}

// Expose hint setter for parent to signal throttled state
defineExpose({
  setThrottleHint: (msg: string) => { throttleHint.value = msg }
})
</script>
