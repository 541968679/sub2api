<template>
  <BaseDialog
    :show="show"
    :title="t('admin.users.smartSchedule.copyFromUserTitle')"
    width="normal"
    :close-on-click-outside="true"
    @close="$emit('close')"
  >
    <div class="space-y-4" data-testid="smart-schedule-copy-from-user-dialog">
      <label class="block space-y-1">
        <span class="text-xs font-medium text-gray-600 dark:text-gray-300">
          {{ t('admin.users.smartSchedule.copyFromUserSearch') }}
        </span>
        <input
          v-model="search"
          type="search"
          class="input w-full"
          :placeholder="t('admin.users.smartSchedule.copyFromUserSearchPlaceholder')"
          data-testid="smart-schedule-copy-from-user-search"
        />
      </label>

      <div
        v-if="searching"
        class="text-xs text-gray-500"
        data-testid="smart-schedule-copy-from-user-searching"
      >
        {{ t('common.loading') }}
      </div>
      <ul
        v-else-if="users.length > 0"
        class="max-h-40 overflow-y-auto rounded-lg border border-gray-200 dark:border-dark-600"
        data-testid="smart-schedule-copy-from-user-results"
      >
        <li v-for="user in users" :key="user.id">
          <button
            type="button"
            class="flex w-full items-center justify-between px-3 py-2 text-left text-sm hover:bg-gray-50 dark:hover:bg-dark-700"
            :class="sourceUserId === user.id ? 'bg-primary-50 dark:bg-primary-900/20' : ''"
            :data-testid="`smart-schedule-copy-from-user-option-${user.id}`"
            @click="selectUser(user.id)"
          >
            <span>{{ user.email || user.username }}</span>
            <span class="text-xs text-gray-400">#{{ user.id }}</span>
          </button>
        </li>
      </ul>
      <p
        v-else-if="search.trim()"
        class="text-xs text-gray-500"
        data-testid="smart-schedule-copy-from-user-empty-search"
      >
        {{ t('admin.users.smartSchedule.copyFromUserEmptySearch') }}
      </p>
      <p v-else class="text-xs text-gray-500" data-testid="smart-schedule-copy-from-user-no-user">
        {{ t('admin.users.smartSchedule.copyFromUserNoUser') }}
      </p>

      <div v-if="previewError" class="text-sm text-red-600" data-testid="smart-schedule-copy-from-user-preview-error">
        {{ previewError }}
      </div>

      <fieldset v-if="preview" class="space-y-2" data-testid="smart-schedule-copy-from-user-slices">
        <label class="flex items-center gap-2 text-sm">
          <input v-model="slices.pool" type="checkbox" data-testid="smart-schedule-copy-from-user-slice-pool" />
          {{ t('admin.users.smartSchedule.copyFromUserSlicePool') }}
        </label>
        <label class="flex items-center gap-2 text-sm" :class="{ 'opacity-50': !slices.pool }">
          <input
            v-model="slices.concurrency"
            type="checkbox"
            :disabled="!slices.pool"
            data-testid="smart-schedule-copy-from-user-slice-concurrency"
          />
          {{ t('admin.users.smartSchedule.copyFromUserSliceConcurrency') }}
        </label>
        <label class="flex items-center gap-2 text-sm" :class="{ 'opacity-50': !slices.pool }">
          <input
            v-model="slices.sort_order"
            type="checkbox"
            :disabled="!slices.pool"
            data-testid="smart-schedule-copy-from-user-slice-sort"
          />
          {{ t('admin.users.smartSchedule.copyFromUserSliceSortOrder') }}
        </label>
        <label class="flex items-center gap-2 text-sm">
          <input v-model="slices.thresholds" type="checkbox" data-testid="smart-schedule-copy-from-user-slice-thresholds" />
          {{ t('admin.users.smartSchedule.copyFromUserSliceThresholds') }}
        </label>
        <label class="flex items-center gap-2 text-sm">
          <input v-model="slices.enabled" type="checkbox" data-testid="smart-schedule-copy-from-user-slice-enabled" />
          {{ t('admin.users.smartSchedule.copyFromUserSliceEnabled') }}
        </label>
      </fieldset>

      <div v-if="preview && slices.pool" class="space-y-1 text-xs text-gray-600 dark:text-gray-300" data-testid="smart-schedule-copy-from-user-diff">
        <p>{{ t('admin.users.smartSchedule.copyFromUserPreviewAdd', { count: preview.add.length }) }}</p>
        <p>{{ t('admin.users.smartSchedule.copyFromUserPreviewRemove', { count: preview.remove.length }) }}</p>
        <p>{{ t('admin.users.smartSchedule.copyFromUserPreviewOverlap', { count: preview.overlap.length }) }}</p>
        <p v-if="preview.skipped_unavailable > 0">
          {{ t('admin.users.smartSchedule.copyFromUserPreviewSkipped', { count: preview.skipped_unavailable }) }}
        </p>
        <p>{{ t('admin.users.smartSchedule.copyFromUserPreviewPaused', { count: preview.source_paused_account_ids.length }) }}</p>
        <p v-if="preview.source_empty" class="text-amber-600" data-testid="smart-schedule-copy-from-user-empty-source">
          {{ t('admin.users.smartSchedule.copyFromUserEmptySource') }}
        </p>
      </div>
      <p v-if="preview && slices.enabled" class="text-xs text-amber-700 dark:text-amber-400" data-testid="smart-schedule-copy-from-user-enabled-delta">
        {{ enabledDeltaLabel }}
      </p>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" data-testid="smart-schedule-copy-from-user-cancel" @click="$emit('close')">
        {{ t('common.cancel') }}
      </button>
      <button
        type="button"
        class="btn btn-primary"
        :disabled="!canConfirm"
        data-testid="smart-schedule-copy-from-user-confirm"
        @click="confirm"
      >
        {{ t('admin.users.smartSchedule.copyFromUserConfirm') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { SmartScheduleCopyFromPreview, SmartScheduleCopySlices, SmartSchedulePlatform } from '@/api/admin/users'
import type { AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{
  show: boolean
  targetUserId: number
  platform: SmartSchedulePlatform
  copying?: boolean
}>()

const emit = defineEmits<{
  close: []
  confirm: [payload: { source_user_id: number; source_revision: string; slices: SmartScheduleCopySlices }]
}>()

const { t } = useI18n()
const search = ref('')
const searching = ref(false)
const users = ref<AdminUser[]>([])
const sourceUserId = ref<number | null>(null)
const preview = ref<SmartScheduleCopyFromPreview | null>(null)
const previewError = ref('')
const slices = reactive<SmartScheduleCopySlices>({
  pool: true,
  concurrency: true,
  sort_order: true,
  thresholds: false,
  enabled: false
})

const canConfirm = computed(() => {
  if (!sourceUserId.value || !preview.value || props.copying) return false
  if (slices.pool && preview.value.source_empty) return false
  return slices.pool || slices.thresholds || slices.enabled
})

const enabledDeltaLabel = computed(() => {
  if (!preview.value) return ''
  if (preview.value.enabled_delta === 'enable') return t('admin.users.smartSchedule.copyFromUserEnabledEnable')
  if (preview.value.enabled_delta === 'disable') return t('admin.users.smartSchedule.copyFromUserEnabledDisable')
  return t('admin.users.smartSchedule.copyFromUserEnabledUnchanged')
})

watch(
  () => slices.pool,
  (pool) => {
    if (!pool) {
      slices.concurrency = false
      slices.sort_order = false
    }
  }
)

let searchTimer: ReturnType<typeof setTimeout> | null = null
watch(search, () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    void loadUsers()
  }, 250)
})

