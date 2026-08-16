<template>
  <div class="group relative" data-testid="user-burn-rate-cell">
    <span
      class="font-mono text-sm tabular-nums"
      :class="
        amount > 0
          ? 'font-medium text-amber-700 dark:text-amber-300'
          : 'text-gray-500 dark:text-dark-400'
      "
    >
      {{ display }}
    </span>
    <div
      class="pointer-events-none absolute bottom-full left-0 z-50 mb-1.5 whitespace-nowrap rounded bg-gray-900 px-2 py-1 text-xs text-white opacity-0 shadow-lg transition-opacity duration-75 group-hover:opacity-100 dark:bg-dark-600"
    >
      {{ t('admin.users.burnRateCellTip') }}
      <div class="absolute left-3 top-full border-4 border-transparent border-t-gray-900 dark:border-t-dark-600"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { BatchUserBurnRateStats } from '@/api/admin/dashboard'
import {
  formatAdminUserBurnRateAmount,
  formatAdminUserBurnRateDisplay,
  type AdminUserBurnRateUnit
} from '@/composables/adminUserListRow'

const props = withDefaults(
  defineProps<{
    stats?: BatchUserBurnRateStats | null
    unit?: AdminUserBurnRateUnit
  }>(),
  {
    stats: null,
    unit: 'hour'
  }
)

const { t } = useI18n()

const perHour = computed(() => props.stats?.burn_rate_per_hour ?? 0)
const amount = computed(() => formatAdminUserBurnRateAmount(perHour.value, props.unit))
const display = computed(() => formatAdminUserBurnRateDisplay(perHour.value, props.unit))
</script>
