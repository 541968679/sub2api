<template>
  <span v-if="props.quotas === undefined" class="text-xs text-gray-400 dark:text-gray-500">…</span>
  <span v-else-if="configured.length === 0" class="text-xs text-gray-400 dark:text-gray-500">
    {{ t('admin.users.platformQuota.cellNotConfigured') }}
  </span>
  <div v-else class="space-y-1 text-xs">
    <div
      v-for="row in configured"
      :key="row.platform"
      class="flex flex-wrap items-center gap-x-2 gap-y-1"
    >
      <span class="w-20 shrink-0 font-mono text-gray-700 dark:text-gray-300">{{ row.platform }}</span>
      <span v-for="window in WINDOWS" :key="window" class="text-gray-500 dark:text-gray-400">
        {{ t(`admin.users.platformQuota.window${capitalize(window)}`) }}
        <span class="text-gray-900 dark:text-white">
          {{ formatUsd(row[`${window}_usage_usd`]) }}/{{ formatLimit(row[`${window}_limit_usd`]) }}
        </span>
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PlatformQuotaItem, PlatformQuotaPlatform } from '@/api/admin/users'

const props = defineProps<{ quotas?: PlatformQuotaItem[] }>()
const { t } = useI18n()

const PLATFORM_ORDER: PlatformQuotaPlatform[] = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok']
const WINDOWS = ['daily', 'weekly', 'monthly'] as const

const configured = computed(() => {
  if (!props.quotas) return []
  return props.quotas
    .filter((quota) =>
      quota.daily_limit_usd != null ||
      quota.weekly_limit_usd != null ||
      quota.monthly_limit_usd != null
    )
    .slice()
    .sort((a, b) => PLATFORM_ORDER.indexOf(a.platform) - PLATFORM_ORDER.indexOf(b.platform))
})

function formatUsd(value: number): string {
  if (!Number.isFinite(value)) return '0'
  return String(Math.round(value * 100) / 100)
}

function formatLimit(value: number | null): string {
  return value == null ? '—' : formatUsd(value)
}

function capitalize(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1)
}
</script>
