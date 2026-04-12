<template>
  <AppLayout>
    <div class="px-4 py-6 sm:px-6" :class="activeTab === 'pricing' ? 'mx-auto max-w-full' : 'mx-auto max-w-7xl'">
      <div class="mb-6">
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('admin.modelConfig.title') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.modelConfig.description') }}</p>
      </div>

      <!-- Tab Bar -->
      <div class="mb-6 border-b border-gray-200 dark:border-gray-700">
        <nav class="-mb-px flex gap-6">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            @click="activeTab = tab.key"
            class="whitespace-nowrap border-b-2 px-1 pb-3 text-sm font-medium transition-colors"
            :class="activeTab === tab.key
              ? 'border-primary-500 text-primary-600 dark:text-primary-400'
              : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'"
          >
            {{ tab.label }}
          </button>
        </nav>
      </div>

      <!-- Tab Content: Pricing -->
      <ModelPricingTab v-if="activeTab === 'pricing'" />

      <!-- Tab Content: Mapping + Test (existing) -->
      <div v-if="activeTab === 'mapping'" class="flex gap-6">
        <!-- Left: Mapping Config -->
        <div class="flex-1 min-w-0 rounded-lg border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-800">
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
              <button @click="resetToBuiltin" class="btn btn-secondary text-sm" :disabled="saving">
                {{ t('admin.modelConfig.resetToDefault') }}
              </button>
              <button @click="saveMapping" class="btn btn-primary text-sm" :disabled="saving || !hasChanges">
                <span v-if="saving" class="mr-1.5 inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
                {{ t('common.save') }}
              </button>
            </div>
          </div>

          <div v-if="loading" class="flex items-center justify-center py-12">
            <span class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></span>
          </div>

          <div v-else>
            <div class="mb-3 flex items-center gap-2">
              <button @click="addRow" class="btn btn-secondary text-xs px-2 py-1">
                + {{ t('admin.modelConfig.addMapping') }}
              </button>
              <span class="text-xs text-gray-400">{{ mappings.length }} {{ t('admin.modelConfig.mappingCount') }}</span>
            </div>

            <div class="space-y-2 max-h-[60vh] overflow-y-auto">
              <div v-for="(row, index) in mappings" :key="index" class="flex items-center gap-2">
                <input v-model="row.from" type="text" class="input flex-1 text-sm" :placeholder="t('admin.modelConfig.requestedModel')" />
                <span class="text-gray-400 dark:text-gray-500">&rarr;</span>
                <input v-model="row.to" type="text" class="input flex-1 text-sm" :placeholder="t('admin.modelConfig.upstreamModel')" />
                <button @click="removeRow(index)" class="flex-shrink-0 rounded p-1 text-gray-400 hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-900/20">
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

        <!-- Right: Model Test -->
        <div class="w-96 flex-shrink-0 rounded-lg border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <h2 class="mb-4 text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.modelConfig.testTitle') }}
          </h2>

          <div class="mb-3">
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.modelConfig.testAccount') }}</label>
            <select v-model="testAccountId" class="input text-sm w-full" :disabled="testRunning">
              <option v-if="antigravityAccounts.length === 0" :value="0">{{ t('admin.modelConfig.noAccounts') }}</option>
              <option v-for="acc in antigravityAccounts" :key="acc.id" :value="acc.id">
                {{ acc.credentials?.email || acc.name }} (ID: {{ acc.id }})
              </option>
            </select>
          </div>

          <div class="mb-3">
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.modelConfig.selectModel') }}</label>
            <select v-model="testModelId" class="input text-sm w-full" :disabled="testRunning">
              <option v-for="m in modelOptions" :key="m" :value="m">{{ m }}</option>
            </select>
          </div>

          <div class="mb-3">
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.modelConfig.testPrompt') }}</label>
            <textarea
              v-model="testPrompt"
              rows="2"
              class="input w-full text-sm resize-none"
              :placeholder="t('admin.modelConfig.testPromptPlaceholder')"
              :disabled="testRunning"
            ></textarea>
          </div>

          <button
            @click="runTest"
            class="btn btn-primary w-full text-sm mb-3"
            :disabled="testRunning || !testAccountId || !testModelId"
          >
            <span v-if="testRunning" class="mr-1.5 inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
            {{ testRunning ? t('admin.modelConfig.testing') : t('admin.modelConfig.sendTest') }}
          </button>

          <div
            ref="terminalRef"
            class="rounded-md bg-gray-900 p-3 text-xs font-mono text-green-400 max-h-[40vh] overflow-y-auto whitespace-pre-wrap"
          >
            <div v-if="testOutput.length === 0" class="text-gray-500">{{ t('admin.modelConfig.testHint') }}</div>
            <div v-for="(line, i) in testOutput" :key="i" :class="line.cls">{{ line.text }}</div>
          </div>
        </div>
      </div>

      <!-- Tab Content: Rate Multipliers -->
      <RateMultiplierOverview v-if="activeTab === 'rate'" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { adminAPI } from '@/api/admin'
import { AppLayout } from '@/components/layout'
import type { Account } from '@/types'
import ModelPricingTab from '@/components/admin/model-pricing/ModelPricingTab.vue'
import RateMultiplierOverview from '@/components/admin/model-pricing/RateMultiplierOverview.vue'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const route = useRoute()

// ===================== Tabs =====================
const tabs = computed(() => [
  { key: 'pricing', label: t('admin.modelConfig.tabs.pricing', 'Model Pricing') },
  { key: 'mapping', label: t('admin.modelConfig.tabs.mapping', 'Model Mapping') },
  { key: 'rate', label: t('admin.modelConfig.tabs.rate', 'Rate Multipliers') },
])

const activeTab = ref((route.query.tab as string) || 'pricing')

