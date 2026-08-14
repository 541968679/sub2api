<template>
  <div v-if="hasConfig" class="relative max-w-56">
    <div class="flex flex-wrap items-center gap-1 max-h-14 overflow-hidden">
      <span
        v-for="user in displayUsers"
        :key="user.id"
        class="inline-flex max-w-36 items-center gap-1 truncate rounded-md bg-gray-100 px-1.5 py-0.5 text-xs text-gray-700 dark:bg-dark-600 dark:text-gray-300"
        :title="userChipTitle(user)"
      >
        <span
          v-if="user.allow"
          class="inline-flex items-center rounded px-1 text-[10px] font-medium bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300"
        >
          {{ t('admin.accounts.userSchedule.modeAllow') }}
        </span>
        <span
          v-if="user.deny"
          class="inline-flex items-center rounded px-1 text-[10px] font-medium bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300"
        >
          {{ t('admin.accounts.userSchedule.modeDeny') }}
        </span>
        <span class="truncate">{{ user.email || `#${user.id}` }}</span>
        <AccountInlineNumberCell
          :model-value="user.max_concurrency ?? 0"
          :min="0"
          :blank-when-zero="true"
          :disabled="disabled"
          :hint="t('admin.accounts.userSchedule.concurrencyHint')"
          @save="(value) => emitSave(user.id, value)"
        />
      </span>
      <button
        v-if="hiddenCount > 0"
        ref="moreButtonRef"
        type="button"
        class="inline-flex items-center gap-0.5 rounded-md px-1.5 py-0.5 text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500 transition-colors cursor-pointer whitespace-nowrap"
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
          class="fixed z-50 min-w-48 max-w-96 rounded-lg border border-gray-200 bg-white p-3 shadow-lg dark:border-dark-600 dark:bg-dark-800"
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
          <div class="flex flex-wrap gap-1.5 max-h-64 overflow-y-auto">
            <span
              v-for="user in users"
              :key="user.id"
              class="inline-flex max-w-full items-center gap-1 truncate rounded-md bg-gray-100 px-1.5 py-0.5 text-xs text-gray-700 dark:bg-dark-600 dark:text-gray-300"
              :title="userChipTitle(user)"
            >
              <span
                v-if="user.allow"
                class="inline-flex items-center rounded px-1 text-[10px] font-medium bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300"
              >
                {{ t('admin.accounts.userSchedule.modeAllow') }}
              </span>
              <span
                v-if="user.deny"
                class="inline-flex items-center rounded px-1 text-[10px] font-medium bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300"
              >
                {{ t('admin.accounts.userSchedule.modeDeny') }}
              </span>
              <span class="truncate">{{ user.email || `#${user.id}` }}</span>
              <AccountInlineNumberCell
                :model-value="user.max_concurrency ?? 0"
                :min="0"
                :blank-when-zero="true"
                :disabled="disabled"
                :hint="t('admin.accounts.userSchedule.concurrencyHint')"
                @save="(value) => emitSave(user.id, value)"
              />
            </span>
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
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account, AccountScheduleUser } from '@/types'
import AccountInlineNumberCell from './AccountInlineNumberCell.vue'

const props = withDefaults(defineProps<{
  account: Account
  maxDisplay?: number
  disabled?: boolean
}>(), {
  maxDisplay: 4,
  disabled: false
})

const emit = defineEmits<{
  save: [payload: { userId: number; maxConcurrency: number | null }]
}>()

const { t } = useI18n()
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

const handleKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Escape') showPopover.value = false
}

onMounted(() => window.addEventListener('keydown', handleKeydown))
onUnmounted(() => window.removeEventListener('keydown', handleKeydown))
</script>
