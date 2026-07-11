<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.codexSessionImport.title')"
    width="wide"
    close-on-click-outside
    @close="handleClose"
  >
    <form id="codex-session-import-form" class="space-y-4" @submit.prevent="handleImport">
      <p class="text-sm text-gray-600 dark:text-dark-300">
        {{ t('admin.accounts.codexSessionImport.hint') }}
      </p>

      <div>
        <label class="input-label" for="codex-session-content">
          {{ t('admin.accounts.codexSessionImport.content') }}
        </label>
        <textarea
          id="codex-session-content"
          v-model="form.content"
          rows="8"
          class="input font-mono text-xs"
          :placeholder="t('admin.accounts.codexSessionImport.placeholder')"
        />
      </div>

      <div class="grid gap-4 md:grid-cols-2">
        <div>
          <label class="input-label" for="codex-session-name">{{ t('admin.accounts.codexSessionImport.name') }}</label>
          <input id="codex-session-name" v-model="form.name" name="name" class="input" />
        </div>
        <div>
          <label class="input-label" for="codex-session-notes">{{ t('admin.accounts.codexSessionImport.notes') }}</label>
          <input id="codex-session-notes" v-model="form.notes" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.codexSessionImport.proxy') }}</label>
          <ProxySelector v-model="form.proxy_id" :proxies="proxies" />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="input-label" for="codex-session-concurrency">{{ t('admin.accounts.codexSessionImport.concurrency') }}</label>
            <input id="codex-session-concurrency" v-model.number="form.concurrency" type="number" min="0" class="input" />
          </div>
          <div>
            <label class="input-label" for="codex-session-priority">{{ t('admin.accounts.codexSessionImport.priority') }}</label>
            <input id="codex-session-priority" v-model.number="form.priority" type="number" min="0" class="input" />
          </div>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="input-label" for="codex-session-rate">{{ t('admin.accounts.codexSessionImport.rateMultiplier') }}</label>
            <input id="codex-session-rate" v-model.number="form.rate_multiplier" type="number" min="0" step="0.01" class="input" />
          </div>
          <div>
            <label class="input-label" for="codex-session-load">{{ t('admin.accounts.codexSessionImport.loadFactor') }}</label>
            <input id="codex-session-load" v-model.number="form.load_factor" type="number" max="10000" class="input" />
          </div>
        </div>
      </div>

      <GroupSelector
        v-model="form.group_ids"
        :groups="groups"
        platform="openai"
        show-toggle-all
      />

      <div class="grid gap-2 sm:grid-cols-2">
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
          <input v-model="form.update_existing" type="checkbox" class="rounded border-gray-300 text-primary-500" />
          {{ t('admin.accounts.codexSessionImport.updateExisting') }}
        </label>
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
          <input v-model="form.skip_default_group_bind" type="checkbox" class="rounded border-gray-300 text-primary-500" />
          {{ t('admin.accounts.codexSessionImport.skipDefaultGroup') }}
        </label>
      </div>

      <div v-if="result" class="rounded-md border border-gray-200 p-3 dark:border-dark-600">
        <p class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.accounts.codexSessionImport.summary', result) }}
        </p>
        <ul v-if="failedItems.length" class="mt-2 max-h-36 space-y-1 overflow-y-auto text-xs text-red-600 dark:text-red-400">
          <li v-for="item in failedItems" :key="item.index">
            #{{ item.index }} {{ item.name || '-' }}: {{ item.message }}
          </li>
        </ul>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="importing" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button type="submit" form="codex-session-import-form" class="btn btn-primary" :disabled="importing">
          {{ importing ? t('admin.accounts.codexSessionImport.importing') : t('admin.accounts.codexSessionImport.submit') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ProxySelector from '@/components/common/ProxySelector.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { AdminGroup, CodexSessionImportRequest, CodexSessionImportResult, Proxy } from '@/types'

const props = defineProps<{ show: boolean; proxies: Proxy[]; groups: AdminGroup[] }>()
const emit = defineEmits<{ close: []; imported: [] }>()
const { t } = useI18n()
const appStore = useAppStore()

const defaultForm = (): Required<Pick<CodexSessionImportRequest,
  'content' | 'name' | 'notes' | 'group_ids' | 'proxy_id' | 'concurrency' | 'priority' |
  'rate_multiplier' | 'load_factor' | 'update_existing' | 'skip_default_group_bind'>> => ({
  content: '',
  name: '',
  notes: '',
  group_ids: [],
  proxy_id: null,
  concurrency: 3,
  priority: 50,
  rate_multiplier: 1,
  load_factor: null,
  update_existing: true,
  skip_default_group_bind: false
})

const form = reactive(defaultForm())
const importing = ref(false)
const result = ref<CodexSessionImportResult | null>(null)
const failedItems = computed(() => result.value?.items?.filter((item) => item.action === 'failed') || [])

watch(() => props.show, (show) => {
  if (!show) return
  Object.assign(form, defaultForm())
  result.value = null
})

const handleClose = () => {
  if (!importing.value) emit('close')
}

const handleImport = async () => {
  if (!form.content.trim()) {
    appStore.showError(t('admin.accounts.codexSessionImport.empty'))
    return
  }
  importing.value = true
  try {
    result.value = await adminAPI.accounts.importCodexSession({
      ...form,
      content: form.content.trim(),
      name: form.name.trim(),
      notes: (form.notes ?? '').trim() || null
    })
    const message = t('admin.accounts.codexSessionImport.summary', result.value)
    if (result.value.failed > 0) appStore.showError(message)
    else appStore.showSuccess(message)
    emit('imported')
  } catch (error: any) {
    appStore.showError(error?.response?.data?.message || t('admin.accounts.codexSessionImport.failed'))
  } finally {
    importing.value = false
  }
}
</script>
