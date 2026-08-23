<template>
  <BaseDialog
    :show="show"
    :title="t('admin.ops.scheduleErrorWhitelist.addFromLogTitle')"
    width="normal"
    :z-index="zIndex"
    data-test="schedule-error-whitelist-from-log"
    @close="close"
  >
    <div v-if="log" class="space-y-3 text-sm">
      <p class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.ops.scheduleErrorWhitelist.addFromLogHint') }}
      </p>
      <p
        v-if="hardCounted"
        class="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-900/30 dark:text-amber-200"
        data-test="schedule-error-whitelist-from-log-blocked"
      >
        {{ t('admin.ops.scheduleErrorWhitelist.hardCountedBlocked') }}
      </p>
      <dl class="grid grid-cols-1 gap-2 text-xs sm:grid-cols-2">
        <div>
          <dt class="text-gray-400">error_type</dt>
          <dd class="font-mono text-gray-900 dark:text-white">{{ log.type || '—' }}</dd>
        </div>
        <div>
          <dt class="text-gray-400">phase</dt>
          <dd class="font-mono text-gray-900 dark:text-white">{{ log.phase || '—' }}</dd>
        </div>
        <div>
          <dt class="text-gray-400">status</dt>
          <dd class="font-mono text-gray-900 dark:text-white">{{ log.status_code || '—' }}</dd>
        </div>
        <div>
          <dt class="text-gray-400">provider_error_code</dt>
          <dd class="font-mono text-gray-900 dark:text-white">{{ log.provider_error_code || '—' }}</dd>
        </div>
      </dl>
      <div>
        <div class="text-xs text-gray-400">{{ t('admin.ops.scheduleErrorWhitelist.customMessageLabel') }}</div>
        <p class="mt-1 break-words font-mono text-xs text-gray-900 dark:text-white">{{ messagePreview }}</p>
      </div>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" @click="close">{{ t('common.cancel') }}</button>
      <button
        type="button"
        class="btn btn-secondary"
        data-test="schedule-error-whitelist-from-log-message"
        :disabled="saving || hardCounted || !log"
        @click="submit('message')"
      >
        {{ t('admin.ops.scheduleErrorWhitelist.addMessage') }}
      </button>
      <button
        type="button"
        class="btn btn-primary"
        data-test="schedule-error-whitelist-from-log-structured"
        :disabled="saving || hardCounted || !log"
        @click="submit('structured')"
      >
        {{ t('admin.ops.scheduleErrorWhitelist.addStructured') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import {
  addScheduleErrorWhitelistFromError,
  type ScheduleErrorFromErrorMode
} from '@/api/admin/settings'
import type { OpsErrorLog } from '@/api/admin/ops'
import { formatOpsListPrimary } from '@/views/admin/ops/utils/errorDetailResponse'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores/app'

const props = withDefaults(defineProps<{
  show: boolean
  log: OpsErrorLog | null
  zIndex?: number
}>(), {
  zIndex: 60
})

const emit = defineEmits<{
  close: []
  added: []
}>()

const { t } = useI18n()
const appStore = useAppStore()
const saving = ref(false)

const hardCounted = computed(() => {
  const log = props.log
  if (!log) return false
  return log.status_code === 502 &&
    String(log.message || '').toLowerCase().includes('upstream request failed')
})

const messagePreview = computed(() => {
  if (!props.log) return ''
  return formatOpsListPrimary(props.log) || String(props.log.message || '').trim()
})

function close() {
  emit('close')
}

async function submit(mode: ScheduleErrorFromErrorMode) {
  if (!props.log || hardCounted.value) return
  saving.value = true
  try {
    await addScheduleErrorWhitelistFromError(props.log.id, mode)
    appStore.showSuccess(t('admin.ops.scheduleErrorWhitelist.added'))
    emit('added')
    close()
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t('admin.ops.scheduleErrorWhitelist.addFailed'))
    )
  } finally {
    saving.value = false
  }
}
</script>
