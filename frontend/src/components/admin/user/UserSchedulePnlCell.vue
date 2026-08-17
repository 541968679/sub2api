<template>
  <button
    type="button"
    class="w-full min-w-[6.5rem] rounded-md px-0.5 py-0.5 text-left transition-colors hover:bg-gray-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-700"
    data-testid="user-schedule-pnl-cell"
    :aria-label="t('admin.users.schedulePnl.openTrend')"
    :title="t('admin.users.schedulePnl.openTrend')"
    @click="emit('click')"
  >
    <div v-if="loading && !hasData" class="h-8 w-16 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
    <div v-else-if="!hasData" class="text-sm text-gray-400 dark:text-dark-500">—</div>
    <div v-else class="flex flex-col gap-0.5 font-mono text-[11px] leading-4">
      <div class="flex flex-col gap-0.5">
        <div class="flex items-baseline gap-1">
          <span class="font-sans text-[10px] text-gray-400 dark:text-gray-500">{{ t('admin.users.schedulePnl.today') }}</span>
          <span class="text-sm font-medium text-gray-900 dark:text-white">{{ today.profit }}</span>
          <span class="text-gray-500 dark:text-gray-400">{{ today.margin }}</span>
        </div>
        <div class="text-[10px] text-gray-400 dark:text-gray-500">
          {{ t('admin.users.schedulePnl.revenue') }} {{ today.revenue }}
          · {{ t('admin.users.schedulePnl.cost') }} {{ today.cost }}
        </div>
      </div>
      <div class="flex flex-col gap-0.5 text-gray-500 dark:text-gray-400">
        <div class="flex items-baseline gap-1">
          <span class="font-sans text-[10px]">{{ t('admin.users.schedulePnl.sevenDay') }}</span>
          <span>{{ seven.profit }}</span>
          <span>{{ seven.margin }}</span>
        </div>
        <div class="text-[10px]">
          {{ t('admin.users.schedulePnl.revenue') }} {{ seven.revenue }}
          · {{ t('admin.users.schedulePnl.cost') }} {{ seven.cost }}
        </div>
      </div>
    </div>
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SchedulePnlSummary } from '@/api/admin/users'
import {
  formatSchedulePnlWindow,
  hasSchedulePnlSummary
} from '@/composables/schedulePnl'

const props = withDefaults(
  defineProps<{
    summary?: SchedulePnlSummary | null
    loading?: boolean
  }>(),
  { summary: null, loading: false }
)

const emit = defineEmits<{ click: [] }>()
const { t } = useI18n()

const hasData = computed(() => hasSchedulePnlSummary(props.summary))
const today = computed(() => formatSchedulePnlWindow(props.summary?.today))
const seven = computed(() => formatSchedulePnlWindow(props.summary?.seven_day))
</script>
