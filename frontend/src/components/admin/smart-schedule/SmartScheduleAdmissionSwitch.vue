<template>
  <div class="relative">
    <button
      type="button"
      data-testid="smart-schedule-admission-switch"
      class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 transition-colors"
      :class="triggerClass"
      :disabled="disabled"
      :title="t('admin.users.smartSchedule.switchStateHint')"
      @click.stop="toggle"
    >
      <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182m0-4.991v4.99" />
      </svg>
      <span class="text-xs">{{ t('admin.users.smartSchedule.switchState') }}</span>
    </button>
    <Teleport to="body">
      <div
        v-if="open"
        class="fixed inset-0 z-40"
        data-testid="smart-schedule-admission-switch-backdrop"
        @click="close"
      />
      <div
        v-if="open"
        class="fixed z-50 min-w-[10rem] rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-600 dark:bg-dark-800"
        :style="menuStyle"
        role="menu"
        data-testid="smart-schedule-admission-switch-menu"
      >
        <button
          v-for="state in PAIR_ADMISSION_LIVE_STATES"
          :key="state"
          type="button"
          role="menuitem"
          class="flex w-full items-center justify-between gap-3 px-3 py-1.5 text-left text-xs hover:bg-gray-50 dark:hover:bg-dark-700"
          :class="current === state ? 'font-medium text-gray-900 dark:text-gray-100' : 'text-gray-600 dark:text-gray-300'"
          :data-testid="`smart-schedule-admission-${state}`"
          @click.stop="pick(state)"
        >
          <span>{{ stateLabel(state) }}</span>
          <span v-if="current === state" aria-hidden="true">✓</span>
        </button>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  PAIR_ADMISSION_LIVE_STATES,
  pairAdmissionLiveState,
  type PairAdmissionLiveState,
  type PoolAdmissionState
} from '@/composables/smartSchedulePoolAdmission'

const props = withDefaults(defineProps<{
  admission: PoolAdmissionState
  paused?: boolean
  pinned?: boolean
  disabled?: boolean
}>(), {
  paused: false,
  pinned: false,
  disabled: false
})

const emit = defineEmits<{
  select: [state: PairAdmissionLiveState]
}>()

const { t } = useI18n()
const open = ref(false)
const triggerX = ref(0)
const triggerY = ref(0)

const current = computed(() => pairAdmissionLiveState(props.admission, props.paused, props.pinned))

const triggerClass = computed(() => {
  if (props.disabled) return 'cursor-not-allowed text-gray-300 dark:text-gray-600'
  if (current.value === 'paused') {
    return 'bg-slate-100 text-slate-700 hover:bg-slate-200 dark:bg-slate-800/60 dark:text-slate-200'
  }
  if (props.admission === 'cooling' || props.admission === 'will_cool') {
    return 'bg-amber-50 text-amber-700 hover:bg-amber-100 dark:bg-amber-900/30 dark:text-amber-300'
  }
  if (props.admission === 'resumed') {
    return 'bg-sky-50 text-sky-700 hover:bg-sky-100 dark:bg-sky-900/30 dark:text-sky-300'
  }
  if (props.admission === 'pinned' || current.value === 'pinned') {
    return 'bg-indigo-50 text-indigo-700 hover:bg-indigo-100 dark:bg-indigo-900/30 dark:text-indigo-300'
  }
  if (props.admission === 'probing') {
    return 'bg-violet-50 text-violet-700 hover:bg-violet-100 dark:bg-violet-900/30 dark:text-violet-300'
  }
  return 'text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700'
})

const menuStyle = computed(() => ({
  top: `${triggerY.value}px`,
  left: `${triggerX.value}px`
}))

function stateLabel(state: PairAdmissionLiveState) {
  if (state === 'paused') return t('admin.users.smartSchedule.admissionPaused')
  if (state === 'cooling') return t('admin.users.smartSchedule.admissionCooling')
  if (state === 'probing') return t('admin.users.smartSchedule.admissionProbing')
  if (state === 'resumed') return t('admin.users.smartSchedule.admissionResumed')
  if (state === 'pinned') return t('admin.users.smartSchedule.admissionPinned')
  return t('admin.users.smartSchedule.admissionSelectable')
}

function close() {
  open.value = false
}

function toggle(event: MouseEvent) {
  if (props.disabled) return
  if (open.value) {
    close()
    return
  }
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  const menuWidth = 168
  const menuHeight = 248
  triggerX.value = Math.max(8, Math.min(rect.left, window.innerWidth - menuWidth - 8))
  triggerY.value = rect.bottom + 4
  if (triggerY.value + menuHeight > window.innerHeight - 8) {
    triggerY.value = Math.max(8, rect.top - menuHeight - 4)
  }
  open.value = true
}

function pick(state: PairAdmissionLiveState) {
  close()
  emit('select', state)
}
</script>
