<template>
  <div class="mb-1 space-y-0.5 text-[11px] leading-tight" data-testid="account-usage-spend">
    <div v-if="loading && todayCost == null && monthCost == null" class="space-y-1">
      <div class="h-3 w-24 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
      <div class="h-3 w-20 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
    </div>
    <template v-else>
      <div class="flex items-center gap-1" :title="t('admin.accounts.usageWindow.todaySpendHint')">
        <span class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.usageWindow.todaySpend') }}</span>
        <span class="font-medium tabular-nums text-gray-800 dark:text-gray-100">${{ formatSpend(todayCost) }}</span>
      </div>
      <div class="flex items-center gap-1" :title="t('admin.accounts.usageWindow.monthSpendHint')">
        <span class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.usageWindow.monthSpend') }}</span>
        <span class="font-medium tabular-nums text-gray-800 dark:text-gray-100">${{ formatSpend(monthCost) }}</span>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

withDefaults(
  defineProps<{
    todayCost?: number | null
    monthCost?: number | null
    loading?: boolean
  }>(),
  {
    todayCost: null,
    monthCost: null,
    loading: false
  }
)

const { t } = useI18n()

function formatSpend(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '0.00'
  return value.toFixed(2)
}
</script>
