<template>
  <AppLayout>
    <div class="mx-auto max-w-4xl px-4 py-6 sm:px-6">
      <div class="mb-6">
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('admin.modelConfig.title') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.modelConfig.description') }}</p>
      </div>

      <!-- Antigravity Default Model Mapping -->
      <div class="rounded-lg border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-800">
        <div class="mb-4 flex items-center justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.modelConfig.antigravityMapping') }}
            </h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.modelConfig.antigravityMappingHint') }}
            </p>
          </div>
          <div class="flex gap-2">
            <button
              @click="resetToBuiltin"
              class="btn btn-secondary text-sm"
              :disabled="saving"
            >
              {{ t('admin.modelConfig.resetToDefault') }}
            </button>
            <button
              @click="saveMapping"
              class="btn btn-primary text-sm"
              :disabled="saving || !hasChanges"
            >
              <span v-if="saving" class="mr-1.5 inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
              {{ t('common.save') }}
            </button>
          </div>
        </div>

        <!-- Loading -->
        <div v-if="loading" class="flex items-center justify-center py-12">
          <span class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></span>
        </div>

        <!-- Mapping Table -->
        <div v-else>
          <div class="mb-3 flex items-center gap-2">
            <button @click="addRow" class="btn btn-secondary text-xs px-2 py-1">
              + {{ t('admin.modelConfig.addMapping') }}
            </button>
            <span class="text-xs text-gray-400">{{ mappings.length }} {{ t('admin.modelConfig.mappingCount') }}</span>
          </div>

          <div class="space-y-2 max-h-[60vh] overflow-y-auto">
            <div
              v-for="(row, index) in mappings"
              :key="index"
              class="flex items-center gap-2"
            >
              <input
                v-model="row.from"
                type="text"
                class="input flex-1 text-sm"
                :placeholder="t('admin.modelConfig.requestedModel')"
              />
              <span class="text-gray-400 dark:text-gray-500">&rarr;</span>
              <input
                v-model="row.to"
                type="text"
                class="input flex-1 text-sm"
                :placeholder="t('admin.modelConfig.upstreamModel')"
              />
              <button
                @click="removeRow(index)"
                class="flex-shrink-0 rounded p-1 text-gray-400 hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-900/20"
              >
                <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          </div>

          <div v-if="mappings.length === 0" class="py-8 text-center text-sm text-gray-400">
            {{ t('admin.modelConfig.noMappings') }}
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { AppLayout } from '@/components/layout'

const { t } = useI18n()
const appStore = useAppStore()

interface MappingRow {
  from: string
  to: string
}

const loading = ref(true)
const saving = ref(false)
const mappings = ref<MappingRow[]>([])
const originalJson = ref('')

const hasChanges = computed(() => {
  return JSON.stringify(toRecord()) !== originalJson.value
})

function toRecord(): Record<string, string> {
  const result: Record<string, string> = {}
  for (const row of mappings.value) {
    const from = row.from.trim()
    const to = row.to.trim()
    if (from && to) {
      result[from] = to
    }
  }
  return result
}

function fromRecord(record: Record<string, string>): MappingRow[] {
  return Object.entries(record)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([from, to]) => ({ from, to }))
}

async function loadMapping() {
  loading.value = true
  try {
    const data = await adminAPI.accounts.getAntigravityDefaultModelMapping()
    mappings.value = fromRecord(data)
    originalJson.value = JSON.stringify(data)
  } catch (e) {
    appStore.showError(t('common.error'))
  } finally {
    loading.value = false
  }
}

async function saveMapping() {
  saving.value = true
  try {
    const record = toRecord()
    await adminAPI.accounts.updateAntigravityDefaultModelMapping(record)
    originalJson.value = JSON.stringify(record)
    appStore.showSuccess(t('admin.modelConfig.saved'))
  } catch (e) {
    appStore.showError(t('common.error'))
  } finally {
    saving.value = false
  }
}

async function resetToBuiltin() {
  if (!confirm(t('admin.modelConfig.confirmReset'))) return
  saving.value = true
  try {
    // Send empty object to clear custom mapping, backend will fall back to builtin
    await adminAPI.accounts.updateAntigravityDefaultModelMapping({})
    await loadMapping()
    appStore.showSuccess(t('admin.modelConfig.resetSuccess'))
  } catch (e) {
    appStore.showError(t('common.error'))
  } finally {
    saving.value = false
  }
}

function addRow() {
  mappings.value.push({ from: '', to: '' })
}

function removeRow(index: number) {
  mappings.value.splice(index, 1)
}

onMounted(loadMapping)
</script>
