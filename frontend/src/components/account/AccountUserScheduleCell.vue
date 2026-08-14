<template>
  <div v-if="hasConfig" class="relative min-w-[11rem] max-w-64">
    <div class="flex flex-col gap-1">
      <div
        v-for="user in displayUsers"
        :key="user.id"
        class="flex flex-col gap-1"
      >
        <div
          class="flex items-center gap-1.5"
          :title="userChipTitle(user)"
        >
        <span
          v-if="user.allow"
          class="inline-flex shrink-0 items-center rounded px-1 text-[10px] font-medium bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300"
        >
          {{ t('admin.accounts.userSchedule.modeAllow') }}
        </span>
        <span
          v-if="user.deny"
          class="inline-flex shrink-0 items-center rounded px-1 text-[10px] font-medium bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300"
        >
          {{ t('admin.accounts.userSchedule.modeDeny') }}
        </span>
        <AccountUserQualityChip
          :user="user"
          :stats="qualityStats"
          :disabled="disabled"
          @start-window="emit('startQualityWindow', user.id)"
        />
        <span class="min-w-0 flex-1 truncate text-xs text-gray-700 dark:text-gray-300">
          {{ user.email || `#${user.id}` }}
        </span>
        <AccountInlineNumberCell
          :model-value="user.max_concurrency ?? 0"
          :min="0"
          :blank-when-zero="true"
          :disabled="disabled"
          :hint="t('admin.accounts.userSchedule.concurrencyHint')"
          @save="(value) => emitSave(user.id, value)"
        />
        <button
          type="button"
          class="shrink-0 rounded px-1 text-[10px] font-medium text-amber-700 hover:bg-amber-50 dark:text-amber-300 dark:hover:bg-amber-900/30"
          :data-testid="`user-schedule-quality-edit-${user.id}`"
          :disabled="disabled"
          @click.stop="toggleQualityEditor(user.id)"
        >
          {{ t('admin.accounts.userSchedule.qualityGateChip') }}
        </button>
        </div>
      <div
        v-if="editingQualityUserId === user.id"
        class="ml-1 grid grid-cols-2 gap-1 rounded border border-amber-200 p-2 dark:border-amber-900/50"
        :data-testid="`user-schedule-quality-editor-${user.id}`"
      >
        <label class="space-y-0.5">
          <span class="block text-[10px] text-gray-500 dark:text-gray-400">{{ t('admin.accounts.userSchedule.qualityMaxP50') }}</span>
          <input
            v-model="qualityDraft.maxP50"
            type="number"
            min="1"
            class="input input-sm w-full"
            data-testid="user-schedule-quality-p50"
          />
        </label>
        <label class="space-y-0.5">
          <span class="block text-[10px] text-gray-500 dark:text-gray-400">{{ t('admin.accounts.userSchedule.qualityMinSuccess') }}</span>
          <input
            v-model="qualityDraft.successPercent"
            type="number"
            min="0"
            max="100"
            step="0.1"
            class="input input-sm w-full"
            data-testid="user-schedule-quality-success"
          />
        </label>
        <label class="space-y-0.5">
          <span class="block text-[10px] text-gray-500 dark:text-gray-400">{{ t('admin.accounts.userSchedule.qualityMinSuccessSamples') }}</span>
          <input
            v-model="qualityDraft.minSuccessSamples"
            type="number"
            min="1"
            class="input input-sm w-full"
            data-testid="user-schedule-quality-success-samples"
          />
        </label>
        <label class="space-y-0.5">
          <span class="block text-[10px] text-gray-500 dark:text-gray-400">{{ t('admin.accounts.userSchedule.qualityMinTtftSamples') }}</span>
          <input
            v-model="qualityDraft.minTtftSamples"
            type="number"
            min="1"
            class="input input-sm w-full"
            data-testid="user-schedule-quality-ttft-samples"
          />
        </label>
        <select v-model="qualityDraft.condition" class="input input-sm col-span-2">
          <option value="or">{{ t('admin.accounts.userSchedule.qualityConditionOr') }}</option>
          <option value="and">{{ t('admin.accounts.userSchedule.qualityConditionAnd') }}</option>
        </select>
        <button
          type="button"
          class="btn btn-secondary btn-xs"
          data-testid="user-schedule-quality-apply-template"
          :disabled="qualityTemplateBusy || disabled"
          @click.stop="applyUserQualityTemplate"
        >
          {{ t('admin.accounts.stability.applyTemplate') }}
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-xs"
          data-testid="user-schedule-quality-save-template"
          :disabled="qualityTemplateBusy || disabled"
          @click.stop="saveUserQualityTemplate"
        >
          {{ t('admin.accounts.stability.saveTemplate') }}
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-xs"
          data-testid="user-schedule-quality-resume"
          :disabled="disabled"
          @click.stop="emitQualityResume(user.id)"
        >
          {{ t('admin.accounts.userSchedule.qualityResume') }}
        </button>
        <button type="button" class="btn btn-secondary btn-xs" data-testid="user-schedule-quality-clear" @click="emitQualityClear(user.id)">
          {{ t('admin.accounts.userSchedule.qualityClear') }}
        </button>
        <button type="button" class="btn btn-primary btn-xs" data-testid="user-schedule-quality-save" @click="emitQualitySave(user.id)">
          {{ t('admin.accounts.userSchedule.qualitySave') }}
        </button>
      </div>
      </div>
      <button
        v-if="hiddenCount > 0"
        ref="moreButtonRef"
        type="button"
        class="self-start inline-flex items-center gap-0.5 rounded-md px-1.5 py-0.5 text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500 transition-colors cursor-pointer whitespace-nowrap"
        @click.stop="showPopover = !showPopover"
      >
        <span>+{{ hiddenCount }}</span>
      </button>
    </div>

    <Teleport to="body">
      <Transition
        enter-active-class="transition duration-150 ease-out"
        enter-from-class="opacity-0 scale-95"
        enter-to-class="opacity-100 scale-100"
        leave-active-class="transition duration-100 ease-in"
        leave-from-class="opacity-100 scale-100"
        leave-to-class="opacity-0 scale-95"
      >
        <div
          v-if="showPopover"
          ref="popoverRef"
          class="fixed z-50 min-w-64 max-w-96 rounded-lg border border-gray-200 bg-white p-3 shadow-lg dark:border-dark-600 dark:bg-dark-800"
          :style="popoverStyle"
        >
          <div class="mb-2 flex items-center justify-between">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.userSchedule.userCountTotal', { count: users.length }) }}
            </span>
            <button
              type="button"
              class="rounded p-0.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-300"
              @click="showPopover = false"
            >
              <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
          <div class="flex max-h-64 flex-col gap-1.5 overflow-y-auto">
            <div
              v-for="user in users"
              :key="user.id"
              class="flex items-center gap-1.5"
              :title="userChipTitle(user)"
            >
              <span
                v-if="user.allow"
                class="inline-flex shrink-0 items-center rounded px-1 text-[10px] font-medium bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300"
              >
                {{ t('admin.accounts.userSchedule.modeAllow') }}
              </span>
              <span
                v-if="user.deny"
                class="inline-flex shrink-0 items-center rounded px-1 text-[10px] font-medium bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300"
              >
                {{ t('admin.accounts.userSchedule.modeDeny') }}
              </span>
              <AccountUserQualityChip
                :user="user"
                :stats="qualityStats"
                :disabled="disabled"
                @start-window="emit('startQualityWindow', user.id)"
              />
              <span class="min-w-0 flex-1 truncate text-xs text-gray-700 dark:text-gray-300">
                {{ user.email || `#${user.id}` }}
              </span>
              <AccountInlineNumberCell
                :model-value="user.max_concurrency ?? 0"
                :min="0"
                :blank-when-zero="true"
                :disabled="disabled"
                :hint="t('admin.accounts.userSchedule.concurrencyHint')"
                @save="(value) => emitSave(user.id, value)"
              />
              <button
                type="button"
                class="shrink-0 rounded px-1 text-[10px] font-medium text-amber-700 hover:bg-amber-50 dark:text-amber-300 dark:hover:bg-amber-900/30"
                :disabled="disabled"
                @click.stop="toggleQualityEditor(user.id)"
              >
                {{ t('admin.accounts.userSchedule.qualityGateChip') }}
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <div
      v-if="showPopover"
      class="fixed inset-0 z-40"
      @click="showPopover = false"
    />
  </div>
  <span v-else class="text-sm text-gray-400 dark:text-dark-500">—</span>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account, AccountScheduleUser, UserQualityGatePatch } from '@/types'