// ===================== Mapping Config =====================
interface MappingRow { from: string; to: string }

const loading = ref(true)
const saving = ref(false)
const mappings = ref<MappingRow[]>([])
const originalJson = ref('')

const hasChanges = computed(() => JSON.stringify(toRecord()) !== originalJson.value)

function toRecord(): Record<string, string> {
  const result: Record<string, string> = {}
  for (const row of mappings.value) {
    const from = row.from.trim()
    const to = row.to.trim()
    if (from && to) result[from] = to
  }
  return result
}

function fromRecord(record: Record<string, string>): MappingRow[] {
  return Object.entries(record).sort(([a], [b]) => a.localeCompare(b)).map(([from, to]) => ({ from, to }))
}

async function loadMapping() {
  loading.value = true
  try {
    const data = await adminAPI.accounts.getAntigravityDefaultModelMapping()
    mappings.value = fromRecord(data)
    originalJson.value = JSON.stringify(data)
  } catch { appStore.showError(t('common.error')) }
  finally { loading.value = false }
}

async function saveMapping() {
  saving.value = true
  try {
    const record = toRecord()
    await adminAPI.accounts.updateAntigravityDefaultModelMapping(record)
    originalJson.value = JSON.stringify(record)
    appStore.showSuccess(t('admin.modelConfig.saved'))
  } catch { appStore.showError(t('common.error')) }
  finally { saving.value = false }
}

async function resetToBuiltin() {
  if (!confirm(t('admin.modelConfig.confirmReset'))) return
  saving.value = true
  try {
    await adminAPI.accounts.updateAntigravityDefaultModelMapping({})
    await loadMapping()
    appStore.showSuccess(t('admin.modelConfig.resetSuccess'))
  } catch { appStore.showError(t('common.error')) }
  finally { saving.value = false }
}

function addRow() { mappings.value.push({ from: '', to: '' }) }
function removeRow(index: number) { mappings.value.splice(index, 1) }

// ===================== Model Test =====================
const antigravityAccounts = ref<Account[]>([])
const testAccountId = ref<number>(0)
const testModelId = ref('')
const testPrompt = ref('Hello, please respond with one short sentence.')
const testRunning = ref(false)
const testOutput = ref<{ text: string; cls: string }[]>([])
const terminalRef = ref<HTMLElement | null>(null)

const modelOptions = computed(() => {
  const models = mappings.value.map(r => r.from.trim()).filter(Boolean)
  return [...new Set(models)].sort()
})

async function loadAccounts() {
  try {
    const resp = await adminAPI.accounts.list(1, 100, { platform: 'antigravity' })
    const items = resp.items || []
    antigravityAccounts.value = items.filter((a: Account) =>
      a.status === 'active' &&
      a.schedulable &&
      !a.error_message &&
      !a.temp_unschedulable_until &&
      !a.rate_limited_at
    )
    if (antigravityAccounts.value.length > 0 && !testAccountId.value) {
      testAccountId.value = antigravityAccounts.value[0].id
    }
  } catch { /* ignore */ }
}

function appendOutput(text: string, cls = 'text-green-400') {
  testOutput.value.push({ text, cls })
  nextTick(() => {
    if (terminalRef.value) terminalRef.value.scrollTop = terminalRef.value.scrollHeight
  })
}

async function runTest() {
  if (!testAccountId.value || !testModelId.value) return
  testRunning.value = true
  testOutput.value = []

  appendOutput(`> Model: ${testModelId.value}`, 'text-cyan-400')
  appendOutput(`> Account: ${antigravityAccounts.value.find(a => a.id === testAccountId.value)?.credentials?.email || testAccountId.value}`, 'text-cyan-400')
  appendOutput(`> Prompt: ${testPrompt.value}`, 'text-gray-500')
  appendOutput('', 'text-gray-500')

  try {
    const token = authStore.token
    const resp = await fetch(`/api/v1/admin/accounts/${testAccountId.value}/test`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
      },
      body: JSON.stringify({ model_id: testModelId.value, prompt: testPrompt.value })
    })

    if (!resp.ok || !resp.body) {
      appendOutput(`Error: HTTP ${resp.status}`, 'text-red-400')
      return
    }

    const reader = resp.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })

      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        const jsonStr = line.slice(6).trim()
        if (!jsonStr || jsonStr === '[DONE]') continue
        try {
          const event = JSON.parse(jsonStr)
          if (event.type === 'content' && event.text) {
            const last = testOutput.value[testOutput.value.length - 1]
            if (last && last.cls === 'text-green-400' && !last.text.startsWith('>')) {
              last.text += event.text
            } else {
              appendOutput(event.text)
            }
            nextTick(() => {
              if (terminalRef.value) terminalRef.value.scrollTop = terminalRef.value.scrollHeight
            })
          } else if (event.type === 'test_start') {
            appendOutput(`> Testing model: ${event.model || testModelId.value}`, 'text-cyan-400')
          } else if (event.type === 'test_complete') {
            appendOutput('')
            if (event.success) {
              appendOutput('> Test passed', 'text-emerald-400')
            } else {
              appendOutput(`> Test failed: ${event.error || 'unknown'}`, 'text-red-400')
            }
          } else if (event.type === 'error') {
            appendOutput(`> Error: ${event.error || event.text || 'unknown'}`, 'text-red-400')
          }
        } catch { /* skip malformed JSON */ }
      }
    }
  } catch (e: any) {
    appendOutput(`> Error: ${e.message || 'Connection failed'}`, 'text-red-400')
  } finally {
    testRunning.value = false
  }
}

// ===================== Init =====================
onMounted(async () => {
  await loadMapping()
  await loadAccounts()
  if (modelOptions.value.length > 0) {
    testModelId.value = modelOptions.value[0]
  }
})
</script>
