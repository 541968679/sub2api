<template>
  <button
    v-if="state === 'resumed'"
    type="button"
    class="inline-flex shrink-0 items-center rounded px-1 text-[10px] font-medium bg-sky-100 text-sky-800 hover:bg-sky-200 dark:bg-sky-900/40 dark:text-sky-300 dark:hover:bg-sky-900/60"
    data-testid="user-schedule-quality-resumed-chip"
    :title="t('admin.accounts.userSchedule.qualityResumedChipTitle')"
    :disabled="disabled"
    @click.stop="emit('startWindow')"
  >
    {{ t('admin.accounts.userSchedule.qualityResumedChip') }}
  </button>
  <span
    v-else-if="state === 'blocked'"
    class="inline-flex shrink-0 items-center rounded px-1 text-[10px] font-medium bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300"
    data-testid="user-schedule-quality-blocked-chip"
    :title="t('admin.accounts.userSchedule.qualityBlockedChipTitle')"
  >
    {{ t('admin.accounts.userSchedule.qualityBlockedChip') }}
  </span>
  <span
    v-else-if="state === 'configured'"
    class="inline-flex shrink-0 items-center rounded px-1 text-[10px] font-medium bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300"
    data-testid="user-schedule-quality-chip"
  >
    {{ t('admin.accounts.userSchedule.qualityGateChip') }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  scheduleUserQualityChipState,
  type AccountQualityChipStats,
  type ScheduleUserQualityChipInput
} from '@/utils/accountQualityHardClose'

const props = withDefaults(defineProps<{
  user: ScheduleUserQualityChipInput
  stats?: AccountQualityChipStats | null
  disabled?: boolean
}>(), {
  stats: null,
  disabled: false
})

const emit = defineEmits<{
  startWindow: []
}>()

const { t } = useI18n()

const state = computed(() => scheduleUserQualityChipState(props.user, props.stats))
</script>