import type { AccountQualityStats } from '@/api/admin/accounts'
import AccountInlineNumberCell from './AccountInlineNumberCell.vue'
import AccountUserQualityChip from './AccountUserQualityChip.vue'
import { useQualityThresholdTemplate } from '@/composables/useQualityThresholdTemplate'
import {
  applyQualityGateFormToDraft,
  optionalNumber,
  percentToSuccessRate,
  qualityGateFormFromDraft,
  successRateToPercent
} from '@/utils/accountQualityHardClose'

const props = withDefaults(defineProps<{
  account: Account
  qualityStats?: AccountQualityStats | null
  maxDisplay?: number
  disabled?: boolean
}>(), {
  qualityStats: null,
  maxDisplay: 4,
  disabled: false
})

const emit = defineEmits<{
  save: [payload: { userId: number; maxConcurrency: number | null }]
  saveQuality: [payload: UserQualityGatePatch]
  resumeQuality: [userId: number]
  startQualityWindow: [userId: number]
}>()

const { t } = useI18n()
const {
  templateBusy: qualityTemplateBusy,
  applyQualityTemplate,
  saveQualityTemplate
} = useQualityThresholdTemplate()
const moreButtonRef = ref<HTMLElement | null>(null)
const popoverRef = ref<HTMLElement | null>(null)
const showPopover = ref(false)

