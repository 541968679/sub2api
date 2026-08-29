<template>
  <div
    class="flex min-w-0 flex-col gap-0.5"
    data-testid="account-public-quality"
    :data-state="admission"
    :title="cellTitle"
  >
    <div v-if="loading && !view" class="h-3 w-14 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
    <template v-else>
      <div class="flex min-w-0 flex-wrap items-center gap-x-1 gap-y-0.5">
        <span
          class="inline-flex w-fit shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium"
          :class="chipClass"
        >
          {{ stateLabel }}
        </span>
        <template v-if="admission === 'cooling'">
          <span
            v-if="softActive"
            class="shrink-0 text-[10px] text-amber-700 dark:text-amber-300"
            data-testid="account-public-quality-soft"
          >
            {{ t('admin.accounts.publicQuality.soft') }}
          </span>
          <span
            v-if="view?.until"
            class="min-w-0 truncate text-[10px] text-amber-700 dark:text-amber-300"
            data-testid="account-public-quality-remaining"
          >
            {{ t('admin.accounts.publicQuality.coolingRemaining', { minutes: cooldownRemainingMinutes(view.until) }) }}
          </span>
        </template>
      </div>
      <span
        v-if="reasonText"
        class="min-w-0 truncate text-[10px] text-gray-500 dark:text-gray-400"
        :title="reasonText"
        data-testid="account-public-quality-reason"
      >
        {{ reasonText }}
      </span>
      <SmartScheduleAdmissionSwitch
        :admission="admission"
        :paused="view?.state === 'paused'"
        :pinned="view?.state === 'pinned'"
        :disabled="switching"
        :hint="t('admin.accounts.publicQuality.switchStateHint')"
        :state-labels="switchLabels"
        @select="emit('select', $event)"
      />
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PublicScheduleQualityView } from '@/api/admin/accounts'
import SmartScheduleAdmissionSwitch from '@/components/admin/smart-schedule/SmartScheduleAdmissionSwitch.vue'
import {
  cooldownRemainingMinutes,
  type PairAdmissionLiveState,
  type PoolAdmissionState
} from '@/composables/smartSchedulePoolAdmission'
import { formatDateTime } from '@/utils/format'

const props = withDefaults(
  defineProps<{
    view?: PublicScheduleQualityView | null
    loading?: boolean
    switching?: boolean
  }>(),
  {
    view: null,
    loading: false,
    switching: false
  }
)

const emit = defineEmits<{
  select: [state: PairAdmissionLiveState]
}>()

const { t } = useI18n()

const admission = computed<PoolAdmissionState>(() => {
  const view = props.view
  if (!view) return 'selectable'
  if (view.state === 'selectable' && view.will_cool) return 'will_cool'
  if (
    view.state === 'cooling' ||
    view.state === 'paused' ||
    view.state === 'probing' ||
    view.state === 'resumed' ||
    view.state === 'pinned'
  ) {
    return view.state
  }
  return 'selectable'
})

const switchLabels = computed(() => ({
  paused: t('admin.accounts.publicQuality.statePaused'),
  cooling: t('admin.accounts.publicQuality.stateCooling'),
  probing: t('admin.accounts.publicQuality.stateProbing'),
  selectable: t('admin.accounts.publicQuality.stateSelectable'),
  resumed: t('admin.accounts.publicQuality.stateResumed'),
  pinned: t('admin.accounts.publicQuality.statePinned')
}))

const stateLabel = computed(() => {
  switch (admission.value) {
    case 'paused':
      return t('admin.accounts.publicQuality.statePaused')
    case 'cooling':
      return t('admin.accounts.publicQuality.stateCooling')
    case 'probing':
      return t('admin.accounts.publicQuality.stateProbing')
    case 'resumed':
      return t('admin.accounts.publicQuality.stateResumed')
    case 'pinned':
      return t('admin.accounts.publicQuality.statePinned')
    case 'will_cool':
      return t('admin.accounts.publicQuality.stateWillCool')
    default:
      return t('admin.accounts.publicQuality.stateSelectable')
  }
})

const chipClass = computed(() => {
  switch (admission.value) {
    case 'paused':
      return 'bg-slate-200 text-slate-800 dark:bg-slate-800/70 dark:text-slate-200'
    case 'cooling':
      return 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-200'
    case 'will_cool':
      return 'bg-orange-50 text-orange-800 dark:bg-orange-900/30 dark:text-orange-200'
    case 'resumed':
      return 'bg-sky-100 text-sky-800 dark:bg-sky-900/40 dark:text-sky-300'
    case 'pinned':
      return 'bg-indigo-100 text-indigo-800 dark:bg-indigo-900/40 dark:text-indigo-300'
    case 'probing':
      return 'bg-violet-100 text-violet-800 dark:bg-violet-900/40 dark:text-violet-300'
    default:
      return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  }
})

const reasonText = computed(() => {
  const reason = props.view?.reason?.trim()
  return reason || ''
})

const softActive = computed(
  () => admission.value === 'cooling' && Boolean(props.view?.resolved?.soft_cooldown)
)

const cellTitle = computed(() => {
  const view = props.view
  const reason = reasonText.value
  if (admission.value === 'cooling' && view?.until) {
    const until = t('admin.accounts.publicQuality.coolingUntil', { time: formatDateTime(view.until) })
    return reason ? `${until} · ${reason}` : until
  }
  const hint = admission.value === 'will_cool' ? t('admin.accounts.publicQuality.stateWillCoolHint') : ''
  if (hint && reason) return `${hint} · ${reason}`
  return hint || reason
})
</script>