watch(
  () => props.show,
  (open) => {
    if (!open) return
    search.value = ''
    users.value = []
    sourceUserId.value = null
    preview.value = null
    previewError.value = ''
    slices.pool = true
    slices.concurrency = true
    slices.sort_order = true
    slices.thresholds = false
    slices.enabled = false
  }
)

async function loadUsers() {
  const keyword = search.value.trim()
  if (!keyword) {
    users.value = []
    return
  }
  searching.value = true
  try {
    const page = await adminAPI.users.list(1, 10, { search: keyword, status: 'active' })
    users.value = (page.items ?? []).filter((user) => user.id !== props.targetUserId)
  } catch {
    users.value = []
  } finally {
    searching.value = false
  }
}

async function selectUser(id: number) {
  sourceUserId.value = id
  previewError.value = ''
  preview.value = null
  try {
    preview.value = await adminAPI.users.previewSmartScheduleCopyFromUser(props.targetUserId, props.platform, id)
  } catch (error: unknown) {
    previewError.value = extractApiErrorMessage(error, t('admin.users.smartSchedule.copyFromUserFailed'))
  }
}

function confirm() {
  if (!canConfirm.value || !sourceUserId.value || !preview.value) return
  emit('confirm', {
    source_user_id: sourceUserId.value,
    source_revision: preview.value.source_revision,
    slices: { ...slices }
  })
}
</script>