const users = computed<AccountScheduleUser[]>(() => props.account?.schedule_users ?? [])

const hasConfig = computed(() => users.value.length > 0)

const displayUsers = computed(() => {
  if (users.value.length <= props.maxDisplay) return users.value
  return users.value.slice(0, props.maxDisplay - 1)
})

const hiddenCount = computed(() => {
  if (users.value.length <= props.maxDisplay) return 0
  return users.value.length - (props.maxDisplay - 1)
})

const popoverStyle = computed(() => {
  if (!moreButtonRef.value) return {}
  const rect = moreButtonRef.value.getBoundingClientRect()
  const viewportHeight = window.innerHeight
  const viewportWidth = window.innerWidth
  let top = rect.bottom + 8
  let left = rect.left
  if (top + 280 > viewportHeight) {
    top = Math.max(8, rect.top - 280)
  }
  if (left + 384 > viewportWidth) {
    left = Math.max(8, viewportWidth - 392)
  }
  return { top: `${top}px`, left: `${left}px` }
})

function userChipTitle(user: AccountScheduleUser): string {
  const email = user.email || `#${user.id}`
  return user.deleted ? `${email} (${t('admin.accounts.userSchedule.deleted')})` : email
}

function emitSave(userId: number, value: number) {
  emit('save', {
    userId,
    maxConcurrency: value >= 1 ? Math.trunc(value) : null
  })
}

const editingQualityUserId = ref<number | null>(null)
const qualityDraft = reactive({
  maxP50: '' as string | number,
  successPercent: '' as string | number,
  minSuccessSamples: '' as string | number,
  minTtftSamples: '' as string | number,
  condition: 'or' as 'or' | 'and'
})

function toggleQualityEditor(userId: number) {
  if (editingQualityUserId.value === userId) {
    editingQualityUserId.value = null
    return
  }
  const user = users.value.find((item) => item.id === userId)
  qualityDraft.maxP50 = user?.quality_max_p50_ttft_ms ?? ''
  qualityDraft.successPercent = successRateToPercent(user?.quality_min_success_rate) ?? ''
  qualityDraft.minSuccessSamples = user?.quality_min_success_samples ?? ''
  qualityDraft.minTtftSamples = user?.quality_min_ttft_samples ?? ''
  qualityDraft.condition = user?.quality_condition === 'and' ? 'and' : 'or'
  editingQualityUserId.value = userId
}

function emitQualitySave(userId: number) {
  const maxP50 = optionalNumber(qualityDraft.maxP50)
  const minSuccessRate = percentToSuccessRate(qualityDraft.successPercent)
  if (maxP50 == null && minSuccessRate == null) {
    emitQualityClear(userId)
    return
  }
  emit('saveQuality', {
    user_id: userId,
    quality_max_p50_ttft_ms: maxP50,
    quality_min_success_rate: minSuccessRate,
    quality_min_success_samples: optionalNumber(qualityDraft.minSuccessSamples),
    quality_min_ttft_samples: optionalNumber(qualityDraft.minTtftSamples),
    quality_condition: qualityDraft.condition
  })
  editingQualityUserId.value = null
}

function emitQualityResume(userId: number) {
  emit('resumeQuality', userId)
}

function applyUserQualityTemplate() {
  void applyQualityTemplate((fields) => {
    applyQualityGateFormToDraft(qualityDraft, fields)
  })
}

function saveUserQualityTemplate() {
  void saveQualityTemplate(qualityGateFormFromDraft(qualityDraft))
}

function emitQualityClear(userId: number) {
  emit('saveQuality', {
    user_id: userId,
    quality_max_p50_ttft_ms: null,
    quality_min_success_rate: null,
    quality_min_success_samples: null,
    quality_min_ttft_samples: null,
    quality_condition: null
  })
  editingQualityUserId.value = null
}

const handleKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Escape') showPopover.value = false
}

onMounted(() => window.addEventListener('keydown', handleKeydown))
onUnmounted(() => window.removeEventListener('keydown', handleKeydown))
</script>
