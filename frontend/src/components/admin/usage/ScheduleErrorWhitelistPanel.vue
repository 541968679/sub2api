<template>
  <div class="flex min-h-0 flex-1 flex-col" data-test="schedule-error-whitelist-card">
    <p class="text-xs text-gray-500 dark:text-gray-400">
      {{ t('admin.ops.scheduleErrorWhitelist.description') }}
    </p>
    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
      {{ t('admin.ops.scheduleErrorWhitelist.checkedHint') }}
    </p>

    <div
      v-if="loading"
      class="mt-4 flex items-center gap-2 text-xs text-gray-500"
    >
      <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
      {{ t('common.loading') }}
    </div>

    <template v-else>
      <div class="mt-3 min-h-0 flex-1 space-y-2 overflow-y-auto">
        <label
          v-for="id in familyIds"
          :key="id"
          class="flex items-start gap-2 rounded-lg border border-gray-100 p-2.5 dark:border-dark-700"
          :data-test="`schedule-error-whitelist-${id}`"
        >
          <input
            v-model="form.families[id]"
            type="checkbox"
            class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          />
          <span>
            <span class="text-xs font-medium text-gray-900 dark:text-white">{{
              t(`admin.ops.scheduleErrorWhitelist.families.${id}.label`)
            }}</span>
            <span class="mt-0.5 block text-[11px] font-normal text-gray-400">
              {{ t(`admin.ops.scheduleErrorWhitelist.families.${id}.hint`) }}
            </span>
          </span>
        </label>
      </div>

      <div class="mt-4 flex justify-end border-t border-gray-100 pt-3 dark:border-dark-700">
        <button
          type="button"
          data-test="schedule-error-whitelist-save"
          class="rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-blue-700 disabled:opacity-50"
          :disabled="saving"
          @click="save"
        >
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  SCHEDULE_ERROR_WHITELIST_FAMILY_IDS,
  defaultScheduleErrorWhitelist,
  getScheduleErrorWhitelist,
  updateScheduleErrorWhitelist,
  type ScheduleErrorWhitelistFamilyId
} from '@/api/admin/settings'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const familyIds = SCHEDULE_ERROR_WHITELIST_FAMILY_IDS
const form = reactive(defaultScheduleErrorWhitelist())

function applySettings(settings: { families?: Record<string, boolean> } | null | undefined) {
  const next = defaultScheduleErrorWhitelist()
  for (const id of SCHEDULE_ERROR_WHITELIST_FAMILY_IDS) {
    if (settings?.families && typeof settings.families[id] === 'boolean') {
      next.families[id] = settings.families[id]
    }
    form.families[id] = next.families[id]
  }
}

async function load() {
  loading.value = true
  try {
    applySettings(await getScheduleErrorWhitelist())
  } catch {
    applySettings(defaultScheduleErrorWhitelist())
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    const families = {} as Record<ScheduleErrorWhitelistFamilyId, boolean>
    for (const id of SCHEDULE_ERROR_WHITELIST_FAMILY_IDS) {
      families[id] = form.families[id] === true
    }
    applySettings(await updateScheduleErrorWhitelist({ families }))
    appStore.showSuccess(t('admin.ops.scheduleErrorWhitelist.saved'))
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t('admin.ops.scheduleErrorWhitelist.saveFailed'))
    )
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void load()
})
</script>
