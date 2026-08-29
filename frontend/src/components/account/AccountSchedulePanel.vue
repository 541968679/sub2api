<template>
  <section
    class="space-y-4 rounded-xl border border-gray-200 bg-white p-3 dark:border-dark-600 dark:bg-dark-800/40 sm:p-4"
    data-testid="edit-account-schedule-panel"
  >
    <div class="grid gap-3 sm:grid-cols-2">
      <label class="flex items-start justify-between gap-3 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
        <div>
          <p class="text-sm font-medium text-gray-900 dark:text-white">
            {{ t('admin.accounts.accountSchedule.master') }}
          </p>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.accountSchedule.masterHint') }}
          </p>
        </div>
        <button
          type="button"
          data-testid="account-schedule-master"
          :disabled="busy"
          :class="toggleClass(localSchedulable)"
          @click="toggleMaster"
        >
          <span :class="knobClass(localSchedulable)" />
        </button>
      </label>
      <label class="flex items-start justify-between gap-3 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
        <div>
          <p class="text-sm font-medium text-gray-900 dark:text-white">
            {{ t('admin.accounts.accountSchedule.publicPool') }}
          </p>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.accountSchedule.publicPoolHint') }}
          </p>
        </div>
        <button
          type="button"
          data-testid="account-schedule-public"
          :disabled="busy"
          :class="toggleClass(localPublic)"
          @click="togglePublic"
        >
          <span :class="knobClass(localPublic)" />
        </button>
      </label>
    </div>

    <div>
      <div class="mb-2 flex flex-wrap items-center gap-2">
        <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.accounts.accountSchedule.smartPool') }}
        </h4>
        <div class="flex flex-wrap gap-1" data-testid="account-schedule-platforms">
          <button
            v-for="platform in platforms"
            :key="platform"
            type="button"
            class="rounded-md px-2 py-1 text-xs"
            :class="
              activePlatform === platform
                ? 'bg-primary-600 text-white'
                : 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200'
            "
            :data-testid="`account-schedule-platform-${platform}`"
            @click="activePlatform = platform"
          >
            {{ platformLabel(platform) }}
          </button>
        </div>
      </div>
      <p class="mb-3 text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.accounts.accountSchedule.smartPoolHint') }}
      </p>

      <div class="mb-3 flex flex-wrap items-end gap-2">
        <div class="min-w-[16rem] flex-1">
          <OpenAIFastPolicyUserSelector
            v-model="addUserIds"
            :known-users="knownUsers"
            @select="onPickUser"
          />
        </div>
        <SmartScheduleAdmissionSwitch
          :admission="'selectable'"
          :disabled="busy || selectedIds.length === 0"
          data-testid="account-schedule-batch-admission"
          @select="applyBatchAdmission"
        />
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          data-testid="account-schedule-remove-selected"
          :disabled="busy || selectedIds.length === 0"
          @click="removeSelected"
        >
          {{ t('admin.accounts.accountSchedule.removeSelected') }}
        </button>
      </div>

      <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-600">
        <table class="min-w-full text-sm">
          <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-700 dark:text-gray-400">
            <tr>
              <th class="px-3 py-2">
                <input
                  type="checkbox"
                  data-testid="account-schedule-select-all"
                  :checked="allSelected"
                  :disabled="members.length === 0"
                  @change="toggleSelectAll"
                />
              </th>
              <th class="px-3 py-2">{{ t('common.email') }}</th>
              <th class="px-3 py-2">{{ t('admin.accounts.accountSchedule.poolStatus') }}</th>
              <th class="px-3 py-2">{{ t('admin.users.smartSchedule.switchState') }}</th>
              <th class="px-3 py-2" />
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading">
              <td colspan="5" class="px-3 py-6 text-center text-gray-500">{{ t('common.loading') }}</td>
            </tr>
            <tr v-else-if="members.length === 0">
              <td
                colspan="5"
                class="px-3 py-6 text-center text-gray-500"
                data-testid="account-schedule-empty"
              >
                {{ t('admin.accounts.accountSchedule.empty') }}
              </td>
            </tr>
            <tr
              v-for="row in members"
              :key="`${row.platform}-${row.user_id}`"
              class="border-t border-gray-100 dark:border-dark-600"
              :data-testid="`account-schedule-member-${row.user_id}`"
            >
              <td class="px-3 py-2">
                <input
                  type="checkbox"
                  :data-testid="`account-schedule-select-${row.user_id}`"
                  :checked="selectedIds.includes(row.user_id)"
                  @change="toggleSelected(row.user_id)"
                />
              </td>
              <td class="px-3 py-2">
                <div class="min-w-0 truncate font-medium text-gray-900 dark:text-white">
                  {{ row.email || `#${row.user_id}` }}
                </div>
                <div class="text-xs text-gray-400">
                  #{{ row.user_id }}
                  <span v-if="row.deleted"> · {{ t('admin.accounts.accountSchedule.deleted') }}</span>
                </div>
              </td>
              <td class="px-3 py-2 text-xs">
                {{
                  row.enabled
                    ? t('admin.accounts.accountSchedule.enabled')
                    : t('admin.accounts.accountSchedule.disabled')
                }}
              </td>
              <td class="px-3 py-2">
                <SmartScheduleAdmissionSwitch
                  :admission="memberAdmission(row).state"
                  :paused="row.paused"
                  :pinned="Boolean(row.pinned)"
                  :disabled="busy"
                  @select="(state) => applyAdmission(row.user_id, state)"
                />
              </td>
              <td class="px-3 py-2 text-right">
                <button
                  type="button"
                  class="text-xs text-red-600 hover:text-red-700"
                  :data-testid="`account-schedule-remove-${row.user_id}`"
                  :disabled="busy"
                  @click="removeMember(row.user_id)"
                >
                  {{ t('admin.accounts.accountSchedule.remove') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Account } from '@/types'
import type { SimpleUser } from '@/api/admin/usage'
import type { SmartScheduleAccountMembership } from '@/api/admin/accounts'
import OpenAIFastPolicyUserSelector from '@/views/admin/settings/OpenAIFastPolicyUserSelector.vue'
import SmartScheduleAdmissionSwitch from '@/components/admin/smart-schedule/SmartScheduleAdmissionSwitch.vue'
import {
  resolvePoolAdmission,
  type PairAdmissionLiveState
} from '@/composables/smartSchedulePoolAdmission'

const props = defineProps<{
  account: Account
}>()

const emit = defineEmits<{
  updated: [account: Account]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const allPlatforms = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok'] as const
const platforms = computed(() => schedulePlatforms(props.account.platform))
const activePlatform = ref(defaultPlatform(props.account.platform))
const members = ref<SmartScheduleAccountMembership[]>([])
const selectedIds = ref<number[]>([])
const addUserIds = ref<number[]>([])
const loading = ref(false)
const busy = ref(false)
const localSchedulable = ref(Boolean(props.account.schedulable))
const localPublic = ref(props.account.public_schedulable !== false)

const knownUsers = computed<SimpleUser[]>(() =>
  members.value.map((row) => ({
    id: row.user_id,
    email: row.email,
    deleted: row.deleted
  }))
)

const allSelected = computed(
  () => members.value.length > 0 && selectedIds.value.length === members.value.length
)

watch(
  () => [props.account.id, props.account.schedulable, props.account.public_schedulable] as const,
  () => {
    localSchedulable.value = Boolean(props.account.schedulable)
    localPublic.value = props.account.public_schedulable !== false
  }
)

watch(
  () => [props.account.id, props.account.platform] as const,
  () => {
    const next = defaultPlatform(props.account.platform)
    if (activePlatform.value !== next) {
      activePlatform.value = next
    }
  }
)

watch(
  () => [props.account.id, activePlatform.value] as const,
  () => {
    selectedIds.value = []
    void loadMembers()
  },
  { immediate: true }
)

function schedulePlatforms(platform: string) {
  if (platform === 'openai') {
    return ['openai', 'antigravity'] as const
  }
  if ((allPlatforms as readonly string[]).includes(platform)) {
    return [platform] as const
  }
  return ['anthropic'] as const
}

function defaultPlatform(platform: string) {
  const allowed = schedulePlatforms(platform)
  return (allowed as readonly string[]).includes(platform) ? platform : allowed[0]
}

function platformLabel(platform: string) {
  return t(`admin.groups.platforms.${platform}`)
}

function toggleClass(on: boolean) {
  return [
    'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors',
    on ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
  ]
}

function knobClass(on: boolean) {
  return [
    'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow transition',
    on ? 'translate-x-5' : 'translate-x-0'
  ]
}

function memberAdmission(row: SmartScheduleAccountMembership) {
  const resumeActive =
    Boolean(row.resume_chip_until) && new Date(row.resume_chip_until as string).getTime() > Date.now()
  return resolvePoolAdmission({
    account: {
      status: props.account.status,
      schedulable: localSchedulable.value,
      temp_unschedulable_until: props.account.temp_unschedulable_until,
      rate_limit_reset_at: props.account.rate_limit_reset_at
    },
    pairCap: null,
    pairCurrent: row.current_concurrency ?? 0,
    cooldownUntil: row.cooldown_until,
    paused: row.paused,
    probing: row.probing,
    pinned: row.pinned,
    qualityHint: resumeActive ? 'resumed' : null
  })
}

async function loadMembers() {
  loading.value = true
  try {
    members.value = await adminAPI.accounts.listSmartScheduleMemberships(
      props.account.id,
      activePlatform.value
    )
    const visible = new Set(members.value.map((row) => row.user_id))
    selectedIds.value = selectedIds.value.filter((id) => visible.has(id))
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.accountSchedule.loadFailed'))
    members.value = []
  } finally {
    loading.value = false
  }
}

async function toggleMaster() {
  busy.value = true
  try {
    const next = !localSchedulable.value
    const updated = await adminAPI.accounts.setSchedulable(props.account.id, next)
    localSchedulable.value = updated.schedulable
    emit('updated', updated)
    appStore.showSuccess(
      next ? t('admin.accounts.schedulableEnabled') : t('admin.accounts.schedulableDisabled')
    )
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.failedToUpdate'))
  } finally {
    busy.value = false
  }
}

async function togglePublic() {
  busy.value = true
  try {
    const next = !localPublic.value
    const updated = await adminAPI.accounts.setPublicSchedulable(props.account.id, next)
    localPublic.value = updated.public_schedulable !== false
    emit('updated', updated)
    appStore.showSuccess(
      next ? t('admin.accounts.accountSchedule.publicOn') : t('admin.accounts.accountSchedule.publicOff')
    )
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.failedToUpdate'))
  } finally {
    busy.value = false
  }
}

async function onPickUser(user: SimpleUser) {
  if (!user?.id) return
  busy.value = true
  try {
    await adminAPI.accounts.addSmartScheduleMember(props.account.id, user.id, activePlatform.value)
    addUserIds.value = []
    appStore.showSuccess(t('admin.accounts.accountSchedule.addSuccess'))
    await loadMembers()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.accountSchedule.addFailed'))
  } finally {
    busy.value = false
  }
}

async function removeMember(userId: number) {
  busy.value = true
  try {
    await adminAPI.accounts.removeSmartScheduleMember(props.account.id, userId, activePlatform.value)
    const remaining = members.value.filter((row) => row.user_id !== userId)
    appStore.showSuccess(
      remaining.length === 0
        ? t('admin.accounts.accountSchedule.lastMemberDisabled')
        : t('admin.accounts.accountSchedule.removeSuccess')
    )
    await loadMembers()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.accountSchedule.removeFailed'))
  } finally {
    busy.value = false
  }
}

async function removeSelected() {
  const ids = [...selectedIds.value]
  if (ids.length === 0) return
  busy.value = true
  try {
    for (const userId of ids) {
      await adminAPI.accounts.removeSmartScheduleMember(props.account.id, userId, activePlatform.value)
    }
    appStore.showSuccess(t('admin.accounts.accountSchedule.removeSuccess'))
    selectedIds.value = []
    await loadMembers()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.accountSchedule.removeFailed'))
  } finally {
    busy.value = false
  }
}

async function applyAdmission(userId: number, state: PairAdmissionLiveState) {
  busy.value = true
  try {
    await adminAPI.accounts.resumeSmartSchedule(props.account.id, userId, state, activePlatform.value)
    await loadMembers()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.failedToUpdate'))
  } finally {
    busy.value = false
  }
}

async function applyBatchAdmission(state: PairAdmissionLiveState) {
  const ids = [...selectedIds.value]
  if (ids.length === 0) return
  busy.value = true
  try {
    await adminAPI.accounts.setSmartScheduleAdmissionBatch(props.account.id, {
      platform: activePlatform.value,
      user_ids: ids,
      state
    })
    await loadMembers()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.failedToUpdate'))
  } finally {
    busy.value = false
  }
}

function toggleSelected(userId: number) {
  selectedIds.value = selectedIds.value.includes(userId)
    ? selectedIds.value.filter((id) => id !== userId)
    : [...selectedIds.value, userId]
}

function toggleSelectAll(event: Event) {
  const checked = (event.target as HTMLInputElement).checked
  selectedIds.value = checked ? members.value.map((row) => row.user_id) : []
}
</script>
